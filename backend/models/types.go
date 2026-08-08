package models

// PtrFloat возвращает указатель на float64.
func PtrFloat(v float64) *float64 { return &v }

// PtrInt возвращает указатель на int.
func PtrInt(v int) *int { return &v }

// PtrString возвращает указатель на string.
func PtrString(v string) *string { return &v }

// ValFloat возвращает значение по указателю или 0.
func ValFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// ValInt возвращает значение по указателю или 0.
func ValInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// ValString возвращает значение по указателю или пустую строку.
func ValString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

type Row struct {
	ID          int     `json:"id"`
	Year        int     `json:"year"`
	Month       int     `json:"month"`
	BrandName   string  `json:"brandName"`
	ProductName string  `json:"productName"`
	NetworkName string  `json:"networkName"`
	MetricType  string  `json:"metricType"`
	MetricValue float64 `json:"metricValue"`
	UnRub       *string `json:"un_rub"`
	Segment     *string `json:"segment"`
	Channel     *string `json:"channel"`
	UpdatedAt   *string `json:"updated_at"`
}

type PromoRow struct {
	ID                      int      `json:"id"`
	NetworkName             *string  `json:"network_name"`
	KAM                     *string  `json:"kam"`
	IDDirectum              *string  `json:"id_directum"`
	DSNumber                *string  `json:"ds_number"`
	Year                    int      `json:"year"`
	Month                   *int     `json:"month"`
	Quarter                 *int     `json:"quarter"`
	SKU                     *string  `json:"sku"`
	Brand                   *string  `json:"brand"`
	BrandAS                 *string  `json:"brand_as"`
	Mechanics               *string  `json:"mechanics"`
	DiscountAmount          *float64 `json:"discount_amount"`
	GTNOpex                 *string  `json:"gtn_opex"`
	Conditions              *string  `json:"conditions"`
	Comments                *string  `json:"comments"`
	BaselineUnits           *float64 `json:"baseline_units"`
	BaselineRub             *float64 `json:"baseline_rub"`
	PlanPromoUnits          *float64 `json:"plan_promo_units"`
	PlanPromoRub            *float64 `json:"plan_promo_rub"`
	PlanInvestmentsRub      *float64 `json:"plan_investments_rub"`
	PlanPromoUpliftUnits    *float64 `json:"plan_promo_uplift_units"`
	PlanPromoUpliftRub      *float64 `json:"plan_promo_uplift_rub"`
	PlanPromoUpliftPctUnits *float64 `json:"plan_promo_uplift_pct_units"`
	PlanPromoUpliftPctRub   *float64 `json:"plan_promo_uplift_pct_rub"`
	PlanInvestmentsPct      *float64 `json:"plan_investments_pct"`
	PlanROI                 *float64 `json:"plan_roi"`
	ContractPrice           *float64 `json:"contract_price"`
	GM                      *float64 `json:"gm"`
	TotalPharmacies         *int     `json:"total_pharmacies"`
	PromoPharmacies         *int     `json:"promo_pharmacies"`
	ActualPromoSalesUnits   *float64 `json:"actual_promo_sales_units"`
	ActualInvestments       *float64 `json:"actual_investments"`
	Status                  *string  `json:"status"`
	ActualPromoRub          *float64 `json:"actual_promo_rub"`
	ActualPromoUpliftUnits  *float64 `json:"actual_promo_uplift_units"`
	ActualPromoUpliftRub    *float64 `json:"actual_promo_uplift_rub"`
	ActualExternalEcomUnits *float64 `json:"actual_external_ecom_units"`
	ActualCorrectedBaseline *float64 `json:"actual_corrected_baseline"`
	ActualROI               *float64 `json:"actual_roi"`
	PlanVsFactRub           *float64 `json:"plan_vs_fact_rub"`
	PlanVsFactInvestments   *float64 `json:"plan_vs_fact_investments"`
	PromoChannel            *string  `json:"channel"`
	Agreement1              *string  `json:"agreement1"`
	Agreement2              *string  `json:"agreement2"`
	Date                    *string  `json:"date"`
	CreatedAt               *string  `json:"created_at"`
	UpdatedAt               *string  `json:"updated_at"`
}

