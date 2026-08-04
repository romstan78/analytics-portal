// ─── Обработчики ────────────────────────────────────────────────────────────

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// ─── Read ──────────────────────────────────────────────────────────────────

func GetPromoFilters(c *gin.Context) {
	params := repository.PromoFilterParams{
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

	// Кэшируем только если фильтры по году/месяцу не заданы (дефолтная страница)
	cacheKey := "filters:" + params.YearFromStr + ":" + params.YearToStr + ":" + strings.Join(params.Months, ",")
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

	g.Go(func() error {
		resKam = repository.GetFilterValues("kam", baseWhere, baseArgs, "kam", mainFilters)
		return nil
	})
	g.Go(func() error {
		resBrand = repository.GetFilterValues("brand_as", baseWhere, baseArgs, "brand_as", mainFilters)
		return nil
	})
	g.Go(func() error {
		resSKU = repository.GetFilterValues("sku", baseWhere, baseArgs, "sku", mainFilters)
		return nil
	})
	g.Go(func() error {
		resNetwork = repository.GetFilterValues("network_name", baseWhere, baseArgs, "network_name", mainFilters)
		return nil
	})
	g.Go(func() error {
		resMechanics = repository.GetFilterValues("mechanics", baseWhere, baseArgs, "mechanics", mainFilters)
		return nil
	})
	g.Go(func() error {
		resStatus = repository.GetFilterValues("status", baseWhere, baseArgs, "status", mainFilters)
		return nil
	})
	g.Go(func() error {
		resChannel = repository.GetChannelFilterValues(baseWhere, baseArgs, mainFilters)
		return nil
	})

	_ = g.Wait()

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

func GetPromoData(c *gin.Context) {
	params := repository.PromoFilterParams{
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
	channels := c.QueryArray("channel")

	all := c.Query("all")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("limit", "100")))

	results, err := repository.GetPromoRows(params, channels, page, pageSize, all == "true")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
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

func applyJSONToRow(r *models.PromoRowDB, input map[string]interface{}) {
	for k, v := range input {
		if k == "id" || k == "deleted_at" || k == "updated_at" || strings.HasPrefix(k, "agreement") {
			continue
		}

		// Если значение nil или строка "<nil>" — пропускаем, чтобы не затирать поле мусором.
		// Пустую строку обрабатываем ниже — она должна очищать строковые поля.
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
		case "comments":
			r.Comments = fmt.Sprint(v)
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

func SavePromo(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// UPDATE
	if id, ok := input["id"]; ok && id != nil {
		idFloat, _ := strconv.ParseFloat(fmt.Sprint(id), 64)
		idInt := int(idFloat)
		if idInt > 0 {
			row, err := repository.FetchExistingRow(idInt)
			if err != nil {
				config.Logger.Error("promo_update_fetch_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Запись не найдена"})
				return
			}

			updatedAt := row.UpdatedAt

			// Применяем входящие данные поверх существующей строки
			applyJSONToRow(row, input)

			// Пересчитываем вычисляемые поля
			dto := services.DBRowToDTO(row)
			calcCtx := services.EnrichFromRepo(&dto)
			calc := services.CalculateFields(&dto, calcCtx)
			services.MergeCalculatedIntoDBRow(row, calc)

			rowsAffected, err := repository.UpdatePromo(idInt, row, updatedAt)
			if err != nil {
				config.Logger.Error("promo_update_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	dto := services.MapToDTO(input)
	calcCtx := services.EnrichFromRepo(&dto)
	calc := services.CalculateFields(&dto, calcCtx)
	row := services.DTOToDBRow(dto, calc)

	newID, err := repository.InsertPromo(row)
	if err != nil {
		config.Logger.Error("promo_insert_failed",
			"error", err.Error(),
			"sku", dto.SKU,
			"network", dto.NetworkName,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	config.Logger.Info("promo_deleted", "id", id, "user", fmt.Sprint(usernameVal), "timestamp", time.Now().Format(time.RFC3339))
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ─── Approvals ─────────────────────────────────────────────────────────────

func GetApprovals(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	params := repository.ApprovalParams{
		Role:           roleStr,
		KAM:            c.Query("kam"),
		ApprovalStatus: c.DefaultQuery("approval_status", "pending"),
		YearStr:        c.Query("year"),
		MonthStr:       c.Query("month"),
		Network:        c.Query("network_name"),
		Brand:          c.Query("brand"),
		Mechanics:      c.Query("mechanics"),
	}

	results, err := repository.GetApprovals(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func ApprovePromo(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	field := "agreement1"
	if roleStr == "agreement2" {
		field = "agreement2"
	}

	var req struct {
		ID      int    `json:"id"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
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

	agreementNum := 1
	if roleStr == "agreement2" {
		agreementNum = 2
	}

	if err := repository.ApprovePromoWithStatus(agreementNum, req.ID, status, comment, legacyValue); err != nil {
		config.Logger.Error("approve_failed", "error", err.Error(), "id", req.ID, "field", field)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
		return
	}

	usernameVal, _ := c.Get("username")
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
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	params := repository.ApprovalFilterParams{
		ApprovalStatus: c.DefaultQuery("approval_status", "pending"),
		KAM:            c.Query("kam"),
		Network:        c.Query("network_name"),
		Brand:          c.Query("brand"),
		MechFilter:     c.Query("mechanics"),
		YearStr:        c.Query("year"),
		MonthStr:       c.Query("month"),
		Role:           roleStr,
	}

	networks, brands, mechanics, kams, err := repository.GetApprovalFilters(params)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"networks": []string{}, "brands": []string{}, "mechanics": []string{}, "kams": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"networks": networks, "brands": brands, "mechanics": mechanics, "kams": kams})
}

func GetApprovalKAMs(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)
	field := "agreement1"
	if roleStr == "agreement2" {
		field = "agreement2"
	}

	kams, err := repository.GetApprovalKAMs(field)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": kams})
}

func GetApprovalNetworks(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)
	kam := c.Query("kam")
	if kam == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}

	field := "p.agreement1"
	if roleStr == "agreement2" {
		field = "p.agreement2"
	}

	networks, err := repository.GetApprovalNetworks(field, kam)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": networks})
}

func GetApprovalBrands(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)
	kam := c.Query("kam")
	network := c.Query("network_name")
	if kam == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}

	field := "p.agreement1"
	if roleStr == "agreement2" {
		field = "p.agreement2"
	}

	brands, err := repository.GetApprovalBrands(field, kam, network)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": brands})
}

// ─── Batch Approve ─────────────────────────────────────────────────────────

func BatchApprovePromo(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	agreementNum := 1
	if roleStr == "agreement2" {
		agreementNum = 2
	}

	var req struct {
		IDs     []int  `json:"ids"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids не может быть пустым"})
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

	rowsAffected, err := repository.BatchApprove(agreementNum, req.IDs, status, comment, legacyValue)
	if err != nil {
		config.Logger.Error("batch_approve_failed", "error", err.Error(), "count", len(req.IDs))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
		return
	}

	usernameVal, _ := c.Get("username")
	config.Logger.Info("batch_approved",
		"count", len(req.IDs),
		"affected", rowsAffected,
		"status", status,
		"user", fmt.Sprint(usernameVal),
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Обновлено", "affected": rowsAffected})
}

// ─── Log helpers (используются для логирования, оставлены для совместимости) ─

func init() {
	// Проверка что middleware и models импортируются (избегаем unused import)
	_ = middleware.RoleRequired
	_ = models.ApprovalRow{}
}
