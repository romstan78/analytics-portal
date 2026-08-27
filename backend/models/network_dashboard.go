package models

// ─── Витрина реестра сетей ──────────────────────────────────────────────────
//
// Дашборд не заводит собственную математику: он складывает те же квартальные
// итоги (NetworkPlanTotals), которые показывает карточка сети, и потому обязан
// сходиться с ней до копейки.
//
// Отличие одно — факт и EAC приходят из помесячных таблиц, а не из квартальных
// колонок tbl_NetworkPlans. Те заполняются только загрузкой отгрузок и
// сохранением прогноза, так что по сети, которую никто не открывал, витрина
// показала бы план без факта.

// NetworkDashboardMetrics — агрегат одного среза витрины.
//
// Плановый объём везде считается обязательством по контракту: валовый пул
// входит целиком, даже если бренды разобрали его не полностью. Исключение —
// разрез по брендам, где у строки есть только собственный план бренда;
// нераспределённый остаток пула вынесен в UndistributedRub отдельно.
type NetworkDashboardMetrics struct {
	NetworkCount int `json:"networkCount"`
	BrandCount   int `json:"brandCount"`

	PlanRub float64 `json:"planRub"`
	FactRub float64 `json:"factRub"`
	// Упаковки идут рядом с рублями, а не вместо них: часть брендов ведут
	// именно в упаковках, и витрина переключается между двумя единицами.
	PlanUnits float64 `json:"planUnits"`
	FactUnits float64 `json:"factUnits"`
	EACUnits  float64 `json:"eacUnits"`
	// EACRub — ожидаемый итог периода: факт закрытых месяцев плюс официальный
	// прогноз открытых. Месяц без прогноза добавляет только уже отгруженное,
	// планом он не достраивается: иначе сеть без прогноза выглядела бы
	// выполняющей план ровно на 100%. Сколько таких месяцев — в
	// OpenCellsWithoutForecast.
	EACRub float64 `json:"eacRub"`

	CompletionPct    *float64 `json:"completionPct"`
	EACCompletionPct *float64 `json:"eacCompletionPct"`
	GapRub           float64  `json:"gapRub"`

	// Инвестиции. Сети работают с разными ставками НДС, поэтому складывать и
	// сравнивать их между собой можно только в базе «без НДС»: на ней и
	// считается InvestmentVarianceRub.
	PlanInvestmentsRub      float64  `json:"planInvestmentsRub"`
	PlanInvestmentsRubNet   float64  `json:"planInvestmentsRubNet"`
	FactInvestmentsRub      float64  `json:"factInvestmentsRub"`
	FactInvestmentsRubNet   float64  `json:"factInvestmentsRubNet"`
	EACInvestmentsRub       float64  `json:"eacInvestmentsRub"`
	EACInvestmentsRubNet    float64  `json:"eacInvestmentsRubNet"`
	InvestmentVarianceRub   float64  `json:"investmentVarianceRub"`
	EffectiveInvestmentsPct *float64 `json:"effectiveInvestmentsPct"`

	// UndistributedRub — остаток валового пула, не разобранный брендами.
	// nil означает, что пула в срезе нет вовсе, а не что остаток нулевой.
	UndistributedRub *float64 `json:"undistributedRub"`

	// Готовность данных. Ячейка — это сеть × бренд × месяц в границах среза;
	// брендом считается тот, у кого есть строка плана в этом квартале.
	ClosedCells              int      `json:"closedCells"`
	ClosedCellsWithFact      int      `json:"closedCellsWithFact"`
	FactCoveragePct          *float64 `json:"factCoveragePct"`
	OpenCellsWithoutForecast int      `json:"openCellsWithoutForecast"`

	// Прошлый год, тот же диапазон кварталов. nil означает, что данных за
	// сопоставимый период нет вовсе, — это не то же самое, что ноль продаж.
	PrevPlanRub   *float64 `json:"prevPlanRub"`
	PrevFactRub   *float64 `json:"prevFactRub"`
	PrevFactUnits *float64 `json:"prevFactUnits"`
	FactYoYPct    *float64 `json:"factYoyPct"`
	PlanYoYPct    *float64 `json:"planYoyPct"`

	// Промо, проведённые в срезе. Канал берётся из справочника механик,
	// где он размечен как онлайн/оффлайн.
	PromoCount          int     `json:"promoCount"`
	PromoOnlineCount    int     `json:"promoOnlineCount"`
	PromoOfflineCount   int     `json:"promoOfflineCount"`
	PromoInvestmentsRub float64 `json:"promoInvestmentsRub"`
}

