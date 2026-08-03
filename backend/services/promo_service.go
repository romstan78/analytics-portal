package services

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"backend/repository"
)

// PromoInputDTO — типизированная структура для входных данных промо-акции.
// Заменяет map[string]interface{} в SavePromo и calculatePromoFields.
type PromoInputDTO struct {
	// Основные поля
	NetworkName     string  `json:"network_name"`
	KAM             string  `json:"kam"`
	Brand           string  `json:"brand"`
	BrandAS         string  `json:"brand_as"`
	SKU             string  `json:"sku"`
	Year            int     `json:"year"`
	Month           int     `json:"month"`
	Quarter         int     `json:"quarter"`
	Mechanics       string  `json:"mechanics"`
	GTNOpex         string  `json:"gtn_opex"`
	IDDirectum      string  `json:"id_directum"`
	DSNumber        string  `json:"ds_number"`
	DiscountAmount  float64 `json:"discount_amount"`
	Conditions      string  `json:"conditions"`
	Comments        string  `json:"comments"`
	EcomSegment     string  `json:"ecom_segment"`
	TotalPharmacies int     `json:"total_pharmacies"`
	PromoPharmacies int     `json:"promo_pharmacies"`
	Status          string  `json:"status"`
	Date            string  `json:"date"`

	// Плановые показатели
	BaselineUnits      float64 `json:"baseline_units"`
	BaselineRub        float64 `json:"baseline_rub"`
	PlanPromoUnits     float64 `json:"plan_promo_units"`
	PlanPromoRub       float64 `json:"plan_promo_rub"`
	PlanInvestmentsRub float64 `json:"plan_investments_rub"`
	ContractPrice      float64 `json:"contract_price"`
	GM                 float64 `json:"gm"`

	// Ключевой регион и сегмент (из Geo-маппинга, если не заданы)
	KeyRegion    string `json:"key_region"`
	Top20Segment string `json:"top20_segment"`

	// OLAP цена
	OlapPrice float64 `json:"olap_price"`

	// Фактические показатели
	ActualPromoSalesUnits   float64 `json:"actual_promo_sales_units"`
	ActualPromoRub          float64 `json:"actual_promo_rub"`
	ActualInvestments       float64 `json:"actual_investments"`
	ActualPromoUpliftUnits  float64 `json:"actual_promo_uplift_units"`
	ActualPromoUpliftRub    float64 `json:"actual_promo_uplift_rub"`
	ActualExternalEcomUnits float64 `json:"actual_external_ecom_units"`
	ActualCorrectedBaseline float64 `json:"actual_corrected_baseline"`
	Agreement1              string  `json:"agreement1"`
	Agreement2              string  `json:"agreement2"`
}

// CalculatedFields — результат вычислений.
type CalculatedFields struct {
	Year                         int     `json:"year"`
	Month                        int     `json:"month"`
	Quarter                      int     `json:"quarter"`
	GM                           float64 `json:"gm"`
	PlanPromoRub                 float64 `json:"plan_promo_rub"`
	PlanPromoUpliftUnits         float64 `json:"plan_promo_uplift_units"`
	PlanPromoUpliftRub           float64 `json:"plan_promo_uplift_rub"`
	PlanPromoUpliftPctUnits      float64 `json:"plan_promo_uplift_pct_units"`
	PlanPromoUpliftPctRub        float64 `json:"plan_promo_uplift_pct_rub"`
	PlanInvestmentsPct           float64 `json:"plan_investments_pct"`
	PlanROI                      float64 `json:"plan_roi"`
	BaselineRub                  float64 `json:"baseline_rub"`
	NetPromoUpliftRub            float64 `json:"net_promo_uplift_rub"`
	NetPromoUpliftPct            float64 `json:"net_promo_uplift_pct"`
	ActualInvestmentsPct         float64 `json:"actual_investments_pct"`
	ActualROI                    float64 `json:"actual_roi"`
	ActualPromoRubWoEcom         float64 `json:"actual_promo_rub_wo_ecom"`
	ActualPromoUpliftUnitsWoEcom float64 `json:"actual_promo_uplift_units_wo_ecom"`
	ActualPromoUpliftRubWoEcom   float64 `json:"actual_promo_uplift_rub_wo_ecom"`
	NetPromoUpliftRubWoEcom      float64 `json:"net_promo_uplift_rub_wo_ecom"`
	NetPromoUpliftPctWoEcom      float64 `json:"net_promo_uplift_pct_wo_ecom"`
	ActualInvestmentsPctWoEcom   float64 `json:"actual_investments_pct_wo_ecom"`
	ActualROIWoEcom              float64 `json:"actual_roi_wo_ecom"`
	PlanVsFactRub                float64 `json:"plan_vs_fact_rub"`
	PlanVsFactInvestments        float64 `json:"plan_vs_fact_investments"`
	TurnoverPerPoint             float64 `json:"turnover_per_point"`
	TurnoverPerPointPromo        float64 `json:"turnover_per_point_promo"`
	PlanPromoCipOlap             float64 `json:"plan_promo_cip_olap"`
	FactPromoCipOlap             float64 `json:"fact_promo_cip_olap"`
	PlanPromoUpliftCipOlap       float64 `json:"plan_promo_uplift_cip_olap"`
	FactPromoUpliftCipOlap       float64 `json:"fact_promo_uplift_cip_olap"`
	Date                         string  `json:"date"`
	OlapPrice                    float64 `json:"olap_price"`
}

