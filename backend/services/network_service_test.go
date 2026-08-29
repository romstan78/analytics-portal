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
		{Quarter: 1, VATIncluded: true, VATRate: 20},
		{Quarter: 2, VATIncluded: false, VATRate: 20},
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

func TestInvestmentRuleCalculatesForecastInvestments(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{
		// Прогноз ниже плана — инвестиции от прогноза считаются тем же процентом.
		{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1200000),
			ForecastRub: models.PtrFloat(900000), InvestmentsPct: models.PtrFloat(10)},
		// Прогноза нет — расчётные поля прогноза остаются пустыми.
		{Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(500000), InvestmentsPct: models.PtrFloat(10)},
	}

	got, _ := BuildNetworkPlanCalculations(plans, periods, nil)

	// Прогноз ниже плана: порог не пройден, прогнозные инвестиции — ноль.
	if v := models.ValFloat(got[0].ForecastInvestmentsRub); v != 0 {
		t.Errorf("инвестиции от прогноза = %v, ожидался 0: план не выполнен", v)
	}
	if models.ValFloat(got[0].InvestmentsRub) != 120000 {
		t.Error("инвестиции от плана не должны зависеть от прогноза")
	}
	if got[1].ForecastInvestmentsRub != nil || got[1].ForecastInvestmentsNet != nil {
		t.Error("без прогноза расчётные поля прогноза не заполняются")
	}

	// Тот же процент от прогноза, но с выполненным планом.
	plans[0].ForecastRub = models.PtrFloat(1300000)
	got, _ = BuildNetworkPlanCalculations(plans, periods, nil)
	if v := models.ValFloat(got[0].ForecastInvestmentsRub); v != 130000 {
		t.Errorf("инвестиции от прогноза = %v, ожидалось 130000", v)
	}
	if v := models.ValFloat(got[0].ForecastInvestmentsNet); v != round2(130000/1.2) {
		t.Errorf("инвестиции от прогноза без НДС = %v, ожидалось %v", v, round2(130000/1.2))
	}
}

func TestCalculateNetworkTotalsSplitsGrossAndSeparateBrands(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{
		// Пул валового объёма.
		{Quarter: 1, BrandAS: nil, PlanRub: models.PtrFloat(600000)},
		// Два бренда внутри пула и один вне его.
		{Quarter: 1, BrandAS: brandPtr("Альфа"), InGross: true, PlanRub: models.PtrFloat(360000), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 1, BrandAS: brandPtr("Бета"), InGross: true, PlanRub: models.PtrFloat(180000), InvestmentsPct: models.PtrFloat(5)},
		{Quarter: 1, BrandAS: brandPtr("Гамма"), PlanRub: models.PtrFloat(250000), InvestmentsPct: models.PtrFloat(4)},
	}

	_, allTotals := BuildNetworkPlanCalculations(plans, periods, nil)
	q1 := allTotals[0]

	if q1.PlanRub != 790000 {
		t.Errorf("план по всем брендам = %v, ожидалось 790000", q1.PlanRub)
	}
	if q1.GrossBrandsPlan != 540000 {
		t.Errorf("план брендов в валовом объёме = %v, ожидалось 540000", q1.GrossBrandsPlan)
	}
	if q1.SeparatePlanRub != 250000 {
		t.Errorf("план отдельных брендов = %v, ожидалось 250000", q1.SeparatePlanRub)
	}
	if q1.GrossBrandsCount != 2 {
		t.Errorf("брендов в пуле = %v, ожидалось 2", q1.GrossBrandsCount)
	}
	// Остаток считается только от брендов пула: «Гамма» в него не входит.
	if models.ValFloat(q1.Undistributed) != 60000 {
		t.Errorf("остаток = %v, ожидалось 60000", models.ValFloat(q1.Undistributed))
	}
	// Обязательство: пул целиком (600000) плюс отдельный бренд (250000).
	if q1.ContractPlanRub != 850000 {
		t.Errorf("обязательство по контракту = %v, ожидалось 850000", q1.ContractPlanRub)
	}
	// 36000 + 9000 + 10000 = 55000 до вычета НДС, / 1.2 = 45833.33 с вычетом.
	if q1.InvestmentsRub != 55000 {
		t.Errorf("инвестиции до вычета НДС = %v, ожидалось 55000", q1.InvestmentsRub)
	}
	if q1.InvestmentsRubNet != 45833.33 {
		t.Errorf("инвестиции с вычетом НДС = %v, ожидалось 45833.33", q1.InvestmentsRubNet)
	}
}

