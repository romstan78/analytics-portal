package repository

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"
)

// ─── Filters ────────────────────────────────────────────────────────────────

type PromoFilterParams struct {
	YearFromStr, YearToStr                            string
	Months                                            []string
	Kams, Brands, SKUs, Networks, Mechanics, Statuses []string
}

func BuildBaseWhere(params PromoFilterParams) (string, []interface{}) {
	where := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	if params.YearFromStr != "" {
		if y, _ := strconv.Atoi(params.YearFromStr); true {
			where += " AND year >= ?"
			args = append(args, y)
		}
	}
	if params.YearToStr != "" {
		if y, _ := strconv.Atoi(params.YearToStr); true {
			where += " AND year <= ?"
			args = append(args, y)
		}
	}
	if len(params.Months) > 0 {
		placeholders := make([]string, 0, len(params.Months))
		for _, m := range params.Months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			where += " AND month IN (" + strings.Join(placeholders, ",") + ")"
		}
	}
	return where, args
}

func AddFilter(col string, values []string) (string, []interface{}) {
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

func ExecDistinct(query string, args []interface{}) []string {
	rows, err := config.DB.Query(query, args...)
	if err != nil {
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

// GetFilterValues возвращает список уникальных значений для конкретной колонки
func GetFilterValues(col string, baseWhere string, baseArgs []interface{}, excludeCol string, filters map[string][]string) []string {
	where := baseWhere
	args := make([]interface{}, len(baseArgs))
	copy(args, baseArgs)
	for filterCol, values := range filters {
		if filterCol != excludeCol {
			cond, newArgs := AddFilter(filterCol, values)
			if cond != "" {
				where += cond
				args = append(args, newArgs...)
			}
		}
	}
	query := "SELECT DISTINCT " + col + " FROM dbo.tbl_PromoActivities " + where + " AND " + col + " IS NOT NULL ORDER BY " + col
	return ExecDistinct(query, args)
}

// GetChannelFilterValues — специальный запрос для канала через JOIN
func GetChannelFilterValues(baseWhere string, baseArgs []interface{}, filters map[string][]string) []string {
	where := baseWhere
	args := make([]interface{}, len(baseArgs))
	copy(args, baseArgs)
	for filterCol, values := range filters {
		cond, newArgs := AddFilter("p."+filterCol, values)
		if cond != "" {
			where += cond
			args = append(args, newArgs...)
		}
	}
	query := "SELECT DISTINCT m.channel FROM dbo.tbl_PromoActivities p LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON p.mechanics = m.mechanics " + where + " AND m.channel IS NOT NULL ORDER BY m.channel"
	return ExecDistinct(query, args)
}

// ─── Promo Data ─────────────────────────────────────────────────────────────

func GetPromoRows(params PromoFilterParams, channels []string, page, pageSize int, getAll bool) ([]models.PromoRow, error) {
	query := `SELECT p.id, p.network_name, p.kam, p.id_directum, p.ds_number, p.year, p.month, p.quarter, p.sku, p.brand, p.brand_as, p.mechanics, p.discount_amount, p.gtn_opex, p.conditions, p.comments, p.total_pharmacies, p.promo_pharmacies, p.baseline_units, p.baseline_rub, p.plan_promo_units, p.plan_promo_rub, p.plan_investments_rub, p.plan_promo_uplift_units, p.plan_promo_uplift_rub, p.plan_promo_uplift_pct_units, p.plan_promo_uplift_pct_rub, p.plan_investments_pct, p.plan_roi, p.contract_price, p.gm, p.actual_promo_sales_units, p.actual_investments, p.status, p.actual_promo_rub, p.actual_promo_uplift_units, p.actual_promo_uplift_rub, p.actual_external_ecom_units, p.actual_corrected_baseline, p.actual_roi, p.plan_vs_fact_rub, p.plan_vs_fact_investments, p.agreement1, p.agreement2, p.date, p.created_at, p.updated_at, m.channel FROM dbo.tbl_PromoActivities p LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON p.mechanics = m.mechanics WHERE p.deleted_at IS NULL`
	args := []interface{}{}

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

	if params.YearFromStr != "" {
		if y, _ := strconv.Atoi(params.YearFromStr); true {
			query += " AND p.year >= ?"
			args = append(args, y)
		}
	}
	if params.YearToStr != "" {
		if y, _ := strconv.Atoi(params.YearToStr); true {
			query += " AND p.year <= ?"
			args = append(args, y)
		}
	}
	if len(params.Months) > 0 {
		placeholders := make([]string, 0, len(params.Months))
		for _, m := range params.Months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			query += " AND p.month IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	appendFilter("p.kam", params.Kams)
	appendFilter("p.brand_as", params.Brands)
	appendFilter("p.sku", params.SKUs)
	appendFilter("p.network_name", params.Networks)
	appendFilter("p.mechanics", params.Mechanics)
	appendFilter("p.status", params.Statuses)
	appendFilter("m.channel", channels)

	if getAll {
		query += " ORDER BY p.year DESC, p.month DESC"
	} else {
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
		return nil, err
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
	return results, nil
}

// ─── SKU / Lookups ──────────────────────────────────────────────────────────

func GetSKUsByBrand(brand string) ([]string, error) {
	rows, err := config.DB.Query("SELECT DISTINCT sku FROM dbo.tbl_PromoActivities WHERE brand_as = ? AND sku IS NOT NULL AND deleted_at IS NULL ORDER BY sku", brand)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skus []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			skus = append(skus, s)
		}
	}
	return skus, nil
}

func GetLastContractPrice(sku string) (*float64, error) {
	var price sql.NullFloat64
	err := config.DB.QueryRow("SELECT TOP 1 contract_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND contract_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC", sku).Scan(&price)
	if err != nil {
		return nil, err
	}
	if price.Valid {
		return &price.Float64, nil
	}
	return nil, nil
}

func GetKAMsByNetwork(network string) ([]string, error) {
	rows, err := config.DB.Query("SELECT DISTINCT kam FROM dbo.tbl_KAMNetworkMapping WHERE network_name = ? AND kam IS NOT NULL ORDER BY kam", network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var kams []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			kams = append(kams, k)
		}
	}
	return kams, nil
}

func GetLastNetworkData(network string) (*int64, error) {
	var totalPharmacies sql.NullInt64
	err := config.DB.QueryRow("SELECT TOP 1 total_pharmacies FROM dbo.tbl_PromoActivities WHERE network_name = ? AND total_pharmacies IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC", network).Scan(&totalPharmacies)
	if err != nil {
		return nil, err
	}
	if totalPharmacies.Valid {
		return &totalPharmacies.Int64, nil
	}
	return nil, nil
}

func GetNetworkGeoMapping(network string) (*models.NetworkGeo, error) {
	var kam, networkType, top20Segment, keyRegion sql.NullString
	err := config.DB.QueryRow(
		"SELECT kam, network_type, top20_segment, key_region FROM dbo.tbl_NetworkGeoMapping WHERE network_name = ?",
		network,
	).Scan(&kam, &networkType, &top20Segment, &keyRegion)
	if err != nil {
		return nil, err
	}
	return &models.NetworkGeo{
		KAM:          kam.String,
		NetworkType:  networkType.String,
		Top20Segment: top20Segment.String,
		KeyRegion:    keyRegion.String,
	}, nil
}

func GetSKUInfo(sku string) (brand, brandAs string, found bool) {
	var b, ba sql.NullString
	err := config.DB.QueryRow("SELECT brand, brand_as FROM dbo.tbl_SKUMapping WHERE sku = ?", sku).Scan(&b, &ba)
	if err != nil {
		return "", "", false
	}
	return b.String, ba.String, true
}

func GetLastSKUData(sku string) (*models.LastSKUData, error) {
	var contractPrice, gm, olapPrice sql.NullFloat64
	var totalPharmacies sql.NullInt64
	var keyRegion, top20Segment sql.NullString

	err := config.DB.QueryRow(
		"SELECT TOP 1 contract_price, gm, total_pharmacies, key_region, top20_segment, olap_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND contract_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC",
		sku,
	).Scan(&contractPrice, &gm, &totalPharmacies, &keyRegion, &top20Segment, &olapPrice)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return &models.LastSKUData{
		ContractPrice:   contractPrice.Float64,
		GM:              gm.Float64,
		TotalPharmacies: totalPharmacies.Int64,
		KeyRegion:       keyRegion.String,
		Top20Segment:    top20Segment.String,
		OlapPrice:       olapPrice.Float64,
	}, nil
}

// ─── History ────────────────────────────────────────────────────────────────

func GetPromoHistory(sku, network, mechanics, yearFrom, yearTo string) ([]models.HistoryRow, error) {
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
		return nil, err
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
	return results, nil
}

// ─── Save / Delete ──────────────────────────────────────────────────────────

func FetchExistingRow(id int) (map[string]interface{}, error) {
	allPromoFields := []string{
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

// UpdatePromo возвращает rowsAffected (0 = конфликт версий)
func UpdatePromo(id int, existing map[string]interface{}, updatedAt string) (int64, error) {
	allPromoFields := []string{
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

	setClauses := []string{}
	values := []interface{}{}
	for _, field := range allPromoFields {
		if field == "updated_at" {
			continue
		}
		if val, ok := existing[field]; ok {
			setClauses = append(setClauses, field+" = ?")
			values = append(values, val)
		}
	}
	setClauses = append(setClauses, "updated_at = GETDATE()")
	values = append(values, id)

	query := "UPDATE dbo.tbl_PromoActivities SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	if updatedAt != "" {
		query += " AND updated_at = ?"
		values = append(values, updatedAt)
	}

	result, err := config.DB.Exec(query, values...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func InsertPromo(input map[string]interface{}) (int64, error) {
	allPromoFields := []string{
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
	return newID, err
}

func SoftDeletePromo(id int) (int64, error) {
	result, err := config.DB.Exec("UPDATE dbo.tbl_PromoActivities SET deleted_at = GETDATE(), updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL", id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ─── Approvals ──────────────────────────────────────────────────────────────

type ApprovalParams struct {
	Role              string
	KAM               string
	ApprovalStatus    string
	YearStr, MonthStr string
}

func GetApprovals(params ApprovalParams) ([]models.ApprovalRow, error) {
	agreementField := "p.agreement1"
	if params.Role == "agreement2" {
		agreementField = "p.agreement2"
	}

	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	query := `
		SELECT TOP 500
			p.id, p.network_name, p.brand_as, p.sku, p.mechanics, p.year, p.month,
			p.baseline_units, p.plan_promo_units, p.actual_promo_sales_units,
			p.plan_investments_rub, p.plan_roi, p.actual_roi,
			p.conditions, p.agreement1, p.agreement2, p.status,
			0 as historical_count,
			CAST(NULL AS FLOAT) as avg_historical_roi
		FROM dbo.tbl_PromoActivities p
		WHERE p.deleted_at IS NULL
	`

	args := []interface{}{}

	if params.YearStr != "" {
		y, _ := strconv.Atoi(params.YearStr)
		query += " AND p.year = ?"
		args = append(args, y)
	} else if params.MonthStr != "" {
		query += " AND p.year >= ?"
		args = append(args, currentYear)
	} else {
		query += " AND (p.year > ? OR (p.year = ? AND p.month >= ?))"
		args = append(args, currentYear, currentYear, currentMonth)
	}

	if params.MonthStr != "" {
		m, _ := strconv.Atoi(params.MonthStr)
		query += " AND p.month = ?"
		args = append(args, m)
	}

	if params.KAM != "" {
		query += " AND p.kam = ?"
		args = append(args, params.KAM)
	}

	switch params.ApprovalStatus {
	case "pending":
		query += fmt.Sprintf(" AND %s IS NULL", agreementField)
	case "commented":
		query += fmt.Sprintf(" AND %s IS NOT NULL AND CHARINDEX(N'согласовано', %s) <> 1 AND CHARINDEX(N'отклонено', %s) <> 1",
			agreementField, agreementField, agreementField)
	case "approved":
		query += fmt.Sprintf(" AND %s IS NOT NULL AND CHARINDEX(N'согласовано', %s) = 1", agreementField, agreementField)
	case "rejected":
		query += fmt.Sprintf(" AND %s IS NOT NULL AND CHARINDEX(N'отклонено', %s) = 1", agreementField, agreementField)
	}

	query += " ORDER BY p.year DESC, p.month DESC, p.network_name"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ApprovalRow
	for rows.Next() {
		var r models.ApprovalRow
		if err := rows.Scan(
			&r.ID, &r.NetworkName, &r.BrandAS, &r.SKU, &r.Mechanics, &r.Year, &r.Month,
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
	return results, nil
}

func ApprovePromo(field string, id int, value string) error {
	_, err := config.DB.Exec(
		fmt.Sprintf("UPDATE dbo.tbl_PromoActivities SET %s = ?, updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL", field),
		value, id,
	)
	return err
}

// ─── Approval Filters ───────────────────────────────────────────────────────

type ApprovalFilterParams struct {
	ApprovalStatus, KAM, Network, Brand, MechFilter, YearStr, MonthStr string
}

func GetApprovalFilters(params ApprovalFilterParams) (networks, brands, mechanics, kams []string, err error) {
	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	query := `
		SELECT DISTINCT p.network_name, p.brand_as, p.mechanics, p.kam
		FROM dbo.tbl_PromoActivities p
		WHERE p.deleted_at IS NULL
	`
	args := []interface{}{}

	if params.YearStr != "" {
		y, _ := strconv.Atoi(params.YearStr)
		query += " AND p.year = ?"
		args = append(args, y)
	} else {
		query += " AND (p.year > ? OR (p.year = ? AND p.month >= ?))"
		args = append(args, currentYear, currentYear, currentMonth)
	}

	if params.MonthStr != "" {
		m, _ := strconv.Atoi(params.MonthStr)
		query += " AND p.month = ?"
		args = append(args, m)
	}

	if params.KAM != "" {
		query += " AND p.kam = ?"
		args = append(args, params.KAM)
	}

	if params.Network != "" {
		query += " AND p.network_name = ?"
		args = append(args, params.Network)
	}

	if params.Brand != "" {
		query += " AND p.brand_as = ?"
		args = append(args, params.Brand)
	}

	if params.MechFilter != "" {
		query += " AND p.mechanics = ?"
		args = append(args, params.MechFilter)
	}

	switch params.ApprovalStatus {
	case "pending":
		query += " AND (p.agreement1 IS NULL OR p.agreement2 IS NULL)"
	case "commented":
		query += " AND ((p.agreement1 IS NOT NULL AND CHARINDEX(N'согласовано', p.agreement1) <> 1 AND CHARINDEX(N'отклонено', p.agreement1) <> 1) OR (p.agreement2 IS NOT NULL AND CHARINDEX(N'согласовано', p.agreement2) <> 1 AND CHARINDEX(N'отклонено', p.agreement2) <> 1))"
	case "approved":
		query += " AND (CHARINDEX(N'согласовано', p.agreement1) = 1 OR CHARINDEX(N'согласовано', p.agreement2) = 1)"
	case "rejected":
		query += " AND (CHARINDEX(N'отклонено', p.agreement1) = 1 OR CHARINDEX(N'отклонено', p.agreement2) = 1)"
	}

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer rows.Close()

	networkSet := make(map[string]bool)
	brandSet := make(map[string]bool)
	mechSet := make(map[string]bool)
	kamSet := make(map[string]bool)

	for rows.Next() {
		var nw, br, mech, k sql.NullString
		if rows.Scan(&nw, &br, &mech, &k) == nil {
			if nw.Valid {
				networkSet[nw.String] = true
			}
			if br.Valid {
				brandSet[br.String] = true
			}
			if mech.Valid {
				mechSet[mech.String] = true
			}
			if k.Valid {
				kamSet[k.String] = true
			}
		}
	}

	for v := range networkSet {
		networks = append(networks, v)
	}
	for v := range brandSet {
		brands = append(brands, v)
	}
	for v := range mechSet {
		mechanics = append(mechanics, v)
	}
	for v := range kamSet {
		kams = append(kams, v)
	}

	sort.Strings(networks)
	sort.Strings(brands)
	sort.Strings(mechanics)
	sort.Strings(kams)

	return networks, brands, mechanics, kams, nil
}

func GetApprovalKAMs(field string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT p.kam 
		FROM dbo.tbl_PromoActivities p 
		WHERE p.deleted_at IS NULL AND %s IS NULL AND p.kam IS NOT NULL
		ORDER BY p.kam
	`, field)

	rows, err := config.DB.Query(query)
	if err != nil {
		return nil, err
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
	return kams, nil
}

func GetApprovalNetworks(field, kam string) ([]string, error) {
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
		return nil, err
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
	return networks, nil
}

func GetApprovalBrands(field, kam, network string) ([]string, error) {
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
		return nil, err
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
	return brands, nil
}