// CalculationContext — контекст с данными, полученными от репозитория,
// которые нужны для расчета, но не пришли от клиента.
type CalculationContext struct {
	GM           float64
	KeyRegion    string
	Top20Segment string
	OlapPrice    float64
}

// EnrichFromRepo — заполняет недостающие данные из БД через репозиторий.
// Раньше это делалось прямыми SQL-запросами в calculatePromoFields.
func EnrichFromRepo(input *PromoInputDTO) CalculationContext {
	ctx := CalculationContext{}

	// GM: если не задан — ищем последнюю запись по SKU
	if input.GM == 0 {
		lastData, err := repository.GetLastSKUData(input.SKU)
		if err == nil && lastData != nil && lastData.GM != 0 {
			ctx.GM = lastData.GM
		} else {
			ctx.GM = 1 // fallback по умолчанию
		}
	} else {
		ctx.GM = input.GM
	}

	// KeyRegion / Top20Segment: если не заданы — ищем из Geo-маппинга
	if input.KeyRegion == "" || input.Top20Segment == "" {
		geo, err := repository.GetNetworkGeoMapping(input.NetworkName)
		if err == nil && geo != nil {
			if input.KeyRegion == "" && geo.KeyRegion != "" {
				input.KeyRegion = geo.KeyRegion
			}
			if input.Top20Segment == "" && geo.Top20Segment != "" {
				input.Top20Segment = geo.Top20Segment
			}
		}
	}
	ctx.KeyRegion = input.KeyRegion
	ctx.Top20Segment = input.Top20Segment

	// OLAP price
	if input.OlapPrice == 0 {
		lastData, err := repository.GetLastSKUData(input.SKU)
		if err == nil && lastData != nil && lastData.OlapPrice != 0 {
			ctx.OlapPrice = lastData.OlapPrice
		}
	} else {
		ctx.OlapPrice = input.OlapPrice
	}

	return ctx
}

