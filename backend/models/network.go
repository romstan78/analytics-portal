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
	Month1Pct   float64  `json:"month1_pct"` // распределение квартального плана: первый месяц
	Month2Pct   float64  `json:"month2_pct"`
	Month3Pct   float64  `json:"month3_pct"`
	FactRub     *float64 `json:"fact_rub"` // факт отгрузок, заполняется загрузкой
	ForecastRub *float64 `json:"forecast_rub"`
	// Факт инвестиций приходит той же загрузкой; процентом не пересчитывается,
	// но база «без НДС» считается по ставке того же квартала.
	FactInvestmentsRub *float64 `json:"fact_investments_rub"`
	FactInvestmentsNet *float64 `json:"fact_investments_rub_net"` // расчётное
	InvestmentsPct     *float64 `json:"investments_pct"`
	InvestmentsRub     *float64 `json:"investments_rub"`     // расчётное: pct от plan_rub, до вычета НДС
	InvestmentsNet     *float64 `json:"investments_rub_net"` // расчётное: инвестиции с вычетом НДС
	// Расчётное: тот же процент, применённый к прогнозу объёма.
	ForecastInvestmentsRub *float64 `json:"forecast_investments_rub"`
	ForecastInvestmentsNet *float64 `json:"forecast_investments_rub_net"`
	UpdatedBy              *string  `json:"updated_by"`
	UpdatedAt              string   `json:"updated_at"`
}

