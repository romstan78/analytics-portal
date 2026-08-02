package handlers

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"

	"github.com/gin-gonic/gin"
)

// ─── Вспомогательные утилиты ────────────────────────────────────────────────

func safeFloat(input map[string]interface{}, key string) float64 {
	val, _ := strconv.ParseFloat(fmt.Sprint(input[key]), 64)
	return val
}

func safeInt(input map[string]interface{}, key string) int {
	val, _ := strconv.Atoi(fmt.Sprint(input[key]))
	return val
}

func safeString(input map[string]interface{}, key string) string {
	return fmt.Sprint(input[key])
}

// calculatePromoFields пересчитывает все производные поля и записывает их в input.
func calculatePromoFields(input map[string]interface{}) {
	ppu := safeFloat(input, "plan_promo_units")
	cp := safeFloat(input, "contract_price")
	bu := safeFloat(input, "baseline_units")
	pir := safeFloat(input, "plan_investments_rub")
	month := safeInt(input, "month")
	year := safeInt(input, "year")
	if year == 0 {
		year = time.Now().Year()
	}
	if month == 0 {
		month = int(time.Now().Month())
	}

	gm := safeFloat(input, "gm")
	if gm == 0 {
		sku := safeString(input, "sku")
		var dbGM sql.NullFloat64
		config.DB.QueryRow(
			"SELECT TOP 1 gm FROM dbo.tbl_PromoActivities WHERE sku = ? AND gm IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC",
			sku,
		).Scan(&dbGM)
		if dbGM.Valid {
			gm = dbGM.Float64
		} else {
			gm = 1
		}
	}

	sku := safeString(input, "sku")
	networkName := safeString(input, "network_name")

	// key_region и top20_segment — приоритет: фронтенд > tbl_NetworkGeoMapping
	if safeString(input, "key_region") == "" || safeString(input, "top20_segment") == "" {
		var kr, t20 sql.NullString
		config.DB.QueryRow(
			"SELECT key_region, top20_segment FROM dbo.tbl_NetworkGeoMapping WHERE network_name = ?",
			networkName,
		).Scan(&kr, &t20)
		if kr.Valid && safeString(input, "key_region") == "" {
			input["key_region"] = kr.String
		}
		if t20.Valid && safeString(input, "top20_segment") == "" {
			input["top20_segment"] = t20.String
		}
	}

	// olap_price — из последнего промо по SKU
	var olapPrice sql.NullFloat64
	config.DB.QueryRow(
		"SELECT TOP 1 olap_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND olap_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC",
		sku,
	).Scan(&olapPrice)
	olap := 0.0
	if olapPrice.Valid {
		olap = olapPrice.Float64
	}

	// ─── Плановые ──────────────────────────────────────────────────────────
	quarter := int(math.Ceil(float64(month) / 3))
	plan_promo_rub := ppu * cp
	plan_promo_uplift_units := ppu - bu
	plan_promo_uplift_rub := plan_promo_uplift_units * cp

	plan_promo_uplift_pct_units := 0.0
	if ppu > 0 {
		plan_promo_uplift_pct_units = (plan_promo_uplift_units / ppu) * 100
	}
	plan_promo_uplift_pct_rub := 0.0
	if plan_promo_rub > 0 {
		plan_promo_uplift_pct_rub = (plan_promo_uplift_rub / plan_promo_rub) * 100
	}
	plan_investments_pct := 0.0
	if plan_promo_rub > 0 {
		plan_investments_pct = (pir / plan_promo_rub) * 100
	}
	plan_roi := 0.0
	if pir > 0 {
		plan_roi = (plan_promo_uplift_rub/pir)*gm*100 - 100
	}
	baseline_rub := bu * cp

	// ─── Фактические ───────────────────────────────────────────────────────
	afu := safeFloat(input, "actual_promo_sales_units")
	afr := safeFloat(input, "actual_promo_rub")
	afi := safeFloat(input, "actual_investments")
	afupl := safeFloat(input, "actual_promo_uplift_units")
	afupr := safeFloat(input, "actual_promo_uplift_rub")
	afeu := safeFloat(input, "actual_external_ecom_units")
	acb := safeFloat(input, "actual_corrected_baseline")
	ph := safeFloat(input, "promo_pharmacies")
	if ph == 0 {
		ph = 1
	}

	net_promo_uplift_rub := afupr * gm
	net_promo_uplift_pct := 0.0
	if afr > 0 {
		net_promo_uplift_pct = (net_promo_uplift_rub / afr) * 100
	}
	actual_investments_pct := 0.0
	if afr > 0 {
		actual_investments_pct = (afi / afr) * 100
	}
	actual_roi := 0.0
	if afi > 0 {
		actual_roi = (afupr/afi)*gm*100 - 100
	}

	actual_promo_rub_wo_ecom := afr - (afeu * cp)
	actual_promo_uplift_units_wo_ecom := afupl - afeu
	actual_promo_uplift_rub_wo_ecom := actual_promo_uplift_units_wo_ecom * cp
	net_promo_uplift_rub_wo_ecom := actual_promo_uplift_rub_wo_ecom * gm
	net_promo_uplift_pct_wo_ecom := 0.0
	if actual_promo_rub_wo_ecom > 0 {
		net_promo_uplift_pct_wo_ecom = (net_promo_uplift_rub_wo_ecom / actual_promo_rub_wo_ecom) * 100
	}
	actual_investments_pct_wo_ecom := 0.0
	if actual_promo_rub_wo_ecom > 0 {
		actual_investments_pct_wo_ecom = (afi / actual_promo_rub_wo_ecom) * 100
	}
	actual_roi_wo_ecom := 0.0
	if afi > 0 {
		actual_roi_wo_ecom = (actual_promo_uplift_rub_wo_ecom/afi)*gm*100 - 100
	}

	plan_vs_fact_rub := 0.0
	if plan_promo_rub > 0 {
		plan_vs_fact_rub = (afr / plan_promo_rub) * 100
	}
	plan_vs_fact_investments := 0.0
	if pir > 0 {
		plan_vs_fact_investments = (afi / pir) * 100
	}

	turnover_per_point := acb / ph
	turnover_per_point_promo := afu / ph

	plan_promo_cip_olap := ppu * olap
	fact_promo_cip_olap := afu * olap
	plan_promo_uplift_cip_olap := plan_promo_uplift_units * olap
	fact_promo_uplift_cip_olap := afupl * olap

	promoDate := fmt.Sprintf("%d-%02d-01", year, month)

	// ─── Запись в input ────────────────────────────────────────────────────
	input["year"] = year
	input["month"] = month
	input["quarter"] = quarter
	input["gm"] = gm
	input["plan_promo_rub"] = plan_promo_rub
	input["plan_promo_uplift_units"] = plan_promo_uplift_units
	input["plan_promo_uplift_rub"] = plan_promo_uplift_rub
	input["plan_promo_uplift_pct_units"] = plan_promo_uplift_pct_units
	input["plan_promo_uplift_pct_rub"] = plan_promo_uplift_pct_rub
	input["plan_investments_pct"] = plan_investments_pct
	input["plan_roi"] = plan_roi
	input["baseline_rub"] = baseline_rub
	input["actual_promo_rub"] = afr
	input["actual_promo_uplift_units"] = afupl
	input["actual_promo_uplift_rub"] = afupr
	input["actual_roi"] = actual_roi
	input["net_promo_uplift_rub"] = net_promo_uplift_rub
	input["net_promo_uplift_pct"] = net_promo_uplift_pct
	input["actual_investments_pct"] = actual_investments_pct
	input["actual_promo_rub_wo_ecom"] = actual_promo_rub_wo_ecom
	input["actual_promo_uplift_units_wo_ecom"] = actual_promo_uplift_units_wo_ecom
	input["actual_promo_uplift_rub_wo_ecom"] = actual_promo_uplift_rub_wo_ecom
	input["net_promo_uplift_rub_wo_ecom"] = net_promo_uplift_rub_wo_ecom
	input["net_promo_uplift_pct_wo_ecom"] = net_promo_uplift_pct_wo_ecom
	input["actual_investments_pct_wo_ecom"] = actual_investments_pct_wo_ecom
	input["actual_roi_wo_ecom"] = actual_roi_wo_ecom
	input["plan_vs_fact_rub"] = plan_vs_fact_rub
	input["plan_vs_fact_investments"] = plan_vs_fact_investments
	input["turnover_per_point"] = turnover_per_point
	input["turnover_per_point_promo"] = turnover_per_point_promo
	input["plan_promo_cip_olap"] = plan_promo_cip_olap
	input["fact_promo_cip_olap"] = fact_promo_cip_olap
	input["plan_promo_uplift_cip_olap"] = plan_promo_uplift_cip_olap
	input["fact_promo_uplift_cip_olap"] = fact_promo_uplift_cip_olap
	input["date"] = promoDate
	input["olap_price"] = olap
}