func TestCalculateNetworkTotalsWithoutGrossBrands(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 3, VATIncluded: false, VATRate: 20}}
	plans := []models.NetworkPlan{
		{Quarter: 3, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100000), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 3, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(50000)},
	}

	_, allTotals := BuildNetworkPlanCalculations(plans, periods, nil)
	q3 := allTotals[2]

	if q3.PlanRub != 150000 || q3.SeparatePlanRub != 150000 {
		t.Errorf("сумма по брендам = %v / %v, ожидалось 150000 / 150000", q3.PlanRub, q3.SeparatePlanRub)
	}
	// Сеть без НДС: обе базы инвестиций совпадают.
	if q3.InvestmentsRub != 10000 || q3.InvestmentsRubNet != 10000 {
		t.Errorf("инвестиции без НДС = %v / %v, ожидалось 10000 / 10000", q3.InvestmentsRub, q3.InvestmentsRubNet)
	}
	if q3.GrossPoolRub != nil || q3.Undistributed != nil {
		t.Error("без пула нет общего объёма и остатка")
	}
	if q3.ContractPlanRub != 150000 {
		t.Errorf("обязательство = %v, ожидалось 150000", q3.ContractPlanRub)
	}
}

func TestCalculateNetworkTotalsGrossBrandsWithoutPool(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 2, VATIncluded: false, VATRate: 0}}
	plans := []models.NetworkPlan{
		{Quarter: 2, BrandAS: brandPtr("Альфа"), InGross: true, PlanRub: models.PtrFloat(300000)},
		{Quarter: 2, BrandAS: brandPtr("Бета"), InGross: true, PlanRub: models.PtrFloat(200000)},
	}

	q2 := CalculateNetworkTotals(plans, periods)[1]

	// Пул не заведён: распределять нечего, остаток не показывается,
	// а обязательство равно сумме планов самих брендов.
	if q2.Undistributed != nil {
		t.Errorf("остаток = %v, ожидалось отсутствие", models.ValFloat(q2.Undistributed))
	}
	if q2.ContractPlanRub != 500000 {
		t.Errorf("обязательство = %v, ожидалось 500000", q2.ContractPlanRub)
	}
}

func TestCalculateNetworkTotalsFactAndForecast(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 4, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{
		{Quarter: 4, BrandAS: nil, PlanRub: models.PtrFloat(500000), ForecastRub: models.PtrFloat(480000)},
		{Quarter: 4, BrandAS: brandPtr("Альфа"), InGross: true, PlanRub: models.PtrFloat(300000),
			FactRub: models.PtrFloat(210000), ForecastRub: models.PtrFloat(290000), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 4, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(100000),
			FactRub: models.PtrFloat(90000), ForecastRub: models.PtrFloat(120000), InvestmentsPct: models.PtrFloat(5)},
	}

	_, allTotals := BuildNetworkPlanCalculations(plans, periods, nil)
	q4 := allTotals[3]

	if q4.FactRub != 300000 {
		t.Errorf("факт = %v, ожидалось 300000", q4.FactRub)
	}
	// Факт пула — только бренды, входящие в общий объём.
	if q4.GrossPoolFactRub != 210000 {
		t.Errorf("факт пула = %v, ожидалось 210000", q4.GrossPoolFactRub)
	}
	if q4.ForecastRub != 410000 {
		t.Errorf("прогноз = %v, ожидалось 410000", q4.ForecastRub)
	}
	if models.ValFloat(q4.GrossPoolFcstRub) != 480000 {
		t.Errorf("прогноз пула = %v, ожидалось 480000", models.ValFloat(q4.GrossPoolFcstRub))
	}
	// «Альфа» в валовом пуле: её порог — весь пул (500000), а прогноз пула
	// всего 290000, поэтому инвестиций она не приносит. «Бета» вне пула и
	// свой план закрыла: 120000 × 5% = 6000, / 1.2 = 5000.
	if q4.ForecastInvestmentsRub != 6000 {
		t.Errorf("инвестиции от прогноза = %v, ожидалось 6000", q4.ForecastInvestmentsRub)
	}
	if q4.ForecastInvestmentsRubNet != 5000 {
		t.Errorf("инвестиции от прогноза без НДС = %v, ожидалось 5000", q4.ForecastInvestmentsRubNet)
	}
}

