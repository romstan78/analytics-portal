package handlers

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"

	"backend/config"
)

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

	var olapPrice sql.NullFloat64
	config.DB.QueryRow(
		"SELECT TOP 1 olap_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND olap_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC",
		sku,
	).Scan(&olapPrice)
	olap := 0.0
	if olapPrice.Valid {
		olap = olapPrice.Float64
	}

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
