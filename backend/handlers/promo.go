// ─── Обработчики ────────────────────────────────────────────────────────────

package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"golang.org/x/sync/errgroup"
)

// ─── Read ──────────────────────────────────────────────────────────────────

// respondIfDedupInProgress отвечает 503, если запись промо временно закрыта
// офлайн-дедупликацией (sync_script/dedupe_promo.py). Возвращает true, когда
// ответ уже отправлен и обработчику больше делать нечего.
func respondIfDedupInProgress(c *gin.Context, err error) bool {
	if !errors.Is(err, repository.ErrPromoDedupInProgress) {
		return false
	}
	config.Logger.Warn("promo_write_rejected_dedup_running", "path", c.FullPath())
	c.Header("Retry-After", "30")
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "Идёт дедупликация промо, запись временно недоступна. Повторите через полминуты.",
	})
	return true
}

// respondKAMNotLinked отвечает учётной записи с ролью kam, у которой нет
// закрепления. Пустая таблица без объяснения читалась бы как «промо нет»,
// поэтому причина называется прямо: чинить её администратору, а не КАМу.
func respondKAMNotLinked(c *gin.Context, username string) {
	config.Logger.Warn("kam_account_without_scope", "user", username, "path", c.FullPath())
	c.JSON(http.StatusForbidden, gin.H{
		"error": "Учётная запись не привязана к КАМу — обратитесь к администратору портала",
	})
}

// promoVisibilityScope возвращает область видимости промо для текущего
// пользователя и сам отвечает клиенту при ошибке. Пустой срез — ограничения нет.
func promoVisibilityScope(c *gin.Context) ([]string, bool) {
	username := fmt.Sprint(mustGet(c, "username"))
	role := fmt.Sprint(mustGet(c, "role"))
	scope, err := repository.GetPromoVisibilityScope(username, role)
	if errors.Is(err, repository.ErrKAMNotLinked) {
		respondKAMNotLinked(c, username)
		return nil, false
	}
	if err != nil {
		config.Logger.Error("promo_scope_failed", "error", err.Error(), "user", username)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось определить область видимости"})
		return nil, false
	}
	return scope, true
}

// promoWriteAllowed проверяет КАМа промо против области пользователя и сам
// отвечает клиенту при отказе. Область на записи та же, что и на чтении:
// править и заводить промо можно только там, где они видны.
func promoWriteAllowed(c *gin.Context, scope []string, kam string) bool {
	if repository.KAMAllowedByScope(scope, kam) {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "промо вне области ведения"})
	return false
}

func GetPromoFilters(c *gin.Context) {
	scope, ok := promoVisibilityScope(c)
	if !ok {
		return
	}
	params := repository.PromoFilterParams{
		AllowedKAMs: scope,
		YearFromStr: c.Query("yearFrom"),
		YearToStr:   c.Query("yearTo"),
		Months:      c.QueryArray("months"),
		Kams:        c.QueryArray("kam"),
		Brands:      c.QueryArray("brand"),
		SKUs:        c.QueryArray("sku"),
		Networks:    c.QueryArray("network_name"),
		Mechanics:   c.QueryArray("mechanics"),
		Statuses:    c.QueryArray("status"),
	}
	// Канал приходит из того же запроса, что и прочие фильтры: он не колонка
	// промо, а свойство механики, поэтому едет отдельным параметром.
	channels := c.QueryArray("channel")

	// Кэшируется любой набор фильтров, а не только дефолтная страница: раньше
	// при первом же выборе КАМа или сети семь запросов уходили в базу заново
	// на каждое открытие панели.
	//
	// Область входит в ключ: без неё срез одного КАМа достался бы другому.
	cacheKey := "filters:" + strings.Join(scope, "|") + ":" +
		params.YearFromStr + ":" + params.YearToStr + ":" + strings.Join(params.Months, ",") +
		":" + strings.Join(params.Kams, ",") + ":" + strings.Join(params.Brands, ",") +
		":" + strings.Join(params.SKUs, ",") + ":" + strings.Join(params.Networks, ",") +
		":" + strings.Join(params.Mechanics, ",") + ":" + strings.Join(params.Statuses, ",") +
		":" + strings.Join(channels, ",")
	if cached, ok := config.FiltersCache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	baseWhere, baseArgs := repository.BuildBaseWhere(params)

	mainFilters := map[string][]string{
		"kam": params.Kams, "brand_as": params.Brands, "sku": params.SKUs,
		"network_name": params.Networks, "mechanics": params.Mechanics, "status": params.Statuses,
	}

	var (
		resKam, resBrand, resSKU, resNetwork, resMechanics, resStatus, resChannel []string
	)

	g, _ := errgroup.WithContext(context.Background())
	// Семь справочников считаются параллельно, но не все сразу: пул на 25
	// соединений один на всех, и веер в семь запросов означал, что четвёртая
	// одновременно открытая панель упирается в его дно. Три — компромисс:
	// панель собирается заметно быстрее последовательной, а один пользователь
	// занимает восьмую часть пула вместо трети.
	g.SetLimit(repository.FilterQueryConcurrency)

	g.Go(func() error {
		resKam = repository.GetFilterValues("kam", baseWhere, baseArgs, "kam", mainFilters, channels)
		return nil
	})
	g.Go(func() error {
		resBrand = repository.GetFilterValues("brand_as", baseWhere, baseArgs, "brand_as", mainFilters, channels)
		return nil
	})
	g.Go(func() error {
		resSKU = repository.GetFilterValues("sku", baseWhere, baseArgs, "sku", mainFilters, channels)
		return nil
	})
	g.Go(func() error {
		resNetwork = repository.GetFilterValues("network_name", baseWhere, baseArgs, "network_name", mainFilters, channels)
		return nil
	})
	g.Go(func() error {
		resMechanics = repository.GetFilterValues("mechanics", baseWhere, baseArgs, "mechanics", mainFilters, channels)
		return nil
	})
	g.Go(func() error {
		resStatus = repository.GetFilterValues("status", baseWhere, baseArgs, "status", mainFilters, channels)
		return nil
	})
	g.Go(func() error {
		resChannel = repository.GetChannelFilterValues(baseWhere, baseArgs, mainFilters)
		return nil
	})

	if err := g.Wait(); err != nil {
		config.Logger.Error("filter_values_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки фильтров"})
		return
	}

	result := gin.H{
		"kam":          resKam,
		"brand":        resBrand,
		"sku":          resSKU,
		"network_name": resNetwork,
		"mechanics":    resMechanics,
		"status":       resStatus,
		"channel":      resChannel,
	}

	config.FiltersCache.Set(cacheKey, result, config.FilterCacheTTL)

	c.JSON(http.StatusOK, result)
}

