package services

import (
	"testing"

	"backend/models"
)

// Периоды без НДС: суммы «с НДС» и «без» совпадают, и проверять можно одну.
var noVAT = []models.NetworkPeriod{{Quarter: 1, VATIncluded: false, VATRate: 20}}

func scale(no int, planRub, pct float64) models.NetworkPlanScale {
	return models.NetworkPlanScale{ScaleNo: no, PlanRub: models.PtrFloat(planRub), InvestmentsPct: models.PtrFloat(pct)}
}

// separateBrand — отдельный бренд с тремя ступенями: 100 / 110 / 120 и
// процентами 5 / 6,5 / 8. Порог первой ступени — план строки.
func separateBrand(forecast float64, capMode string, capPct *float64) models.NetworkPlan {
	return models.NetworkPlan{
		Quarter: 1, BrandAS: brandPtr("Гамма"),
		PlanRub: models.PtrFloat(100), InvestmentsPct: models.PtrFloat(5),
		ForecastRub: models.PtrFloat(forecast),
		CapMode:     capMode, CapPct: capPct,
		Scales: []models.NetworkPlanScale{scale(2, 110, 6.5), scale(3, 120, 8)},
	}
}

func TestScalesSingleOpenScaleEqualsLegacyRule(t *testing.T) {
	plans := []models.NetworkPlan{{
		Quarter: 1, BrandAS: brandPtr("Альфа"),
		PlanRub: models.PtrFloat(1000), InvestmentsPct: models.PtrFloat(10),
		ForecastRub: models.PtrFloat(1300), FactRub: models.PtrFloat(700),
	}}
	got, totals := BuildNetworkPlanCalculations(plans, noVAT, nil)
	row := got[0]
	if v := models.ValFloat(row.ForecastInvestmentsRub); v != 130 {
		t.Errorf("без ступеней и крышки — объём × процент: %v, ожидалось 130", v)
	}
	if row.ForecastScale != 1 || models.ValFloat(row.ForecastBaseRub) != 1300 {
		t.Errorf("достигнута ступень 1 с базой в весь объём, получено ступень %d, база %v", row.ForecastScale, models.ValFloat(row.ForecastBaseRub))
	}
	if row.FactScale != 0 || models.ValFloat(row.FactInvestmentsRub) != 0 {
		t.Errorf("факт ниже плана: ступень 0 и явный ноль, получено %d / %v", row.FactScale, models.ValFloat(row.FactInvestmentsRub))
	}
	if len(row.Scales) != 1 || models.ValFloat(row.Scales[0].PlanInvestmentsRub) != 100 {
		t.Errorf("ступень 1 синтезируется из строки с плановыми инвестициями 100: %+v", row.Scales)
	}
	if len(totals[0].Scales) != 1 || totals[0].Scales[0].ContractPlanRub != 1000 {
		t.Errorf("итог квартала несёт ступень 1: %+v", totals[0].Scales)
	}
}

func TestScalesReachedByForecastAndByFact(t *testing.T) {
	cases := []struct {
		name         string
		forecast     float64
		wantScale    int
		wantInvest   float64
		wantEarned   bool
		wantPlanInv1 float64
	}{
		{"ниже первой ступени", 90, 0, 0, false, 5},
		{"первая ступень", 105, 1, 5.25, true, 5},
		{"вторая ступень: коридор оплачивается целиком", 115, 2, 7.48, true, 5},
		{"третья ступень, открытая крышка", 140, 3, 11.2, true, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plans := []models.NetworkPlan{separateBrand(tc.forecast, models.CapModeOpen, nil)}
			got, _ := BuildNetworkPlanCalculations(plans, noVAT, nil)
			row := got[0]
			if row.ForecastScale != tc.wantScale {
				t.Errorf("ступень = %d, ожидалась %d", row.ForecastScale, tc.wantScale)
			}
			if v := models.ValFloat(row.ForecastInvestmentsRub); v != tc.wantInvest {
				t.Errorf("к выплате = %v, ожидалось %v", v, tc.wantInvest)
			}
			if row.ForecastInvestmentsEarned != tc.wantEarned {
				t.Errorf("earned = %v, ожидалось %v", row.ForecastInvestmentsEarned, tc.wantEarned)
			}
			if v := models.ValFloat(row.InvestmentsRub); v != tc.wantPlanInv1 {
				t.Errorf("плановые инвестиции строки — по ступени 1: %v", v)
			}
			// Плановые инвестиции есть у каждой ступени: 100×5 %, 110×6,5 %, 120×8 %.
			wantPlanned := []float64{5, 7.15, 9.6}
			for i, s := range row.Scales {
				if v := models.ValFloat(s.PlanInvestmentsRub); v != wantPlanned[i] {
					t.Errorf("ступень %d: плановые инвестиции %v, ожидалось %v", s.ScaleNo, v, wantPlanned[i])
				}
				if s.ForecastReached != (s.ScaleNo <= tc.wantScale) {
					t.Errorf("ступень %d: reached = %v", s.ScaleNo, s.ForecastReached)
				}
			}
		})
	}

	// Факт меряется фактом: прогноз на третьей ступени не делает факт достигнутым.
	plan := separateBrand(140, models.CapModeOpen, nil)
	plan.FactRub = models.PtrFloat(112)
	got, _ := BuildNetworkPlanCalculations([]models.NetworkPlan{plan}, noVAT, nil)
	if got[0].FactScale != 2 || models.ValFloat(got[0].FactInvestmentsRub) != 7.28 {
		t.Errorf("факт 112 — вторая ступень, 112 × 6,5 %% = 7,28; получено %d / %v", got[0].FactScale, models.ValFloat(got[0].FactInvestmentsRub))
	}
}

