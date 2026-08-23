// ─── Реестр сетей ───────────────────────────────────────────────────────────

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
)

// ─── Вспомогательные ───────────────────────────────────────────────────────

func networkIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID сети"})
		return 0, false
	}
	return id, true
}

// planYear — год из query; по умолчанию текущий.
func planYear(c *gin.Context) (int, bool) {
	raw := strings.TrimSpace(c.Query("year"))
	if raw == "" {
		return time.Now().Year(), true
	}
	year, err := strconv.Atoi(raw)
	if err != nil || year < 2000 || year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный год"})
		return 0, false
	}
	return year, true
}

func planQuarter(c *gin.Context) (int, bool) {
	raw := strings.TrimSpace(c.Query("quarter"))
	if raw == "" {
		month := int(time.Now().Month())
		return (month-1)/3 + 1, true
	}
	quarter, err := strconv.Atoi(raw)
	if err != nil || quarter < 1 || quarter > 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный квартал"})
		return 0, false
	}
	return quarter, true
}

func currentUser(c *gin.Context) (username, role string) {
	if v, ok := c.Get("username"); ok {
		username = fmt.Sprint(v)
	}
	if v, ok := c.Get("role"); ok {
		role = fmt.Sprint(v)
	}
	return username, role
}

// jsonString сериализует набор изменений для changed_fields аудит-лога.
func jsonString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func validNetworkType(t string) bool {
	return t == "regular" || t == "warehouse"
}