// NetworkPlanTotals — итоги квартала для шапки сетки планов.
// НДС применяется только к инвестициям: план, факт и прогноз остаются теми,
// что ввёл КАМ или принесла загрузка.
//
// Валовый объём — свойство бренда: в одном квартале часть брендов входит в общий
// объём контракта (пул), часть планируется отдельно. Поэтому план по кварталу
// разложен на две части, а остаток к распределению считается только от брендов пула.
type NetworkPlanTotals struct {
	Quarter int `json:"quarter"`

	// Планы
	PlanRub          float64  `json:"plan_rub"`           // сумма планов всех брендов
	GrossBrandsPlan  float64  `json:"gross_brands_plan"`  // из них — бренды в валовом объёме
	SeparatePlanRub  float64  `json:"separate_plan_rub"`  // из них — бренды вне валового объёма
	GrossPoolRub     *float64 `json:"gross_pool_rub"`     // объём валового пула, строка без бренда
	Undistributed    *float64 `json:"undistributed"`      // пул − планы брендов пула
	ContractPlanRub  float64  `json:"contract_plan_rub"`  // обязательство: пул (или бренды пула) + отдельные
	GrossBrandsCount int      `json:"gross_brands_count"` // сколько брендов в пуле

	// Факт и прогноз
	FactRub          float64  `json:"fact_rub"`
	ForecastRub      float64  `json:"forecast_rub"`
	GrossPoolFactRub float64  `json:"gross_pool_fact_rub"`     // факт брендов пула
	GrossPoolFcstRub *float64 `json:"gross_pool_forecast_rub"` // прогноз объёма пула, строка без бренда

	// Инвестиции: от плана и от прогноза, до вычета НДС и с вычетом.
	// Факт инвестиций приходит загрузкой и процентом не пересчитывается,
	// поэтому база «без НДС» считается по ставке того же квартала.
	InvestmentsRub            float64 `json:"investments_rub"`
	InvestmentsRubNet         float64 `json:"investments_rub_net"`
	ForecastInvestmentsRub    float64 `json:"forecast_investments_rub"`
	ForecastInvestmentsRubNet float64 `json:"forecast_investments_rub_net"`
	FactInvestmentsRub        float64 `json:"fact_investments_rub"`
	FactInvestmentsRubNet     float64 `json:"fact_investments_rub_net"`
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

// ─── Ответы API реестра сетей ───────────────────────────────────────────────

// NetworkPlanResponse — данные вкладки «Планы» за год.
type NetworkPlanResponse struct {
	Network    Network             `json:"network"`
	Year       int                 `json:"year"`
	Periods    []NetworkPeriod     `json:"periods"`
	Plans      []NetworkPlan       `json:"plans"`
	Totals     []NetworkPlanTotals `json:"totals"`
	YearTotals NetworkPlanTotals   `json:"year_totals"`
}

// NetworkPlanSaveResponse — состояние года после сохранения.
type NetworkPlanSaveResponse struct {
	Message    string              `json:"message"`
	Year       int                 `json:"year"`
	Periods    []NetworkPeriod     `json:"periods"`
	Plans      []NetworkPlan       `json:"plans"`
	Totals     []NetworkPlanTotals `json:"totals"`
	YearTotals NetworkPlanTotals   `json:"year_totals"`
}

// NetworkPlanPreviewResponse — пересчёт несохранённого черновика.
// В БД ничего не пишется: ответ показывает, что получится после сохранения.
type NetworkPlanPreviewResponse struct {
	Year       int                 `json:"year"`
	Periods    []NetworkPeriod     `json:"periods"`
	Plans      []NetworkPlan       `json:"plans"`
	Totals     []NetworkPlanTotals `json:"totals"`
	YearTotals NetworkPlanTotals   `json:"year_totals"`
}

// NetworkListResponse — список сетей реестра.
type NetworkListResponse struct {
	Data []Network `json:"data"`
}

// NetworkSaveResponse — карточка сети после создания или правки.
type NetworkSaveResponse struct {
	Message string  `json:"message"`
	Data    Network `json:"data"`
}

// NetworkCommentsResponse — переписка по сети.
type NetworkCommentsResponse struct {
	Message string           `json:"message,omitempty"`
	Data    []NetworkComment `json:"data"`
}

// NetworkAuditResponse — история изменений карточки и планов.
type NetworkAuditResponse struct {
	Data []AuditLogRow `json:"data"`
}

// NetworkBrandsResponse — бренды, доступные для строк плана.
type NetworkBrandsResponse struct {
	Data []string `json:"data"`
}

// NetworkMonthlyFact — атомарный факт сети. SKU может быть пустым, если источник
// отдаёт готовый итог бренда; квартальные суммы всегда собираются из этих строк.
type NetworkMonthlyFact struct {
	ID                 int64    `json:"id"`
	NetworkID          int      `json:"network_id"`
	Year               int      `json:"year"`
	Month              int      `json:"month"`
	BrandAS            string   `json:"brand_as"`
	SKU                *string  `json:"sku"`
	FactRub            *float64 `json:"fact_rub"`
	FactUnits          *float64 `json:"fact_units"`
	FactInvestmentsRub *float64 `json:"fact_investments_rub"`
	IsFinal            bool     `json:"is_final"`
	SourceName         *string  `json:"source_name"`
	UpdatedAt          string   `json:"updated_at"`
}

// NetworkForecastLine — сохранённая ручная/официальная версия прогноза месяца.
// Строка без SKU является официальным прогнозом бренда; SKU-строки объясняют его
// снизу и могут заполняться постепенно.
type NetworkForecastLine struct {
	ID                     int64    `json:"id"`
	NetworkID              int      `json:"network_id"`
	Year                   int      `json:"year"`
	Month                  int      `json:"month"`
	BrandAS                string   `json:"brand_as"`
	SKU                    *string  `json:"sku"`
	ForecastRub            *float64 `json:"forecast_rub"`
	ForecastUnits          *float64 `json:"forecast_units"`
	ForecastInvestmentsRub *float64 `json:"forecast_investments_rub"`
	SystemForecastRub      *float64 `json:"system_forecast_rub"`
	SystemForecastUnits    *float64 `json:"system_forecast_units"`
	Confidence             *string  `json:"confidence"`
	AdjustmentReason       *string  `json:"adjustment_reason"`
	UpdatedBy              *string  `json:"updated_by"`
	UpdatedAt              string   `json:"updated_at"`
}

// NetworkPromoIndicator — компактная сводка запланированных промо в ячейке
// бренд+месяц. Детали промо остаются в промо-реестре и открываются по фильтру.
type NetworkPromoIndicator struct {
	Year               int     `json:"year"`
	Month              int     `json:"month"`
	BrandAS            string  `json:"brand_as"`
	PromoCount         int     `json:"promo_count"`
	ApprovedCount      int     `json:"approved_count"`
	DraftCount         int     `json:"draft_count"`
	PlanPromoUnits     float64 `json:"plan_promo_units"`
	PlanPromoRub       float64 `json:"plan_promo_rub"`
	PlanInvestmentsRub float64 `json:"plan_investments_rub"`
	PlanUpliftRub      float64 `json:"plan_uplift_rub"`
	PlanUpliftUnits    float64 `json:"plan_uplift_units"`
}

// NetworkForecastMonth — готовая ячейка прогноза. План распределён из квартала,
// факт загружен, официальный и системный прогнозы показаны рядом.
type NetworkForecastMonth struct {
	Year                   int      `json:"year"`
	Quarter                int      `json:"quarter"`
	Month                  int      `json:"month"`
	BrandAS                string   `json:"brand_as"`
	SKU                    *string  `json:"sku"`
	ContractPrice          *float64 `json:"contract_price"`
	PlanRub                *float64 `json:"plan_rub"`
	PlanInvestmentsRub     *float64 `json:"plan_investments_rub"`
	FactRub                *float64 `json:"fact_rub"`
	FactUnits              *float64 `json:"fact_units"`
	FactInvestmentsRub     *float64 `json:"fact_investments_rub"`
	ForecastRub            *float64 `json:"forecast_rub"`
	ForecastUnits          *float64 `json:"forecast_units"`
	ForecastInvestmentsRub *float64 `json:"forecast_investments_rub"`
	SystemForecastRub      *float64 `json:"system_forecast_rub"`
	SystemForecastUnits    *float64 `json:"system_forecast_units"`
	EACRub                 *float64 `json:"eac_rub"`
	EACInvestmentsRub      *float64 `json:"eac_investments_rub"`
	Confidence             *string  `json:"confidence"`
	AdjustmentReason       *string  `json:"adjustment_reason"`
	PromoCount             int      `json:"promo_count"`
	ApprovedPromoCount     int      `json:"approved_promo_count"`
	DraftPromoCount        int      `json:"draft_promo_count"`
	PromoPlanUnits         float64  `json:"promo_plan_units"`
	PromoPlanRub           float64  `json:"promo_plan_rub"`
	PromoInvestmentsRub    float64  `json:"promo_investments_rub"`
	PromoUpliftRub         float64  `json:"promo_uplift_rub"`
	IsClosed               bool     `json:"is_closed"`
	IsCurrent              bool     `json:"is_current"`
	UpdatedAt              string   `json:"updated_at"`
}

// NetworkForecastBrandTotals — итог одной строки бренда за выбранный квартал.
type NetworkForecastBrandTotals struct {
	BrandAS               string   `json:"brand_as"`
	PlanRub               float64  `json:"plan_rub"`
	FactRub               float64  `json:"fact_rub"`
	EACRub                float64  `json:"eac_rub"`
	CompletionPct         *float64 `json:"completion_pct"`
	GapRub                float64  `json:"gap_rub"`
	PlanInvestmentsRub    float64  `json:"plan_investments_rub"`
	FactInvestmentsRub    float64  `json:"fact_investments_rub"`
	EACInvestmentsRub     float64  `json:"eac_investments_rub"`
	InvestmentVarianceRub float64  `json:"investment_variance_rub"`
	PromoCount            int      `json:"promo_count"`
}

// NetworkForecastTotals — верхние карточки рабочего места прогноза.
type NetworkForecastTotals struct {
	PlanRub               float64  `json:"plan_rub"`
	FactRub               float64  `json:"fact_rub"`
	EACRub                float64  `json:"eac_rub"`
	CompletionPct         *float64 `json:"completion_pct"`
	GapRub                float64  `json:"gap_rub"`
	PlanInvestmentsRub    float64  `json:"plan_investments_rub"`
	FactInvestmentsRub    float64  `json:"fact_investments_rub"`
	EACInvestmentsRub     float64  `json:"eac_investments_rub"`
	InvestmentVarianceRub float64  `json:"investment_variance_rub"`
	PromoCount            int      `json:"promo_count"`
}

type NetworkForecastResponse struct {
	Network Network                      `json:"network"`
	Year    int                          `json:"year"`
	Quarter int                          `json:"quarter"`
	Months  []NetworkForecastMonth       `json:"months"`
	Brands  []NetworkForecastBrandTotals `json:"brands"`
	Totals  NetworkForecastTotals        `json:"totals"`
}

type NetworkForecastSaveResponse struct {
	Message string                  `json:"message"`
	Data    NetworkForecastResponse `json:"data"`
}

// NetworkContractPrice — цена договора с периодом действия и последней
// доступной OLAP-ценой для сравнения.
type NetworkContractPrice struct {
	ID            int64    `json:"id"`
	NetworkID     int      `json:"network_id"`
	BrandAS       string   `json:"brand_as"`
	SKU           string   `json:"sku"`
	ContractPrice float64  `json:"contract_price"`
	ValidFrom     string   `json:"valid_from"`
	ValidTo       string   `json:"valid_to"`
	SourceType    string   `json:"source_type"`
	SourceYear    *int     `json:"source_year"`
	SourceMonth   *int     `json:"source_month"`
	IsConfirmed   bool     `json:"is_confirmed"`
	OlapPrice     *float64 `json:"olap_price"`
	OlapYear      *int     `json:"olap_year"`
	OlapMonth     *int     `json:"olap_month"`
	UpdatedBy     *string  `json:"updated_by"`
	UpdatedAt     string   `json:"updated_at"`
}

type NetworkPricesResponse struct {
	Network Network                `json:"network"`
	Year    int                    `json:"year"`
	Data    []NetworkContractPrice `json:"data"`
}

type NetworkPricesSaveResponse struct {
	Message string                `json:"message"`
	Data    NetworkPricesResponse `json:"data"`
}
