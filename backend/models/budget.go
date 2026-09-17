package models

// BudgetCell contains server-calculated values in rubles (percentages for Pct).
type BudgetCell struct {
	A        *float64    `json:"a"`
	B        *float64    `json:"b"`
	Delta    *float64    `json:"delta"`
	Mark     string      `json:"mark"`
	Editable bool        `json:"editable"`
	Edited   *BudgetEdit `json:"edited"`
}

type BudgetEdit struct {
	Base  float64 `json:"base"`
	Value float64 `json:"value"`
	Who   string  `json:"who"`
	When  string  `json:"when"`
}
type BudgetSources struct {
	To          []string `json:"to"`
	Investments []string `json:"investments"`
}
type BudgetVersion struct {
	ID        int           `json:"id"`
	Year      int           `json:"year"`
	Code      string        `json:"code"`
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	FrozenAt  string        `json:"frozenAt"`
	CreatedBy string        `json:"createdBy"`
	UpdatedAt string        `json:"updatedAt"`
	Sources   BudgetSources `json:"sources"`
}
type BudgetPromo struct {
	ID        int      `json:"id"`
	NetworkID int      `json:"networkId"`
	Network   string   `json:"network"`
	Brand     string   `json:"brand"`
	Quarter   int      `json:"quarter"`
	Month     int      `json:"month"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Mechanics string   `json:"mechanics"`
	PlanRub   float64  `json:"planRub"`
	FactRub   float64  `json:"factRub"`
	BudgetRub float64  `json:"budgetRub"`
	BudgetNet float64  `json:"budgetNet"`
	Included  bool     `json:"included"`
	A         *float64 `json:"a"`
	B         *float64 `json:"b"`
	Delta     *float64 `json:"delta"`
	Change    string   `json:"change"`
}
type BudgetPromoResponse struct {
	Data      []BudgetPromo `json:"data"`
	Total     int           `json:"total"`
	Sum       float64       `json:"sum"`
	Top10Pct  *float64      `json:"top10Pct"`
	UpdatedAt string        `json:"updatedAt"`
}

type BudgetLine struct {
	Q    []BudgetCell `json:"q"`
	Year BudgetCell   `json:"year"`
}

type BudgetBrand struct {
	VATFactors   []float64     `json:"vatFactors"`
	OlapTo       BudgetLine    `json:"olapTo"`
	Sales        BudgetLine    `json:"sales"`
	SalesSS      BudgetLine    `json:"salesSS"`
	SalesSSWO    BudgetLine    `json:"salesSSWO"`
	SalesPURE    BudgetLine    `json:"salesPURE"`
	SalesOMNI    BudgetLine    `json:"salesOMNI"`
	SalesMP      BudgetLine    `json:"salesMP"`
	Brand        string        `json:"brand"`
	NetworkID    int           `json:"networkId"`
	To           BudgetLine    `json:"to"`
	Plan         BudgetLine    `json:"plan"`
	Fact         BudgetLine    `json:"fact"`
	GtnPlan      BudgetLine    `json:"gtnPlan"`
	GtnContract  BudgetLine    `json:"gtnContract"`
	OpexContract BudgetLine    `json:"opexContract"`
	GtnPromo     BudgetLine    `json:"gtnPromo"`
	OpexPromo    BudgetLine    `json:"opexPromo"`
	Investments  BudgetLine    `json:"investments"`
	Pct          BudgetLine    `json:"pct"`
	Networks     []BudgetBrand `json:"networks"`
}

type BudgetResponse struct {
	TypeMembers         map[string][]int `json:"typeMembers"`
	Year                int              `json:"year"`
	Version             string           `json:"version"`
	Brands              []BudgetBrand    `json:"brands"`
	Total               BudgetBrand      `json:"total"`
	QuarterStates       []string         `json:"quarterStates"`
	NetworkTypes        []string         `json:"networkTypes"`
	ForecastCoveragePct *float64         `json:"forecastCoveragePct"`
	VersionInfo         BudgetVersion    `json:"versionInfo"`
	CompareInfo         BudgetVersion    `json:"compareInfo"`
	Compare             string           `json:"compare"`
	CompareStates       []string         `json:"compareStates"`
	CanEdit             bool             `json:"canEdit"`
	EditReason          string           `json:"editReason"`
}