// PreviewPromoCalculations пересчитывает черновик карточки промо.
//
// Единственный источник формул — services: до этого ROI и uplift считались
// дважды — здесь и построчно в браузере (frontend/src/utils/calcUtils.ts), —
// и синхронизировать две копии было некому. Тот же приём уже применён к
// реестру сетей: PreviewNetworkPlan считает черновик на сервере, а TypeScript
// отвечает только за форматирование.
//
// В базу ничего не пишется: это расчёт по присланным числам.
func PreviewPromoCalculations(c *gin.Context) {
	var input services.PromoInputDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}
	calcCtx := services.EnrichFromRepo(&input)
	c.JSON(http.StatusOK, services.CalculateFields(&input, calcCtx))
}

func GetPromoData(c *gin.Context) {
	deletedFilter := c.DefaultQuery("deletedFilter", "")
	if deletedFilter != "" {
		role, _ := c.Get("role")
		if fmt.Sprint(role) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "просмотр удалённых записей доступен только администратору"})
			return
		}
	}

	scope, ok := promoVisibilityScope(c)
	if !ok {
		return
	}
	params := repository.PromoFilterParams{
		AllowedKAMs:   scope,
		YearFromStr:   c.Query("yearFrom"),
		YearToStr:     c.Query("yearTo"),
		Months:        c.QueryArray("months"),
		Kams:          c.QueryArray("kam"),
		Brands:        c.QueryArray("brand"),
		SKUs:          c.QueryArray("sku"),
		Networks:      c.QueryArray("network_name"),
		Mechanics:     c.QueryArray("mechanics"),
		Statuses:      c.QueryArray("status"),
		DeletedFilter: deletedFilter,
		Search:        c.Query("search"),
		SortField:     c.Query("sortField"),
		SortDirection: c.Query("sortDirection"),
	}
	channels := c.QueryArray("channel")

	all := c.Query("all") == "true"
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("limit", "100")))

	// Выборку целиком запрашивает выгрузка. Она уходит в память браузера, и
	// потолок здесь единственный: без него растущая база однажды не «замедлит»
	// вкладку, а уронит её.
	if all {
		limit := promoRowsMaxRows()
		totalRows, err := repository.PromoRowsCount(params, channels)
		if err != nil {
			config.Logger.Error("promo_rows_count_failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
			return
		}
		if totalRows > limit {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf(
					"Выборка слишком большая: %d строк при лимите %d. Уточните фильтры или используйте выгрузку в Excel.",
					totalRows, limit,
				),
				"total": totalRows,
				"limit": limit,
				"data":  []interface{}{},
			})
			return
		}
	}

	results, err := repository.GetPromoRows(params, channels, page, pageSize, all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	if all {
		c.JSON(http.StatusOK, models.PromoDataResponse{Data: results})
		return
	}

	totalRows, err := repository.PromoRowsCount(params, channels)
	if err != nil {
		config.Logger.Error("promo_rows_count_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, models.PromoDataResponse{Data: results, TotalRows: &totalRows})
}

// defaultPromoMaxRows — потолок выборки промо целиком (all=true). В отличие от
// выгрузки продаж эти строки живут в памяти вкладки, поэтому потолок ниже.
const defaultPromoMaxRows = 50000

func promoRowsMaxRows() int {
	if raw := strings.TrimSpace(os.Getenv("PROMO_MAX_ROWS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultPromoMaxRows
}

// GetPromoDashboard возвращает агрегированную витрину промо. Сырые строки
// используются только внутри backend; ROI и проценты план-факт пересчитываются
// на сопоставимом срезе, а не усредняются из готовых процентов строк.
func GetPromoDashboard(c *gin.Context) {
	deletedFilter := c.DefaultQuery("deletedFilter", "")
	if deletedFilter != "" {
		role, _ := c.Get("role")
		if fmt.Sprint(role) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "просмотр удалённых записей доступен только администратору"})
			return
		}
	}

	scope, ok := promoVisibilityScope(c)
	if !ok {
		return
	}
	dashboard, err := services.BuildPromoDashboard(repository.PromoFilterParams{
		AllowedKAMs:   scope,
		YearFromStr:   c.Query("yearFrom"),
		YearToStr:     c.Query("yearTo"),
		Months:        c.QueryArray("months"),
		Kams:          c.QueryArray("kam"),
		Brands:        c.QueryArray("brand"),
		SKUs:          c.QueryArray("sku"),
		Networks:      c.QueryArray("network_name"),
		Mechanics:     c.QueryArray("mechanics"),
		Statuses:      c.QueryArray("status"),
		DeletedFilter: deletedFilter,
	}, c.QueryArray("channel"))
	if err != nil {
		config.Logger.Error("promo_dashboard_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить промо-дашборд"})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func GetPromoByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID промо"})
		return
	}

	scope, ok := promoVisibilityScope(c)
	if !ok {
		return
	}

	row, err := repository.GetPromoByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Промо не найдено"})
		return
	}
	// Прямое обращение по id обходит фильтры списка, поэтому область
	// проверяется и здесь: иначе чужое промо открывалось бы по ссылке.
	if row.KAM != nil && !repository.KAMAllowedByScope(scope, *row.KAM) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Промо не найдено"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить промо"})
		return
	}
	c.JSON(http.StatusOK, row)
}

