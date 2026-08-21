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
type NetworkPeriod struct {
	ID           int     `json:"id"`
	NetworkID    int     `json:"network_id"`
	Year         int     `json:"year"`
	Quarter      int     `json:"quarter"`
	VATIncluded  bool    `json:"vat_included"`
	VATRate      float64 `json:"vat_rate"`
	ContractType string  `json:"contract_type"` // regular | gross
	UpdatedAt    string  `json:"updated_at"`
}

// NetworkPlan — строка плана: бренд на квартал.
// BrandAS = nil — строка общего объёма валового контракта.
type NetworkPlan struct {
	ID             int      `json:"id"`
	NetworkID      int      `json:"network_id"`
	Year           int      `json:"year"`
	Quarter        int      `json:"quarter"`
	BrandAS        *string  `json:"brand_as"`
	PlanRub        *float64 `json:"plan_rub"`   // план как введён, НДС к нему не применяется
	PlanUnits      *float64 `json:"plan_units"` // задел под таблицу цен контракта
	InvestmentsPct *float64 `json:"investments_pct"`
	InvestmentsRub *float64 `json:"investments_rub"`     // расчётное: pct от plan_rub, до вычета НДС
	InvestmentsNet *float64 `json:"investments_rub_net"` // расчётное: инвестиции с вычетом НДС
	UpdatedBy      *string  `json:"updated_by"`
	UpdatedAt      string   `json:"updated_at"`
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
