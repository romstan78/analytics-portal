package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"

	"golang.org/x/sync/errgroup"
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

func FetchExistingRow(id int) (*models.PromoRowDB, error) {
	row := config.DB.QueryRow(
		"SELECT network_name, kam, brand, brand_as, sku, "+
			"year, month, quarter, mechanics, gtn_opex, "+
			"baseline_units, baseline_rub, "+
			"plan_promo_units, plan_promo_rub, plan_investments_rub, "+
			"plan_promo_uplift_units, plan_promo_uplift_rub, "+
			"plan_promo_uplift_pct_units, plan_promo_uplift_pct_rub, "+
			"plan_investments_pct, plan_roi, "+
			"contract_price, gm, "+
			"id_directum, ds_number, discount_amount, "+
			"conditions, comments, ecom_segment, "+
			"total_pharmacies, promo_pharmacies, "+
			"status, date, "+
			"key_region, top20_segment, olap_price, "+
			"plan_promo_cip_olap, fact_promo_cip_olap, "+
			"plan_promo_uplift_cip_olap, fact_promo_uplift_cip_olap, "+
			"actual_promo_sales_units, actual_investments, "+
			"actual_promo_rub, actual_promo_uplift_units, actual_promo_uplift_rub, "+
			"actual_external_ecom_units, actual_corrected_baseline, "+
			"agreement1, agreement2, "+
			"net_promo_uplift_rub, net_promo_uplift_pct, "+
			"actual_investments_pct, actual_roi, "+
			"actual_promo_rub_wo_ecom, actual_promo_uplift_units_wo_ecom, "+
			"actual_promo_uplift_rub_wo_ecom, "+
			"net_promo_uplift_rub_wo_ecom, net_promo_uplift_pct_wo_ecom, "+
			"actual_investments_pct_wo_ecom, actual_roi_wo_ecom, "+
			"plan_vs_fact_rub, plan_vs_fact_investments, "+
			"turnover_per_point, turnover_per_point_promo, "+
			"updated_at "+
			"FROM dbo.tbl_PromoActivities WHERE id = ? AND deleted_at IS NULL",
		id,
	)

	var r models.PromoRowDB
	var nsNetworkName, nsKAM, nsBrand, nsBrandAS, nsSKU sql.NullString
	var nsMechanics, nsGTNOpex sql.NullString
	var nsIDDirectum, nsDSNumber, nsConditions, nsComments, nsEcomSegment, nsStatus, nsDate sql.NullString
	var nsKeyRegion, nsTop20Segment sql.NullString
	var nsAgreement1, nsAgreement2 sql.NullString
	var nsUpdatedAt sql.NullString
	var nQuarter, nTotalPharmacies, nPromoPharmacies sql.NullInt64
	var nBaselineUnits, nBaselineRub sql.NullFloat64
	var nPlanPromoUnits, nPlanPromoRub, nPlanInvestmentsRub sql.NullFloat64
	var nPlanPromoUpliftUnits, nPlanPromoUpliftRub sql.NullFloat64
	var nPlanPromoUpliftPctUnits, nPlanPromoUpliftPctRub, nPlanInvestmentsPct, nPlanROI sql.NullFloat64
	var nContractPrice, nGM, nDiscountAmount sql.NullFloat64
	var nOlapPrice, nPlanPromoCipOlap, nFactPromoCipOlap, nPlanPromoUpliftCipOlap, nFactPromoUpliftCipOlap sql.NullFloat64
	var nActualPromoSalesUnits, nActualInvestments, nActualPromoRub, nActualPromoUpliftUnits, nActualPromoUpliftRub sql.NullFloat64
	var nActualExternalEcomUnits, nActualCorrectedBaseline sql.NullFloat64
	var nNetPromoUpliftRub, nNetPromoUpliftPct, nActualInvestmentsPct, nActualROI sql.NullFloat64
	var nActualPromoRubWoEcom, nActualPromoUpliftUnitsWoEcom, nActualPromoUpliftRubWoEcom sql.NullFloat64
	var nNetPromoUpliftRubWoEcom, nNetPromoUpliftPctWoEcom, nActualInvestmentsPctWoEcom, nActualROIWoEcom sql.NullFloat64
	var nPlanVsFactRub, nPlanVsFactInvestments, nTurnoverPerPoint, nTurnoverPerPointPromo sql.NullFloat64

	err := row.Scan(
		&nsNetworkName, &nsKAM, &nsBrand, &nsBrandAS, &nsSKU,
		&r.Year, &r.Month, &nQuarter, &nsMechanics, &nsGTNOpex,
		&nBaselineUnits, &nBaselineRub,
		&nPlanPromoUnits, &nPlanPromoRub, &nPlanInvestmentsRub,
		&nPlanPromoUpliftUnits, &nPlanPromoUpliftRub,
		&nPlanPromoUpliftPctUnits, &nPlanPromoUpliftPctRub,
		&nPlanInvestmentsPct, &nPlanROI,
		&nContractPrice, &nGM,
		&nsIDDirectum, &nsDSNumber, &nDiscountAmount,
		&nsConditions, &nsComments, &nsEcomSegment,
		&nTotalPharmacies, &nPromoPharmacies,
		&nsStatus, &nsDate,
		&nsKeyRegion, &nsTop20Segment, &nOlapPrice,
		&nPlanPromoCipOlap, &nFactPromoCipOlap,
		&nPlanPromoUpliftCipOlap, &nFactPromoUpliftCipOlap,
		&nActualPromoSalesUnits, &nActualInvestments,
		&nActualPromoRub, &nActualPromoUpliftUnits, &nActualPromoUpliftRub,
		&nActualExternalEcomUnits, &nActualCorrectedBaseline,
		&nsAgreement1, &nsAgreement2,
		&nNetPromoUpliftRub, &nNetPromoUpliftPct,
		&nActualInvestmentsPct, &nActualROI,
		&nActualPromoRubWoEcom, &nActualPromoUpliftUnitsWoEcom,
		&nActualPromoUpliftRubWoEcom,
		&nNetPromoUpliftRubWoEcom, &nNetPromoUpliftPctWoEcom,
		&nActualInvestmentsPctWoEcom, &nActualROIWoEcom,
		&nPlanVsFactRub, &nPlanVsFactInvestments,
		&nTurnoverPerPoint, &nTurnoverPerPointPromo,
		&nsUpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	r.ID = id
	r.NetworkName = nsNetworkName.String
	r.KAM = nsKAM.String
	r.Brand = nsBrand.String
	r.BrandAS = nsBrandAS.String
	r.SKU = nsSKU.String
	if nQuarter.Valid {
		v := int(nQuarter.Int64)
		r.Quarter = &v
	}
	r.Mechanics = nsMechanics.String
	r.GTNOpex = nsGTNOpex.String
	if nBaselineUnits.Valid {
		v := nBaselineUnits.Float64
		r.BaselineUnits = &v
	}
	if nBaselineRub.Valid {
		v := nBaselineRub.Float64
		r.BaselineRub = &v
	}
	if nPlanPromoUnits.Valid {
		v := nPlanPromoUnits.Float64
		r.PlanPromoUnits = &v
	}
	if nPlanPromoRub.Valid {
		v := nPlanPromoRub.Float64
		r.PlanPromoRub = &v
	}
	if nPlanInvestmentsRub.Valid {
		v := nPlanInvestmentsRub.Float64
		r.PlanInvestmentsRub = &v
	}
	if nPlanPromoUpliftUnits.Valid {
		v := nPlanPromoUpliftUnits.Float64
		r.PlanPromoUpliftUnits = &v
	}
	if nPlanPromoUpliftRub.Valid {
		v := nPlanPromoUpliftRub.Float64
		r.PlanPromoUpliftRub = &v
	}
	if nPlanPromoUpliftPctUnits.Valid {
		v := nPlanPromoUpliftPctUnits.Float64
		r.PlanPromoUpliftPctUnits = &v
	}
	if nPlanPromoUpliftPctRub.Valid {
		v := nPlanPromoUpliftPctRub.Float64
		r.PlanPromoUpliftPctRub = &v
	}
	if nPlanInvestmentsPct.Valid {
		v := nPlanInvestmentsPct.Float64
		r.PlanInvestmentsPct = &v
	}
	if nPlanROI.Valid {
		v := nPlanROI.Float64
		r.PlanROI = &v
	}
	if nContractPrice.Valid {
		v := nContractPrice.Float64
		r.ContractPrice = &v
	}
	if nGM.Valid {
		v := nGM.Float64
		r.GM = &v
	}
	r.IDDirectum = nsIDDirectum.String
	r.DSNumber = nsDSNumber.String
	if nDiscountAmount.Valid {
		v := nDiscountAmount.Float64
		r.DiscountAmount = &v
	}
	r.Conditions = nsConditions.String
	r.Comments = nsComments.String
	r.EcomSegment = nsEcomSegment.String
	if nTotalPharmacies.Valid {
		v := int(nTotalPharmacies.Int64)
		r.TotalPharmacies = &v
	}
	if nPromoPharmacies.Valid {
		v := int(nPromoPharmacies.Int64)
		r.PromoPharmacies = &v
	}
	r.Status = nsStatus.String
	r.Date = nsDate.String
	r.KeyRegion = nsKeyRegion.String
	r.Top20Segment = nsTop20Segment.String
	if nOlapPrice.Valid {
		v := nOlapPrice.Float64
		r.OlapPrice = &v
	}
	if nPlanPromoCipOlap.Valid {
		v := nPlanPromoCipOlap.Float64
		r.PlanPromoCipOlap = &v
	}
	if nFactPromoCipOlap.Valid {
		v := nFactPromoCipOlap.Float64
		r.FactPromoCipOlap = &v
	}
	if nPlanPromoUpliftCipOlap.Valid {
		v := nPlanPromoUpliftCipOlap.Float64
		r.PlanPromoUpliftCipOlap = &v
	}
	if nFactPromoUpliftCipOlap.Valid {
		v := nFactPromoUpliftCipOlap.Float64
		r.FactPromoUpliftCipOlap = &v
	}
	if nActualPromoSalesUnits.Valid {
		v := nActualPromoSalesUnits.Float64
		r.ActualPromoSalesUnits = &v
	}
	if nActualInvestments.Valid {
		v := nActualInvestments.Float64
		r.ActualInvestments = &v
	}
	if nActualPromoRub.Valid {
		v := nActualPromoRub.Float64
		r.ActualPromoRub = &v
	}
	if nActualPromoUpliftUnits.Valid {
		v := nActualPromoUpliftUnits.Float64
		r.ActualPromoUpliftUnits = &v
	}
	if nActualPromoUpliftRub.Valid {
		v := nActualPromoUpliftRub.Float64
		r.ActualPromoUpliftRub = &v
	}
	if nActualExternalEcomUnits.Valid {
		v := nActualExternalEcomUnits.Float64
		r.ActualExternalEcomUnits = &v
	}
	if nActualCorrectedBaseline.Valid {
		v := nActualCorrectedBaseline.Float64
		r.ActualCorrectedBaseline = &v
	}
	r.Agreement1 = nsAgreement1.String
	r.Agreement2 = nsAgreement2.String
	if nNetPromoUpliftRub.Valid {
		v := nNetPromoUpliftRub.Float64
		r.NetPromoUpliftRub = &v
	}
	if nNetPromoUpliftPct.Valid {
		v := nNetPromoUpliftPct.Float64
		r.NetPromoUpliftPct = &v
	}
	if nActualInvestmentsPct.Valid {
		v := nActualInvestmentsPct.Float64
		r.ActualInvestmentsPct = &v
	}
	if nActualROI.Valid {
		v := nActualROI.Float64
		r.ActualROI = &v
	}
	if nActualPromoRubWoEcom.Valid {
		v := nActualPromoRubWoEcom.Float64
		r.ActualPromoRubWoEcom = &v
	}
	if nActualPromoUpliftUnitsWoEcom.Valid {
		v := nActualPromoUpliftUnitsWoEcom.Float64
		r.ActualPromoUpliftUnitsWoEcom = &v
	}
	if nActualPromoUpliftRubWoEcom.Valid {
		v := nActualPromoUpliftRubWoEcom.Float64
		r.ActualPromoUpliftRubWoEcom = &v
	}
	if nNetPromoUpliftRubWoEcom.Valid {
		v := nNetPromoUpliftRubWoEcom.Float64
		r.NetPromoUpliftRubWoEcom = &v
	}
	if nNetPromoUpliftPctWoEcom.Valid {
		v := nNetPromoUpliftPctWoEcom.Float64
		r.NetPromoUpliftPctWoEcom = &v
	}
	if nActualInvestmentsPctWoEcom.Valid {
		v := nActualInvestmentsPctWoEcom.Float64
		r.ActualInvestmentsPctWoEcom = &v
	}
	if nActualROIWoEcom.Valid {
		v := nActualROIWoEcom.Float64
		r.ActualROIWoEcom = &v
	}
	if nPlanVsFactRub.Valid {
		v := nPlanVsFactRub.Float64
		r.PlanVsFactRub = &v
	}
	if nPlanVsFactInvestments.Valid {
		v := nPlanVsFactInvestments.Float64
		r.PlanVsFactInvestments = &v
	}
	if nTurnoverPerPoint.Valid {
		v := nTurnoverPerPoint.Float64
		r.TurnoverPerPoint = &v
	}
	if nTurnoverPerPointPromo.Valid {
		v := nTurnoverPerPointPromo.Float64
		r.TurnoverPerPointPromo = &v
	}
	r.UpdatedAt = nsUpdatedAt.String

	return &r, nil
}

// UpdatePromo возвращает rowsAffected (0 = конфликт версий)
func UpdatePromo(id int, r *models.PromoRowDB, updatedAt string) (int64, error) {
	query := `UPDATE dbo.tbl_PromoActivities SET 
		network_name = ?, kam = ?, brand = ?, brand_as = ?, sku = ?,
		year = ?, month = ?, quarter = ?, mechanics = ?, gtn_opex = ?,
		baseline_units = ?, baseline_rub = ?,
		plan_promo_units = ?, plan_promo_rub = ?, plan_investments_rub = ?,
		plan_promo_uplift_units = ?, plan_promo_uplift_rub = ?,
		plan_promo_uplift_pct_units = ?, plan_promo_uplift_pct_rub = ?,
		plan_investments_pct = ?, plan_roi = ?,
		contract_price = ?, gm = ?,
		id_directum = ?, ds_number = ?, discount_amount = ?,
		conditions = ?, comments = ?, ecom_segment = ?,
		total_pharmacies = ?, promo_pharmacies = ?,
		status = ?, date = ?,
		key_region = ?, top20_segment = ?, olap_price = ?,
		plan_promo_cip_olap = ?, fact_promo_cip_olap = ?,
		plan_promo_uplift_cip_olap = ?, fact_promo_uplift_cip_olap = ?,
		actual_promo_sales_units = ?, actual_investments = ?,
		actual_promo_rub = ?, actual_promo_uplift_units = ?, actual_promo_uplift_rub = ?,
		actual_external_ecom_units = ?, actual_corrected_baseline = ?,
		agreement1 = ?, agreement2 = ?,
		net_promo_uplift_rub = ?, net_promo_uplift_pct = ?,
		actual_investments_pct = ?, actual_roi = ?,
		actual_promo_rub_wo_ecom = ?, actual_promo_uplift_units_wo_ecom = ?,
		actual_promo_uplift_rub_wo_ecom = ?,
		net_promo_uplift_rub_wo_ecom = ?, net_promo_uplift_pct_wo_ecom = ?,
		actual_investments_pct_wo_ecom = ?, actual_roi_wo_ecom = ?,
		plan_vs_fact_rub = ?, plan_vs_fact_investments = ?,
		turnover_per_point = ?, turnover_per_point_promo = ?,
		updated_at = GETDATE()
		WHERE id = ? AND updated_at = ?`

	values := []interface{}{
		r.NetworkName, r.KAM, r.Brand, r.BrandAS, r.SKU,
		r.Year, r.Month, r.Quarter, r.Mechanics, r.GTNOpex,
		r.BaselineUnits, r.BaselineRub,
		r.PlanPromoUnits, r.PlanPromoRub, r.PlanInvestmentsRub,
		r.PlanPromoUpliftUnits, r.PlanPromoUpliftRub,
		r.PlanPromoUpliftPctUnits, r.PlanPromoUpliftPctRub,
		r.PlanInvestmentsPct, r.PlanROI,
		r.ContractPrice, r.GM,
		r.IDDirectum, r.DSNumber, r.DiscountAmount,
		r.Conditions, r.Comments, r.EcomSegment,
		r.TotalPharmacies, r.PromoPharmacies,
		r.Status, r.Date,
		r.KeyRegion, r.Top20Segment, r.OlapPrice,
		r.PlanPromoCipOlap, r.FactPromoCipOlap,
		r.PlanPromoUpliftCipOlap, r.FactPromoUpliftCipOlap,
		r.ActualPromoSalesUnits, r.ActualInvestments,
		r.ActualPromoRub, r.ActualPromoUpliftUnits, r.ActualPromoUpliftRub,
		r.ActualExternalEcomUnits, r.ActualCorrectedBaseline,
		r.Agreement1, r.Agreement2,
		r.NetPromoUpliftRub, r.NetPromoUpliftPct,
		r.ActualInvestmentsPct, r.ActualROI,
		r.ActualPromoRubWoEcom, r.ActualPromoUpliftUnitsWoEcom,
		r.ActualPromoUpliftRubWoEcom,
		r.NetPromoUpliftRubWoEcom, r.NetPromoUpliftPctWoEcom,
		r.ActualInvestmentsPctWoEcom, r.ActualROIWoEcom,
		r.PlanVsFactRub, r.PlanVsFactInvestments,
		r.TurnoverPerPoint, r.TurnoverPerPointPromo,
		id, updatedAt,
	}

	result, err := config.DB.Exec(query, values...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func InsertPromo(r *models.PromoRowDB) (int64, error) {
	query := `INSERT INTO dbo.tbl_PromoActivities (
		network_name, kam, brand, brand_as, sku,
		year, month, quarter, mechanics, gtn_opex,
		baseline_units, baseline_rub,
		plan_promo_units, plan_promo_rub, plan_investments_rub,
		plan_promo_uplift_units, plan_promo_uplift_rub,
		plan_promo_uplift_pct_units, plan_promo_uplift_pct_rub,
		plan_investments_pct, plan_roi,
		contract_price, gm,
		id_directum, ds_number, discount_amount,
		conditions, comments, ecom_segment,
		total_pharmacies, promo_pharmacies,
		status, date,
		key_region, top20_segment, olap_price,
		plan_promo_cip_olap, fact_promo_cip_olap,
		plan_promo_uplift_cip_olap, fact_promo_uplift_cip_olap,
		actual_promo_sales_units, actual_investments,
		actual_promo_rub, actual_promo_uplift_units, actual_promo_uplift_rub,
		actual_external_ecom_units, actual_corrected_baseline,
		agreement1, agreement2,
		net_promo_uplift_rub, net_promo_uplift_pct,
		actual_investments_pct, actual_roi,
		actual_promo_rub_wo_ecom, actual_promo_uplift_units_wo_ecom,
		actual_promo_uplift_rub_wo_ecom,
		net_promo_uplift_rub_wo_ecom, net_promo_uplift_pct_wo_ecom,
		actual_investments_pct_wo_ecom, actual_roi_wo_ecom,
		plan_vs_fact_rub, plan_vs_fact_investments,
		turnover_per_point, turnover_per_point_promo,
		updated_at
	) OUTPUT INSERTED.id VALUES (
		?, ?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?,
		?, ?, ?,
		?, ?,
		?, ?,
		?, ?,
		?, ?,
		?, ?, ?,
		?, ?, ?,
		?, ?,
		?, ?,
		?, ?, ?,
		?, ?,
		?, ?,
		?, ?,
		?, ?, ?,
		?, ?,
		?, ?,
		?, ?,
		?, ?,
		?, ?,
		?,
		?, ?,
		?, ?,
		?, ?,
		?, ?,
		GETDATE()
	)`

	var newID int64
	err := config.DB.QueryRow(query,
		r.NetworkName, r.KAM, r.Brand, r.BrandAS, r.SKU,
		r.Year, r.Month, r.Quarter, r.Mechanics, r.GTNOpex,
		r.BaselineUnits, r.BaselineRub,
		r.PlanPromoUnits, r.PlanPromoRub, r.PlanInvestmentsRub,
		r.PlanPromoUpliftUnits, r.PlanPromoUpliftRub,
		r.PlanPromoUpliftPctUnits, r.PlanPromoUpliftPctRub,
		r.PlanInvestmentsPct, r.PlanROI,
		r.ContractPrice, r.GM,
		r.IDDirectum, r.DSNumber, r.DiscountAmount,
		r.Conditions, r.Comments, r.EcomSegment,
		r.TotalPharmacies, r.PromoPharmacies,
		r.Status, r.Date,
		r.KeyRegion, r.Top20Segment, r.OlapPrice,
		r.PlanPromoCipOlap, r.FactPromoCipOlap,
		r.PlanPromoUpliftCipOlap, r.FactPromoUpliftCipOlap,
		r.ActualPromoSalesUnits, r.ActualInvestments,
		r.ActualPromoRub, r.ActualPromoUpliftUnits, r.ActualPromoUpliftRub,
		r.ActualExternalEcomUnits, r.ActualCorrectedBaseline,
		r.Agreement1, r.Agreement2,
		r.NetPromoUpliftRub, r.NetPromoUpliftPct,
		r.ActualInvestmentsPct, r.ActualROI,
		r.ActualPromoRubWoEcom, r.ActualPromoUpliftUnitsWoEcom,
		r.ActualPromoUpliftRubWoEcom,
		r.NetPromoUpliftRubWoEcom, r.NetPromoUpliftPctWoEcom,
		r.ActualInvestmentsPctWoEcom, r.ActualROIWoEcom,
		r.PlanVsFactRub, r.PlanVsFactInvestments,
		r.TurnoverPerPoint, r.TurnoverPerPointPromo,
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
	Network           string
	Brand             string
	Mechanics         string
}

func GetApprovals(params ApprovalParams) ([]models.ApprovalRow, error) {
	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	query := `
		SELECT TOP 500
			p.id, p.network_name, p.brand_as, p.sku, p.mechanics, p.year, p.month,
			p.baseline_units, p.plan_promo_units, p.actual_promo_sales_units,
			p.plan_investments_rub, p.plan_roi, p.actual_roi,
			p.conditions, p.agreement1, p.agreement2, p.status,
			p.agreement1_status, p.agreement1_comment,
			p.agreement2_status, p.agreement2_comment,
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

	if params.Network != "" {
		query += " AND p.network_name = ?"
		args = append(args, params.Network)
	}

	if params.Brand != "" {
		query += " AND p.brand_as = ?"
		args = append(args, params.Brand)
	}

	if params.Mechanics != "" {
		query += " AND p.mechanics = ?"
		args = append(args, params.Mechanics)
	}

	// Используем agreement1_status/agreement2_status вместо CHARINDEX-парсинга
	statusField := "p.agreement1_status"
	if params.Role == "agreement2" {
		statusField = "p.agreement2_status"
	}

	switch params.ApprovalStatus {
	case "pending":
		query += fmt.Sprintf(" AND %s IS NULL", statusField)
	case "commented":
		query += fmt.Sprintf(" AND %s = 'commented'", statusField)
	case "approved":
		query += fmt.Sprintf(" AND %s = 'approved'", statusField)
	case "rejected":
		query += fmt.Sprintf(" AND %s = 'rejected'", statusField)
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
			&r.Agreement1Status, &r.Agreement1Comment,
			&r.Agreement2Status, &r.Agreement2Comment,
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

// ApprovePromoWithStatus — обновляет agreement1/agreement2 и новые поля _status/_comment
func ApprovePromoWithStatus(agreementNum int, id int, status string, comment string, legacyValue string) error {
	statusField := fmt.Sprintf("agreement%d_status", agreementNum)
	commentField := fmt.Sprintf("agreement%d_comment", agreementNum)
	agreementField := fmt.Sprintf("agreement%d", agreementNum)

	query := fmt.Sprintf(
		"UPDATE dbo.tbl_PromoActivities SET %s = ?, %s = ?, %s = ?, updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL",
		agreementField, statusField, commentField,
	)
	_, err := config.DB.Exec(query, legacyValue, status, comment, id)
	return err
}

// ─── Approval Filters ───────────────────────────────────────────────────────

type ApprovalFilterParams struct {
	ApprovalStatus, KAM, Network, Brand, MechFilter, YearStr, MonthStr, Role string
}

// buildApprovalWhere — строит WHERE-условия для страницы согласования.
// excludeCol — колонка, которую НЕ фильтруем (чтобы не сужать саму себя).
func buildApprovalWhere(params ApprovalFilterParams, excludeCol string) (string, []interface{}) {
	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	where := "p.deleted_at IS NULL"
	args := []interface{}{}

	if params.YearStr != "" {
		y, _ := strconv.Atoi(params.YearStr)
		where += " AND p.year = ?"
		args = append(args, y)
	} else {
		where += " AND (p.year > ? OR (p.year = ? AND p.month >= ?))"
		args = append(args, currentYear, currentYear, currentMonth)
	}

	if params.MonthStr != "" {
		m, _ := strconv.Atoi(params.MonthStr)
		where += " AND p.month = ?"
		args = append(args, m)
	}

	if params.KAM != "" && excludeCol != "kam" {
		where += " AND p.kam = ?"
		args = append(args, params.KAM)
	}

	if params.Network != "" && excludeCol != "network_name" {
		where += " AND p.network_name = ?"
		args = append(args, params.Network)
	}

	if params.Brand != "" && excludeCol != "brand_as" {
		where += " AND p.brand_as = ?"
		args = append(args, params.Brand)
	}

	if params.MechFilter != "" && excludeCol != "mechanics" {
		where += " AND p.mechanics = ?"
		args = append(args, params.MechFilter)
	}

	filterStatusField := "p.agreement1_status"
	if params.Role == "agreement2" {
		filterStatusField = "p.agreement2_status"
	}

	switch params.ApprovalStatus {
	case "pending":
		where += fmt.Sprintf(" AND %s IS NULL", filterStatusField)
	case "commented":
		where += fmt.Sprintf(" AND %s = 'commented'", filterStatusField)
	case "approved":
		where += fmt.Sprintf(" AND %s = 'approved'", filterStatusField)
	case "rejected":
		where += fmt.Sprintf(" AND %s = 'rejected'", filterStatusField)
	}

	return where, args
}

// GetApprovalFilters — перекрёстная фильтрация: 4 горутины, excludeCol для каждой колонки.
func GetApprovalFilters(params ApprovalFilterParams) (networks, brands, mechanics, kams []string, err error) {
	var (
		resNetwork, resBrand, resMech, resKam []string
	)

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		where, args := buildApprovalWhere(params, "network_name")
		query := "SELECT DISTINCT p.network_name FROM dbo.tbl_PromoActivities p WHERE " + where + " AND p.network_name IS NOT NULL ORDER BY p.network_name"
		resNetwork = ExecDistinct(query, args)
		return nil
	})
	g.Go(func() error {
		where, args := buildApprovalWhere(params, "brand_as")
		query := "SELECT DISTINCT p.brand_as FROM dbo.tbl_PromoActivities p WHERE " + where + " AND p.brand_as IS NOT NULL ORDER BY p.brand_as"
		resBrand = ExecDistinct(query, args)
		return nil
	})
	g.Go(func() error {
		where, args := buildApprovalWhere(params, "mechanics")
		query := "SELECT DISTINCT p.mechanics FROM dbo.tbl_PromoActivities p WHERE " + where + " AND p.mechanics IS NOT NULL ORDER BY p.mechanics"
		resMech = ExecDistinct(query, args)
		return nil
	})
	g.Go(func() error {
		where, args := buildApprovalWhere(params, "kam")
		query := "SELECT DISTINCT p.kam FROM dbo.tbl_PromoActivities p WHERE " + where + " AND p.kam IS NOT NULL ORDER BY p.kam"
		resKam = ExecDistinct(query, args)
		return nil
	})

	_ = g.Wait()

	return resNetwork, resBrand, resMech, resKam, nil
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
