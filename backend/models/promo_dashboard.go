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
	// Отклонения факта от сопоставимого плана. Продажи и uplift даны в обеих
	// единицах — переключатель «уп./₽» не должен пересчитывать их в браузере.
	SalesVarianceUnits    *float64 `json:"salesVarianceUnits"`
	SalesVarianceRub      *float64 `json:"salesVarianceRub"`
	InvestmentVarianceRub *float64 `json:"investmentVarianceRub"`
	UpliftVarianceUnits   *float64 `json:"upliftVarianceUnits"`
	UpliftVarianceRub     *float64 `json:"upliftVarianceRub"`
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

// PromoApprovalAccessResponse — доступна ли пользователю страница согласования.
//
// Роль сама по себе ответа не даёт: КАМ допускается к согласованию только при
// наличии закрепления за чужими КАМами, а ступень следует из него же. Поэтому
// интерфейс спрашивает доступ у сервера, а не выводит его из роли.
type PromoApprovalAccessResponse struct {
	Allowed bool `json:"allowed"`
	// ApprovalRole пуст у администратора: он выбирает ступень сам.
	ApprovalRole string `json:"approval_role"`
	// Scoped — доступ ограничен закреплением, а не открыт по всей очереди.
	Scoped bool `json:"scoped"`
}