// NetworkDashboardPromoTag — метка проведённого промо: короткий код механики
// для показа на плитке и полное название для подсказки.
type NetworkDashboardPromoTag struct {
	Code      string  `json:"code"`
	Mechanics string  `json:"mechanics"`
	Channel   string  `json:"channel"` // онлайн | оффлайн | не указан
	Count     int     `json:"count"`
	PlanRub   float64 `json:"planRub"`
}

// NetworkDashboardPeriodPoint — точка квартального тренда.
type NetworkDashboardPeriodPoint struct {
	Year    int                     `json:"year"`
	Quarter int                     `json:"quarter"`
	Metrics NetworkDashboardMetrics `json:"metrics"`
}

// NetworkDashboardMonthPoint — точка месячного тренда.
//
// Тип отдельный, а не общий с кварталом, намеренно: на месяце реальны не все
// величины. Инвестиций здесь нет — процент инвестиций ведётся на квартальной
// строке бренда, и раскладывать его по месяцам значило бы показывать
// вычисленное как измеренное.
//
// План месяца — квартальное обязательство, распределённое по схеме из профиля
// сети (те же три процента, что применяет карточка). Это раскладка плана, а не
// отдельный план на месяц: помесячных планов в реестре не существует.
type NetworkDashboardMonthPoint struct {
	Year    int `json:"year"`
	Month   int `json:"month"`
	Quarter int `json:"quarter"`

	PlanRub   float64 `json:"planRub"`
	PlanUnits float64 `json:"planUnits"`
	FactRub   float64 `json:"factRub"`
	FactUnits float64 `json:"factUnits"`
	EACRub    float64 `json:"eacRub"`
	EACUnits  float64 `json:"eacUnits"`

	PrevFactRub   *float64 `json:"prevFactRub"`
	PrevFactUnits *float64 `json:"prevFactUnits"`

	PromoCount        int `json:"promoCount"`
	PromoOnlineCount  int `json:"promoOnlineCount"`
	PromoOfflineCount int `json:"promoOfflineCount"`

	// Closed — месяц уже закрыт, то есть его факт окончательный.
	Closed bool `json:"closed"`
	// CellsWithoutForecast — открытые ячейки бренда без официального прогноза.
	CellsWithoutForecast int `json:"cellsWithoutForecast"`
}

// NetworkDashboardBreakdown — агрегат одного измерения: сеть, бренд или КАМ.
// NetworkID заполнен только в разрезе сетей — по нему открывается карточка.
type NetworkDashboardBreakdown struct {
	Name      string                  `json:"name"`
	NetworkID *int                    `json:"networkId"`
	KAM       *string                 `json:"kam"`
	Metrics   NetworkDashboardMetrics `json:"metrics"`
}

// NetworkDashboardCell — ячейка тепловой карты «сеть × квартал».
type NetworkDashboardCell struct {
	NetworkID int                        `json:"networkId"`
	Name      string                     `json:"name"`
	Quarter   int                        `json:"quarter"`
	Metrics   NetworkDashboardMetrics    `json:"metrics"`
	PromoTags []NetworkDashboardPromoTag `json:"promoTags"`
}

// NetworkDashboardResponse — полный ответ /api/networks/dashboard.
type NetworkDashboardResponse struct {
	Year int `json:"year"`
	// SelectedQuarters — кварталы среза по возрастанию. Набор, а не диапазон:
	// сравнить Q1 с Q3 границами «от и до» нельзя.
	SelectedQuarters []int `json:"selectedQuarters"`
	AvailableYears   []int `json:"availableYears"`

	Summary         NetworkDashboardMetrics       `json:"summary"`
	Quarters        []NetworkDashboardPeriodPoint `json:"quarters"`
	Months          []NetworkDashboardMonthPoint  `json:"months"`
	Networks        []NetworkDashboardBreakdown   `json:"networks"`
	Brands          []NetworkDashboardBreakdown   `json:"brands"`
	KAMs            []NetworkDashboardBreakdown   `json:"kams"`
	NetworkQuarters []NetworkDashboardCell        `json:"networkQuarters"`
}