// ─── Обработчики ────────────────────────────────────────────────────────────

func GetPromoFilters(c *gin.Context) {
	yearFromStr := c.Query("yearFrom")
	yearToStr := c.Query("yearTo")
	months := c.QueryArray("months")
	kams := c.QueryArray("kam")
	brands := c.QueryArray("brand")
	skus := c.QueryArray("sku")
	networks := c.QueryArray("network_name")
	mechanics := c.QueryArray("mechanics")
	statuses := c.QueryArray("status")

	baseWhere := "WHERE deleted_at IS NULL"
	baseArgs := []interface{}{}
	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			baseWhere += " AND year >= ?"
			baseArgs = append(baseArgs, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			baseWhere += " AND year <= ?"
			baseArgs = append(baseArgs, y)
		}
	}
	if len(months) > 0 {
		placeholders := make([]string, 0, len(months))
		for _, m := range months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				baseArgs = append(baseArgs, val)
			}
		}
		if len(placeholders) > 0 {
			baseWhere += " AND month IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	addFilter := func(col string, values []string) (string, []interface{}) {
		if len(values) == 0 {
			return "", nil
		}
		placeholders := make([]string, 0, len(values))
		newArgs := []interface{}{}
		for _, v := range values {
			if v != "" {
				placeholders = append(placeholders, "?")
				newArgs = append(newArgs, v)
			}
		}
		if len(placeholders) > 0 {
			return " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")", newArgs
		}
		return "", nil
	}

	mainFilters := map[string][]string{
		"kam": kams, "brand_as": brands, "sku": skus,
		"network_name": networks, "mechanics": mechanics, "status": statuses,
	}

	getDistinctMain := func(col string, excludeFilter string) []string {
		where := baseWhere
		args := make([]interface{}, len(baseArgs))
		copy(args, baseArgs)
		for filterCol, values := range mainFilters {
			if filterCol != excludeFilter {
				cond, newArgs := addFilter(filterCol, values)
				if cond != "" {
					where += cond
					args = append(args, newArgs...)
				}
			}
		}
		query := "SELECT DISTINCT " + col + " FROM dbo.tbl_PromoActivities " + where + " AND " + col + " IS NOT NULL ORDER BY " + col
		rows, e := config.DB.Query(query, args...)
		if e != nil {
			return []string{}
		}
		defer rows.Close()
		var vals []string
		for rows.Next() {
			var v sql.NullString
			if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
				vals = append(vals, v.String)
			}
		}
		return vals
	}

	getDistinctChannel := func() []string {
		where := baseWhere
		args := make([]interface{}, len(baseArgs))
		copy(args, baseArgs)
		for filterCol, values := range mainFilters {
			cond, newArgs := addFilter("p."+filterCol, values)
			if cond != "" {
				where += cond
				args = append(args, newArgs...)
			}
		}
		query := "SELECT DISTINCT m.channel FROM dbo.tbl_PromoActivities p LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON p.mechanics = m.mechanics " + where + " AND m.channel IS NOT NULL ORDER BY m.channel"
		rows, e := config.DB.Query(query, args...)
		if e != nil {
			return []string{}
		}
		defer rows.Close()
		var vals []string
		for rows.Next() {
			var v sql.NullString
			if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
				vals = append(vals, v.String)
			}
		}
		return vals
	}

	c.JSON(http.StatusOK, gin.H{
		"kam":          getDistinctMain("kam", "kam"),
		"brand":        getDistinctMain("brand_as", "brand_as"),
		"sku":          getDistinctMain("sku", "sku"),
		"network_name": getDistinctMain("network_name", "network_name"),
		"mechanics":    getDistinctMain("mechanics", "mechanics"),
		"status":       getDistinctMain("status", "status"),
		"channel":      getDistinctChannel(),
	})
}