// Фактические инвестиции считаются правилом от фактического ТО, а перечисленная
// по документам сумма живёт отдельно: одну из другой вывести нельзя.
func TestCalculateNetworkTotalsFactInvestments(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{
		// План закрыт фактом: 320000 × 10% = 32000, / 1.2 = 26666.67.
		{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(300000),
			FactRub: models.PtrFloat(320000), InvestmentsPct: models.PtrFloat(10),
			PaidInvestmentsRub: models.PtrFloat(26400)},
		// План не закрыт: правило даёт ноль независимо от перечисленного.
		{Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(100000),
			FactRub: models.PtrFloat(40000), InvestmentsPct: models.PtrFloat(5)},
	}

	got, allTotals := BuildNetworkPlanCalculations(plans, periods, nil)
	q1 := allTotals[0]

	if q1.InvestmentsRub != 35000 {
		t.Errorf("инвестиции по плану = %v, ожидалось 35000", q1.InvestmentsRub)
	}
	if q1.FactInvestmentsRub != 32000 {
		t.Errorf("факт инвестиций = %v, ожидалось 32000", q1.FactInvestmentsRub)
	}
	if q1.FactInvestmentsRubNet != 26666.67 {
		t.Errorf("факт инвестиций без НДС = %v, ожидалось 26666.67", q1.FactInvestmentsRubNet)
	}
	// Перечисленное правило не трогает: это платёжный факт, а не расчёт.
	if models.ValFloat(got[0].PaidInvestmentsRub) != 26400 {
		t.Errorf("перечислено = %v, ожидалось 26400", models.ValFloat(got[0].PaidInvestmentsRub))
	}
}

// Обе базы НДС заполняются всегда. У сети без НДС они равны — это обещание
// потребителю колонок, а не совпадение.
func TestInvestmentRuleFillsBothVATBases(t *testing.T) {
	periods := []models.NetworkPeriod{
		{Quarter: 1, VATIncluded: true, VATRate: 20},
		{Quarter: 2, VATIncluded: false, VATRate: 20},
	}
	plans := []models.NetworkPlan{
		{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100000),
			FactRub: models.PtrFloat(120000), ForecastRub: models.PtrFloat(120000),
			InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 2, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100000),
			FactRub: models.PtrFloat(120000), ForecastRub: models.PtrFloat(120000),
			InvestmentsPct: models.PtrFloat(10)},
	}

	got, _ := BuildNetworkPlanCalculations(plans, periods, nil)

	// Сеть с НДС 20%: 12000 / 1.2 = 10000.
	if v := models.ValFloat(got[0].FactInvestmentsNet); v != 10000 {
		t.Errorf("факт без НДС = %v, ожидалось 10000", v)
	}
	if v := models.ValFloat(got[0].ForecastInvestmentsNet); v != 10000 {
		t.Errorf("прогноз без НДС = %v, ожидалось 10000", v)
	}
	if v := models.ValFloat(got[0].InvestmentsNet); v != round2(10000/1.2) {
		t.Errorf("план без НДС = %v, ожидалось %v", v, round2(10000/1.2))
	}
	// Сеть без НДС: обе базы совпадают.
	if models.ValFloat(got[1].FactInvestmentsRub) != models.ValFloat(got[1].FactInvestmentsNet) {
		t.Errorf("без НДС базы обязаны совпадать: %#v", got[1])
	}
	if v := models.ValFloat(got[1].FactInvestmentsNet); v != 12000 {
		t.Errorf("сеть без НДС: факт = %v, ожидалось 12000", v)
	}
}

