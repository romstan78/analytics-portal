package services

import (
	"testing"

	"backend/models"
)

func brandPtr(v string) *string { return &v }

func TestNetRub(t *testing.T) {
	cases := []struct {
		name        string
		gross       float64
		vatIncluded bool
		vatRate     float64
		want        float64
	}{
		{"сеть с НДС 20%", 120000, true, 20, 100000},
		{"сеть без НДС", 120000, false, 20, 120000},
		{"ставка не задана", 120000, true, 0, 120000},
		{"округление до копеек", 100, true, 20, 83.33},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NetRub(tc.gross, tc.vatIncluded, tc.vatRate); got != tc.want {
				t.Errorf("NetRub(%v, %v, %v) = %v, ожидалось %v", tc.gross, tc.vatIncluded, tc.vatRate, got, tc.want)
			}
		})
	}
}

func TestEnrichNetworkPlansAppliesVATToInvestmentsOnly(t *testing.T) {
	periods := []models.NetworkPeriod{
		{Quarter: 1, VATIncluded: true, VATRate: 20, ContractType: "regular"},
		{Quarter: 2, VATIncluded: false, VATRate: 20, ContractType: "regular"},
	}
	plans := []models.NetworkPlan{
		{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1200000), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 2, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1200000), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(500000)},
	}

	got := EnrichNetworkPlans(plans, periods)

	// План не пересчитывается ни при каком НДС.
	if models.ValFloat(got[0].PlanRub) != 1200000 || models.ValFloat(got[1].PlanRub) != 1200000 {
		t.Error("план не должен меняться от настройки НДС")
	}
	if v := models.ValFloat(got[0].InvestmentsRub); v != 120000 {
		t.Errorf("Q1 инвестиции до вычета НДС = %v, ожидалось 120000", v)
	}
	if v := models.ValFloat(got[0].InvestmentsNet); v != 100000 {
		t.Errorf("Q1 инвестиции с вычетом НДС = %v, ожидалось 100000", v)
	}
	if v := models.ValFloat(got[1].InvestmentsNet); v != 120000 {
		t.Errorf("Q2 без НДС: инвестиции = %v, ожидалось 120000", v)
	}
	if got[2].InvestmentsRub != nil {
		t.Error("без процента инвестиций расчётные поля не заполняются")
	}
}

func TestCalculateNetworkTotalsGrossContract(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20, ContractType: "gross"}}
	plans := []models.NetworkPlan{
		{Quarter: 1, BrandAS: nil, PlanRub: models.PtrFloat(600000)},
		{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(360000), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(180000), InvestmentsPct: models.PtrFloat(5)},
	}

	q1 := CalculateNetworkTotals(plans, periods)[0]

	if q1.PlanRub != 540000 {
		t.Errorf("распределено = %v, ожидалось 540000", q1.PlanRub)
	}
	if models.ValFloat(q1.GrossPlanRub) != 600000 {
		t.Errorf("общий объём = %v, ожидалось 600000", models.ValFloat(q1.GrossPlanRub))
	}
	if models.ValFloat(q1.Undistributed) != 60000 {
		t.Errorf("остаток = %v, ожидалось 60000", models.ValFloat(q1.Undistributed))
	}
	// 36000 + 9000 = 45000 до вычета НДС, / 1.2 = 37500 с вычетом.
	if q1.InvestmentsRub != 45000 {
		t.Errorf("инвестиции до вычета НДС = %v, ожидалось 45000", q1.InvestmentsRub)
	}
	if q1.InvestmentsRubNet != 37500 {
		t.Errorf("инвестиции с вычетом НДС = %v, ожидалось 37500", q1.InvestmentsRubNet)
	}
}

func TestCalculateNetworkTotalsRegularContract(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 3, VATIncluded: false, VATRate: 20, ContractType: "regular"}}
	plans := []models.NetworkPlan{
		{Quarter: 3, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100000), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 3, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(50000)},
	}

	q3 := CalculateNetworkTotals(plans, periods)[2]

	if q3.PlanRub != 150000 {
		t.Errorf("сумма по брендам = %v, ожидалось 150000", q3.PlanRub)
	}
	// Сеть без НДС: обе базы инвестиций совпадают.
	if q3.InvestmentsRub != 10000 || q3.InvestmentsRubNet != 10000 {
		t.Errorf("инвестиции без НДС = %v / %v, ожидалось 10000 / 10000", q3.InvestmentsRub, q3.InvestmentsRubNet)
	}
	if q3.GrossPlanRub != nil || q3.Undistributed != nil {
		t.Error("у обычного контракта нет общего объёма и остатка")
	}
}
