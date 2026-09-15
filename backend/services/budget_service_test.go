package services

import (
	"testing"
	"time"

	"backend/models"
	"backend/repository"

	"github.com/xuri/excelize/v2"
)

func TestBudgetMixedQuarterInvestmentSources(t *testing.T) {
	c := dashboardCase{networks: []models.Network{dashboardNetwork(1, "Сеть", "КАМ")}}
	for q := 1; q <= 4; q++ {
		c.plans = append(c.plans, models.NetworkPlan{NetworkID: 1, Year: 2026, Quarter: q, BrandAS: brandPtr("Бренд"), PlanRub: models.PtrFloat(1000), InvestmentsPct: models.PtrFloat(10)})
	}
	c.plans[0].PaidInvestmentsRub = models.PtrFloat(45)
	got := AggregateBudgetRegistry(c.data(), BudgetFilter{Year: 2026, Gross: true, InvestmentSources: [4]string{"fact", "forecast", "plan", "plan"}}, dashboardNow)
	if v := got.Total.GtnContract.Q[0]; *v.A != 45 || v.Mark != "paid" {
		t.Fatalf("Q1: %+v", v)
	}
	if v := got.Total.GtnContract.Q[1]; v.Mark != "forecast" {
		t.Fatalf("Q2: %+v", v)
	}
	for _, q := range []int{2, 3} {
		if v := got.Total.GtnContract.Q[q]; *v.A != 100 || v.Mark != "budget" {
			t.Fatalf("Q%d: %+v", q+1, v)
		}
	}
	if got.Total.GtnContract.Year.Mark != "partial" {
		t.Fatal("mixed annual source must retain a mixed marker")
	}
}

func TestBudgetReconcilesRegistryAndPool(t *testing.T) {
	c := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Сеть", "КАМ")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, PlanRub: models.PtrFloat(1200)},
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), InGross: true, PlanRub: models.PtrFloat(1000), InvestmentsPct: models.PtrFloat(10)},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Альфа", 300, 0), dashboardFact(1, 2, "Альфа", 400, 0), dashboardFact(1, 3, "Альфа", 500, 0)},
		opex:  []models.NetworkOpexBudgetRow{{NetworkID: 1, Year: 2026, Month: 1, BrandAS: "Альфа", AmountRub: 120.12, AmountNet: 100.1}},
	}
	b := AggregateBudgetRegistry(c.data(), BudgetFilter{Year: 2026}, dashboardNow)
	d := AggregateNetworkDashboard(c.data(), dashboardFilter(1), dashboardNow)
	for name, pair := range map[string][2]float64{
		"plan": {*b.Total.Plan.Q[0].A, d.Summary.PlanRub}, "fact": {*b.Total.Fact.Q[0].A, d.Summary.FactRub},
		"eac": {*b.Total.To.Q[0].A, d.Summary.EACRub}, "gtnPlan": {*b.Total.GtnPlan.Q[0].A, d.Summary.PlanInvestmentsRubNet},
		"opex": {*b.Total.OpexContract.Q[0].A, d.Summary.RegistryOpexBudgetRubNet},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s: budget=%v dashboard=%v", name, pair[0], pair[1])
		}
	}
	found := false
	for _, brand := range b.Brands {
		if brand.Brand == "Нераспределённый остаток пула" {
			found = true
			if *brand.Plan.Q[0].A != 200 {
				t.Fatalf("pool: %+v", brand.Plan.Q[0])
			}
		}
	}
	if !found {
		t.Fatal("missing pool remainder")
	}
}

func TestBudgetAnnualPercentUsesTotals(t *testing.T) {
	r := budgetBrand("Бренд", 0, [4]budgetAmounts{{to: 100, gtn: 10}, {to: 900, gtn: 180}}, BudgetFilter{})
	if *r.Pct.Year.A != 19 {
		t.Fatalf("annual percentage = %v", *r.Pct.Year.A)
	}
	if r.Pct.Q[2].A != nil {
		t.Fatal("zero denominator must have no percentage")
	}
	p := budgetBrand("Бренд", 0, [4]budgetAmounts{{to: 100, gtn: 10, opex: 20, promoGTN: 30, promoOPEX: 40}}, BudgetFilter{Source: "promo", Kind: "opex"})
	if *p.Investments.Q[0].A != 40 {
		t.Fatal("source/type intersection is incorrect")
	}
}