func TestScalesCapOnLastScale(t *testing.T) {
	cases := []struct {
		name     string
		capMode  string
		capPct   *float64
		wantBase float64
	}{
		{"открытая — весь объём", models.CapModeOpen, nil, 140},
		{"процентная — не выше плана × 1,05", models.CapModePct, models.PtrFloat(5), 126},
		{"процентная, объём внутри коридора", models.CapModePct, models.PtrFloat(20), 140},
		{"закрытая — ровно план ступени", models.CapModeClosed, nil, 120},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plans := []models.NetworkPlan{separateBrand(140, tc.capMode, tc.capPct)}
			got, _ := BuildNetworkPlanCalculations(plans, noVAT, nil)
			row := got[0]
			if v := models.ValFloat(row.ForecastBaseRub); v != tc.wantBase {
				t.Errorf("база = %v, ожидалось %v", v, tc.wantBase)
			}
			if v := models.ValFloat(row.ForecastInvestmentsRub); v != round2(tc.wantBase*0.08) {
				t.Errorf("к выплате = %v, ожидалось %v", v, round2(tc.wantBase*0.08))
			}
		})
	}
}

func TestScalesClosedCapOnIntermediateScale(t *testing.T) {
	// Закрытая крышка режет и промежуточные коридоры: на второй ступени
	// платят ровно 110 × 6,5 %, а не за все 115.
	plans := []models.NetworkPlan{separateBrand(115, models.CapModeClosed, nil)}
	got, _ := BuildNetworkPlanCalculations(plans, noVAT, nil)
	row := got[0]
	if row.ForecastScale != 2 || models.ValFloat(row.ForecastBaseRub) != 110 {
		t.Errorf("ступень %d, база %v; ожидалась ступень 2 с базой 110", row.ForecastScale, models.ValFloat(row.ForecastBaseRub))
	}
	if v := models.ValFloat(row.ForecastInvestmentsRub); v != 7.15 {
		t.Errorf("к выплате = %v, ожидалось 7,15", v)
	}
	// Процентная крышка промежуточную ступень не трогает.
	plans = []models.NetworkPlan{separateBrand(115, models.CapModePct, models.PtrFloat(1))}
	got, _ = BuildNetworkPlanCalculations(plans, noVAT, nil)
	if v := models.ValFloat(got[0].ForecastBaseRub); v != 115 {
		t.Errorf("процентная крышка на второй ступени не режет: база %v, ожидалось 115", v)
	}
}

