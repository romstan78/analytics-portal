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

func TestEnrichNetworkPlansCalculatesForecastInvestments(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{
		// Прогноз ниже плана — инвестиции от прогноза считаются тем же процентом.
		{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1200000),
			ForecastRub: models.PtrFloat(900000), InvestmentsPct: models.PtrFloat(10)},
		// Прогноза нет — расчётные поля прогноза остаются пустыми.
		{Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(500000), InvestmentsPct: models.PtrFloat(10)},
	}

	got := EnrichNetworkPlans(plans, periods)

	if v := models.ValFloat(got[0].ForecastInvestmentsRub); v != 90000 {
		t.Errorf("инвестиции от прогноза = %v, ожидалось 90000", v)
	}
	if v := models.ValFloat(got[0].ForecastInvestmentsNet); v != 75000 {
		t.Errorf("инвестиции от прогноза без НДС = %v, ожидалось 75000", v)
	}
	if models.ValFloat(got[0].InvestmentsRub) != 120000 {
		t.Error("инвестиции от плана не должны зависеть от прогноза")
	}
	if got[1].ForecastInvestmentsRub != nil || got[1].ForecastInvestmentsNet != nil {
		t.Error("без прогноза расчётные поля прогноза не заполняются")
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

	q1 := CalculateNetworkTotals(plans, periods)[0]

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

	q3 := CalculateNetworkTotals(plans, periods)[2]

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

	q4 := CalculateNetworkTotals(plans, periods)[3]

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
	// 29000 + 6000 = 35000 до вычета НДС, / 1.2 = 29166.67.
	if q4.ForecastInvestmentsRub != 35000 {
		t.Errorf("инвестиции от прогноза = %v, ожидалось 35000", q4.ForecastInvestmentsRub)
	}
	if q4.ForecastInvestmentsRubNet != 29166.67 {
		t.Errorf("инвестиции от прогноза без НДС = %v, ожидалось 29166.67", q4.ForecastInvestmentsRubNet)
	}
}

func TestCalculateNetworkTotalsFactInvestments(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	plans := []models.NetworkPlan{
		// Факт инвестиций пришёл загрузкой и процентом не пересчитывается:
		// по плану вышло бы 30000, по факту закрыли 26400.
		{Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(300000),
			InvestmentsPct: models.PtrFloat(10), FactInvestmentsRub: models.PtrFloat(26400)},
		{Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(100000),
			InvestmentsPct: models.PtrFloat(5)},
	}

	q1 := CalculateNetworkTotals(plans, periods)[0]

	if q1.InvestmentsRub != 35000 {
		t.Errorf("инвестиции по плану = %v, ожидалось 35000", q1.InvestmentsRub)
	}
	if q1.FactInvestmentsRub != 26400 {
		t.Errorf("факт инвестиций = %v, ожидалось 26400", q1.FactInvestmentsRub)
	}
	// 26400 / 1.2 = 22000.
	if q1.FactInvestmentsRubNet != 22000 {
		t.Errorf("факт инвестиций без НДС = %v, ожидалось 22000", q1.FactInvestmentsRubNet)
	}
}

func TestEnrichNetworkPlansComputesFactInvestmentsNet(t *testing.T) {
	periods := []models.NetworkPeriod{
		{Quarter: 1, VATIncluded: true, VATRate: 20},
		{Quarter: 2, VATIncluded: false, VATRate: 20},
	}
	// Процента нет: факт инвестиций всё равно должен получить базу без НДС.
	plans := []models.NetworkPlan{
		{Quarter: 1, BrandAS: brandPtr("Альфа"), FactInvestmentsRub: models.PtrFloat(120000)},
		{Quarter: 2, BrandAS: brandPtr("Альфа"), FactInvestmentsRub: models.PtrFloat(120000)},
	}

	got := EnrichNetworkPlans(plans, periods)

	if v := models.ValFloat(got[0].FactInvestmentsNet); v != 100000 {
		t.Errorf("факт без НДС = %v, ожидалось 100000", v)
	}
	if v := models.ValFloat(got[1].FactInvestmentsNet); v != 120000 {
		t.Errorf("сеть без НДС: факт = %v, ожидалось 120000", v)
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

func TestPreviewNetworkPlansTakesFactFromStored(t *testing.T) {
	periods := []models.NetworkPeriod{{Quarter: 1, VATIncluded: true, VATRate: 20}}
	stored := []models.NetworkPlan{{
		ID: 7, NetworkID: 3, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
		FactRub: models.PtrFloat(900000), FactInvestmentsRub: models.PtrFloat(120000),
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
	if v := models.ValFloat(plans[0].FactInvestmentsNet); v != 100000 {
		t.Errorf("факт инвестиций без НДС = %v, ожидалось 100000", v)
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
