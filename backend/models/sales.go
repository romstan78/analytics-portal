package models

// Структуры ответов интернет-продаж. Держатся в models, а не в хендлерах,
// чтобы контракт API был описан в одном месте: из этих типов генерируется
// frontend/src/types/api.generated.ts (см. backend/cmd/tsgen).

// SalesFilterOptions — справочники для панели фильтров.
type SalesFilterOptions struct {
	Year        []string `json:"year"`
	BrandName   []string `json:"brandName"`
	ProductName []string `json:"productName"`
	NetworkName []string `json:"networkName"`
	UnRub       []string `json:"un_rub"`
	Segment     []string `json:"segment"`
	Channel     []string `json:"channel"`
	// Связка сегмент↔канал: выбор одного сужает список другого.
	SegmentChannelMap map[string][]string `json:"segmentChannelMap"`
	ChannelSegmentMap map[string][]string `json:"channelSegmentMap"`
}

// SalesDataResponse — постраничная выборка строк.
// TotalRows отсутствует при all=true: выгрузка отдаётся потоком целиком.
type SalesDataResponse struct {
	Data      []Row `json:"data"`
	TotalRows *int  `json:"totalRows,omitempty"`
}

// SalesNetworkOptionsResponse — сети, по которым есть данные при текущих фильтрах.
type SalesNetworkOptionsResponse struct {
	NetworkName []string `json:"networkName"`
}

// DrilldownResponse — разбивка одной пары «бренд × сеть» по периодам.
type DrilldownResponse struct {
	BrandName   string         `json:"brandName"`
	NetworkName string         `json:"networkName"`
	Data        []DrilldownRow `json:"data"`
}

// ─── Иерархическая сводная ────────────────────────────────────────────────

// SalesPivotPeriod описывает одну числовую колонку. При детализации по
// кварталам/месяцам каждый год заканчивается колонкой kind=total.
type SalesPivotPeriod struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Year  int    `json:"year"`
	Kind  string `json:"kind"` // month | quarter | total
}

// SalesPivotNode — узел дерева channel → segment → network → sku.
// Values индексируются ключами из SalesPivotPeriod.
type SalesPivotNode struct {
	ID       string             `json:"id"`
	Level    string             `json:"level"`
	Name     string             `json:"name"`
	Values   map[string]float64 `json:"values"`
	Children []SalesPivotNode   `json:"children"`
}

// SalesPivotResponse используется одновременно экраном и Excel-выгрузкой.
type SalesPivotResponse struct {
	AnalysisYear     int                `json:"analysisYear"`
	Channel          string             `json:"channel"`
	Segments         []string           `json:"segments"`
	Unit             string             `json:"unit"`
	Granularity      string             `json:"granularity"`
	CurrencySource   string             `json:"currencySource"`
	Periods          []SalesPivotPeriod `json:"periods"`
	Rows             []SalesPivotNode   `json:"rows"`
	Totals           map[string]float64 `json:"totals"`
	PreviousTotalKey string             `json:"previousTotalKey"`
	CurrentTotalKey  string             `json:"currentTotalKey"`
	LeafRows         int                `json:"leafRows"`
}

// ─── Дашборд ───────────────────────────────────────────────────────────────

// SalesDashboardPoint — точка помесячного тренда.
type SalesDashboardPoint struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Value float64 `json:"value"`
}

// SalesDashboardRank — позиция рейтинга: имя и суммарное значение.
type SalesDashboardRank struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// SalesDashboardSeriesPoint — точка тренда с разбивкой по имени серии.
type SalesDashboardSeriesPoint struct {
	Name  string  `json:"name"`
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Value float64 `json:"value"`
}

// SalesDashboardFocusPoint — точка тренда выбранного продукта или сети.
type SalesDashboardFocusPoint struct {
	Type  string  `json:"type"` // product | network
	Name  string  `json:"name"`
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Value float64 `json:"value"`
}

// SalesDashboardNetworkBreakdown — объём сети в разрезе канала и сегмента.
type SalesDashboardNetworkBreakdown struct {
	Network string  `json:"network"`
	Channel string  `json:"channel"`
	Segment string  `json:"segment"`
	Value   float64 `json:"value"`
}

