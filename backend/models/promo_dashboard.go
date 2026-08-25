package models

// PromoDashboardMetrics — агрегаты одного промо-среза. Плановые показатели
// содержат весь выбранный срез, comparable-поля — только строки, где есть
// одновременно фактические продажи и фактические инвестиции.
type PromoDashboardMetrics struct {
	PromoCount                   int      `json:"promoCount"`
	FactReadyCount               int      `json:"factReadyCount"`
	FactCoveragePct              *float64 `json:"factCoveragePct"`
	PlanUnits                    float64  `json:"planUnits"`
	ComparablePlanUnits          float64  `json:"comparablePlanUnits"`
	ActualUnits                  *float64 `json:"actualUnits"`
	PlanInvestmentsRub           float64  `json:"planInvestmentsRub"`
	ComparablePlanInvestmentsRub float64  `json:"comparablePlanInvestmentsRub"`
	ActualInvestmentsRub         *float64 `json:"actualInvestmentsRub"`
	PlanUpliftUnits              float64  `json:"planUpliftUnits"`
	ComparablePlanUpliftUnits    float64  `json:"comparablePlanUpliftUnits"`
	ActualUpliftUnits            *float64 `json:"actualUpliftUnits"`
	PlanROI                      *float64 `json:"planRoi"`
	ComparablePlanROI            *float64 `json:"comparablePlanRoi"`
	ActualROI                    *float64 `json:"actualRoi"`
	SalesCompletionPct           *float64 `json:"salesCompletionPct"`
	InvestmentCompletionPct      *float64 `json:"investmentCompletionPct"`
	SalesVarianceUnits           *float64 `json:"salesVarianceUnits"`
	InvestmentVarianceRub        *float64 `json:"investmentVarianceRub"`
}

// PromoDashboardTrendPoint — помесячная точка план-факт.
type PromoDashboardTrendPoint struct {
	Year    int                   `json:"year"`
	Month   int                   `json:"month"`
	Metrics PromoDashboardMetrics `json:"metrics"`
}

// PromoDashboardBreakdown — агрегат выбранного измерения.
type PromoDashboardBreakdown struct {
	Name    string                `json:"name"`
	Metrics PromoDashboardMetrics `json:"metrics"`
}

// PromoDashboardCalendarPoint — ячейка календаря для сети или бренда.
type PromoDashboardCalendarPoint struct {
	Name    string                `json:"name"`
	Year    int                   `json:"year"`
	Month   int                   `json:"month"`
	Metrics PromoDashboardMetrics `json:"metrics"`
}

// PromoDashboardResponse — полный ответ /api/promo/dashboard.
type PromoDashboardResponse struct {
	AvailableYears  []int                         `json:"availableYears"`
	Summary         PromoDashboardMetrics         `json:"summary"`
	Trend           []PromoDashboardTrendPoint    `json:"trend"`
	Networks        []PromoDashboardBreakdown     `json:"networks"`
	Brands          []PromoDashboardBreakdown     `json:"brands"`
	SKUs            []PromoDashboardBreakdown     `json:"skus"`
	Mechanics       []PromoDashboardBreakdown     `json:"mechanics"`
	NetworkCalendar []PromoDashboardCalendarPoint `json:"networkCalendar"`
	BrandCalendar   []PromoDashboardCalendarPoint `json:"brandCalendar"`
}