func TestSumYearTotalsAddsQuarters(t *testing.T) {
	totals := []NetworkPlanTotals{
		{Quarter: 1, PlanRub: 100, GrossBrandsPlan: 60, SeparatePlanRub: 40, ContractPlanRub: 100,
			GrossBrandsCount: 2, FactRub: 90, ForecastRub: 110, InvestmentsRub: 10, InvestmentsRubNet: 8.33},
		{Quarter: 2, PlanRub: 200, GrossBrandsPlan: 120, SeparatePlanRub: 80, ContractPlanRub: 200,
			GrossBrandsCount: 3, FactRub: 180, ForecastRub: 220, InvestmentsRub: 20, InvestmentsRubNet: 16.67},
	}

	year := SumYearTotals(totals)

	if year.PlanRub != 300 || year.ContractPlanRub != 300 {
		t.Errorf("план за год = %v, обязательство = %v, ожидалось 300 и 300", year.PlanRub, year.ContractPlanRub)
	}
	if year.FactRub != 270 || year.ForecastRub != 330 {
		t.Errorf("факт = %v, прогноз = %v, ожидалось 270 и 330", year.FactRub, year.ForecastRub)
	}
	if year.InvestmentsRubNet != 25 {
		t.Errorf("инвестиции без НДС = %v, ожидалось 25", year.InvestmentsRubNet)
	}
	// Состав пула — не сумма: бренд, стоящий в двух кварталах, один и тот же.
	if year.GrossBrandsCount != 3 {
		t.Errorf("брендов в пуле = %d, ожидалось 3 (максимум по кварталу)", year.GrossBrandsCount)
	}
}

// Год без единого заведённого пула остаётся без остатка, а не с нулём:
// ноль читался бы как «пул разобран полностью».
func TestSumYearTotalsKeepsPoolAbsent(t *testing.T) {
	year := SumYearTotals([]NetworkPlanTotals{{Quarter: 1}, {Quarter: 2}})
	if year.GrossPoolRub != nil || year.Undistributed != nil || year.GrossPoolFcstRub != nil {
		t.Errorf("пул не заводился, но поля заполнены: %#v", year)
	}

	pool := 500.0
	withPool := SumYearTotals([]NetworkPlanTotals{
		{Quarter: 1, GrossPoolRub: &pool, Undistributed: models.PtrFloat(200)},
		{Quarter: 2},
	})
	if models.ValFloat(withPool.GrossPoolRub) != 500 || models.ValFloat(withPool.Undistributed) != 200 {
		t.Errorf("пул за год = %#v, ожидалось 500 и остаток 200", withPool)
	}
}

func TestCalculateNetworkPeriodGroupTotalsPortfolioAndBrand(t *testing.T) {
	periods := []models.NetworkPeriod{
		{Quarter: 1, VATIncluded: true, VATRate: 20},
		{Quarter: 2, VATIncluded: true, VATRate: 20},
	}
	plans := []models.NetworkPlan{
		{Quarter: 1, BrandAS: nil, PlanRub: models.PtrFloat(1000)},
		{Quarter: 1, BrandAS: brandPtr("Альфа"), InGross: true, PlanRub: models.PtrFloat(600),
			FactRub: models.PtrFloat(400), InvestmentsPct: models.PtrFloat(10), PaidInvestmentsRub: models.PtrFloat(48)},
		{Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(200),
			FactRub: models.PtrFloat(100), InvestmentsPct: models.PtrFloat(20), PaidInvestmentsRub: models.PtrFloat(24)},
		{Quarter: 2, BrandAS: nil, PlanRub: models.PtrFloat(1200)},
		{Quarter: 2, BrandAS: brandPtr("Альфа"), InGross: true, PlanRub: models.PtrFloat(700),
			FactRub: models.PtrFloat(800), InvestmentsPct: models.PtrFloat(10), PaidInvestmentsRub: models.PtrFloat(72)},
		{Quarter: 2, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(300),
			FactRub: models.PtrFloat(200), InvestmentsPct: models.PtrFloat(20), PaidInvestmentsRub: models.PtrFloat(36)},
	}
	groups := []models.NetworkPeriodGroup{
		{StartQuarter: 1, EndQuarter: 2},
		{StartQuarter: 1, EndQuarter: 2, BrandAS: brandPtr("Бета")},
	}
	plans, totals := BuildNetworkPlanCalculations(plans, periods, groups)

	got := CalculateNetworkPeriodGroupTotals(groups, plans, totals)
	if len(got) != 2 {
		t.Fatalf("итогов = %d, ожидалось 2", len(got))
	}
	// Портфельный план использует валовые обязательства: (1000+200)+(1200+300).
	if got[0].PlanRub != 2700 || got[0].FactRub != 1500 || got[0].InvestmentsRub != 230 {
		t.Errorf("портфель Q1–Q2 рассчитан неверно: %#v", got[0])
	}
	// Портфель Q1–Q2 закрыт фактом на 1500 из 2700 — по правилу инвестиций
	// не возникает, сколько бы ни было перечислено по документам.
	if got[0].FactInvestmentsRubNet != 0 {
		t.Errorf("факт инвестиций портфеля без НДС = %v, ожидался 0", got[0].FactInvestmentsRubNet)
	}
	if got[1].PlanRub != 500 || got[1].FactRub != 300 || got[1].InvestmentsRub != 100 {
		t.Errorf("бренд Q1–Q2 рассчитан неверно: %#v", got[1])
	}
}