type HistoryRow struct {
	ID                     int      `json:"id"`
	NetworkName            *string  `json:"network_name"`
	Year                   int      `json:"year"`
	Month                  int      `json:"month"`
	Mechanics              *string  `json:"mechanics"`
	SKU                    *string  `json:"sku"`
	BaselineUnits          *float64 `json:"baseline_units"`
	PlanPromoUnits         *float64 `json:"plan_promo_units"`
	ActualPromoSalesUnits  *float64 `json:"actual_promo_sales_units"`
	PlanPromoUpliftUnits   *float64 `json:"plan_promo_uplift_units"`
	ActualPromoUpliftUnits *float64 `json:"actual_promo_uplift_units"`
	PlanROI                *float64 `json:"plan_roi"`
	ActualROI              *float64 `json:"actual_roi"`
}

type CommentRow struct {
	ID          int     `json:"id"`
	PromoID     int     `json:"promo_id"`
	UserName    string  `json:"user_name"`
	Role        string  `json:"role"`
	CommentText string  `json:"comment_text"`
	CreatedAt   *string `json:"created_at"`
}

type DrilldownRow struct {
	Year       int     `json:"year"`
	Month      int     `json:"month"`
	MetricType string  `json:"metricType"`
	TotalValue float64 `json:"totalValue"`
	UnRub      *string `json:"un_rub"`
	Segment    *string `json:"segment"`
	Channel    *string `json:"channel"`
}

type NetworkGeo struct {
	KAM          string `json:"kam"`
	NetworkType  string `json:"network_type"`
	Top20Segment string `json:"top20_segment"`
	KeyRegion    string `json:"key_region"`
}

type LastSKUData struct {
	ContractPrice   float64 `json:"contract_price"`
	GM              float64 `json:"gm"`
	TotalPharmacies int64   `json:"total_pharmacies"`
	KeyRegion       string  `json:"key_region"`
	Top20Segment    string  `json:"top20_segment"`
	OlapPrice       float64 `json:"olap_price"`
}

// PromoRowDB — полная структура для записи/чтения строки в БД.
// Числовые поля — указатели, чтобы отличать NULL от 0.
// Строковые поля — value-типы (пустая строка = NULL/отсутствует).
type PromoRowDB struct {
	ID                           int      `json:"id"`
	NetworkName                  string   `json:"network_name"`
	KAM                          string   `json:"kam"`
	Brand                        string   `json:"brand"`
	BrandAS                      string   `json:"brand_as"`
	SKU                          string   `json:"sku"`
	Year                         int      `json:"year"`
	Month                        int      `json:"month"`
	Quarter                      *int     `json:"quarter"`
	Mechanics                    string   `json:"mechanics"`
	GTNOpex                      string   `json:"gtn_opex"`
	BaselineUnits                *float64 `json:"baseline_units"`
	BaselineRub                  *float64 `json:"baseline_rub"`
	PlanPromoUnits               *float64 `json:"plan_promo_units"`
	PlanPromoRub                 *float64 `json:"plan_promo_rub"`
	PlanInvestmentsRub           *float64 `json:"plan_investments_rub"`
	PlanPromoUpliftUnits         *float64 `json:"plan_promo_uplift_units"`
	PlanPromoUpliftRub           *float64 `json:"plan_promo_uplift_rub"`
	PlanPromoUpliftPctUnits      *float64 `json:"plan_promo_uplift_pct_units"`
	PlanPromoUpliftPctRub        *float64 `json:"plan_promo_uplift_pct_rub"`
	PlanInvestmentsPct           *float64 `json:"plan_investments_pct"`
	PlanROI                      *float64 `json:"plan_roi"`
	ContractPrice                *float64 `json:"contract_price"`
	GM                           *float64 `json:"gm"`
	IDDirectum                   string   `json:"id_directum"`
	DSNumber                     string   `json:"ds_number"`
	DiscountAmount               *float64 `json:"discount_amount"`
	Conditions                   string   `json:"conditions"`
	Comments                     string   `json:"comments"`
	EcomSegment                  string   `json:"ecom_segment"`
	TotalPharmacies              *int     `json:"total_pharmacies"`
	PromoPharmacies              *int     `json:"promo_pharmacies"`
	Status                       string   `json:"status"`
	Date                         string   `json:"date"`
	KeyRegion                    string   `json:"key_region"`
	Top20Segment                 string   `json:"top20_segment"`
	OlapPrice                    *float64 `json:"olap_price"`
	PlanPromoCipOlap             *float64 `json:"plan_promo_cip_olap"`
	FactPromoCipOlap             *float64 `json:"fact_promo_cip_olap"`
	PlanPromoUpliftCipOlap       *float64 `json:"plan_promo_uplift_cip_olap"`
	FactPromoUpliftCipOlap       *float64 `json:"fact_promo_uplift_cip_olap"`
	ActualPromoSalesUnits        *float64 `json:"actual_promo_sales_units"`
	ActualInvestments            *float64 `json:"actual_investments"`
	ActualPromoRub               *float64 `json:"actual_promo_rub"`
	ActualPromoUpliftUnits       *float64 `json:"actual_promo_uplift_units"`
	ActualPromoUpliftRub         *float64 `json:"actual_promo_uplift_rub"`
	ActualExternalEcomUnits      *float64 `json:"actual_external_ecom_units"`
	ActualCorrectedBaseline      *float64 `json:"actual_corrected_baseline"`
	Agreement1                   string   `json:"agreement1"`
	Agreement2                   string   `json:"agreement2"`
	NetPromoUpliftRub            *float64 `json:"net_promo_uplift_rub"`
	NetPromoUpliftPct            *float64 `json:"net_promo_uplift_pct"`
	ActualInvestmentsPct         *float64 `json:"actual_investments_pct"`
	ActualROI                    *float64 `json:"actual_roi"`
	ActualPromoRubWoEcom         *float64 `json:"actual_promo_rub_wo_ecom"`
	ActualPromoUpliftUnitsWoEcom *float64 `json:"actual_promo_uplift_units_wo_ecom"`
	ActualPromoUpliftRubWoEcom   *float64 `json:"actual_promo_uplift_rub_wo_ecom"`
	NetPromoUpliftRubWoEcom      *float64 `json:"net_promo_uplift_rub_wo_ecom"`
	NetPromoUpliftPctWoEcom      *float64 `json:"net_promo_uplift_pct_wo_ecom"`
	ActualInvestmentsPctWoEcom   *float64 `json:"actual_investments_pct_wo_ecom"`
	ActualROIWoEcom              *float64 `json:"actual_roi_wo_ecom"`
	PlanVsFactRub                *float64 `json:"plan_vs_fact_rub"`
	PlanVsFactInvestments        *float64 `json:"plan_vs_fact_investments"`
	TurnoverPerPoint             *float64 `json:"turnover_per_point"`
	TurnoverPerPointPromo        *float64 `json:"turnover_per_point_promo"`
	UpdatedAt                    string   `json:"updated_at"`
}