// respondNetworkError переводит ошибки репозитория в HTTP-коды.
func respondNetworkError(c *gin.Context, err error, logEvent string) {
	switch {
	case errors.Is(err, repository.ErrNetworkNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Сеть не найдена"})
	case errors.Is(err, repository.ErrNetworkExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Сеть с таким названием уже есть в реестре"})
	case errors.Is(err, repository.ErrNetworkConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "Данные изменены другим пользователем. Обновите страницу и повторите."})
	case errors.Is(err, repository.ErrNetworkPriceOverlap):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Периоды цены одного SKU не должны пересекаться"})
	case errors.Is(err, repository.ErrNetworkClosedMonth):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Закрытый месяц нельзя изменять"})
	default:
		config.Logger.Error(logEvent, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обработки запроса"})
	}
}

// ─── Карточка сети ──────────────────────────────────────────────────────────

// GetNetworks — список сетей реестра.
func GetNetworks(c *gin.Context) {
	networks, err := repository.ListNetworks(
		strings.TrimSpace(c.Query("search")),
		strings.TrimSpace(c.Query("kam")),
		c.Query("include_inactive") == "1",
	)
	if err != nil {
		respondNetworkError(c, err, "networks_list_failed")
		return
	}
	c.JSON(http.StatusOK, models.NetworkListResponse{Data: networks})
}

type networkInput struct {
	Name        string  `json:"name"`
	KAM         string  `json:"kam"`
	NetworkType string  `json:"network_type"`
	IsActive    *bool   `json:"is_active"`
	UpdatedAt   string  `json:"updated_at"`
	VATIncluded *bool   `json:"vat_included"` // настройки первого периода
	VATRate     float64 `json:"vat_rate"`
	Year        int     `json:"year"`
}

// CreateNetwork заводит сеть и, если переданы настройки, сразу открывает год:
// четыре квартала с одинаковым НДС и типом контракта.
func CreateNetwork(c *gin.Context) {
	var input networkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите название сети"})
		return
	}
	if input.NetworkType == "" {
		input.NetworkType = "regular"
	}
	if !validNetworkType(input.NetworkType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Тип сети: regular или warehouse"})
		return
	}

	id, err := repository.InsertNetwork(input.Name, input.KAM, input.NetworkType)
	if err != nil {
		respondNetworkError(c, err, "network_create_failed")
		return
	}

	username, _ := currentUser(c)
	_ = repository.InsertEntityAuditLog("network", id, username, "CREATE",
		fmt.Sprintf(`{"name":%q,"network_type":%q}`, input.Name, input.NetworkType))

	// Первый год открываем сразу, чтобы КАМ попал в готовую сетку планов.
	if input.Year > 0 {
		vatIncluded := true
		if input.VATIncluded != nil {
			vatIncluded = *input.VATIncluded
		}
		vatRate := input.VATRate
		if vatRate <= 0 {
			vatRate = 20
		}
		periods := make([]models.NetworkPeriod, 0, 4)
		for q := 1; q <= 4; q++ {
			periods = append(periods, models.NetworkPeriod{
				Quarter: q, VATIncluded: vatIncluded, VATRate: vatRate,
			})
		}
		if _, err := repository.SaveNetworkPlan(repository.SaveNetworkPlanInput{
			NetworkID: id, Year: input.Year, Periods: periods, UserName: username,
		}); err != nil {
			config.Logger.Error("network_create_periods_failed", "error", err.Error(), "network_id", id)
		}
	}

	network, err := repository.GetNetworkByID(id)
	if err != nil {
		respondNetworkError(c, err, "network_create_refetch_failed")
		return
	}
	config.Logger.Info("network_created", "id", id, "name", input.Name, "user", username)
	c.JSON(http.StatusOK, models.NetworkSaveResponse{Message: "Created", Data: network})
}

// UpdateNetwork правит карточку сети.
func UpdateNetwork(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}

	var input networkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, err := repository.GetNetworkByID(id)
	if err != nil {
		respondNetworkError(c, err, "network_update_fetch_failed")
		return
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = current.Name
	}
	networkType := input.NetworkType
	if networkType == "" {
		networkType = current.NetworkType
	}
	if !validNetworkType(networkType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Тип сети: regular или warehouse"})
		return
	}
	isActive := current.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	if err := repository.UpdateNetwork(id, name, input.KAM, networkType, isActive, input.UpdatedAt); err != nil {
		respondNetworkError(c, err, "network_update_failed")
		return
	}

	username, _ := currentUser(c)
	changes := map[string]interface{}{}
	if current.Name != name {
		changes["name"] = map[string]interface{}{"old": current.Name, "new": name}
	}
	if current.NetworkType != networkType {
		changes["network_type"] = map[string]interface{}{"old": current.NetworkType, "new": networkType}
	}
	if models.ValString(current.KAM) != strings.TrimSpace(input.KAM) {
		changes["kam"] = map[string]interface{}{"old": models.ValString(current.KAM), "new": strings.TrimSpace(input.KAM)}
	}
	if current.IsActive != isActive {
		changes["is_active"] = map[string]interface{}{"old": current.IsActive, "new": isActive}
	}
	if len(changes) > 0 {
		_ = repository.InsertEntityAuditLog("network", id, username, "UPDATE", jsonString(changes))
	}

	network, err := repository.GetNetworkByID(id)
	if err != nil {
		respondNetworkError(c, err, "network_update_refetch_failed")
		return
	}
	c.JSON(http.StatusOK, models.NetworkSaveResponse{Message: "Updated", Data: network})
}

// ─── Планы ──────────────────────────────────────────────────────────────────

// GetNetworkPlan отдаёт всё, что нужно вкладке «Планы»: карточку, кварталы,
// строки плана с расчётом НДС и итоги по кварталам.
func GetNetworkPlan(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	year, ok := planYear(c)
	if !ok {
		return
	}

	network, err := repository.GetNetworkByID(id)
	if err != nil {
		respondNetworkError(c, err, "network_plan_fetch_failed")
		return
	}
	periods, err := repository.GetNetworkPeriods(id, year)
	if err != nil {
		respondNetworkError(c, err, "network_periods_failed")
		return
	}
	plans, err := repository.GetNetworkPlans(id, year)
	if err != nil {
		respondNetworkError(c, err, "network_plans_failed")
		return
	}

	plans = services.EnrichNetworkPlans(plans, periods)
	totals := services.CalculateNetworkTotals(plans, periods)
	c.JSON(http.StatusOK, models.NetworkPlanResponse{
		Network:    network,
		Year:       year,
		Periods:    periods,
		Plans:      plans,
		Totals:     totals,
		YearTotals: services.SumYearTotals(totals),
	})
}