func TestCalculateNetworkAnnualInvestmentCumulativeGrossAndBrand(t *testing.T) {
	periods := []models.NetworkPeriod{
		{Quarter: 1}, {Quarter: 2}, {Quarter: 3}, {Quarter: 4},
	}
	plans := []models.NetworkPlan{}
	for quarter, grossEAC := range []float64{600, 1100, 900, 1400} {
		q := quarter + 1
		plans = append(plans,
			models.NetworkPlan{Quarter: q, BrandAS: nil, PlanRub: models.PtrFloat(1000)},
			models.NetworkPlan{
				Quarter: q, BrandAS: brandPtr("Альфа"), InGross: true,
				PlanRub: models.PtrFloat(600), ForecastRub: models.PtrFloat(grossEAC),
				InvestmentsPct: models.PtrFloat(10),
			},
			models.NetworkPlan{
				Quarter: q, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(250),
				ForecastRub: models.PtrFloat(250), InvestmentsPct: models.PtrFloat(20),
			},
		)
	}
	// Выплаченные суммы Q1-Q3 не обязаны совпадать с процентом от объёма.
	plans[1].PaidInvestmentsRub = models.PtrFloat(40)
	plans[4].PaidInvestmentsRub = models.PtrFloat(50)
	plans[7].PaidInvestmentsRub = models.PtrFloat(60)
	plans[2].PaidInvestmentsRub = models.PtrFloat(20)
	plans[5].PaidInvestmentsRub = models.PtrFloat(20)
	plans[8].PaidInvestmentsRub = models.PtrFloat(20)
	// Q4 вычитается по официальному прогнозу выплат, а не по расчётному начислению.
	// Он введён человеком, поэтому правило его не пересчитывает.
	plans[10].ForecastInvestmentsRub = models.PtrFloat(100)
	plans[10].ForecastInvestmentsOverridden = true
	plans[11].ForecastInvestmentsRub = models.PtrFloat(50)
	plans[11].ForecastInvestmentsOverridden = true

	plans, totals := BuildNetworkPlanCalculations(plans, periods, nil)
	got := CalculateNetworkAnnualInvestmentCumulative(plans, periods, totals)

	if got.PortfolioPlanRub != 5000 || got.PortfolioEACRub != 5000 || !got.PortfolioCompleted {
		t.Fatalf("портфельный порог рассчитан неверно: %#v", got)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("строк кумулятива = %d, ожидалось gross + бренд", len(got.Rows))
	}
	gross := got.Rows[0]
	if gross.ScopeType != "gross" || gross.PlanRub != 4000 || gross.EACRub != 4000 || !gross.Eligible {
		t.Errorf("валовый объём рассчитан неверно: %#v", gross)
	}
	if gross.AccruedInvestmentsRub != 400 || gross.PaidInvestmentsRub != 150 ||
		gross.Q4ForecastInvestmentsRub != 100 || gross.SupplementRub != 150 {
		t.Errorf("доплата валового объёма рассчитана неверно: %#v", gross)
	}
	brand := got.Rows[1]
	if models.ValString(brand.BrandAS) != "Бета" || brand.PlanRub != 1000 || brand.EACRub != 1000 || !brand.Eligible {
		t.Errorf("отдельный бренд рассчитан неверно: %#v", brand)
	}
	if brand.AccruedInvestmentsRub != 200 || brand.PaidInvestmentsRub != 60 ||
		brand.Q4ForecastInvestmentsRub != 50 || brand.SupplementRub != 90 {
		t.Errorf("доплата бренда рассчитана неверно: %#v", brand)
	}
	if got.TotalSupplementRub != 240 || got.TotalSupplementRubNet != 240 {
		t.Errorf("общая доплата = %v / %v, ожидалось 240 / 240", got.TotalSupplementRub, got.TotalSupplementRubNet)
	}
}

