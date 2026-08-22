package models

// Network — карточка сети в реестре: то, что не зависит от периода.
type Network struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	KAM         *string `json:"kam"`
	NetworkType string  `json:"network_type"` // regular | warehouse
	IsActive    bool    `json:"is_active"`
	CreatedAt   *string `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// NetworkPeriod — атрибуты сети, действующие на конкретный квартал.
// vat_included = сеть работает с НДС в этом квартале; НДС применяется
// только к инвестициям, планы от него не зависят.
// Тип контракта здесь не хранится: валовый объём — свойство бренда (NetworkPlan.InGross).
type NetworkPeriod struct {
	ID          int     `json:"id"`
	NetworkID   int     `json:"network_id"`
	Year        int     `json:"year"`
	Quarter     int     `json:"quarter"`
	VATIncluded bool    `json:"vat_included"`
	VATRate     float64 `json:"vat_rate"`
	UpdatedAt   string  `json:"updated_at"`
}

// NetworkPlan — строка плана: бренд на квартал.
// BrandAS = nil — строка общего объёма валового контракта (пул), в который
// входят бренды с InGross = true. Остальные бренды планируются отдельно.
type NetworkPlan struct {
	ID          int      `json:"id"`
	NetworkID   int      `json:"network_id"`
	Year        int      `json:"year"`
	Quarter     int      `json:"quarter"`
	BrandAS     *string  `json:"brand_as"`
	InGross     bool     `json:"in_gross"`   // бренд входит в валовый объём этого квартала
	PlanRub     *float64 `json:"plan_rub"`   // план как введён, НДС к нему не применяется
	PlanUnits   *float64 `json:"plan_units"` // задел под таблицу цен контракта
	FactRub     *float64 `json:"fact_rub"`   // факт отгрузок, заполняется загрузкой
	ForecastRub *float64 `json:"forecast_rub"`
	// Факт инвестиций приходит той же загрузкой; процентом не пересчитывается.
	FactInvestmentsRub *float64 `json:"fact_investments_rub"`
	InvestmentsPct     *float64 `json:"investments_pct"`
	InvestmentsRub     *float64 `json:"investments_rub"`     // расчётное: pct от plan_rub, до вычета НДС
	InvestmentsNet     *float64 `json:"investments_rub_net"` // расчётное: инвестиции с вычетом НДС
	// Расчётное: тот же процент, применённый к прогнозу объёма.
	ForecastInvestmentsRub *float64 `json:"forecast_investments_rub"`
	ForecastInvestmentsNet *float64 `json:"forecast_investments_rub_net"`
	UpdatedBy              *string  `json:"updated_by"`
	UpdatedAt              string   `json:"updated_at"`
}

// NetworkComment — комментарий к сети целиком либо к конкретной ячейке плана.
type NetworkComment struct {
	ID          int64   `json:"id"`
	NetworkID   int     `json:"network_id"`
	Year        *int    `json:"year"`
	Quarter     *int    `json:"quarter"`
	BrandAS     *string `json:"brand_as"`
	UserName    string  `json:"user_name"`
	Role        string  `json:"role"`
	CommentText string  `json:"comment_text"`
	CreatedAt   *string `json:"created_at"`
}