func TestScalesSKUOverridesInsideClosedBrand(t *testing.T) {
	// Бренд закрыт, план второй ступени 110 разложен по SKU: A 60, B 40,
	// остаток 10 без SKU-строки. SKU B — открытая крышка и свой процент.
	plan := separateBrand(130, models.CapModeClosed, nil)
	plan.Scales[0].SKUs = []models.NetworkPlanScaleSKU{
		{SKU: "A", PlanRub: models.PtrFloat(60), ForecastRub: models.PtrFloat(70)},
		{SKU: "B", PlanRub: models.PtrFloat(40), ForecastRub: models.PtrFloat(45),
			InvestmentsPct: models.PtrFloat(7), CapMode: models.CapModeOpen},
	}
	// Третья ступень не достигнута: 130 > 120? Достигнута. Опустим прогноз до 115.
	plan.ForecastRub = models.PtrFloat(115)
	plan.Scales[0].SKUs[0].ForecastRub = models.PtrFloat(62)
	plan.Scales[0].SKUs[1].ForecastRub = models.PtrFloat(41)

	got, _ := BuildNetworkPlanCalculations([]models.NetworkPlan{plan}, noVAT, nil)
	row := got[0]
	if row.ForecastScale != 2 {
		t.Fatalf("ожидалась ступень 2, получена %d", row.ForecastScale)
	}
	// A: закрытая → база 60 × 6,5 % = 3,9; B: открытая → 41 × 7 % = 2,87;
	// остаток бренда: план 10, объём 115−62−41 = 12, закрытая → 10 × 6,5 % = 0,65.
	if v := models.ValFloat(row.ForecastBaseRub); v != 111 {
		t.Errorf("база = %v, ожидалось 60 + 41 + 10 = 111", v)
	}
	if v := models.ValFloat(row.ForecastInvestmentsRub); v != 7.42 {
		t.Errorf("к выплате = %v, ожидалось 3,9 + 2,87 + 0,65 = 7,42", v)
	}
	second := row.Scales[1]
	if v := models.ValFloat(second.SKUs[1].ForecastInvestmentsRub); v != 2.87 {
		t.Errorf("SKU B: %v, ожидалось 2,87", v)
	}
	if v := models.ValFloat(second.SKUs[0].ForecastBaseRub); v != 60 {
		t.Errorf("SKU A при закрытой крышке — база в план: %v", v)
	}
	// Плановые инвестиции ступени учитывают процент SKU: 60×6,5 + 40×7 + 10×6,5 = 7,35.
	if v := models.ValFloat(second.PlanInvestmentsRub); v != 7.35 {
		t.Errorf("плановые инвестиции ступени 2 = %v, ожидалось 7,35", v)
	}
}

func TestScalesPoolCapAndGrossBrandPlans(t *testing.T) {
	// Пул: пороги 100 / 110 / 120, крышка +5 % на последней ступени.
	// Бренды A и B распределяют его: A 60 / 66 / 72, B 40 / 44 / 48.
	pool := models.NetworkPlan{
		Quarter: 1, PlanRub: models.PtrFloat(100),
		CapMode: models.CapModePct, CapPct: models.PtrFloat(5),
		Scales: []models.NetworkPlanScale{
			{ScaleNo: 2, PlanRub: models.PtrFloat(110)},
			{ScaleNo: 3, PlanRub: models.PtrFloat(120)},
		},
	}
	brandA := models.NetworkPlan{
		Quarter: 1, BrandAS: brandPtr("A"), InGross: true,
		PlanRub: models.PtrFloat(60), InvestmentsPct: models.PtrFloat(5),
		ForecastRub: models.PtrFloat(79),
		Scales:      []models.NetworkPlanScale{scale(2, 66, 6.5), scale(3, 72, 8)},
	}
	brandB := models.NetworkPlan{
		Quarter: 1, BrandAS: brandPtr("B"), InGross: true,
		PlanRub: models.PtrFloat(40), InvestmentsPct: models.PtrFloat(4),
		ForecastRub: models.PtrFloat(52),
		Scales:      []models.NetworkPlanScale{scale(2, 44, 5), scale(3, 48, 6)},
	}
	got, totals := BuildNetworkPlanCalculations([]models.NetworkPlan{pool, brandA, brandB}, noVAT, nil)

	// Пул: 131 ≥ 120 — третья ступень.
	if got[0].ForecastScale != 3 || !got[0].Scales[2].ForecastReached {
		t.Errorf("пул должен закрыть третью ступень: %d", got[0].ForecastScale)
	}
	// A: min(79, 72 × 1,05 = 75,6) × 8 % = 6,05; B: min(52, 50,4) × 6 % = 3,02.
	if v := models.ValFloat(got[1].ForecastBaseRub); v != 75.6 {
		t.Errorf("A база = %v, ожидалось 75,6", v)
	}
	if v := models.ValFloat(got[1].ForecastInvestmentsRub); v != 6.05 {
		t.Errorf("A к выплате = %v, ожидалось 6,05", v)
	}
	if v := models.ValFloat(got[2].ForecastInvestmentsRub); v != 3.02 {
		t.Errorf("B к выплате = %v, ожидалось 3,02", v)
	}
	if got[1].InvestmentScope != "gross" || got[1].ForecastScale != 3 {
		t.Errorf("валовый бренд платится по ступени пула: %s / %d", got[1].InvestmentScope, got[1].ForecastScale)
	}

	// Остаток к распределению по ступеням: 100−100, 110−110, 120−120.
	scales := totals[0].Scales
	if len(scales) != 3 {
		t.Fatalf("три ступени в итогах, получено %d", len(scales))
	}
	for _, s := range scales {
		if models.ValFloat(s.Undistributed) != 0 {
			t.Errorf("ступень %d: остаток %v, ожидался 0", s.ScaleNo, models.ValFloat(s.Undistributed))
		}
	}
	if scales[1].ContractPlanRub != 110 || scales[2].InvestmentsRub != round2(72*0.08+48*0.06) {
		t.Errorf("итоги ступеней: %+v", scales)
	}

	// Бренд с короткой лестницей в трёхступенчатом пуле платится по своей верхней.
	brandB.Scales = brandB.Scales[:1]
	got, _ = BuildNetworkPlanCalculations([]models.NetworkPlan{pool, brandA, brandB}, noVAT, nil)
	if got[2].ForecastScale != 2 {
		t.Errorf("у B две ступени — платится по второй, получено %d", got[2].ForecastScale)
	}
	// Пул на последней ступени, а у B план на второй — крышка пула режет от него: min(52, 44×1,05=46,2) × 5 %.
	if v := models.ValFloat(got[2].ForecastInvestmentsRub); v != 2.31 {
		t.Errorf("B к выплате = %v, ожидалось 2,31", v)
	}
}