func TestCalculateNetworkAnnualInvestmentCumulativeRequiresPortfolioAndScopeCompletion(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{
		{Quarter: 1, BrandAS: brandPtr("Выполнен"), PlanRub: models.PtrFloat(100),
			ForecastRub: models.PtrFloat(120), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 1, BrandAS: brandPtr("Не выполнен"), PlanRub: models.PtrFloat(200),
			FactRub: models.PtrFloat(50), InvestmentsPct: models.PtrFloat(10)},
	}
	plans, totals := BuildNetworkPlanCalculations(plans, periods, nil)
	got := CalculateNetworkAnnualInvestmentCumulative(plans, periods, totals)

	if got.PortfolioEACRub != 170 {
		t.Errorf("EAC должен использовать forecast с fallback на факт, получено %v", got.PortfolioEACRub)
	}
	if got.PortfolioCompleted {
		t.Error("портфель не должен считаться выполненным")
	}
	if len(got.Rows) != 2 {
		t.Fatalf("строк = %d, ожидалось 2", len(got.Rows))
	}
	for _, row := range got.Rows {
		if row.Eligible || row.SupplementRub != 0 || row.SupplementRubNet != 0 {
			t.Errorf("без выполнения портфеля доплаты быть не должно: %#v", row)
		}
	}
}

func TestCalculateNetworkAnnualInvestmentCumulativeRequiresRowCompletionAndClampsSupplement(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1}}
	plans := []models.NetworkPlan{
		{Quarter: 1, BrandAS: brandPtr("A"), PlanRub: models.PtrFloat(100),
			ForecastRub: models.PtrFloat(110), InvestmentsPct: models.PtrFloat(10),
			PaidInvestmentsRub: models.PtrFloat(20)},
		{Quarter: 1, BrandAS: brandPtr("B"), PlanRub: models.PtrFloat(100),
			ForecastRub: models.PtrFloat(90), InvestmentsPct: models.PtrFloat(10)},
	}
	plans, totals := BuildNetworkPlanCalculations(plans, periods, nil)
	got := CalculateNetworkAnnualInvestmentCumulative(plans, periods, totals)

	if !got.PortfolioCompleted {
		t.Fatal("портфель с EAC 200 при плане 200 должен быть выполнен")
	}
	if len(got.Rows) != 2 {
		t.Fatalf("строк = %d, ожидалось 2", len(got.Rows))
	}
	if !got.Rows[0].Eligible || got.Rows[0].SupplementRub != 0 {
		t.Errorf("выполненный бренд должен быть доступен, но доплата не может быть отрицательной: %#v", got.Rows[0])
	}
	if got.Rows[1].Eligible || got.Rows[1].SupplementRub != 0 {
		t.Errorf("невыполненный бренд не должен получать доплату: %#v", got.Rows[1])
	}
}

func TestCalculateNetworkAnnualInvestmentCumulativeForNetworkUsesProfileFlag(t *testing.T) {
	plans := []models.NetworkPlan{{
		Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
		ForecastRub: models.PtrFloat(100), InvestmentsPct: models.PtrFloat(10),
	}}
	periods := []models.NetworkPeriod{{Quarter: 1}}
	totals := CalculateNetworkTotals(EnrichNetworkPlans(plans, periods), periods)

	if got := CalculateNetworkAnnualInvestmentCumulativeForNetwork(
		models.Network{}, plans, periods, totals,
	); got != nil {
		t.Fatalf("для сети с выключенным флагом ожидался nil, получено %#v", got)
	}
	if got := CalculateNetworkAnnualInvestmentCumulativeForNetwork(
		models.Network{HasAnnualInvestmentCumulative: true}, plans, periods, totals,
	); got == nil {
		t.Fatal("для сети с включённым флагом ожидался расчёт")
	}
}