func GetPromoData(c *gin.Context) {
	yearFromStr := c.Query("yearFrom")
	yearToStr := c.Query("yearTo")
	months := c.QueryArray("months")
	kams := c.QueryArray("kam")
	brands := c.QueryArray("brand")
	skus := c.QueryArray("sku")
	networks := c.QueryArray("network_name")
	mechanics := c.QueryArray("mechanics")
	statuses := c.QueryArray("status")
	channels := c.QueryArray("channel")

	query := `SELECT p.id, p.network_name, p.kam, p.id_directum, p.ds_number, p.year, p.month, p.quarter, p.sku, p.brand, p.brand_as, p.mechanics, p.discount_amount, p.gtn_opex, p.conditions, p.comments, p.total_pharmacies, p.promo_pharmacies, p.baseline_units, p.baseline_rub, p.plan_promo_units, p.plan_promo_rub, p.plan_investments_rub, p.plan_promo_uplift_units, p.plan_promo_uplift_rub, p.plan_promo_uplift_pct_units, p.plan_promo_uplift_pct_rub, p.plan_investments_pct, p.plan_roi, p.contract_price, p.gm, p.actual_promo_sales_units, p.actual_investments, p.status, p.actual_promo_rub, p.actual_promo_uplift_units, p.actual_promo_uplift_rub, p.actual_external_ecom_units, p.actual_corrected_baseline, p.actual_roi, p.plan_vs_fact_rub, p.plan_vs_fact_investments, p.agreement1, p.agreement2, p.date, p.created_at, p.updated_at, m.channel FROM dbo.tbl_PromoActivities p LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON p.mechanics = m.mechanics WHERE p.deleted_at IS NULL`
	args := []interface{}{}

	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			query += " AND p.year >= ?"
			args = append(args, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			query += " AND p.year <= ?"
			args = append(args, y)
		}
	}
	if len(months) > 0 {
		placeholders := make([]string, 0, len(months))
		for _, m := range months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			query += " AND p.month IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	appendFilter := func(col string, values []string) {
		if len(values) > 0 {
			placeholders := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					placeholders = append(placeholders, "?")
					args = append(args, v)
				}
			}
			if len(placeholders) > 0 {
				query += " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}
	appendFilter("p.kam", kams)
	appendFilter("p.brand_as", brands)
	appendFilter("p.sku", skus)
	appendFilter("p.network_name", networks)
	appendFilter("p.mechanics", mechanics)
	appendFilter("p.status", statuses)
	appendFilter("m.channel", channels)

	all := c.Query("all")
	if all == "true" {
		query += " ORDER BY p.year DESC, p.month DESC"
	} else {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("limit", "100")))
		if pageSize <= 0 {
			pageSize = 100
		}
		if pageSize > 1000 {
			pageSize = 1000
		}
		offset := page * pageSize
		query += " ORDER BY p.year DESC, p.month DESC OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
		args = append(args, offset, pageSize)
	}

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	defer rows.Close()

	var results []models.PromoRow
	for rows.Next() {
		var r models.PromoRow
		if err := rows.Scan(&r.ID, &r.NetworkName, &r.KAM, &r.IDDirectum, &r.DSNumber, &r.Year, &r.Month, &r.Quarter, &r.SKU, &r.Brand, &r.BrandAS, &r.Mechanics, &r.DiscountAmount, &r.GTNOpex, &r.Conditions, &r.Comments, &r.TotalPharmacies, &r.PromoPharmacies, &r.BaselineUnits, &r.BaselineRub, &r.PlanPromoUnits, &r.PlanPromoRub, &r.PlanInvestmentsRub, &r.PlanPromoUpliftUnits, &r.PlanPromoUpliftRub, &r.PlanPromoUpliftPctUnits, &r.PlanPromoUpliftPctRub, &r.PlanInvestmentsPct, &r.PlanROI, &r.ContractPrice, &r.GM, &r.ActualPromoSalesUnits, &r.ActualInvestments, &r.Status, &r.ActualPromoRub, &r.ActualPromoUpliftUnits, &r.ActualPromoUpliftRub, &r.ActualExternalEcomUnits, &r.ActualCorrectedBaseline, &r.ActualROI, &r.PlanVsFactRub, &r.PlanVsFactInvestments, &r.Agreement1, &r.Agreement2, &r.Date, &r.CreatedAt, &r.UpdatedAt, &r.PromoChannel); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil {
		results = []models.PromoRow{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func GetSKUByBrand(c *gin.Context) {
	brand := c.Query("brand")
	if brand == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	rows, _ := config.DB.Query("SELECT DISTINCT sku FROM dbo.tbl_PromoActivities WHERE brand_as = ? AND sku IS NOT NULL AND deleted_at IS NULL ORDER BY sku", brand)
	if rows == nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	defer rows.Close()
	var skus []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			skus = append(skus, s)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": skus})
}

func GetLastContractPrice(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{"price": nil})
		return
	}
	var price sql.NullFloat64
	config.DB.QueryRow("SELECT TOP 1 contract_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND contract_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC", sku).Scan(&price)
	if !price.Valid {
		c.JSON(http.StatusOK, gin.H{"price": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"price": price.Float64})
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
	rows, _ := config.DB.Query("SELECT DISTINCT kam FROM dbo.tbl_KAMNetworkMapping WHERE network_name = ? AND kam IS NOT NULL ORDER BY kam", network)
	if rows == nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	defer rows.Close()
	var kams []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			kams = append(kams, k)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": kams})
}

func GetLastNetworkData(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	var totalPharmacies sql.NullInt64
	config.DB.QueryRow("SELECT TOP 1 total_pharmacies FROM dbo.tbl_PromoActivities WHERE network_name = ? AND total_pharmacies IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC", network).Scan(&totalPharmacies)
	c.JSON(http.StatusOK, gin.H{"total_pharmacies": totalPharmacies.Int64})
}

func GetNetworkGeoMapping(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "network is required"})
		return
	}

	var kam, networkType, top20Segment, keyRegion sql.NullString
	err := config.DB.QueryRow(
		"SELECT kam, network_type, top20_segment, key_region FROM dbo.tbl_NetworkGeoMapping WHERE network_name = ?",
		network,
	).Scan(&kam, &networkType, &top20Segment, &keyRegion)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"kam": nil, "network_type": nil, "top20_segment": nil, "key_region": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"kam":           kam.String,
		"network_type":  networkType.String,
		"top20_segment": top20Segment.String,
		"key_region":    keyRegion.String,
	})
}