func TestScalesUndistributedPerScale(t *testing.T) {
	pool := models.NetworkPlan{
		Quarter: 1, PlanRub: models.PtrFloat(100),
		Scales: []models.NetworkPlanScale{{ScaleNo: 2, PlanRub: models.PtrFloat(110)}},
	}
	brand := models.NetworkPlan{
		Quarter: 1, BrandAS: brandPtr("A"), InGross: true,
		PlanRub: models.PtrFloat(60), InvestmentsPct: models.PtrFloat(5),
		Scales: []models.NetworkPlanScale{scale(2, 66, 6.5)},
	}
	_, totals := BuildNetworkPlanCalculations([]models.NetworkPlan{pool, brand}, noVAT, nil)
	scales := totals[0].Scales
	if models.ValFloat(scales[0].Undistributed) != 40 || models.ValFloat(scales[1].Undistributed) != 44 {
		t.Errorf("остаток по ступеням 40 / 44, получено %v / %v", models.ValFloat(scales[0].Undistributed), models.ValFloat(scales[1].Undistributed))
	}
	// Ступень 1 итогов повторяет поля квартала.
	if models.ValFloat(totals[0].Undistributed) != 40 || totals[0].ContractPlanRub != scales[0].ContractPlanRub {
		t.Error("ступень 1 итогов должна повторять квартальные поля")
	}
	year := SumYearTotals(totals)
	if len(year.Scales) != 2 || models.ValFloat(year.Scales[1].Undistributed) != 44 {
		t.Errorf("годовые итоги по ступеням: %+v", year.Scales)
	}
}

func TestScalesPortfolioGroupSumsLadders(t *testing.T) {
	periods := []models.NetworkPeriod{
		{Quarter: 1, VATIncluded: false}, {Quarter: 2, VATIncluded: false},
	}
	q1 := separateBrand(95, models.CapModeOpen, nil)
	q2 := separateBrand(130, models.CapModeOpen, nil)
	q2.Quarter = 2
	groups := []models.NetworkPeriodGroup{{Year: 2026, StartQuarter: 1, EndQuarter: 2}}

	// Портфель: пороги 200 / 220 / 240, объём 225 — вторая ступень для обоих кварталов.
	got, _ := BuildNetworkPlanCalculations([]models.NetworkPlan{q1, q2}, periods, groups)
	for _, row := range got {
		if row.InvestmentScope != "portfolio" || row.ForecastScale != 2 {
			t.Errorf("Q%d: область %s, ступень %d; ожидались portfolio / 2", row.Quarter, row.InvestmentScope, row.ForecastScale)
		}
	}
	// Q1 сам по себе не закрыл бы и первую ступень, но в объединении платится по второй.
	if v := models.ValFloat(got[0].ForecastInvestmentsRub); v != round2(95*0.065) {
		t.Errorf("Q1 к выплате = %v, ожидалось 95 × 6,5 %%", v)
	}

	// Квартал без верхних ступеней укорачивает лестницу периода до одной.
	q2.Scales = nil
	got, _ = BuildNetworkPlanCalculations([]models.NetworkPlan{q1, q2}, periods, groups)
	if got[0].ForecastScale != 1 {
		t.Errorf("лестница периода — по кварталу с наименьшим числом ступеней: %d", got[0].ForecastScale)
	}
}