func GetSKUByBrand(c *gin.Context) {
	brand := c.Query("brand")
	if brand == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	skus, _ := repository.GetSKUsByBrand(brand)
	if skus == nil {
		skus = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": skus})
}

func GetLastContractPrice(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{"price": nil})
		return
	}
	price, _ := repository.GetLastContractPrice(sku)
	c.JSON(http.StatusOK, gin.H{"price": price})
}

func GetInvestmentTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []string{"GTN", "GTN в ОС", "OPEX", "OPEX Marketing"}})
}

func GetKAMByNetwork(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	kams, _ := repository.GetKAMsByNetwork(network)
	if kams == nil {
		kams = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": kams})
}

func GetLastNetworkData(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	total, _ := repository.GetLastNetworkData(network)
	if total != nil {
		c.JSON(http.StatusOK, gin.H{"total_pharmacies": *total})
	} else {
		c.JSON(http.StatusOK, gin.H{"total_pharmacies": 0})
	}
}

func GetNetworkGeoMapping(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "network is required"})
		return
	}
	geo, err := repository.GetNetworkGeoMapping(network)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"kam": nil, "network_type": nil, "top20_segment": nil, "key_region": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"kam":           geo.KAM,
		"network_type":  geo.NetworkType,
		"top20_segment": geo.Top20Segment,
		"key_region":    geo.KeyRegion,
	})
}

func GetPromoCommentsHandler(c *gin.Context) {
	id := c.Param("id")
	promoID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	dbComments, _ := repository.GetPromoComments(promoID)
	legacyComments := repository.FetchPromoCommentsFallback(promoID)

	if len(dbComments) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": legacyComments})
		return
	}

	// Склеиваем историю: legacyComments содержит ВСЮ историю из текстового поля,
	// dbComments содержит только новые записи из таблицы.
	diff := len(legacyComments) - len(dbComments)
	if diff > 0 {
		combined := append(legacyComments[:diff], dbComments...)
		c.JSON(http.StatusOK, gin.H{"data": combined})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dbComments})
}

func GetPromoHistoryFiltered(c *gin.Context) {
	sku := c.Query("sku")
	network := c.Query("network_name")
	mechanics := c.Query("mechanics")
	yearFrom := c.Query("yearFrom")
	yearTo := c.Query("yearTo")

	results, err := repository.GetPromoHistory(sku, network, mechanics, yearFrom, yearTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func GetSKUInfo(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{"brand": nil, "brand_as": nil})
		return
	}
	brand, brandAs, found := repository.GetSKUInfo(sku)
	if !found {
		c.JSON(http.StatusOK, gin.H{"brand": nil, "brand_as": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"brand": brand, "brand_as": brandAs})
}

func GetLastSKUData(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	data, err := repository.GetLastSKUData(sku)
	if err != nil && err.Error() != "sql: no rows in result set" {
		config.Logger.Error("get_last_sku_data_failed", "error", err.Error(), "sku", sku)
	}
	if data == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"contract_price":   data.ContractPrice,
		"gm":               data.GM,
		"total_pharmacies": data.TotalPharmacies,
		"key_region":       data.KeyRegion,
		"top20_segment":    data.Top20Segment,
		"olap_price":       data.OlapPrice,
	})
}

// ─── Save / Delete ─────────────────────────────────────────────────────────

// applyJSONToRow — применяет входящий map[string]interface{} поверх существующей строки БД.
// Поля id, deleted_at, updated_at, agreement*_status/comment, comments — пропускаются.
func applyJSONToRow(r *models.PromoRowDB, input map[string]interface{}) {
	for k, v := range input {
		if k == "id" || k == "deleted_at" || k == "updated_at" ||
			k == "agreement1_status" || k == "agreement1_comment" ||
			k == "agreement2_status" || k == "agreement2_comment" ||
			k == "comments" {
			continue
		}

		if v == nil {
			continue
		}
		strVal := fmt.Sprint(v)
		if strVal == "<nil>" {
			continue
		}

		switch k {
		case "network_name":
			r.NetworkName = fmt.Sprint(v)
		case "kam":
			r.KAM = fmt.Sprint(v)
		case "brand":
			r.Brand = fmt.Sprint(v)
		case "brand_as":
			r.BrandAS = fmt.Sprint(v)
		case "sku":
			r.SKU = fmt.Sprint(v)
		case "year":
			r.Year, _ = strconv.Atoi(fmt.Sprint(v))
		case "month":
			r.Month, _ = strconv.Atoi(fmt.Sprint(v))
		case "quarter":
			if val, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
				r.Quarter = &val
			}
		case "mechanics":
			r.Mechanics = fmt.Sprint(v)
		case "gtn_opex":
			r.GTNOpex = fmt.Sprint(v)
		case "baseline_units":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.BaselineUnits = &val
			}
		case "baseline_rub":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.BaselineRub = &val
			}
		case "plan_promo_units":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.PlanPromoUnits = &val
			}
		case "plan_promo_rub":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.PlanPromoRub = &val
			}
		case "plan_investments_rub":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.PlanInvestmentsRub = &val
			}
		case "contract_price":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ContractPrice = &val
			}
		case "gm":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.GM = &val
			}
		case "id_directum":
			r.IDDirectum = fmt.Sprint(v)
		case "ds_number":
			r.DSNumber = fmt.Sprint(v)
		case "discount_amount":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.DiscountAmount = &val
			}
		case "conditions":
			r.Conditions = fmt.Sprint(v)
		case "ecom_segment":
			r.EcomSegment = fmt.Sprint(v)
		case "status":
			r.Status = fmt.Sprint(v)
		case "total_pharmacies":
			if val, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
				r.TotalPharmacies = &val
			}
		case "promo_pharmacies":
			if val, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
				r.PromoPharmacies = &val
			}
		case "date":
			r.Date = fmt.Sprint(v)
		case "key_region":
			r.KeyRegion = fmt.Sprint(v)
		case "top20_segment":
			r.Top20Segment = fmt.Sprint(v)
		case "olap_price":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.OlapPrice = &val
			}
		case "actual_promo_sales_units":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ActualPromoSalesUnits = &val
			}
		case "actual_investments":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ActualInvestments = &val
			}
		case "actual_promo_rub":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ActualPromoRub = &val
			}
		case "actual_promo_uplift_units":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ActualPromoUpliftUnits = &val
			}
		case "actual_promo_uplift_rub":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ActualPromoUpliftRub = &val
			}
		case "actual_external_ecom_units":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ActualExternalEcomUnits = &val
			}
		case "actual_corrected_baseline":
			if val, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				r.ActualCorrectedBaseline = &val
			}
		}
	}
}