func GetPromoHistoryFiltered(c *gin.Context) {
	sku := c.Query("sku")
	network := c.Query("network_name")
	mechanics := c.Query("mechanics")
	yearFrom := c.Query("yearFrom")
	yearTo := c.Query("yearTo")

	query := "SELECT TOP 10 id, network_name, year, month, mechanics, sku, baseline_units, plan_promo_units, actual_promo_sales_units, plan_promo_uplift_units, actual_promo_uplift_units, plan_roi, actual_roi FROM dbo.tbl_PromoActivities WHERE deleted_at IS NULL"
	args := []interface{}{}
	if sku != "" {
		query += " AND sku = ?"
		args = append(args, sku)
	}
	if network != "" {
		query += " AND network_name = ?"
		args = append(args, network)
	}
	if mechanics != "" {
		query += " AND mechanics = ?"
		args = append(args, mechanics)
	}
	if yearFrom != "" {
		if y, _ := strconv.Atoi(yearFrom); true {
			query += " AND year >= ?"
			args = append(args, y)
		}
	}
	if yearTo != "" {
		if y, _ := strconv.Atoi(yearTo); true {
			query += " AND year <= ?"
			args = append(args, y)
		}
	}
	query += " ORDER BY year DESC, month DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}
	defer rows.Close()

	var results []models.HistoryRow
	for rows.Next() {
		var r models.HistoryRow
		if err := rows.Scan(&r.ID, &r.NetworkName, &r.Year, &r.Month, &r.Mechanics, &r.SKU, &r.BaselineUnits, &r.PlanPromoUnits, &r.ActualPromoSalesUnits, &r.PlanPromoUpliftUnits, &r.ActualPromoUpliftUnits, &r.PlanROI, &r.ActualROI); err != nil {
			continue
		}
		results = append(results, r)
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func GetSKUInfo(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{"brand": nil, "brand_as": nil})
		return
	}
	var brand, brandAs sql.NullString
	config.DB.QueryRow("SELECT brand, brand_as FROM dbo.tbl_SKUMapping WHERE sku = ?", sku).Scan(&brand, &brandAs)
	if !brand.Valid {
		c.JSON(http.StatusOK, gin.H{"brand": nil, "brand_as": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"brand": brand.String, "brand_as": brandAs.String})
}

func GetLastSKUData(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	var contractPrice, gm, olapPrice sql.NullFloat64
	var totalPharmacies sql.NullInt64
	var keyRegion, top20Segment sql.NullString

	err := config.DB.QueryRow(
		"SELECT TOP 1 contract_price, gm, total_pharmacies, key_region, top20_segment, olap_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND contract_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC",
		sku,
	).Scan(&contractPrice, &gm, &totalPharmacies, &keyRegion, &top20Segment, &olapPrice)

	if err != nil && err != sql.ErrNoRows {
		config.Logger.Error("get_last_sku_data_failed", "error", err.Error(), "sku", sku)
	}

	c.JSON(http.StatusOK, gin.H{
		"contract_price":   contractPrice.Float64,
		"gm":               gm.Float64,
		"total_pharmacies": totalPharmacies.Int64,
		"key_region":       keyRegion.String,
		"top20_segment":    top20Segment.String,
		"olap_price":       olapPrice.Float64,
	})
}

// ─── Сохранение (INSERT + UPDATE) ──────────────────────────────────────────

var allPromoFields = []string{
	"network_name", "kam", "brand", "brand_as", "sku",
	"year", "month", "quarter", "mechanics", "gtn_opex",
	"baseline_units", "baseline_rub",
	"plan_promo_units", "plan_promo_rub", "plan_investments_rub",
	"plan_promo_uplift_units", "plan_promo_uplift_rub",
	"plan_promo_uplift_pct_units", "plan_promo_uplift_pct_rub",
	"plan_investments_pct", "plan_roi",
	"contract_price", "gm",
	"id_directum", "ds_number", "discount_amount",
	"conditions", "comments", "ecom_segment",
	"total_pharmacies", "promo_pharmacies",
	"status", "date",
	"key_region", "top20_segment", "olap_price",
	"plan_promo_cip_olap", "fact_promo_cip_olap",
	"plan_promo_uplift_cip_olap", "fact_promo_uplift_cip_olap",
	"actual_promo_sales_units", "actual_investments",
	"actual_promo_rub", "actual_promo_uplift_units", "actual_promo_uplift_rub",
	"actual_external_ecom_units", "actual_corrected_baseline",
	"agreement1", "agreement2",
	"net_promo_uplift_rub", "net_promo_uplift_pct",
	"actual_investments_pct", "actual_roi",
	"actual_promo_rub_wo_ecom", "actual_promo_uplift_units_wo_ecom",
	"actual_promo_uplift_rub_wo_ecom",
	"net_promo_uplift_rub_wo_ecom", "net_promo_uplift_pct_wo_ecom",
	"actual_investments_pct_wo_ecom", "actual_roi_wo_ecom",
	"plan_vs_fact_rub", "plan_vs_fact_investments",
	"turnover_per_point", "turnover_per_point_promo",
	"updated_at",
}

func fetchExistingRow(id int) (map[string]interface{}, error) {
	row := config.DB.QueryRow(
		"SELECT "+strings.Join(allPromoFields, ", ")+" FROM dbo.tbl_PromoActivities WHERE id = ? AND deleted_at IS NULL",
		id,
	)

	existing := make(map[string]interface{})
	dest := make([]interface{}, len(allPromoFields))
	for i := range allPromoFields {
		var v sql.NullString
		dest[i] = &v
	}

	if err := row.Scan(dest...); err != nil {
		return nil, err
	}

	for i, field := range allPromoFields {
		ns := dest[i].(*sql.NullString)
		if ns.Valid {
			if f, err := strconv.ParseFloat(ns.String, 64); err == nil {
				existing[field] = f
			} else if i, err := strconv.Atoi(ns.String); err == nil {
				existing[field] = i
			} else {
				existing[field] = ns.String
			}
		}
	}
	return existing, nil
}

func SavePromo(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ─── UPDATE ────────────────────────────────────────────────────────────
	if id, ok := input["id"]; ok && id != nil {
		idFloat, _ := strconv.ParseFloat(fmt.Sprint(id), 64)
		idInt := int(idFloat)
		if idInt > 0 {
			existing, err := fetchExistingRow(idInt)
			if err != nil {
				config.Logger.Error("promo_update_fetch_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Запись не найдена"})
				return
			}

			for k, v := range input {
				if k != "id" && k != "deleted_at" && k != "updated_at" {
					existing[k] = v
				}
			}

			calculatePromoFields(existing)

			oldUpdatedAt := input["updated_at"]

			setClauses := []string{}
			values := []interface{}{}
			for _, field := range allPromoFields {
				if field == "updated_at" {
					// пропускаем — задаётся через GETDATE()
					continue
				}
				if val, ok := existing[field]; ok {
					setClauses = append(setClauses, field+" = ?")
					values = append(values, val)
				}
			}
			setClauses = append(setClauses, "updated_at = GETDATE()")
			values = append(values, idInt, oldUpdatedAt)

			result, err := config.DB.Exec(
				"UPDATE dbo.tbl_PromoActivities SET "+strings.Join(setClauses, ", ")+" WHERE id = ? AND updated_at = ?",
				values...,
			)
			if err != nil {
				config.Logger.Error("promo_update_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				config.Logger.Info("promo_update_conflict", "id", idInt)
				c.JSON(http.StatusConflict, gin.H{"error": "Запись изменена другим пользователем. Обновите страницу."})
				return
			}

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

	// ─── INSERT ────────────────────────────────────────────────────────────
	calculatePromoFields(input)
	delete(input, "id")

	placeholders := make([]string, len(allPromoFields))
	values := make([]interface{}, len(allPromoFields))
	for i, f := range allPromoFields {
		placeholders[i] = "?"
		if val, ok := input[f]; ok {
			values[i] = val
		} else {
			values[i] = nil
		}
	}

	var newID int64
	err := config.DB.QueryRow(
		fmt.Sprintf("INSERT INTO dbo.tbl_PromoActivities (%s) OUTPUT INSERTED.id VALUES (%s)",
			strings.Join(allPromoFields, ", "),
			strings.Join(placeholders, ", ")),
		values...,
	).Scan(&newID)
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

	result, err := config.DB.Exec("UPDATE dbo.tbl_PromoActivities SET deleted_at = GETDATE(), updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL", id)
	if err != nil {
		config.Logger.Error("promo_delete_failed", "id", id, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена или уже удалена"})
		return
	}

	config.Logger.Info("promo_deleted", "id", id, "user", "system", "timestamp", time.Now().Format(time.RFC3339))
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ─── Согласование ──────────────────────────────────────────────────────────

// GetApprovals возвращает список промо, ожидающих согласования.
// agreement1 IS NULL — для роли agreement1, agreement2 IS NULL — для agreement2.
// Фильтры: kam, network_name, brand, year, month (клиентские, бэкенд не фильтрует).
func GetApprovals(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	agreementField := "p.agreement1"
	if roleStr == "agreement2" {
		agreementField = "p.agreement2"
	}

	kam := c.Query("kam")
	approvalStatus := c.DefaultQuery("approval_status", "pending")

	// Только промо с текущего месяца и далее (не исторические)
	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	query := fmt.Sprintf(`
		SELECT TOP 500
			p.id, p.network_name, p.sku, p.mechanics, p.year, p.month,
			p.baseline_units, p.plan_promo_units, p.actual_promo_sales_units,
			p.plan_investments_rub, p.plan_roi, p.actual_roi,
			p.conditions, p.agreement1, p.agreement2, p.status,
			0 as historical_count,
			CAST(NULL AS FLOAT) as avg_historical_roi
		FROM dbo.tbl_PromoActivities p
		WHERE p.deleted_at IS NULL
		  AND (p.year > ? OR (p.year = ? AND p.month >= ?))
	`, agreementField)

	args := []interface{}{currentYear, currentYear, currentMonth}

	// Фильтр по состоянию согласования
	switch approvalStatus {
	case "pending":
		query += fmt.Sprintf(" AND %s IS NULL", agreementField)
	case "commented":
		query += fmt.Sprintf(" AND %s IS NOT NULL AND %s NOT LIKE 'согласовано%%' AND %s NOT LIKE 'отклонено%%'", agreementField, agreementField, agreementField)
	case "approved":
		query += fmt.Sprintf(" AND %s LIKE 'согласовано%%'", agreementField)
	case "rejected":
		query += fmt.Sprintf(" AND %s LIKE 'отклонено%%'", agreementField)
		// "all" — без дополнительного фильтра
	}

	if kam != "" {
		query += " AND p.kam = ?"
		args = append(args, kam)
	}

	query += " ORDER BY p.year DESC, p.month DESC, p.network_name"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	defer rows.Close()

	var results []models.ApprovalRow
	for rows.Next() {
		var r models.ApprovalRow
		if err := rows.Scan(
			&r.ID, &r.NetworkName, &r.SKU, &r.Mechanics, &r.Year, &r.Month,
			&r.BaselineUnits, &r.PlanPromoUnits, &r.ActualPromoSalesUnits,
			&r.PlanInvestmentsRub, &r.PlanROI, &r.ActualROI,
			&r.Conditions, &r.Agreement1, &r.Agreement2, &r.Status,
			&r.HistoricalCount, &r.AvgHistoricalROI,
		); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil {
		results = []models.ApprovalRow{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

// ApprovePromo — три режима:
//
//	status="comment"      → только комментарий в agreement поле
//	status="согласовано"  → "согласовано" + опционально ": комментарий"
//	status="отклонено"    → "отклонено"   + опционально ": комментарий"
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

	_, err := config.DB.Exec(
		fmt.Sprintf("UPDATE dbo.tbl_PromoActivities SET %s = ?, updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL", field),
		value, req.ID,
	)
	if err != nil {
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

// GetApprovalKAMs возвращает список KAM'ов, у которых есть промо на согласовании.
func GetApprovalKAMs(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	field := "agreement1"
	if roleStr == "agreement2" {
		field = "agreement2"
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT p.kam 
		FROM dbo.tbl_PromoActivities p 
		WHERE p.deleted_at IS NULL AND %s IS NULL AND p.kam IS NOT NULL
		ORDER BY p.kam
	`, field)

	rows, err := config.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	defer rows.Close()

	var kams []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			kams = append(kams, k)
		}
	}
	if kams == nil {
		kams = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": kams})
}

// GetApprovalNetworks возвращает сети для выбранного KAM на согласовании.
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

	query := fmt.Sprintf(`
		SELECT DISTINCT p.network_name 
		FROM dbo.tbl_PromoActivities p 
		WHERE p.deleted_at IS NULL 
		  AND %s IS NULL 
		  AND p.kam = ? 
		  AND p.network_name IS NOT NULL
		ORDER BY p.network_name
	`, field)

	rows, err := config.DB.Query(query, kam)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	defer rows.Close()

	var networks []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			networks = append(networks, n)
		}
	}
	if networks == nil {
		networks = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": networks})
}

// GetApprovalBrands возвращает бренды для выбранного KAM и сети на согласовании.
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

	query := fmt.Sprintf(`
		SELECT DISTINCT p.brand_as 
		FROM dbo.tbl_PromoActivities p 
		WHERE p.deleted_at IS NULL 
		  AND %s IS NULL 
		  AND p.kam = ? 
		  AND p.brand_as IS NOT NULL
	`, field)
	args := []interface{}{kam}

	if network != "" {
		query += " AND p.network_name = ?"
		args = append(args, network)
	}

	query += " ORDER BY p.brand_as"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	defer rows.Close()

	var brands []string
	for rows.Next() {
		var b string
		if rows.Scan(&b) == nil {
			brands = append(brands, b)
		}
	}
	if brands == nil {
		brands = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": brands})
}