func TestInvestmentRuleRequiresOneHundredPercent(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 3}, {Quarter: 4}}
	plans := []models.NetworkPlan{
		{Quarter: 3, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
			ForecastRub: models.PtrFloat(99), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 4, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
			ForecastRub: models.PtrFloat(101), InvestmentsPct: models.PtrFloat(10)},
	}

	got, totals := BuildNetworkPlanCalculations(plans, periods, nil)
	// Ноль, а не nil: процент задан, просто ничего не заработано.
	if got[0].ForecastInvestmentsEarned || models.ValFloat(got[0].ForecastInvestmentsRub) != 0 {
		t.Errorf("Q3 ниже 100%% не должен приносить инвестиции: %#v", got[0])
	}
	if got[0].ForecastInvestmentsRub == nil {
		t.Error("Q3: ожидался явный ноль, а не отсутствие значения")
	}
	if !got[1].ForecastInvestmentsEarned || models.ValFloat(got[1].ForecastInvestmentsRub) != 10.1 {
		t.Errorf("Q4 выше 100%% считается от EAC: %#v", got[1])
	}
	// Плановые инвестиции порога не знают — они есть в обоих кварталах.
	if models.ValFloat(got[0].InvestmentsRub) != 10 || models.ValFloat(got[1].InvestmentsRub) != 10 {
		t.Errorf("плановые инвестиции не должны зависеть от выполнения: %#v", got)
	}
	if totals[2].ForecastInvestmentsRub != 0 || totals[3].ForecastInvestmentsRub != 10.1 {
		t.Errorf("квартальные инвестиции рассчитаны неверно: %#v", totals)
	}
}

// Фактические инвестиции меряются фактом, а не прогнозом: прогноз, закрывший
// план, ещё не отгрузка.
func TestInvestmentRuleMeasuresFactByFact(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1}}
	plans := []models.NetworkPlan{{
		Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
		FactRub: models.PtrFloat(60), ForecastRub: models.PtrFloat(110),
		InvestmentsPct: models.PtrFloat(10),
	}}

	got, _ := BuildNetworkPlanCalculations(plans, periods, nil)
	if !got[0].ForecastInvestmentsEarned || models.ValFloat(got[0].ForecastInvestmentsRub) != 11 {
		t.Errorf("прогноз закрыл план — прогнозные инвестиции ожидались: %#v", got[0])
	}
	if got[0].FactInvestmentsEarned || models.ValFloat(got[0].FactInvestmentsRub) != 0 {
		t.Errorf("факт план не закрыл — фактические инвестиции должны быть нулём: %#v", got[0])
	}
}

func TestInvestmentRuleUsesCombinedPeriod(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 3}, {Quarter: 4}}
	plans := []models.NetworkPlan{
		{Quarter: 3, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
			ForecastRub: models.PtrFloat(99), InvestmentsPct: models.PtrFloat(10)},
		{Quarter: 4, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
			ForecastRub: models.PtrFloat(101), InvestmentsPct: models.PtrFloat(10)},
	}
	groups := []models.NetworkPeriodGroup{{StartQuarter: 3, EndQuarter: 4, BrandAS: brandPtr("Альфа")}}

	got, totals := BuildNetworkPlanCalculations(plans, periods, groups)
	if !got[0].ForecastInvestmentsEarned || !got[1].ForecastInvestmentsEarned ||
		models.ValFloat(got[0].ForecastInvestmentsRub) != 9.9 ||
		models.ValFloat(got[1].ForecastInvestmentsRub) != 10.1 {
		t.Errorf("объединённый период ровно 100%% оплачивает оба квартала: %#v", got)
	}
	combined := CalculateNetworkPeriodGroupTotals(groups, got, totals)
	if len(combined) != 1 || !combined[0].Completed || combined[0].ForecastInvestmentsRub != 20 {
		t.Errorf("итог объединённого периода рассчитан неверно: %#v", combined)
	}
}

func TestInvestmentRuleFromFactBypassesThreshold(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{{
		Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
		FactRub: models.PtrFloat(40), ForecastRub: models.PtrFloat(50),
		InvestmentsPct: models.PtrFloat(10), PayInvestmentsFromFact: true,
	}}

	got, totals := BuildNetworkPlanCalculations(plans, periods, nil)
	// Порога нет: процент считается с того ТО, которое есть у показателя.
	if !got[0].FactInvestmentsEarned || models.ValFloat(got[0].FactInvestmentsRub) != 4 ||
		models.ValFloat(got[0].FactInvestmentsNet) != 3.33 {
		t.Errorf("оплата от факта считается без порога: %#v", got[0])
	}
	if models.ValFloat(got[0].ForecastInvestmentsRub) != 5 {
		t.Errorf("прогнозные при оплате от факта считаются от EAC: %#v", got[0])
	}
	if totals[0].Completed || totals[0].FactInvestmentsRub != 4 {
		t.Errorf("план может быть не выполнен, но оплата от факта остаётся: %#v", totals[0])
	}
}