// SalesDashboardMetricComparison — год к году по одной единице измерения.
type SalesDashboardMetricComparison struct {
	Current  float64 `json:"current"`
	Previous float64 `json:"previous"`
}

// SalesDashboardMetricComparisons — сравнение во всех трёх единицах сразу.
type SalesDashboardMetricComparisons struct {
	Rub   SalesDashboardMetricComparison `json:"rub"`
	Eur   SalesDashboardMetricComparison `json:"eur"`
	Units SalesDashboardMetricComparison `json:"units"`
}

// SalesDashboardDriver — вклад измерения в изменение год к году.
type SalesDashboardDriver struct {
	Name         string   `json:"name"`
	Current      float64  `json:"current"`
	Previous     float64  `json:"previous"`
	Delta        float64  `json:"delta"`
	DeltaPercent *float64 `json:"deltaPercent"`
}

// SalesDashboardRankDetail — строка рейтинга с динамикой и долей.
type SalesDashboardRankDetail struct {
	Name       string   `json:"name"`
	Value      float64  `json:"value"`
	Previous   float64  `json:"previous"`
	YoYPercent *float64 `json:"yoyPercent"`
	Share      float64  `json:"share"`
	Rank       int      `json:"rank"`
	RankChange int      `json:"rankChange"`
}

// SalesDashboardEcomShare — доля Ecom внутри семейства сегментов OLAP.
// Applicable = false, если выбранный канал к семейству не относится.
type SalesDashboardEcomShare struct {
	Applicable    bool     `json:"applicable"`
	Family        string   `json:"family"`
	Full          float64  `json:"full"`
	WithoutEcom   float64  `json:"withoutEcom"`
	Ecom          float64  `json:"ecom"`
	Share         *float64 `json:"share"`
	PreviousFull  float64  `json:"previousFull"`
	PreviousEcom  float64  `json:"previousEcom"`
	PreviousShare *float64 `json:"previousShare"`
}

// SalesDashboardSummary — карточки верхнего ряда дашборда.
type SalesDashboardSummary struct {
	Total           float64  `json:"total"`
	AveragePerMonth float64  `json:"averagePerMonth"`
	ActiveNetworks  int      `json:"activeNetworks"`
	ActiveProducts  int      `json:"activeProducts"`
	Periods         int      `json:"periods"`
	LatestYear      int      `json:"latestYear"`
	LatestMonth     int      `json:"latestMonth"`
	LatestValue     *float64 `json:"latestValue"`
	PreviousValue   *float64 `json:"previousValue"`
	YearAgoValue    *float64 `json:"yearAgoValue"`
}

// SalesDashboardResponse — полный ответ /api/sales/dashboard.
type SalesDashboardResponse struct {
	AnalysisYear    int      `json:"analysisYear"`
	Channel         string   `json:"channel"`
	ChannelSegments []string `json:"channelSegments"`
	Segment         string   `json:"segment"`
	Segments        []string `json:"segments"`
	Unit            string   `json:"unit"`

	Summary           SalesDashboardSummary           `json:"summary"`
	Trend             []SalesDashboardPoint           `json:"trend"`
	PreviousYearTrend []SalesDashboardPoint           `json:"previousYearTrend"`
	MetricComparisons SalesDashboardMetricComparisons `json:"metricComparisons"`
	CurrencySource    string                          `json:"currencySource"`
	EcomShare         SalesDashboardEcomShare         `json:"ecomShare"`

	NetworkDrivers []SalesDashboardDriver     `json:"networkDrivers"`
	ProductDrivers []SalesDashboardDriver     `json:"productDrivers"`
	NetworkRanking []SalesDashboardRankDetail `json:"networkRanking"`
	ProductRanking []SalesDashboardRankDetail `json:"productRanking"`

	FocusTrends      []SalesDashboardFocusPoint       `json:"focusTrends"`
	TopNetworks      []SalesDashboardRank             `json:"topNetworks"`
	TopProducts      []SalesDashboardRank             `json:"topProducts"`
	SegmentTotals    []SalesDashboardRank             `json:"segmentTotals"`
	NetworkTrends    []SalesDashboardSeriesPoint      `json:"networkTrends"`
	ChannelTrends    []SalesDashboardSeriesPoint      `json:"channelTrends"`
	NetworkBreakdown []SalesDashboardNetworkBreakdown `json:"networkBreakdown"`
}