type savePlanInput struct {
	Year    int `json:"year"`
	Periods []struct {
		Quarter     int     `json:"quarter"`
		VATIncluded bool    `json:"vat_included"`
		VATRate     float64 `json:"vat_rate"`
	} `json:"periods"`
	Plans []repository.NetworkPlanInput `json:"plans"`
}

// SaveNetworkPlan сохраняет кварталы и строки плана одной транзакцией.
func SaveNetworkPlan(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}

	var input savePlanInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Year < 2000 || input.Year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный год"})
		return
	}
	if _, err := repository.GetNetworkByID(id); err != nil {
		respondNetworkError(c, err, "network_plan_save_fetch_failed")
		return
	}

	periods := make([]models.NetworkPeriod, 0, len(input.Periods))
	for _, p := range input.Periods {
		if p.VATRate < 0 || p.VATRate >= 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ставка НДС вне диапазона"})
			return
		}
		periods = append(periods, models.NetworkPeriod{
			Quarter: p.Quarter, VATIncluded: p.VATIncluded, VATRate: p.VATRate,
		})
	}

	username, _ := currentUser(c)
	diff, err := repository.SaveNetworkPlan(repository.SaveNetworkPlanInput{
		NetworkID: id, Year: input.Year, Periods: periods, Plans: input.Plans, UserName: username,
	})
	if err != nil {
		respondNetworkError(c, err, "network_plan_save_failed")
		return
	}
	if diff != "" {
		_ = repository.InsertEntityAuditLog("network_plan", id, username, "UPDATE", diff)
	}

	updatedPeriods, err := repository.GetNetworkPeriods(id, input.Year)
	if err != nil {
		respondNetworkError(c, err, "network_periods_failed")
		return
	}
	updatedPlans, err := repository.GetNetworkPlans(id, input.Year)
	if err != nil {
		respondNetworkError(c, err, "network_plans_failed")
		return
	}
	updatedPlans = services.EnrichNetworkPlans(updatedPlans, updatedPeriods)
	totals := services.CalculateNetworkTotals(updatedPlans, updatedPeriods)

	config.Logger.Info("network_plan_saved", "network_id", id, "year", input.Year, "user", username)
	c.JSON(http.StatusOK, models.NetworkPlanSaveResponse{
		Message:    "Saved",
		Year:       input.Year,
		Periods:    updatedPeriods,
		Plans:      updatedPlans,
		Totals:     totals,
		YearTotals: services.SumYearTotals(totals),
	})
}

// PreviewNetworkPlan пересчитывает несохранённый черновик сетки.
// Единственный источник расчёта НДС и итогов — backend: интерфейс только
// показывает то, что вернул этот эндпоинт. В БД ничего не пишется.
func PreviewNetworkPlan(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}

	var input savePlanInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Year < 2000 || input.Year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный год"})
		return
	}

	periods := make([]models.NetworkPeriod, 0, len(input.Periods))
	for _, p := range input.Periods {
		if p.VATRate < 0 || p.VATRate >= 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ставка НДС вне диапазона"})
			return
		}
		periods = append(periods, models.NetworkPeriod{
			NetworkID: id, Year: input.Year,
			Quarter: p.Quarter, VATIncluded: p.VATIncluded, VATRate: p.VATRate,
		})
	}

	draft := make([]services.NetworkPlanDraft, 0, len(input.Plans))
	for _, p := range input.Plans {
		if p.Quarter < 1 || p.Quarter > 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный квартал"})
			return
		}
		month1Pct, month2Pct, month3Pct := 30.0, 30.0, 40.0
		if p.Month1Pct != nil {
			month1Pct = *p.Month1Pct
		}
		if p.Month2Pct != nil {
			month2Pct = *p.Month2Pct
		}
		if p.Month3Pct != nil {
			month3Pct = *p.Month3Pct
		}
		draft = append(draft, services.NetworkPlanDraft{
			Quarter:        p.Quarter,
			BrandAS:        p.BrandAS,
			InGross:        p.InGross,
			PlanRub:        p.PlanRub,
			ForecastRub:    p.ForecastRub,
			InvestmentsPct: p.InvestmentsPct,
			Month1Pct:      month1Pct,
			Month2Pct:      month2Pct,
			Month3Pct:      month3Pct,
		})
	}

	// Факт в черновик не входит: он приходит загрузкой, поэтому берётся
	// из сохранённых строк, а не из тела запроса.
	stored, err := repository.GetNetworkPlans(id, input.Year)
	if err != nil {
		respondNetworkError(c, err, "network_plan_preview_failed")
		return
	}

	plans, totals, yearTotals := services.PreviewNetworkPlans(draft, stored, periods)
	c.JSON(http.StatusOK, models.NetworkPlanPreviewResponse{
		Year:       input.Year,
		Periods:    periods,
		Plans:      plans,
		Totals:     totals,
		YearTotals: yearTotals,
	})
}

