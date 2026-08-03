package services

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"backend/models"
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
func EnrichFromRepo(input *PromoInputDTO) CalculationContext {
	ctx := CalculationContext{}

	if input.GM == 0 {
		lastData, err := repository.GetLastSKUData(input.SKU)
		if err == nil && lastData != nil && lastData.GM != 0 {
			ctx.GM = lastData.GM
		} else {
			ctx.GM = 1
		}
	} else {
		ctx.GM = input.GM
	}

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

// ─── Указатели (алиасы на функции пакета models) ─────────────────────────

var ptrFloat = models.PtrFloat
var ptrInt = models.PtrInt
var valFloat = models.ValFloat
var valInt = models.ValInt

// MergeCalculatedIntoDBRow — записывает вычисленные поля напрямую в структуру БД.
func MergeCalculatedIntoDBRow(r *models.PromoRowDB, c CalculatedFields) {
	r.Year = c.Year
	r.Month = c.Month
	r.Quarter = ptrInt(c.Quarter)
	r.GM = ptrFloat(c.GM)
	r.PlanPromoRub = ptrFloat(c.PlanPromoRub)
	r.PlanPromoUpliftUnits = ptrFloat(c.PlanPromoUpliftUnits)
	r.PlanPromoUpliftRub = ptrFloat(c.PlanPromoUpliftRub)
	r.PlanPromoUpliftPctUnits = ptrFloat(c.PlanPromoUpliftPctUnits)
	r.PlanPromoUpliftPctRub = ptrFloat(c.PlanPromoUpliftPctRub)
	r.PlanInvestmentsPct = ptrFloat(c.PlanInvestmentsPct)
	r.PlanROI = ptrFloat(c.PlanROI)
	r.BaselineRub = ptrFloat(c.BaselineRub)
	r.NetPromoUpliftRub = ptrFloat(c.NetPromoUpliftRub)
	r.NetPromoUpliftPct = ptrFloat(c.NetPromoUpliftPct)
	r.ActualInvestmentsPct = ptrFloat(c.ActualInvestmentsPct)
	r.ActualROI = ptrFloat(c.ActualROI)
	r.ActualPromoRubWoEcom = ptrFloat(c.ActualPromoRubWoEcom)
	r.ActualPromoUpliftUnitsWoEcom = ptrFloat(c.ActualPromoUpliftUnitsWoEcom)
	r.ActualPromoUpliftRubWoEcom = ptrFloat(c.ActualPromoUpliftRubWoEcom)
	r.NetPromoUpliftRubWoEcom = ptrFloat(c.NetPromoUpliftRubWoEcom)
	r.NetPromoUpliftPctWoEcom = ptrFloat(c.NetPromoUpliftPctWoEcom)
	r.ActualInvestmentsPctWoEcom = ptrFloat(c.ActualInvestmentsPctWoEcom)
	r.ActualROIWoEcom = ptrFloat(c.ActualROIWoEcom)
	r.PlanVsFactRub = ptrFloat(c.PlanVsFactRub)
	r.PlanVsFactInvestments = ptrFloat(c.PlanVsFactInvestments)
	r.TurnoverPerPoint = ptrFloat(c.TurnoverPerPoint)
	r.TurnoverPerPointPromo = ptrFloat(c.TurnoverPerPointPromo)
	r.PlanPromoCipOlap = ptrFloat(c.PlanPromoCipOlap)
	r.FactPromoCipOlap = ptrFloat(c.FactPromoCipOlap)
	r.PlanPromoUpliftCipOlap = ptrFloat(c.PlanPromoUpliftCipOlap)
	r.FactPromoUpliftCipOlap = ptrFloat(c.FactPromoUpliftCipOlap)
	r.Date = c.Date
	r.OlapPrice = ptrFloat(c.OlapPrice)
}

// DTOToDBRow — создаёт PromoRowDB из DTO и вычисленных полей (для INSERT).
func DTOToDBRow(dto PromoInputDTO, c CalculatedFields) *models.PromoRowDB {
	r := &models.PromoRowDB{
		NetworkName:             dto.NetworkName,
		KAM:                     dto.KAM,
		Brand:                   dto.Brand,
		BrandAS:                 dto.BrandAS,
		SKU:                     dto.SKU,
		Mechanics:               dto.Mechanics,
		GTNOpex:                 dto.GTNOpex,
		IDDirectum:              dto.IDDirectum,
		DSNumber:                dto.DSNumber,
		DiscountAmount:          ptrFloat(dto.DiscountAmount),
		Conditions:              dto.Conditions,
		Comments:                dto.Comments,
		EcomSegment:             dto.EcomSegment,
		TotalPharmacies:         ptrInt(dto.TotalPharmacies),
		PromoPharmacies:         ptrInt(dto.PromoPharmacies),
		Status:                  dto.Status,
		BaselineUnits:           ptrFloat(dto.BaselineUnits),
		PlanPromoUnits:          ptrFloat(dto.PlanPromoUnits),
		PlanInvestmentsRub:      ptrFloat(dto.PlanInvestmentsRub),
		ContractPrice:           ptrFloat(dto.ContractPrice),
		KeyRegion:               dto.KeyRegion,
		Top20Segment:            dto.Top20Segment,
		ActualPromoSalesUnits:   ptrFloat(dto.ActualPromoSalesUnits),
		ActualPromoRub:          ptrFloat(dto.ActualPromoRub),
		ActualInvestments:       ptrFloat(dto.ActualInvestments),
		ActualPromoUpliftUnits:  ptrFloat(dto.ActualPromoUpliftUnits),
		ActualPromoUpliftRub:    ptrFloat(dto.ActualPromoUpliftRub),
		ActualExternalEcomUnits: ptrFloat(dto.ActualExternalEcomUnits),
		ActualCorrectedBaseline: ptrFloat(dto.ActualCorrectedBaseline),
		Agreement1:              dto.Agreement1,
		Agreement2:              dto.Agreement2,
	}
	MergeCalculatedIntoDBRow(r, c)
	return r
}

// MapToDTO — парсит JSON (map[string]interface{}) в PromoInputDTO (для INSERT).
func MapToDTO(input map[string]interface{}) PromoInputDTO {
	return PromoInputDTO{
		NetworkName:             stringVal(input, "network_name"),
		KAM:                     stringVal(input, "kam"),
		Brand:                   stringVal(input, "brand"),
		BrandAS:                 stringVal(input, "brand_as"),
		SKU:                     stringVal(input, "sku"),
		Year:                    intVal(input, "year"),
		Month:                   intVal(input, "month"),
		Quarter:                 intVal(input, "quarter"),
		Mechanics:               stringVal(input, "mechanics"),
		GTNOpex:                 stringVal(input, "gtn_opex"),
		IDDirectum:              stringVal(input, "id_directum"),
		DSNumber:                stringVal(input, "ds_number"),
		DiscountAmount:          floatVal(input, "discount_amount"),
		Conditions:              stringVal(input, "conditions"),
		Comments:                stringVal(input, "comments"),
		EcomSegment:             stringVal(input, "ecom_segment"),
		TotalPharmacies:         intVal(input, "total_pharmacies"),
		PromoPharmacies:         intVal(input, "promo_pharmacies"),
		Status:                  stringVal(input, "status"),
		Date:                    stringVal(input, "date"),
		BaselineUnits:           floatVal(input, "baseline_units"),
		BaselineRub:             floatVal(input, "baseline_rub"),
		PlanPromoUnits:          floatVal(input, "plan_promo_units"),
		PlanPromoRub:            floatVal(input, "plan_promo_rub"),
		PlanInvestmentsRub:      floatVal(input, "plan_investments_rub"),
		ContractPrice:           floatVal(input, "contract_price"),
		GM:                      floatVal(input, "gm"),
		KeyRegion:               stringVal(input, "key_region"),
		Top20Segment:            stringVal(input, "top20_segment"),
		OlapPrice:               floatVal(input, "olap_price"),
		ActualPromoSalesUnits:   floatVal(input, "actual_promo_sales_units"),
		ActualPromoRub:          floatVal(input, "actual_promo_rub"),
		ActualInvestments:       floatVal(input, "actual_investments"),
		ActualPromoUpliftUnits:  floatVal(input, "actual_promo_uplift_units"),
		ActualPromoUpliftRub:    floatVal(input, "actual_promo_uplift_rub"),
		ActualExternalEcomUnits: floatVal(input, "actual_external_ecom_units"),
		ActualCorrectedBaseline: floatVal(input, "actual_corrected_baseline"),
		Agreement1:              stringVal(input, "agreement1"),
		Agreement2:              stringVal(input, "agreement2"),
	}
}

// DBRowToDTO — конвертирует строку БД в DTO (для расчета вычисляемых полей при UPDATE).
func DBRowToDTO(r *models.PromoRowDB) PromoInputDTO {
	return PromoInputDTO{
		NetworkName:             r.NetworkName,
		KAM:                     r.KAM,
		Brand:                   r.Brand,
		BrandAS:                 r.BrandAS,
		SKU:                     r.SKU,
		Year:                    r.Year,
		Month:                   r.Month,
		Quarter:                 valInt(r.Quarter),
		Mechanics:               r.Mechanics,
		GTNOpex:                 r.GTNOpex,
		IDDirectum:              r.IDDirectum,
		DSNumber:                r.DSNumber,
		DiscountAmount:          valFloat(r.DiscountAmount),
		Conditions:              r.Conditions,
		Comments:                r.Comments,
		EcomSegment:             r.EcomSegment,
		TotalPharmacies:         valInt(r.TotalPharmacies),
		PromoPharmacies:         valInt(r.PromoPharmacies),
		Status:                  r.Status,
		Date:                    r.Date,
		BaselineUnits:           valFloat(r.BaselineUnits),
		BaselineRub:             valFloat(r.BaselineRub),
		PlanPromoUnits:          valFloat(r.PlanPromoUnits),
		PlanPromoRub:            valFloat(r.PlanPromoRub),
		PlanInvestmentsRub:      valFloat(r.PlanInvestmentsRub),
		ContractPrice:           valFloat(r.ContractPrice),
		GM:                      valFloat(r.GM),
		KeyRegion:               r.KeyRegion,
		Top20Segment:            r.Top20Segment,
		OlapPrice:               valFloat(r.OlapPrice),
		ActualPromoSalesUnits:   valFloat(r.ActualPromoSalesUnits),
		ActualPromoRub:          valFloat(r.ActualPromoRub),
		ActualInvestments:       valFloat(r.ActualInvestments),
		ActualPromoUpliftUnits:  valFloat(r.ActualPromoUpliftUnits),
		ActualPromoUpliftRub:    valFloat(r.ActualPromoUpliftRub),
		ActualExternalEcomUnits: valFloat(r.ActualExternalEcomUnits),
		ActualCorrectedBaseline: valFloat(r.ActualCorrectedBaseline),
		Agreement1:              r.Agreement1,
		Agreement2:              r.Agreement2,
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func floatVal(m map[string]interface{}, key string) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprint(m[key]), 64)
	return v
}
func intVal(m map[string]interface{}, key string) int {
	v, _ := strconv.Atoi(fmt.Sprint(m[key]))
	return v
}
func stringVal(m map[string]interface{}, key string) string {
	return fmt.Sprint(m[key])
}

// DBRowToMap — конвертирует PromoRowDB в map для JSON-ответа (обратная совместимость с фронтендом).
func DBRowToMap(r *models.PromoRowDB) map[string]interface{} {
	return map[string]interface{}{
		"id":                                r.ID,
		"network_name":                      r.NetworkName,
		"kam":                               r.KAM,
		"brand":                             r.Brand,
		"brand_as":                          r.BrandAS,
		"sku":                               r.SKU,
		"year":                              r.Year,
		"month":                             r.Month,
		"quarter":                           r.Quarter,
		"mechanics":                         r.Mechanics,
		"gtn_opex":                          r.GTNOpex,
		"baseline_units":                    r.BaselineUnits,
		"baseline_rub":                      r.BaselineRub,
		"plan_promo_units":                  r.PlanPromoUnits,
		"plan_promo_rub":                    r.PlanPromoRub,
		"plan_investments_rub":              r.PlanInvestmentsRub,
		"plan_promo_uplift_units":           r.PlanPromoUpliftUnits,
		"plan_promo_uplift_rub":             r.PlanPromoUpliftRub,
		"plan_promo_uplift_pct_units":       r.PlanPromoUpliftPctUnits,
		"plan_promo_uplift_pct_rub":         r.PlanPromoUpliftPctRub,
		"plan_investments_pct":              r.PlanInvestmentsPct,
		"plan_roi":                          r.PlanROI,
		"contract_price":                    r.ContractPrice,
		"gm":                                r.GM,
		"id_directum":                       r.IDDirectum,
		"ds_number":                         r.DSNumber,
		"discount_amount":                   r.DiscountAmount,
		"conditions":                        r.Conditions,
		"comments":                          r.Comments,
		"ecom_segment":                      r.EcomSegment,
		"total_pharmacies":                  r.TotalPharmacies,
		"promo_pharmacies":                  r.PromoPharmacies,
		"status":                            r.Status,
		"date":                              r.Date,
		"key_region":                        r.KeyRegion,
		"top20_segment":                     r.Top20Segment,
		"olap_price":                        r.OlapPrice,
		"plan_promo_cip_olap":               r.PlanPromoCipOlap,
		"fact_promo_cip_olap":               r.FactPromoCipOlap,
		"plan_promo_uplift_cip_olap":        r.PlanPromoUpliftCipOlap,
		"fact_promo_uplift_cip_olap":        r.FactPromoUpliftCipOlap,
		"actual_promo_sales_units":          r.ActualPromoSalesUnits,
		"actual_investments":                r.ActualInvestments,
		"actual_promo_rub":                  r.ActualPromoRub,
		"actual_promo_uplift_units":         r.ActualPromoUpliftUnits,
		"actual_promo_uplift_rub":           r.ActualPromoUpliftRub,
		"actual_external_ecom_units":        r.ActualExternalEcomUnits,
		"actual_corrected_baseline":         r.ActualCorrectedBaseline,
		"agreement1":                        r.Agreement1,
		"agreement2":                        r.Agreement2,
		"net_promo_uplift_rub":              r.NetPromoUpliftRub,
		"net_promo_uplift_pct":              r.NetPromoUpliftPct,
		"actual_investments_pct":            r.ActualInvestmentsPct,
		"actual_roi":                        r.ActualROI,
		"actual_promo_rub_wo_ecom":          r.ActualPromoRubWoEcom,
		"actual_promo_uplift_units_wo_ecom": r.ActualPromoUpliftUnitsWoEcom,
		"actual_promo_uplift_rub_wo_ecom":   r.ActualPromoUpliftRubWoEcom,
		"net_promo_uplift_rub_wo_ecom":      r.NetPromoUpliftRubWoEcom,
		"net_promo_uplift_pct_wo_ecom":      r.NetPromoUpliftPctWoEcom,
		"actual_investments_pct_wo_ecom":    r.ActualInvestmentsPctWoEcom,
		"actual_roi_wo_ecom":                r.ActualROIWoEcom,
		"plan_vs_fact_rub":                  r.PlanVsFactRub,
		"plan_vs_fact_investments":          r.PlanVsFactInvestments,
		"turnover_per_point":                r.TurnoverPerPoint,
		"turnover_per_point_promo":          r.TurnoverPerPointPromo,
		"updated_at":                        r.UpdatedAt,
	}
}