func TestAnnualCumulativeIncludesFactBasedAccrualWithoutYearCompletion(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1}}
	plans := []models.NetworkPlan{{
		Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100),
		FactRub: models.PtrFloat(40), ForecastRub: models.PtrFloat(50),
		InvestmentsPct: models.PtrFloat(10), PayInvestmentsFromFact: true,
	}}
	plans, totals := BuildNetworkPlanCalculations(plans, periods, nil)
	got := CalculateNetworkAnnualInvestmentCumulative(plans, periods, totals)

	if got.PortfolioCompleted || len(got.Rows) != 1 {
		t.Fatalf("годовой план должен быть не выполнен: %#v", got)
	}
	row := got.Rows[0]
	if row.Eligible || row.FactBasedAccruedInvestmentsRub != 4 || row.SupplementRub != 4 ||
		got.TotalSupplementRub != 4 {
		t.Errorf("безусловное начисление от факта потерялось в кумулятиве: %#v", got)
	}
}

func TestPreviewNetworkPlansTakesFactFromStored(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	stored := []models.NetworkPlan{{
		ID: 7, NetworkID: 3, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
		FactRub: models.PtrFloat(900000), PaidInvestmentsRub: models.PtrFloat(120000),
	}}
	// Черновик несёт только то, что вводит пользователь.
	draft := []NetworkPlanDraft{{
		Quarter: 1, BrandAS: brandPtr("Альфа"), InGross: true,
		PlanRub: models.PtrFloat(1200000), InvestmentsPct: models.PtrFloat(10),
	}}

	plans, totals, year := PreviewNetworkPlans(draft, stored, periods)

	if len(plans) != 1 {
		t.Fatalf("строк плана = %d, ожидалась 1", len(plans))
	}
	if models.ValFloat(plans[0].FactRub) != 900000 {
		t.Errorf("факт = %v, ожидалось 900000 из сохранённой строки", models.ValFloat(plans[0].FactRub))
	}
	if v := models.ValFloat(plans[0].InvestmentsRub); v != 120000 {
		t.Errorf("инвестиции = %v, ожидалось 120000", v)
	}
	if v := models.ValFloat(plans[0].InvestmentsNet); v != 100000 {
		t.Errorf("инвестиции без НДС = %v, ожидалось 100000", v)
	}
	// План 1 200 000 фактом 900 000 не закрыт: фактических инвестиций нет,
	// а перечисленное по документам остаётся на своём месте.
	if v := models.ValFloat(plans[0].FactInvestmentsRub); v != 0 {
		t.Errorf("факт инвестиций = %v, ожидался 0", v)
	}
	if v := models.ValFloat(plans[0].PaidInvestmentsRub); v != 120000 {
		t.Errorf("перечислено = %v, ожидалось 120000", v)
	}
	if totals[0].GrossBrandsPlan != 1200000 || totals[0].FactRub != 900000 {
		t.Errorf("итоги квартала неверны: %#v", totals[0])
	}
	if year.PlanRub != 1200000 {
		t.Errorf("итог года = %v, ожидалось 1200000", year.PlanRub)
	}
}

// Строка, которой в черновике нет, в итоги не попадает: иначе таблица
// показывала бы суммы по брендам, скрытым с экрана.
func TestPreviewNetworkPlansIgnoresRowsOutsideDraft(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: false}}
	stored := []models.NetworkPlan{
		{Quarter: 1, BrandAS: brandPtr("Альфа"), FactRub: models.PtrFloat(100)},
		{Quarter: 1, BrandAS: brandPtr("Скрытый"), FactRub: models.PtrFloat(999)},
	}
	draft := []NetworkPlanDraft{{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(50)}}

	plans, totals, _ := PreviewNetworkPlans(draft, stored, periods)

	if len(plans) != 1 {
		t.Fatalf("строк = %d, ожидалась только строка черновика", len(plans))
	}
	if totals[0].FactRub != 100 {
		t.Errorf("факт квартала = %v, ожидалось 100 без скрытого бренда", totals[0].FactRub)
	}
}