func TestScalesIgnoredForPayFromFact(t *testing.T) {
	plan := separateBrand(140, models.CapModeClosed, nil)
	plan.PayInvestmentsFromFact = true
	plan.FactRub = models.PtrFloat(50)
	got, _ := BuildNetworkPlanCalculations([]models.NetworkPlan{plan}, noVAT, nil)
	row := got[0]
	if row.ForecastScale != 0 || row.FactScale != 0 {
		t.Errorf("«от факта» ступеней не имеет: %d / %d", row.ForecastScale, row.FactScale)
	}
	if v := models.ValFloat(row.ForecastInvestmentsRub); v != 7 {
		t.Errorf("процент бренда со всего объёма без крышки: %v, ожидалось 140 × 5 %%", v)
	}
	if v := models.ValFloat(row.FactInvestmentsRub); v != 2.5 || models.ValFloat(row.FactBaseRub) != 50 {
		t.Errorf("факт 50 × 5 %% = 2,5 без порога, получено %v", v)
	}
}

func TestScalesLadderStopsAtNonIncreasingThreshold(t *testing.T) {
	plan := separateBrand(140, models.CapModeOpen, nil)
	plan.Scales[1].PlanRub = models.PtrFloat(105) // третья ступень ниже второй — не ступень
	got, _ := BuildNetworkPlanCalculations([]models.NetworkPlan{plan}, noVAT, nil)
	if got[0].ForecastScale != 2 {
		t.Errorf("лестница обрывается на нерастущем пороге: ступень %d, ожидалась 2", got[0].ForecastScale)
	}
}

func TestCompletePairUsesMonthlyPrices(t *testing.T) {
	distribution := [3]float64{30, 30, 40}
	// Цена меняется внутри квартала: 100 в первых двух месяцах, 110 в третьем.
	price := func(month int) *float64 {
		if month == 3 {
			return models.PtrFloat(110)
		}
		return models.PtrFloat(100)
	}
	rub, units := completePair(nil, models.PtrFloat(1000), "units", 1, distribution, price)
	// 300×100 + 300×100 + 400×110 = 104 000.
	if models.ValFloat(rub) != 104000 || models.ValFloat(units) != 1000 {
		t.Errorf("упаковки → рубли по месяцам: %v / %v", models.ValFloat(rub), models.ValFloat(units))
	}

	rub, units = completePair(models.PtrFloat(104000), nil, "rub", 1, distribution, price)
	// 31 200/100 + 31 200/100 + 41 600/110 = 312 + 312 + 378,18.
	if models.ValFloat(units) != 1002.18 || models.ValFloat(rub) != 104000 {
		t.Errorf("рубли → упаковки по месяцам: %v / %v", models.ValFloat(rub), models.ValFloat(units))
	}

	// Без цены вторая метрика остаётся, какой была.
	noPrice := func(int) *float64 { return nil }
	rub, units = completePair(models.PtrFloat(500), models.PtrFloat(7), "rub", 1, distribution, noPrice)
	if models.ValFloat(rub) != 500 || models.ValFloat(units) != 7 {
		t.Errorf("без цены сохранённая пара не стирается: %v / %v", models.ValFloat(rub), models.ValFloat(units))
	}

	// Режим «упаковки», но клиент прислал только рубли: рубли — введённое.
	rub, units = completePair(models.PtrFloat(1000), nil, "units", 1, distribution, price)
	if models.ValFloat(rub) != 1000 || units == nil {
		t.Errorf("рубли без упаковок в режиме units считаются введёнными: %v / %v", models.ValFloat(rub), units)
	}
}