// CalculateFields — чистая функция расчета всех вычисляемых полей.
// Не делает запросов в БД, только математика.
func CalculateFields(input *PromoInputDTO, ctx CalculationContext) CalculatedFields {
	ppu := input.PlanPromoUnits
	cp := input.ContractPrice
	bu := input.BaselineUnits
	pir := input.PlanInvestmentsRub
	month := input.Month
	year := input.Year
	if year == 0 {
		year = time.Now().Year()
	}
	if month == 0 {
		month = int(time.Now().Month())
	}

	gm := ctx.GM
	olap := ctx.OlapPrice

	quarter := int(math.Ceil(float64(month) / 3))
	planPromoRub := ppu * cp
	planPromoUpliftUnits := ppu - bu
	planPromoUpliftRub := planPromoUpliftUnits * cp

	planPromoUpliftPctUnits := 0.0
	if ppu > 0 {
		planPromoUpliftPctUnits = (planPromoUpliftUnits / ppu) * 100
	}
	planPromoUpliftPctRub := 0.0
	if planPromoRub > 0 {
		planPromoUpliftPctRub = (planPromoUpliftRub / planPromoRub) * 100
	}
	planInvestmentsPct := 0.0
	if planPromoRub > 0 {
		planInvestmentsPct = (pir / planPromoRub) * 100
	}
	planROI := 0.0
	if pir > 0 {
		planROI = (planPromoUpliftRub/pir)*gm*100 - 100
	}
	baselineRub := bu * cp

	afu := input.ActualPromoSalesUnits
	afr := input.ActualPromoRub
	afi := input.ActualInvestments
	afupl := input.ActualPromoUpliftUnits
	afupr := input.ActualPromoUpliftRub
	afeu := input.ActualExternalEcomUnits
	acb := input.ActualCorrectedBaseline
	ph := float64(input.PromoPharmacies)
	if ph == 0 {
		ph = 1
	}

	netPromoUpliftRub := afupr * gm
	netPromoUpliftPct := 0.0
	if afr > 0 {
		netPromoUpliftPct = (netPromoUpliftRub / afr) * 100
	}
	actualInvestmentsPct := 0.0
	if afr > 0 {
		actualInvestmentsPct = (afi / afr) * 100
	}
	actualROI := 0.0
	if afi > 0 {
		actualROI = (afupr/afi)*gm*100 - 100
	}

	actualPromoRubWoEcom := afr - (afeu * cp)
	actualPromoUpliftUnitsWoEcom := afupl - afeu
	actualPromoUpliftRubWoEcom := actualPromoUpliftUnitsWoEcom * cp
	netPromoUpliftRubWoEcom := actualPromoUpliftRubWoEcom * gm
	netPromoUpliftPctWoEcom := 0.0
	if actualPromoRubWoEcom > 0 {
		netPromoUpliftPctWoEcom = (netPromoUpliftRubWoEcom / actualPromoRubWoEcom) * 100
	}
	actualInvestmentsPctWoEcom := 0.0
	if actualPromoRubWoEcom > 0 {
		actualInvestmentsPctWoEcom = (afi / actualPromoRubWoEcom) * 100
	}
	actualROIWoEcom := 0.0
	if afi > 0 {
		actualROIWoEcom = (actualPromoUpliftRubWoEcom/afi)*gm*100 - 100
	}

	planVsFactRub := 0.0
	if planPromoRub > 0 {
		planVsFactRub = (afr / planPromoRub) * 100
	}
	planVsFactInvestments := 0.0
	if pir > 0 {
		planVsFactInvestments = (afi / pir) * 100
	}

	turnoverPerPoint := acb / ph
	turnoverPerPointPromo := afu / ph
	planPromoCipOlap := ppu * olap
	factPromoCipOlap := afu * olap
	planPromoUpliftCipOlap := planPromoUpliftUnits * olap
	factPromoUpliftCipOlap := afupl * olap

	return CalculatedFields{
		Year:                         year,
		Month:                        month,
		Quarter:                      quarter,
		GM:                           gm,
		PlanPromoRub:                 planPromoRub,
		PlanPromoUpliftUnits:         planPromoUpliftUnits,
		PlanPromoUpliftRub:           planPromoUpliftRub,
		PlanPromoUpliftPctUnits:      planPromoUpliftPctUnits,
		PlanPromoUpliftPctRub:        planPromoUpliftPctRub,
		PlanInvestmentsPct:           planInvestmentsPct,
		PlanROI:                      planROI,
		BaselineRub:                  baselineRub,
		NetPromoUpliftRub:            netPromoUpliftRub,
		NetPromoUpliftPct:            netPromoUpliftPct,
		ActualInvestmentsPct:         actualInvestmentsPct,
		ActualROI:                    actualROI,
		ActualPromoRubWoEcom:         actualPromoRubWoEcom,
		ActualPromoUpliftUnitsWoEcom: actualPromoUpliftUnitsWoEcom,
		ActualPromoUpliftRubWoEcom:   actualPromoUpliftRubWoEcom,
		NetPromoUpliftRubWoEcom:      netPromoUpliftRubWoEcom,
		NetPromoUpliftPctWoEcom:      netPromoUpliftPctWoEcom,
		ActualInvestmentsPctWoEcom:   actualInvestmentsPctWoEcom,
		ActualROIWoEcom:              actualROIWoEcom,
		PlanVsFactRub:                planVsFactRub,
		PlanVsFactInvestments:        planVsFactInvestments,
		TurnoverPerPoint:             turnoverPerPoint,
		TurnoverPerPointPromo:        turnoverPerPointPromo,
		PlanPromoCipOlap:             planPromoCipOlap,
		FactPromoCipOlap:             factPromoCipOlap,
		PlanPromoUpliftCipOlap:       planPromoUpliftCipOlap,
		FactPromoUpliftCipOlap:       factPromoUpliftCipOlap,
		Date:                         PromoDate(year, month),
		OlapPrice:                    olap,
	}
}

