package services

import (
	"math"
	"testing"

	"backend/repository"
)

func promoFloat(value float64) *float64 { return &value }
func promoString(value string) *string  { return &value }

func assertPromoFloat(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 0.0001 {
		t.Fatalf("%s = %v, want %.4f", name, got, want)
	}
}

func TestAggregatePromoDashboardUsesComparableFactCohort(t *testing.T) {
	rows := []repository.PromoDashboardRow{
		{
			Year: 2025, Month: 1,
			NetworkName: promoString("Сеть А"), BrandAS: promoString("Бренд 1"),
			SKU: promoString("SKU 1"), Mechanics: promoString("Скидка"),
			PlanPromoUnits: promoFloat(100), PlanInvestmentsRub: promoFloat(20),
			PlanPromoUpliftUnits: promoFloat(20), PlanPromoUpliftRub: promoFloat(200), GM: promoFloat(0.5),
			ActualPromoSalesUnits: promoFloat(110), ActualInvestments: promoFloat(25),
			ActualPromoUpliftUnits: promoFloat(30), ActualPromoUpliftRub: promoFloat(300),
		},
		{
			Year: 2025, Month: 1,
			NetworkName: promoString("Сеть Б"), BrandAS: promoString("Бренд 1"),
			SKU: promoString("SKU 2"), Mechanics: promoString("Бонус"),
			PlanPromoUnits: promoFloat(50), PlanInvestmentsRub: promoFloat(10),
			PlanPromoUpliftUnits: promoFloat(10), PlanPromoUpliftRub: promoFloat(100), GM: promoFloat(0.5),
		},
		{
			Year: 2025, Month: 2,
			NetworkName: promoString("Сеть А"), BrandAS: promoString("Бренд 2"),
			SKU: promoString("SKU 1"), Mechanics: promoString("Скидка"),
			PlanPromoUnits: promoFloat(80), PlanInvestmentsRub: promoFloat(16),
			PlanPromoUpliftUnits: promoFloat(16), PlanPromoUpliftRub: promoFloat(160), GM: promoFloat(0.5),
			ActualPromoSalesUnits: promoFloat(0), ActualInvestments: promoFloat(0),
			ActualPromoUpliftUnits: promoFloat(0), ActualPromoUpliftRub: promoFloat(0),
		},
	}

	dashboard := AggregatePromoDashboard(rows)
	got := dashboard.Summary
	if got.PromoCount != 3 || got.FactReadyCount != 2 {
		t.Fatalf("counts = %d/%d, want 3/2", got.PromoCount, got.FactReadyCount)
	}
	assertPromoFloat(t, "fact coverage", got.FactCoveragePct, 200.0/3.0)
	if got.PlanUnits != 230 || got.ComparablePlanUnits != 180 {
		t.Fatalf("plan units = %.0f/%.0f, want 230/180", got.PlanUnits, got.ComparablePlanUnits)
	}
	assertPromoFloat(t, "actual units", got.ActualUnits, 110)
	assertPromoFloat(t, "sales completion", got.SalesCompletionPct, 110.0/180.0*100)
	assertPromoFloat(t, "investment completion", got.InvestmentCompletionPct, 25.0/36.0*100)
	assertPromoFloat(t, "plan roi", got.PlanROI, 400)
	assertPromoFloat(t, "comparable plan roi", got.ComparablePlanROI, 400)
	assertPromoFloat(t, "actual roi", got.ActualROI, 500)
	assertPromoFloat(t, "sales variance", got.SalesVarianceUnits, -70)
	assertPromoFloat(t, "investment variance", got.InvestmentVarianceRub, -11)

	if len(dashboard.AvailableYears) != 1 || dashboard.AvailableYears[0] != 2025 {
		t.Fatalf("available years = %v, want [2025]", dashboard.AvailableYears)
	}
	if len(dashboard.Trend) != 2 || dashboard.Trend[0].Month != 1 || dashboard.Trend[1].Month != 2 {
		t.Fatalf("unexpected trend: %+v", dashboard.Trend)
	}
	if len(dashboard.Networks) != 2 || dashboard.Networks[0].Name != "Сеть А" {
		t.Fatalf("unexpected network breakdown: %+v", dashboard.Networks)
	}
	if len(dashboard.NetworkCalendar) != 3 || len(dashboard.BrandCalendar) != 2 {
		t.Fatalf("calendar sizes = %d/%d, want 3/2", len(dashboard.NetworkCalendar), len(dashboard.BrandCalendar))
	}
}

func TestAggregatePromoDashboardSkipsInvalidPeriodsAndKeepsUnknownDimensions(t *testing.T) {
	rows := []repository.PromoDashboardRow{
		{Year: 0, Month: 1, PlanPromoUnits: promoFloat(100)},
		{Year: 2025, Month: 13, PlanPromoUnits: promoFloat(100)},
		{Year: 2025, Month: 3, PlanPromoUnits: promoFloat(50)},
	}

	dashboard := AggregatePromoDashboard(rows)
	if dashboard.Summary.PromoCount != 1 || dashboard.Summary.PlanUnits != 50 {
		t.Fatalf("summary = %+v, want one valid row", dashboard.Summary)
	}
	if len(dashboard.Networks) != 1 || dashboard.Networks[0].Name != promoUnknownDimension {
		t.Fatalf("networks = %+v, want unknown dimension", dashboard.Networks)
	}
	if dashboard.Summary.ActualUnits != nil || dashboard.Summary.ActualROI != nil {
		t.Fatalf("fact metrics must stay nil without a comparable fact row: %+v", dashboard.Summary)
	}
}

// Инвестиции календаря берут факт там, где он заполнен, и план во всех
// остальных строках. Промо с фактом инвестиций, но без факта продаж, в
// сопоставимый срез не попадает — а в эту сумму обязано попасть фактом.
func TestAggregatePromoDashboardEffectiveInvestmentsPreferFact(t *testing.T) {
	rows := []repository.PromoDashboardRow{
		{
			Year: 2025, Month: 1, NetworkName: promoString("Сеть А"),
			PlanInvestmentsRub:    promoFloat(20),
			ActualPromoSalesUnits: promoFloat(110), ActualInvestments: promoFloat(25),
		},
		{
			Year: 2025, Month: 1, NetworkName: promoString("Сеть А"),
			PlanInvestmentsRub: promoFloat(10), ActualInvestments: promoFloat(7),
		},
		{
			Year: 2025, Month: 1, NetworkName: promoString("Сеть А"),
			PlanInvestmentsRub: promoFloat(16),
		},
	}

	got := AggregatePromoDashboard(rows).Summary
	if got.EffectiveInvestmentsRub != 48 {
		t.Fatalf("effective investments = %.2f, want 48", got.EffectiveInvestmentsRub)
	}
	if got.FactInvestmentsCount != 2 || got.PromoCount != 3 {
		t.Fatalf("fact investments count = %d из %d, want 2 из 3", got.FactInvestmentsCount, got.PromoCount)
	}
	// Сопоставимый срез не меняется: строка без факта продаж в него не входит.
	assertPromoFloat(t, "actual investments", got.ActualInvestmentsRub, 25)
	if got.ComparablePlanInvestmentsRub != 20 {
		t.Fatalf("comparable plan investments = %.2f, want 20", got.ComparablePlanInvestmentsRub)
	}
}