// ─── Помесячный прогноз ─────────────────────────────────────────────────────

func loadNetworkForecast(networkID, year, quarter int) (models.NetworkForecastResponse, error) {
	network, err := repository.GetNetworkByID(networkID)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	periods, err := repository.GetNetworkPeriods(networkID, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	plans, err := repository.GetNetworkPlans(networkID, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	facts, err := repository.GetNetworkMonthlyFacts(networkID, year-1, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	forecasts, err := repository.GetNetworkForecastLines(networkID, year, quarter)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	promos, err := repository.GetNetworkPromoIndicators(network.Name, year, quarter)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	prices, err := repository.GetNetworkContractPrices(networkID, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	return services.BuildNetworkForecast(
		network, year, quarter, plans, periods, facts, forecasts, promos, prices, time.Now(),
	), nil
}

func GetNetworkForecast(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	year, ok := planYear(c)
	if !ok {
		return
	}
	quarter, ok := planQuarter(c)
	if !ok {
		return
	}
	response, err := loadNetworkForecast(id, year, quarter)
	if err != nil {
		respondNetworkError(c, err, "network_forecast_failed")
		return
	}
	c.JSON(http.StatusOK, response)
}

type saveForecastInput struct {
	Year    int                               `json:"year"`
	Quarter int                               `json:"quarter"`
	Lines   []repository.NetworkForecastInput `json:"lines"`
}

func SaveNetworkForecast(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	var input saveForecastInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Year < 2000 || input.Year > 2100 || input.Quarter < 1 || input.Quarter > 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный период прогноза"})
		return
	}
	if _, err := repository.GetNetworkByID(id); err != nil {
		respondNetworkError(c, err, "network_forecast_save_fetch_failed")
		return
	}
	username, _ := currentUser(c)
	if err := repository.SaveNetworkForecastLines(repository.SaveNetworkForecastInput{
		NetworkID: id, Year: input.Year, Quarter: input.Quarter,
		Lines: input.Lines, UserName: username,
	}); err != nil {
		respondNetworkError(c, err, "network_forecast_save_failed")
		return
	}
	response, err := loadNetworkForecast(id, input.Year, input.Quarter)
	if err != nil {
		respondNetworkError(c, err, "network_forecast_refetch_failed")
		return
	}
	if err := repository.UpdateNetworkPlanForecastRollup(id, input.Year, input.Quarter, response.Brands); err != nil {
		config.Logger.Error("network_forecast_rollup_failed", "error", err.Error(), "network_id", id)
	}
	_ = repository.InsertEntityAuditLog(
		"network_forecast", id, username, "UPDATE",
		jsonString(map[string]interface{}{"year": input.Year, "quarter": input.Quarter, "lines": len(input.Lines)}),
	)
	c.JSON(http.StatusOK, models.NetworkForecastSaveResponse{Message: "Saved", Data: response})
}

// ─── Цены контракта ─────────────────────────────────────────────────────────

func GetNetworkPrices(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	year, ok := planYear(c)
	if !ok {
		return
	}
	network, err := repository.GetNetworkByID(id)
	if err != nil {
		respondNetworkError(c, err, "network_prices_fetch_failed")
		return
	}
	prices, err := repository.GetNetworkContractPrices(id, year)
	if err != nil {
		respondNetworkError(c, err, "network_prices_failed")
		return
	}
	c.JSON(http.StatusOK, models.NetworkPricesResponse{Network: network, Year: year, Data: prices})
}

type savePricesInput struct {
	Year int                                    `json:"year"`
	Rows []repository.NetworkContractPriceInput `json:"rows"`
}

func SaveNetworkPrices(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	var input savePricesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Year < 2000 || input.Year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный год"})
		return
	}
	network, err := repository.GetNetworkByID(id)
	if err != nil {
		respondNetworkError(c, err, "network_prices_save_fetch_failed")
		return
	}
	username, _ := currentUser(c)
	if err := repository.SaveNetworkContractPrices(repository.SaveNetworkPricesInput{
		NetworkID: id, Rows: input.Rows, UserName: username,
	}); err != nil {
		respondNetworkError(c, err, "network_prices_save_failed")
		return
	}
	prices, err := repository.GetNetworkContractPrices(id, input.Year)
	if err != nil {
		respondNetworkError(c, err, "network_prices_refetch_failed")
		return
	}
	_ = repository.InsertEntityAuditLog(
		"network_price", id, username, "UPDATE",
		jsonString(map[string]interface{}{"year": input.Year, "rows": len(input.Rows)}),
	)
	c.JSON(http.StatusOK, models.NetworkPricesSaveResponse{
		Message: "Saved",
		Data:    models.NetworkPricesResponse{Network: network, Year: input.Year, Data: prices},
	})
}

// ─── Комментарии, история, справочники ──────────────────────────────────────

// GetNetworkComments — все комментарии сети.
func GetNetworkComments(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	comments, err := repository.GetNetworkComments(id)
	if err != nil {
		respondNetworkError(c, err, "network_comments_failed")
		return
	}
	c.JSON(http.StatusOK, models.NetworkCommentsResponse{Data: comments})
}

type commentInput struct {
	Text    string  `json:"comment_text"`
	Year    *int    `json:"year"`
	Quarter *int    `json:"quarter"`
	BrandAS *string `json:"brand_as"`
}

// AddNetworkComment добавляет комментарий к сети или к ячейке плана.
func AddNetworkComment(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}

	var input commentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	text := strings.TrimSpace(input.Text)
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Комментарий пустой"})
		return
	}
	if input.Quarter != nil && (*input.Quarter < 1 || *input.Quarter > 4) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный квартал"})
		return
	}
	if _, err := repository.GetNetworkByID(id); err != nil {
		respondNetworkError(c, err, "network_comment_fetch_failed")
		return
	}

	username, role := currentUser(c)
	if err := repository.InsertNetworkComment(models.NetworkComment{
		NetworkID: id, Year: input.Year, Quarter: input.Quarter, BrandAS: input.BrandAS,
		UserName: username, Role: role, CommentText: text,
	}); err != nil {
		respondNetworkError(c, err, "network_comment_insert_failed")
		return
	}

	comments, err := repository.GetNetworkComments(id)
	if err != nil {
		respondNetworkError(c, err, "network_comments_failed")
		return
	}
	c.JSON(http.StatusOK, models.NetworkCommentsResponse{Message: "Created", Data: comments})
}

// GetNetworkAudit — история изменений карточки и планов сети.
func GetNetworkAudit(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	log, err := repository.GetNetworkAuditLog(id)
	if err != nil {
		respondNetworkError(c, err, "network_audit_failed")
		return
	}
	c.JSON(http.StatusOK, models.NetworkAuditResponse{Data: log})
}

// GetNetworkBrands — бренды для строк плана.
func GetNetworkBrands(c *gin.Context) {
	brands, err := repository.GetBrandOptions()
	if err != nil {
		respondNetworkError(c, err, "network_brands_failed")
		return
	}
	c.JSON(http.StatusOK, models.NetworkBrandsResponse{Data: brands})
}