// PromoDate — форматирует год и месяц в строку "YYYY-MM-01".
func PromoDate(year, month int) string {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// ToMap — преобразует CalculatedFields в map[string]interface{} для обратной
// совместимости с текущими repository.InsertPromo / UpdatePromo.
// TODO: удалить после полного перехода на типизированные структуры.
func (c CalculatedFields) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"year":                              c.Year,
		"month":                             c.Month,
		"quarter":                           c.Quarter,
		"gm":                                c.GM,
		"plan_promo_rub":                    c.PlanPromoRub,
		"plan_promo_uplift_units":           c.PlanPromoUpliftUnits,
		"plan_promo_uplift_rub":             c.PlanPromoUpliftRub,
		"plan_promo_uplift_pct_units":       c.PlanPromoUpliftPctUnits,
		"plan_promo_uplift_pct_rub":         c.PlanPromoUpliftPctRub,
		"plan_investments_pct":              c.PlanInvestmentsPct,
		"plan_roi":                          c.PlanROI,
		"baseline_rub":                      c.BaselineRub,
		"net_promo_uplift_rub":              c.NetPromoUpliftRub,
		"net_promo_uplift_pct":              c.NetPromoUpliftPct,
		"actual_investments_pct":            c.ActualInvestmentsPct,
		"actual_roi":                        c.ActualROI,
		"actual_promo_rub_wo_ecom":          c.ActualPromoRubWoEcom,
		"actual_promo_uplift_units_wo_ecom": c.ActualPromoUpliftUnitsWoEcom,
		"actual_promo_uplift_rub_wo_ecom":   c.ActualPromoUpliftRubWoEcom,
		"net_promo_uplift_rub_wo_ecom":      c.NetPromoUpliftRubWoEcom,
		"net_promo_uplift_pct_wo_ecom":      c.NetPromoUpliftPctWoEcom,
		"actual_investments_pct_wo_ecom":    c.ActualInvestmentsPctWoEcom,
		"actual_roi_wo_ecom":                c.ActualROIWoEcom,
		"plan_vs_fact_rub":                  c.PlanVsFactRub,
		"plan_vs_fact_investments":          c.PlanVsFactInvestments,
		"turnover_per_point":                c.TurnoverPerPoint,
		"turnover_per_point_promo":          c.TurnoverPerPointPromo,
		"plan_promo_cip_olap":               c.PlanPromoCipOlap,
		"fact_promo_cip_olap":               c.FactPromoCipOlap,
		"plan_promo_uplift_cip_olap":        c.PlanPromoUpliftCipOlap,
		"fact_promo_uplift_cip_olap":        c.FactPromoUpliftCipOlap,
		"date":                              c.Date,
		"olap_price":                        c.OlapPrice,
	}
}

// MapToDTO — преобразует map[string]interface{} в PromoInputDTO.
// Используется для обратной совместимости в SavePromo.
func MapToDTO(input map[string]interface{}) PromoInputDTO {
	return PromoInputDTO{
		NetworkName:             safeString(input, "network_name"),
		KAM:                     safeString(input, "kam"),
		Brand:                   safeString(input, "brand"),
		BrandAS:                 safeString(input, "brand_as"),
		SKU:                     safeString(input, "sku"),
		Year:                    safeInt(input, "year"),
		Month:                   safeInt(input, "month"),
		Quarter:                 safeInt(input, "quarter"),
		Mechanics:               safeString(input, "mechanics"),
		GTNOpex:                 safeString(input, "gtn_opex"),
		IDDirectum:              safeString(input, "id_directum"),
		DSNumber:                safeString(input, "ds_number"),
		DiscountAmount:          safeFloat(input, "discount_amount"),
		Conditions:              safeString(input, "conditions"),
		Comments:                safeString(input, "comments"),
		EcomSegment:             safeString(input, "ecom_segment"),
		TotalPharmacies:         safeInt(input, "total_pharmacies"),
		PromoPharmacies:         safeInt(input, "promo_pharmacies"),
		Status:                  safeString(input, "status"),
		Date:                    safeString(input, "date"),
		BaselineUnits:           safeFloat(input, "baseline_units"),
		BaselineRub:             safeFloat(input, "baseline_rub"),
		PlanPromoUnits:          safeFloat(input, "plan_promo_units"),
		PlanPromoRub:            safeFloat(input, "plan_promo_rub"),
		PlanInvestmentsRub:      safeFloat(input, "plan_investments_rub"),
		ContractPrice:           safeFloat(input, "contract_price"),
		GM:                      safeFloat(input, "gm"),
		KeyRegion:               safeString(input, "key_region"),
		Top20Segment:            safeString(input, "top20_segment"),
		OlapPrice:               safeFloat(input, "olap_price"),
		ActualPromoSalesUnits:   safeFloat(input, "actual_promo_sales_units"),
		ActualPromoRub:          safeFloat(input, "actual_promo_rub"),
		ActualInvestments:       safeFloat(input, "actual_investments"),
		ActualPromoUpliftUnits:  safeFloat(input, "actual_promo_uplift_units"),
		ActualPromoUpliftRub:    safeFloat(input, "actual_promo_uplift_rub"),
		ActualExternalEcomUnits: safeFloat(input, "actual_external_ecom_units"),
		ActualCorrectedBaseline: safeFloat(input, "actual_corrected_baseline"),
		Agreement1:              safeString(input, "agreement1"),
		Agreement2:              safeString(input, "agreement2"),
	}
}

// MergeCalculatedIntoMap — сливает CalculatedFields в существующий map.
func MergeCalculatedIntoMap(m map[string]interface{}, c CalculatedFields) {
	for k, v := range c.ToMap() {
		m[k] = v
	}
}

// ─── helpers (копия из promo_utils.go для независимости) ───

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

// Выше нужны импорты "fmt" и "strconv", оставлены намеренно.