func TestBudgetSourceChoicesAreIndependentOfDateAndPayments(t *testing.T) {
	c := dashboardCase{networks: []models.Network{dashboardNetwork(1, "Сеть", "КАМ")}, plans: []models.NetworkPlan{{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Бренд"), PlanRub: models.PtrFloat(1000), InvestmentsPct: models.PtrFloat(10), PaidInvestmentsRub: models.PtrFloat(45)}}, facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Бренд", 1200, 700)}}
	f := BudgetFilter{Year: 2026, Gross: true, ToSources: [4]string{"fact"}, InvestmentSources: [4]string{"plan"}}
	r := AggregateBudgetRegistry(c.data(), f, dashboardNow)
	if *r.Total.To.Q[0].A != 1200 || *r.Total.GtnContract.Q[0].A != 100 {
		t.Fatalf("independent fact TO and plan GTN: %+v", r.Total)
	}
	f.InvestmentSources[0] = "fact"
	r = AggregateBudgetRegistry(c.data(), f, dashboardNow)
	if *r.Total.GtnContract.Q[0].A != 45 {
		t.Fatal("payment must come from registry, not monthly accrued investments")
	}
	c.plans[0].PaidInvestmentsRub = nil
	r = AggregateBudgetRegistry(c.data(), f, dashboardNow)
	if *r.Total.GtnContract.Q[0].A != 0 {
		t.Fatal("explicit fact source must not fall back to plan/forecast")
	}
}
func TestBudgetAllocationPreservesCentsAndSignedWeights(t *testing.T) {
	for _, weights := range [][]float64{{1, 1, 1}, {100, -20, 40}} {
		r, err := AllocateBudgetEdit(100.01, weights)
		if err != nil {
			t.Fatal(err)
		}
		sum := 0.0
		for _, v := range r {
			sum = round2(sum + v)
		}
		if sum != 100.01 {
			t.Fatalf("allocation sum %v", sum)
		}
	}
	if _, err := AllocateBudgetEdit(1, []float64{0, 0}); err == nil {
		t.Fatal("zero baseline must reject an arbitrary distribution")
	}
}
func TestBudgetSnapshotPreservesComparisonAndExtras(t *testing.T) {
	f := BudgetFilter{Year: 2026}
	network := budgetBrand("Сеть", 1, [4]budgetAmounts{{to: 100, gtn: 20}}, f)
	network.SalesSS.Q[0].A = models.PtrFloat(80)
	network.OlapTo.Q[0].A = models.PtrFloat(150)
	network.To.Q[0].Edited = &models.BudgetEdit{Base: 90, Value: 100, Who: "analyst", When: "2026-09-14"}
	brand := budgetBrand("Бренд", 0, [4]budgetAmounts{}, f)
	brand.Networks = []models.BudgetBrand{network}
	r := &models.BudgetResponse{Brands: []models.BudgetBrand{brand}, TypeMembers: map[string][]int{"Chain": {1}}}
	budgetRebuild(r, f)
	lines := budgetSnapshot(r, r)
	clone := &models.BudgetResponse{Brands: []models.BudgetBrand{}, TypeMembers: r.TypeMembers}
	budgetApplyLines(clone, lines, f)
	if *clone.Total.To.Q[0].A != 100 || *clone.Total.SalesSS.Q[0].A != 80 || *clone.Total.OlapTo.Q[0].A != 150 {
		t.Fatal("snapshot lost financial bases")
	}
	budgetPresentation(clone, BudgetFilter{Base: "ss"})
	if *clone.Total.Pct.Q[0].A != 25 || clone.Total.Pct.Q[3].A != nil {
		t.Fatal("sales denominator or future null is incorrect")
	}
	compare := &models.BudgetResponse{Brands: []models.BudgetBrand{}, Total: budgetBrand("Итого", 0, [4]budgetAmounts{{to: 80, gtn: 10}}, f)}
	budgetCompare(r, compare, "abs")
	if *r.Total.To.Q[0].Delta != 20 {
		t.Fatal("wrong delta")
	}
	filtered := &models.BudgetResponse{Brands: []models.BudgetBrand{}, TypeMembers: r.TypeMembers}
	budgetApplyLines(filtered, lines, BudgetFilter{NetworkTypes: []string{"missing"}})
	if len(filtered.Brands) != 0 {
		t.Fatal("stored edits widened network filter")
	}
}
func TestBudgetOlapAndSalesClosedMonthRules(t *testing.T) {
	c := dashboardCase{networks: []models.Network{dashboardNetwork(1, "Сеть", "КАМ")}, plans: []models.NetworkPlan{{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Бренд"), PlanRub: models.PtrFloat(1000), PlanUnits: models.PtrFloat(100)}}, facts: []models.NetworkMonthlyFact{dashboardFactUnits(1, 1, "Бренд", 1000, 100)}}
	f := BudgetFilter{Year: 2026, ToSources: [4]string{"fact"}}
	r := AggregateBudgetRegistry(c.data(), f, dashboardNow)
	addBudgetBases(r, c.data(), f, []repository.BudgetPrice{{Brand: "Бренд", SKU: "SKU-1", Price: 12}}, []repository.BudgetSale{{NetworkID: 1, Brand: "Бренд", Month: 1, Segment: "OLAP SS", Rub: 900}, {NetworkID: 1, Brand: "Бренд", Month: 12, Segment: "OLAP SS", Rub: 999}}, dashboardNow)
	if *r.Total.OlapTo.Q[0].A != 1200 {
		t.Fatal("SKU quantities were not converted at OLAP price")
	}
	if *r.Total.SalesSS.Q[0].A != 900 || r.Total.SalesSS.Q[3].A != nil {
		t.Fatal("future generated sales leaked into the budget")
	}
}

func TestBudgetExcelRendersFourSheetsAndServerValues(t *testing.T) {
	f := BudgetFilter{Year: 2026}
	b := budgetBrand("Бренд", 0, [4]budgetAmounts{{to: 1200000, gtn: 120000}}, f)
	r := &models.BudgetResponse{Year: 2026, Version: "LIVE", Brands: []models.BudgetBrand{b}, Total: b, QuarterStates: []string{"closed", "closed", "current", "future"}}
	file, err := renderBudgetExcel(f, "LIVE", "", "abs", "analyst", r, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if len(file.GetSheetList()) != 4 {
		t.Fatal("Excel must contain four sheets")
	}
	value, err := file.GetCellValue("Бюджет", "B3", excelize.Options{RawCellValue: true})
	if err != nil || value != "1200000" {
		t.Fatalf("turnover cell = %s, %v", value, err)
	}
	if _, err := file.WriteToBuffer(); err != nil {
		t.Fatal(err)
	}
}

func TestBudgetAnnualSupplementUsesRegistryPayments(t *testing.T) {
	n := dashboardNetwork(1, "Сеть", "КАМ")
	n.HasAnnualInvestmentCumulative = true
	c := dashboardCase{networks: []models.Network{n}}
	for q := 1; q <= 4; q++ {
		p := models.NetworkPlan{NetworkID: 1, Year: 2026, Quarter: q, BrandAS: brandPtr("Бренд"), PlanRub: models.PtrFloat(100), InvestmentsPct: models.PtrFloat(10)}
		if q < 4 {
			p.PaidInvestmentsRub = models.PtrFloat(10)
		}
		c.plans = append(c.plans, p)
		c.facts = append(c.facts, dashboardFact(1, q*3-2, "Бренд", 125, 0), dashboardFact(1, q*3-1, "Бренд", 0, 0), dashboardFact(1, q*3, "Бренд", 0, 0))
	}
	f := BudgetFilter{Year: 2026, Gross: true, InvestmentSources: [4]string{"fact", "fact", "fact", "forecast"}}
	r := AggregateBudgetRegistry(c.data(), f, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if *r.Total.GtnContract.Q[3].A != 20 || *r.Total.GtnContract.Year.A != 50 {
		t.Fatalf("expected Q4 remaining 20 and annual 50, got %v / %v", *r.Total.GtnContract.Q[3].A, *r.Total.GtnContract.Year.A)
	}
}

func TestBudgetOlapPortfolioIgnoresUnpricedPoolInLiveAndSnapshot(t *testing.T) {
	f := BudgetFilter{Year: 2026, Base: "reg-olap"}
	makeResponse := func(turnover, investment float64) *models.BudgetResponse {
		var amounts [4]budgetAmounts
		for q := range amounts {
			amounts[q] = budgetAmounts{to: 100, gtn: investment}
		}
		n := budgetBrand("Сеть", 1, amounts, f)
		for q := range n.OlapTo.Q {
			n.OlapTo.Q[q].A = models.PtrFloat(turnover)
		}
		brand := budgetBrand("Бренд", 0, [4]budgetAmounts{}, f)
		brand.Networks = []models.BudgetBrand{n}
		pool := budgetBrand("Нераспределённый остаток пула", 0, [4]budgetAmounts{}, f)
		pool.Networks = []models.BudgetBrand{budgetBrand("Сеть", 1, [4]budgetAmounts{{to: 10}, {to: -5}, {to: 15}, {to: 20}}, f)}
		r := &models.BudgetResponse{Brands: []models.BudgetBrand{brand, pool}}
		budgetRebuild(r, f)
		return r
	}
	a, b := makeResponse(200, 20), makeResponse(160, 24)
	clone := &models.BudgetResponse{Brands: []models.BudgetBrand{}}
	budgetApplyLines(clone, budgetSnapshot(b, b), f)
	budgetPresentation(a, f)
	budgetPresentation(clone, f)
	budgetCompare(a, clone, "abs")
	for q := 0; q < 4; q++ {
		if valueOrZero(a.Total.To.Q[q].Delta) != 40 || valueOrZero(a.Total.Pct.Q[q].Delta) != -5 {
			t.Fatalf("Q%d: missing or wrong portfolio delta: TO=%+v pct=%+v", q+1, a.Total.To.Q[q], a.Total.Pct.Q[q])
		}
	}
	if valueOrZero(a.Total.To.Year.Delta) != 160 || valueOrZero(a.Total.Pct.Year.Delta) != -5 {
		t.Fatal("wrong annual delta")
	}
	// A missing price for an actual brand must still make the total unavailable.
	missing := makeResponse(200, 20)
	missing.Brands[0].Networks[0].OlapTo.Q[0].A = nil
	budgetRebuild(missing, f)
	budgetPresentation(missing, f)
	if missing.Total.To.Q[0].A != nil || missing.Total.Pct.Q[0].A != nil || missing.Total.To.Year.A != nil {
		t.Fatal("real missing OLAP price was silently ignored")
	}
}