// respondExistingPromo отдаёт запись, созданную прошлой попыткой с тем же
// ключом идемпотентности. Ответ повторяет обычный ответ на создание: для
// клиента повтор ничем не отличается от первого удачного сохранения.
func respondExistingPromo(c *gin.Context, promoID int) {
	row, err := repository.FetchExistingRow(promoID)
	if err != nil {
		config.Logger.Error("promo_idempotency_refetch_failed", "error", err.Error(), "id", promoID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось прочитать созданное промо"})
		return
	}
	config.Logger.Info("promo_create_repeated", "id", promoID)
	c.JSON(http.StatusOK, gin.H{"message": "Created", "id": promoID, "data": services.DBRowToMap(row)})
}

func SavePromo(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scope, ok := promoVisibilityScope(c)
	if !ok {
		return
	}

	// UPDATE
	if id, ok := input["id"]; ok && id != nil {
		idFloat, _ := strconv.ParseFloat(fmt.Sprint(id), 64)
		idInt := int(idFloat)
		if idInt > 0 {
			row, err := repository.FetchExistingRow(idInt)
			if err != nil {
				if strings.Contains(err.Error(), "no rows") {
					// Проверяем, существует ли запись вообще (без deleted_at)
					var exists int
					if err2 := config.DB.QueryRow("SELECT COUNT(*) FROM dbo.tbl_PromoActivities WHERE id = ?", idInt).Scan(&exists); err2 == nil && exists > 0 {
						c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Запись удалена (soft-delete). ID=%d", idInt)})
					} else {
						// Ни имени базы, ни учётной записи сервера в ответе:
						// клиенту хватает того, что записи нет, а строка
						// подключения — подсказка тому, кто ищет вход.
						c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Запись ID=%d не найдена", idInt)})
					}
				} else {
					// Текст ошибки БД остаётся в логе: в ответе он раскрывает
					// схему и устройство запроса, а пользователю не помогает.
					config.Logger.Error("promo_update_fetch_failed", "error", err.Error(), "id", idInt)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось прочитать запись"})
				}
				return
			}

			// КАМ берётся из базы, а не из тела запроса: иначе чужое промо
			// правилось бы подменой поля.
			if !promoWriteAllowed(c, scope, row.KAM) {
				return
			}

			// Берём updated_at из запроса клиента для Optimistic Locking
			clientUpdatedAt := fmt.Sprint(input["updated_at"])

			// Сохраняем копию старой строки для аудит-лога
			oldRow := *row

			// Применяем входящие данные поверх существующей строки
			applyJSONToRow(row, input)

			// Перенос промо за пределы своей области закрыт так же, как правка
			// чужого: иначе запись уходила бы из видимости одним сохранением.
			if !promoWriteAllowed(c, scope, row.KAM) {
				return
			}

			// Сохраняем комментарий КАМ с датой и ролью, не затирая историю согласования
			if newComment, ok := input["comments"]; ok {
				var newCommentStr string
				if newComment != nil {
					newCommentStr = strings.TrimSpace(fmt.Sprint(newComment))
				}
				if newCommentStr != "" && newCommentStr != "<nil>" {
					// Сохраняем ВСЮ историю (как структурированные строки вида
					// [DD.MM.YYYY роль|автор]: текст, так и любые прочие строки),
					// чтобы ничего не терялось при последовательных сохранениях.
					var historyLines []string
					for _, line := range strings.Split(row.Comments, "\n") {
						line = strings.TrimSpace(line)
						if line != "" {
							historyLines = append(historyLines, line)
						}
					}
					// Добавляем новый комментарий КАМ с меткой
					usernameVal, _ := c.Get("username")
					timestamp := time.Now().Format("02.01.2006")
					kamLine := fmt.Sprintf("[%s КАМ|%s]: %s", timestamp, fmt.Sprint(usernameVal), newCommentStr)
					historyLines = append(historyLines, kamLine)
					row.Comments = strings.Join(historyLines, "\n")
					// Дублируем в новую таблицу комментариев
					_ = repository.InsertComment(idInt, fmt.Sprint(usernameVal), "КАМ", newCommentStr)
				}
			}

			// Пересчитываем вычисляемые поля
			recalcDTO := services.DBRowToDTO(row)
			calcCtx := services.EnrichFromRepo(&recalcDTO)
			calc := services.CalculateFields(&recalcDTO, calcCtx)
			services.MergeCalculatedIntoDBRow(row, calc)

			rowsAffected, err := repository.UpdatePromo(idInt, row, clientUpdatedAt)
			if respondIfDedupInProgress(c, err) {
				return
			}
			if err != nil {
				config.Logger.Error("promo_update_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить промо"})
				return
			}
			if rowsAffected == 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "Данные были изменены другим пользователем. Обновите страницу и попробуйте снова."})
				return
			}

			// Перечитываем строку из БД, чтобы получить точный updated_at из GETDATE()
			refetched, fetchErr := repository.FetchExistingRow(idInt)
			if fetchErr != nil {
				config.Logger.Error("promo_update_refetch_failed", "error", fetchErr.Error(), "id", idInt)
				// Возвращаем как есть, без обновлённого updated_at
				c.JSON(http.StatusOK, gin.H{"message": "Updated", "id": idInt, "data": services.DBRowToMap(row)})
				return
			}

			usernameVal, _ := c.Get("username")
			// Запись в аудит-лог: сравниваем старую версию до изменений с новой
			if diffJSON := repository.DiffPromoRows(&oldRow, row); diffJSON != "" {
				_ = repository.InsertAuditLog(idInt, fmt.Sprint(usernameVal), "UPDATE", diffJSON)
			}
			config.Logger.Info("promo_updated",
				"id", idInt,
				"sku", row.SKU,
				"network", row.NetworkName,
				"user", fmt.Sprint(usernameVal),
				"timestamp", time.Now().Format(time.RFC3339),
			)
			c.JSON(http.StatusOK, gin.H{"message": "Updated", "id": idInt, "data": services.DBRowToMap(refetched)})
			return
		}
	}

	// INSERT
	usernameForKey := fmt.Sprint(mustGet(c, "username"))
	// Ключ идемпотентности: ответ на «Сохранить» мог не дойти, и повтор с тем
	// же ключом обязан вернуть прежний результат, а не создать дубль. Нужен
	// только вставке — обновление от повторов защищает optimistic locking.
	idempotencyKey, hasIdempotencyKey := "", false
	if raw, ok := input["idempotency_key"]; ok && raw != nil {
		idempotencyKey, hasIdempotencyKey = repository.NormalizePromoIdempotencyKey(fmt.Sprint(raw))
	}
	if hasIdempotencyKey {
		if existingID, found, err := repository.FindPromoByIdempotencyKey(idempotencyKey, usernameForKey); err != nil {
			config.Logger.Error("promo_idempotency_lookup_failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать промо"})
			return
		} else if found {
			respondExistingPromo(c, existingID)
			return
		}
	}

	dto := services.MapToDTO(input)
	calcCtx := services.EnrichFromRepo(&dto)
	calc := services.CalculateFields(&dto, calcCtx)
	row := services.DTOToDBRow(dto, calc)

	if !promoWriteAllowed(c, scope, row.KAM) {
		return
	}

	// Первый комментарий КАМ при создании промо тоже оформляем как строку истории
	// вида [DD.MM.YYYY КАМ|автор]: текст, чтобы он корректно накапливался дальше.
	insertComments := strings.TrimSpace(fmt.Sprint(row.Comments))
	if insertComments != "" && insertComments != "<nil>" {
		usernameVal, _ := c.Get("username")
		ts := time.Now().Format("02.01.2006")
		row.Comments = fmt.Sprintf("[%s КАМ|%s]: %s", ts, fmt.Sprint(usernameVal), insertComments)
	}

	newID, err := repository.InsertPromoWithKey(row, idempotencyKey, usernameForKey)
	if respondIfDedupInProgress(c, err) {
		return
	}
	// Ключ занял одновременный повтор: запись уже создана им, своя вставка
	// откатилась. Отдаём созданное — ровно то, чего ждал клиент.
	if errors.Is(err, repository.ErrPromoIdempotencyKeyTaken) {
		existingID, found, findErr := repository.FindPromoByIdempotencyKey(idempotencyKey, usernameForKey)
		if findErr == nil && found {
			respondExistingPromo(c, existingID)
			return
		}
		config.Logger.Error("promo_idempotency_race_unresolved", "key", idempotencyKey)
		c.JSON(http.StatusConflict, gin.H{"error": "Сохранение уже выполняется. Обновите страницу."})
		return
	}
	if err == nil && insertComments != "" && insertComments != "<nil>" {
		usernameVal, _ := c.Get("username")
		_ = repository.InsertComment(int(newID), fmt.Sprint(usernameVal), "КАМ", insertComments)
	}
	if err != nil {
		config.Logger.Error("promo_insert_failed",
			"error", err.Error(),
			"sku", dto.SKU,
			"network", dto.NetworkName,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать промо"})
		return
	}

	row.ID = int(newID)

	usernameVal, _ := c.Get("username")
	config.Logger.Info("promo_created",
		"id", newID,
		"sku", dto.SKU,
		"network", dto.NetworkName,
		"year", dto.Year,
		"month", dto.Month,
		"plan_units", dto.PlanPromoUnits,
		"plan_rub", dto.PlanPromoRub,
		"user", fmt.Sprint(usernameVal),
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Created", "id": newID, "data": services.DBRowToMap(row)})
}

func DeletePromo(c *gin.Context) {
	id := c.Param("id")

	if _, err := strconv.Atoi(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	idInt, _ := strconv.Atoi(id)
	rows, err := repository.SoftDeletePromo(idInt)
	if respondIfDedupInProgress(c, err) {
		return
	}
	if err != nil {
		config.Logger.Error("promo_delete_failed", "id", id, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена или уже удалена"})
		return
	}

	usernameVal, _ := c.Get("username")
	// Запись в аудит-лог
	_ = repository.InsertAuditLog(idInt, fmt.Sprint(usernameVal), "DELETE", "")
	config.Logger.Info("promo_deleted", "id", id, "user", fmt.Sprint(usernameVal), "timestamp", time.Now().Format(time.RFC3339))
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func RestorePromo(c *gin.Context) {
	id := c.Param("id")

	if _, err := strconv.Atoi(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	idInt, _ := strconv.Atoi(id)
	rows, err := repository.RestorePromo(idInt)
	if respondIfDedupInProgress(c, err) {
		return
	}
	if err != nil {
		config.Logger.Error("promo_restore_failed", "id", id, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Restore failed"})
		return
	}
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена или не была удалена"})
		return
	}

	usernameVal, _ := c.Get("username")
	_ = repository.InsertAuditLog(idInt, fmt.Sprint(usernameVal), "RESTORE", "")
	config.Logger.Info("promo_restored", "id", id, "user", fmt.Sprint(usernameVal), "timestamp", time.Now().Format(time.RFC3339))
	c.JSON(http.StatusOK, gin.H{"message": "Restored"})
}

// ─── Approvals ─────────────────────────────────────────────────────────────

func agreementNumberForRole(role, requestedRole string) (int, bool) {
	switch role {
	case "agreement1":
		return 1, true
	case "agreement2":
		return 2, true
	case "admin":
		// Администратор может работать на любом этапе, но обязан явно
		// передать выбранный этап. Это исключает неявное изменение agreement1.
		switch requestedRole {
		case "agreement1":
			return 1, true
		case "agreement2":
			return 2, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// approvalAccess — что пользователю разрешено на странице согласования.
// Scope пуст, если ограничения нет: так работают согласующие, заведённые до
// появления области согласования.
type approvalAccess struct {
	AgreementNum int
	Scope        []string
}

// resolveApprovalAccess определяет ступень и область согласования.
//
// Ступень роли agreement1/agreement2 следует из самой роли, администратор
// обязан назвать её явно. У роли kam ступени в роли нет — она берётся из
// области: КАМ допускается к согласованию только там, где за ним закреплены
// чужие промо. Свои промо в область не входят и потому ему недоступны.
func resolveApprovalAccess(c *gin.Context, requestedRole string) (approvalAccess, bool) {
	role := fmt.Sprint(mustGet(c, "role"))
	username := fmt.Sprint(mustGet(c, "username"))

	if role == "kam" {
		stages, err := repository.GetApprovalScopeStages(username)
		if err != nil || len(stages) == 0 {
			return approvalAccess{}, false
		}
		// Ступень определяется закреплением, а не тем, что прислал клиент:
		// у роли kam ступени в роли нет, и интерфейс по умолчанию отправляет
		// первую. Запрошенная ступень принимается, только если закрепление на
		// ней есть; иначе берётся единственная имеющаяся.
		stage := stages[0]
		if requested, ok := approvalStageFromRole(requestedRole); ok && containsInt(stages, requested) {
			stage = requested
		} else if len(stages) > 1 {
			return approvalAccess{}, false
		}
		scope, err := repository.GetApprovalScope(username, stage)
		if err != nil || len(scope) == 0 {
			return approvalAccess{}, false
		}
		return approvalAccess{AgreementNum: stage, Scope: scope}, true
	}

	agreementNum, ok := agreementNumberForRole(role, requestedRole)
	if !ok {
		return approvalAccess{}, false
	}
	scope, err := repository.GetApprovalScope(username, agreementNum)
	if err != nil {
		return approvalAccess{}, false
	}
	return approvalAccess{AgreementNum: agreementNum, Scope: scope}, true
}

// allowsKAM проверяет конкретного КАМа против области.
func (a approvalAccess) allowsKAM(kam string) bool {
	return repository.KAMAllowedByScope(a.Scope, kam)
}

func approvalStageFromRole(role string) (int, bool) {
	switch role {
	case "agreement1":
		return 1, true
	case "agreement2":
		return 2, true
	}
	return 0, false
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mustGet(c *gin.Context, key string) any {
	value, _ := c.Get(key)
	return value
}

// approvalIDsInScope проверяет промо перед записью и сам отвечает клиенту при
// отказе. Возвращает true, только если согласование разрешено по всем строкам.
func approvalIDsInScope(c *gin.Context, access approvalAccess, ids []int) bool {
	if len(access.Scope) == 0 {
		return true
	}
	kams, err := repository.GetPromoKAMs(ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось проверить область согласования"})
		return false
	}
	for _, kam := range kams {
		if !access.allowsKAM(kam) {
			c.JSON(http.StatusForbidden, gin.H{"error": "промо вне области согласования"})
			return false
		}
	}
	return true
}

// GetApprovalAccess сообщает интерфейсу, показывать ли страницу согласования.
// Доступен любому авторизованному: тому, кому согласование не положено, он
// отвечает allowed=false, а не ошибкой.
func GetApprovalAccess(c *gin.Context) {
	role := fmt.Sprint(mustGet(c, "role"))
	if role == "admin" {
		// Администратор работает на любой ступени, но выбирает её сам.
		c.JSON(http.StatusOK, models.PromoApprovalAccessResponse{Allowed: true})
		return
	}
	access, ok := resolveApprovalAccess(c, c.Query("approval_role"))
	if !ok {
		c.JSON(http.StatusOK, models.PromoApprovalAccessResponse{Allowed: false})
		return
	}
	c.JSON(http.StatusOK, models.PromoApprovalAccessResponse{
		Allowed:      true,
		ApprovalRole: fmt.Sprintf("agreement%d", access.AgreementNum),
		Scoped:       len(access.Scope) > 0,
	})
}

func GetApprovals(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)
	access, ok := resolveApprovalAccess(c, c.Query("approval_role"))
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ к согласованию запрещён"})
		return
	}
	agreementNum := access.AgreementNum
	effectiveRole := fmt.Sprintf("agreement%d", agreementNum)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

	// Фильтр по КАМу приходит от клиента, поэтому вне области он не сужает
	// выборку, а закрывает её: иначе подстановка чужого КАМа выдала бы
	// пустой список вместо отказа и выглядела бы как отсутствие промо.
	if requested := c.Query("kam"); requested != "" && !access.allowsKAM(requested) {
		c.JSON(http.StatusForbidden, gin.H{"error": "КАМ вне области согласования"})
		return
	}

	params := repository.ApprovalParams{
		Role:           effectiveRole,
		KAM:            c.Query("kam"),
		AllowedKAMs:    access.Scope,
		ApprovalStatus: c.DefaultQuery("approval_status", "pending"),
		YearStr:        c.Query("year"),
		MonthStr:       c.Query("month"),
		Network:        c.Query("network_name"),
		Brand:          c.Query("brand"),
		Mechanics:      c.Query("mechanics"),
		HasComments:    c.Query("has_comments") == "1",
		Page:           page,
		PageSize:       pageSize,
	}

	results, total, err := repository.GetApprovals(params)
	if err != nil {
		config.Logger.Error("promo_approvals_failed", "error", err.Error(), "role", roleStr, "status", params.ApprovalStatus)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	// Действующая ступень возвращается явно: у роли kam она следует из
	// закрепления, и интерфейс иначе подписал бы колонки чужой ступенью.
	c.JSON(http.StatusOK, gin.H{"data": results, "total": total, "approval_role": effectiveRole})
}

func ApprovePromo(c *gin.Context) {
	var req struct {
		ID           int    `json:"id"`
		UpdatedAt    string `json:"updated_at"`
		Status       string `json:"status"`
		Comment      string `json:"comment"`
		ApprovalRole string `json:"approval_role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}
	if req.ID <= 0 || strings.TrimSpace(req.UpdatedAt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id и updated_at обязательны"})
		return
	}
	access, ok := resolveApprovalAccess(c, req.ApprovalRole)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ к согласованию запрещён"})
		return
	}
	agreementNum := access.AgreementNum
	// КАМ промо берётся из базы, а не из запроса: иначе ограничение области
	// обходилось бы подменой поля в теле запроса.
	if !approvalIDsInScope(c, access, []int{req.ID}) {
		return
	}
	field := fmt.Sprintf("agreement%d", agreementNum)

	var status string
	var comment string
	var legacyValue string
	switch req.Status {
	case "comment":
		if req.Comment == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "комментарий не может быть пустым"})
			return
		}
		status = "commented"
		comment = req.Comment
		legacyValue = req.Comment
	case "согласовано":
		status = "approved"
		comment = req.Comment
		legacyValue = "согласовано"
		if req.Comment != "" {
			legacyValue = "согласовано: " + req.Comment
		}
	case "отклонено":
		status = "rejected"
		comment = req.Comment
		legacyValue = "отклонено"
		if req.Comment != "" {
			legacyValue = "отклонено: " + req.Comment
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "допустимые status: comment, согласовано, отклонено"})
		return
	}

	usernameVal, _ := c.Get("username")
	if err := repository.ApprovePromoWithStatus(
		agreementNum, req.ID, req.UpdatedAt, status, comment, legacyValue, fmt.Sprint(usernameVal),
	); err != nil {
		if respondIfDedupInProgress(c, err) {
			return
		}
		config.Logger.Error("approve_failed", "error", err.Error(), "id", req.ID, "field", field)
		var conflictErr *repository.ApprovalConflictError
		if errors.As(err, &conflictErr) {
			c.JSON(http.StatusConflict, gin.H{
				"error":        "Карточка была изменена. Обновите список и повторите действие",
				"conflict_ids": conflictErr.IDs,
			})
			return
		}
		if errors.Is(err, repository.ErrPromoNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Промо не найдено или удалено"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
		return
	}

	config.Logger.Info("promo_approved",
		"id", req.ID,
		"field", field,
		"status", status,
		"comment", comment,
		"user", fmt.Sprint(usernameVal),
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Обновлено"})
}

// ─── Approval Filters ─────────────────────────────────────────────────────

func GetApprovalFilters(c *gin.Context) {
	access, ok := resolveApprovalAccess(c, c.Query("approval_role"))
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ к согласованию запрещён"})
		return
	}
	effectiveRole := fmt.Sprintf("agreement%d", access.AgreementNum)
	if requested := c.Query("kam"); requested != "" && !access.allowsKAM(requested) {
		c.JSON(http.StatusForbidden, gin.H{"error": "КАМ вне области согласования"})
		return
	}

	params := repository.ApprovalFilterParams{
		ApprovalStatus: c.DefaultQuery("approval_status", "pending"),
		KAM:            c.Query("kam"),
		AllowedKAMs:    access.Scope,
		Network:        c.Query("network_name"),
		Brand:          c.Query("brand"),
		MechFilter:     c.Query("mechanics"),
		YearStr:        c.Query("year"),
		MonthStr:       c.Query("month"),
		Role:           effectiveRole,
	}

	networks, brands, mechanics, kams, err := repository.GetApprovalFilters(params)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"networks": []string{}, "brands": []string{}, "mechanics": []string{}, "kams": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"networks": networks, "brands": brands, "mechanics": mechanics, "kams": kams})
}

// ─── Batch Approve ─────────────────────────────────────────────────────────

func BatchApprovePromo(c *gin.Context) {
	var req struct {
		Items []struct {
			ID        int    `json:"id"`
			UpdatedAt string `json:"updated_at"`
		} `json:"items"`
		Status       string `json:"status"`
		Comment      string `json:"comment"`
		ApprovalRole string `json:"approval_role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}
	access, ok := resolveApprovalAccess(c, req.ApprovalRole)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ к согласованию запрещён"})
		return
	}
	agreementNum := access.AgreementNum
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items не может быть пустым"})
		return
	}
	if len(req.Items) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "за один запрос можно согласовать не более 500 промо"})
		return
	}
	items := make([]repository.BatchApproveItem, 0, len(req.Items))
	seenIDs := make(map[int]struct{}, len(req.Items))
	for _, item := range req.Items {
		if item.ID <= 0 || strings.TrimSpace(item.UpdatedAt) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "каждый элемент должен содержать id и updated_at"})
			return
		}
		if _, exists := seenIDs[item.ID]; exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id в пакете не должны повторяться"})
			return
		}
		seenIDs[item.ID] = struct{}{}
		items = append(items, repository.BatchApproveItem{ID: item.ID, UpdatedAt: item.UpdatedAt})
	}
	// Пакет проверяется целиком: одна строка вне области отменяет весь запрос,
	// иначе частичное применение оставило бы согласование в неясном состоянии.
	batchIDs := make([]int, 0, len(items))
	for _, item := range items {
		batchIDs = append(batchIDs, item.ID)
	}
	if !approvalIDsInScope(c, access, batchIDs) {
		return
	}

	var status string
	var comment string
	var legacyValue string
	switch req.Status {
	case "comment":
		if req.Comment == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "комментарий не может быть пустым"})
			return
		}
		status = "commented"
		comment = req.Comment
		legacyValue = req.Comment
	case "согласовано":
		status = "approved"
		comment = req.Comment
		legacyValue = "согласовано"
		if req.Comment != "" {
			legacyValue = "согласовано: " + req.Comment
		}
	case "отклонено":
		status = "rejected"
		comment = req.Comment
		legacyValue = "отклонено"
		if req.Comment != "" {
			legacyValue = "отклонено: " + req.Comment
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "допустимые status: comment, согласовано, отклонено"})
		return
	}

	usernameVal, _ := c.Get("username")
	rowsAffected, err := repository.BatchApprove(
		agreementNum, items, status, comment, legacyValue, fmt.Sprint(usernameVal),
	)
	if err != nil {
		if respondIfDedupInProgress(c, err) {
			return
		}
		config.Logger.Error("batch_approve_failed", "error", err.Error(), "count", len(req.Items))
		var conflictErr *repository.ApprovalConflictError
		if errors.As(err, &conflictErr) {
			c.JSON(http.StatusConflict, gin.H{
				"error":        "Часть карточек была изменена или удалена. Пакет не применён",
				"conflict_ids": conflictErr.IDs,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
		return
	}

	config.Logger.Info("batch_approved",
		"count", len(req.Items),
		"affected", rowsAffected,
		"status", status,
		"user", fmt.Sprint(usernameVal),
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Обновлено", "affected": rowsAffected})
}

// ─── Excel Export ────────────────────────────────────────────────────────────

func ExportPromoExcel(c *gin.Context) {
	scope, ok := promoVisibilityScope(c)
	if !ok {
		return
	}
	params := repository.PromoFilterParams{
		AllowedKAMs: scope,
		YearFromStr: c.Query("yearFrom"),
		YearToStr:   c.Query("yearTo"),
		Months:      c.QueryArray("months"),
		Kams:        c.QueryArray("kam"),
		Brands:      c.QueryArray("brand"),
		SKUs:        c.QueryArray("sku"),
		Networks:    c.QueryArray("network_name"),
		Mechanics:   c.QueryArray("mechanics"),
		Statuses:    c.QueryArray("status"),
		// Поиск и сортировка идут в выгрузку вместе с фильтрами: файл должен
		// повторять то, что видно в таблице, а не всю выборку в другом порядке.
		Search:        c.Query("search"),
		SortField:     c.Query("sortField"),
		SortDirection: c.Query("sortDirection"),
	}
	channels := c.QueryArray("channel")

	rows, err := repository.GetPromoRowsStream(params, channels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения данных"})
		return
	}
	defer rows.Close()

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Promo Data"
	f.SetSheetName("Sheet1", sheet)

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "StreamWriter creation failed"})
		return
	}

	// Заголовки
	headers := []interface{}{
		"ID промо", "Год", "Месяц", "Канал", "Сеть", "Бренд", "SKU", "Механика",
		"План (уп)", "Факт (уп)", "План инвест.", "Факт инвест.",
		"Согласование 1", "Согласование 2", "Статус",
	}
	if err := sw.SetRow("A1", headers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Header write failed"})
		return
	}

	// Данные — пишем напрямую из курсора БД
	rowNum := 2
	for rows.Next() {
		r, err := repository.ScanPromoRow(rows)
		if err != nil {
			config.Logger.Error("promo_excel_row_scan_failed", "row", rowNum, "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения данных для Excel"})
			return
		}
		vals := []interface{}{
			r.ID,
			r.Year,
			models.ValInt(r.Month),
			models.ValString(r.PromoChannel),
			models.ValString(r.NetworkName),
			models.ValString(r.BrandAS),
			models.ValString(r.SKU),
			models.ValString(r.Mechanics),
			models.ValFloat(r.PlanPromoUnits),
			models.ValFloat(r.ActualPromoSalesUnits),
			models.ValFloat(r.PlanInvestmentsRub),
			models.ValFloat(r.ActualInvestments),
			models.ValString(r.Agreement1),
			models.ValString(r.Agreement2),
			models.ValString(r.Status),
		}
		cell, _ := excelize.CoordinatesToCellName(1, rowNum)
		if err := sw.SetRow(cell, vals); err != nil {
			config.Logger.Error("promo_excel_row_write_failed", "row", rowNum, "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка формирования Excel"})
			return
		}
		rowNum++
	}
	if err := rows.Err(); err != nil {
		config.Logger.Error("promo_excel_rows_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения данных для Excel"})
		return
	}

	if err := sw.Flush(); err != nil {
		config.Logger.Error("promo_excel_stream_flush_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка формирования Excel"})
		return
	}

	// Стиль заголовка
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"6366F1"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)

	// Автоширина
	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		f.SetColWidth(sheet, col, col, 18)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=promo-export_%s.xlsx", time.Now().Format("2006-01-02")))
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		config.Logger.Error("excel_export_failed", "error", err.Error())
	}
}

// ─── Log helpers (используются для логирования, оставлены для совместимости) ─

func init() {
	// Проверка что middleware и models импортируются (избегаем unused import)
	_ = middleware.RoleRequired
	_ = models.ApprovalRow{}
}