type AuditLogRow struct {
	ID            int     `json:"id"`
	EntityType    string  `json:"entity_type"`
	EntityID      int     `json:"entity_id"`
	UserName      string  `json:"user_name"`
	ActionType    string  `json:"action_type"`
	ChangedFields *string `json:"changed_fields"` // JSON
	CreatedAt     *string `json:"created_at"`
}

type ApprovalRow struct {
	ID                    int      `json:"id"`
	NetworkName           *string  `json:"network_name"`
	BrandAS               *string  `json:"brand_as"`
	SKU                   *string  `json:"sku"`
	Mechanics             *string  `json:"mechanics"`
	Year                  int      `json:"year"`
	Month                 *int     `json:"month"`
	BaselineUnits         *float64 `json:"baseline_units"`
	PlanPromoUnits        *float64 `json:"plan_promo_units"`
	ActualPromoSalesUnits *float64 `json:"actual_promo_sales_units"`
	PlanInvestmentsRub    *float64 `json:"plan_investments_rub"`
	PlanROI               *float64 `json:"plan_roi"`
	ActualROI             *float64 `json:"actual_roi"`
	Conditions            *string  `json:"conditions"`
	Comments              *string  `json:"comments"`           // история переписки КАМ + согласующих
	Agreement1            *string  `json:"agreement1"`         // обратная совместимость
	Agreement1Status      *string  `json:"agreement1_status"`  // pending/approved/rejected/commented
	Agreement1Comment     *string  `json:"agreement1_comment"` // текст комментария
	Agreement2            *string  `json:"agreement2"`         // обратная совместимость
	Agreement2Status      *string  `json:"agreement2_status"`
	Agreement2Comment     *string  `json:"agreement2_comment"`
	Status                *string  `json:"status"`
	HistoricalCount       int      `json:"historical_count"`
	AvgHistoricalROI      *float64 `json:"avg_historical_roi"`
}
