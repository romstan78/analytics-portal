// ─── Обработчики ────────────────────────────────────────────────────────────

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/repository"

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

	c.JSON(http.StatusOK, gin.H{
		"kam":          resKam,
		"brand":        resBrand,
		"sku":          resSKU,
		"network_name": resNetwork,
		"mechanics":    resMechanics,
		"status":       resStatus,
		"channel":      resChannel,
	})
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
			existing, err := repository.FetchExistingRow(idInt)
			if err != nil {
				config.Logger.Error("promo_update_fetch_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Запись не найдена"})
				return
			}

			// Сохраняем текущий updated_at для Optimistic Locking
			updatedAt := safeString(existing, "updated_at")

			for k, v := range input {
				if k != "id" && k != "deleted_at" && k != "updated_at" && k != "agreement1" && k != "agreement2" {
					existing[k] = v
				}
			}

			calculatePromoFields(existing)

			rowsAffected, err := repository.UpdatePromo(idInt, existing, updatedAt)
			if err != nil {
				config.Logger.Error("promo_update_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if rowsAffected == 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "Данные были изменены другим пользователем. Обновите страницу и попробуйте снова."})
				return
			}

			existing["updated_at"] = time.Now().Format("2006-01-02T15:04:05.9999999-07:00")

			config.Logger.Info("promo_updated",
				"id", idInt,
				"sku", fmt.Sprint(existing["sku"]),
				"network", fmt.Sprint(existing["network_name"]),
				"user", "system",
				"timestamp", time.Now().Format(time.RFC3339),
			)
			c.JSON(http.StatusOK, gin.H{"message": "Updated", "id": idInt, "data": existing})
			return
		}
	}

	// INSERT
	calculatePromoFields(input)
	delete(input, "id")

	newID, err := repository.InsertPromo(input)
	if err != nil {
		config.Logger.Error("promo_insert_failed",
			"error", err.Error(),
			"sku", fmt.Sprint(input["sku"]),
			"network", fmt.Sprint(input["network_name"]),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	config.Logger.Info("promo_created",
		"id", newID,
		"sku", fmt.Sprint(input["sku"]),
		"network", fmt.Sprint(input["network_name"]),
		"year", safeInt(input, "year"),
		"month", safeInt(input, "month"),
		"plan_units", safeFloat(input, "plan_promo_units"),
		"plan_rub", safeFloat(input, "plan_promo_rub"),
		"user", "system",
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Created", "id": newID, "data": input})
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

	config.Logger.Info("promo_deleted", "id", id, "user", "system", "timestamp", time.Now().Format(time.RFC3339))
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

	var value string
	switch req.Status {
	case "comment":
		if req.Comment == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "комментарий не может быть пустым"})
			return
		}
		value = req.Comment
	case "согласовано":
		value = "согласовано"
		if req.Comment != "" {
			value = "согласовано: " + req.Comment
		}
	case "отклонено":
		value = "отклонено"
		if req.Comment != "" {
			value = "отклонено: " + req.Comment
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "допустимые status: comment, согласовано, отклонено"})
		return
	}

	if err := repository.ApprovePromo(field, req.ID, value); err != nil {
		config.Logger.Error("approve_failed", "error", err.Error(), "id", req.ID, "field", field)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
		return
	}

	config.Logger.Info("promo_approved",
		"id", req.ID,
		"field", field,
		"value", value,
		"user", "system",
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Обновлено"})
}

// ─── Approval Filters ─────────────────────────────────────────────────────

func GetApprovalFilters(c *gin.Context) {
	params := repository.ApprovalFilterParams{
		ApprovalStatus: c.DefaultQuery("approval_status", "pending"),
		KAM:            c.Query("kam"),
		Network:        c.Query("network_name"),
		Brand:          c.Query("brand"),
		MechFilter:     c.Query("mechanics"),
		YearStr:        c.Query("year"),
		MonthStr:       c.Query("month"),
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

// ─── Log helpers (используются для логирования, оставлены для совместимости) ─

func init() {
	// Проверка что middleware и models импортируются (избегаем unused import)
	_ = middleware.RoleRequired
	_ = models.ApprovalRow{}
}
