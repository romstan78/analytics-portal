package services

import (
	"math"
	"strconv"

	"backend/models"
)

// NetworkPlanTotals определён в models: из него генерируется тип фронтенда.
type NetworkPlanTotals = models.NetworkPlanTotals

// round2 округляет до копеек, чтобы расчётные суммы не тянули хвост float.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// NetRub переводит сумму инвестиций в базу «с вычетом НДС».
// Сеть без НДС в этом квартале — сумма остаётся как есть.
func NetRub(gross float64, vatIncluded bool, vatRate float64) float64 {
	if !vatIncluded || vatRate <= 0 {
		return round2(gross)
	}
	return round2(gross / (1 + vatRate/100))
}

// periodsByQuarter — быстрый доступ к настройкам НДС нужного квартала.
func periodsByQuarter(periods []models.NetworkPeriod) map[int]models.NetworkPeriod {
	byQuarter := make(map[int]models.NetworkPeriod, len(periods))
	for _, p := range periods {
		byQuarter[p.Quarter] = p
	}
	return byQuarter
}

// investmentsFor считает инвестиции от суммы объёма: до вычета НДС и с вычетом.
func investmentsFor(volume, pct float64, vatIncluded bool, vatRate float64) (gross, net float64) {
	gross = round2(volume * pct / 100)
	return gross, NetRub(gross, vatIncluded, vatRate)
}

// EnrichNetworkPlans заполняет расчётные поля строк плана: рубли инвестиций
// от процента — отдельно для планового объёма и для прогноза, каждый раз
// до вычета НДС и с вычетом, если сеть работает с НДС в этом квартале.
func EnrichNetworkPlans(plans []models.NetworkPlan, periods []models.NetworkPeriod) []models.NetworkPlan {
	byQuarter := periodsByQuarter(periods)

	for i := range plans {
		p := &plans[i]
		period, ok := byQuarter[p.Quarter]
		vatIncluded, vatRate := false, 0.0
		if ok {
			vatIncluded, vatRate = period.VATIncluded, period.VATRate
		}

		// Факт инвестиций пришёл суммой: процентом не пересчитывается, но базу
		// «без НДС» показываем по ставке того же квартала.
		if p.FactInvestmentsRub != nil {
			net := NetRub(*p.FactInvestmentsRub, vatIncluded, vatRate)
			p.FactInvestmentsNet = &net
		}

		if p.InvestmentsPct == nil {
			continue
		}
		if p.PlanRub != nil {
			gross, net := investmentsFor(*p.PlanRub, *p.InvestmentsPct, vatIncluded, vatRate)
			p.InvestmentsRub = &gross
			p.InvestmentsNet = &net
		}
		if p.ForecastRub != nil {
			gross, net := investmentsFor(*p.ForecastRub, *p.InvestmentsPct, vatIncluded, vatRate)
			p.ForecastInvestmentsRub = &gross
			p.ForecastInvestmentsNet = &net
		}
	}
	return plans
}

// CalculateNetworkTotals считает итоги по кварталам.
// Строка без бренда — общий объём валового контракта; бренды с in_gross его
// распределяют, остаток показывается отдельно. Бренды без in_gross к пулу
// отношения не имеют и в остаток не попадают.
func CalculateNetworkTotals(plans []models.NetworkPlan, periods []models.NetworkPeriod) []NetworkPlanTotals {
	byQuarter := periodsByQuarter(periods)

	totals := make([]NetworkPlanTotals, 4)
	for i := range totals {
		totals[i].Quarter = i + 1
	}

	for _, p := range plans {
		if p.Quarter < 1 || p.Quarter > 4 {
			continue
		}
		t := &totals[p.Quarter-1]
		period := byQuarter[p.Quarter]

		// Строка пула: объём контракта целиком, инвестиции по ней не ведутся.
		if p.BrandAS == nil {
			if p.PlanRub != nil {
				pool := round2(*p.PlanRub)
				t.GrossPoolRub = &pool
			}
			if p.ForecastRub != nil {
				fcst := round2(*p.ForecastRub)
				t.GrossPoolFcstRub = &fcst
			}
			continue
		}

		if p.InGross {
			t.GrossBrandsCount++
		}
		if p.PlanRub != nil {
			t.PlanRub = round2(t.PlanRub + *p.PlanRub)
			if p.InGross {
				t.GrossBrandsPlan = round2(t.GrossBrandsPlan + *p.PlanRub)
			} else {
				t.SeparatePlanRub = round2(t.SeparatePlanRub + *p.PlanRub)
			}
		}
		if p.FactRub != nil {
			t.FactRub = round2(t.FactRub + *p.FactRub)
			if p.InGross {
				t.GrossPoolFactRub = round2(t.GrossPoolFactRub + *p.FactRub)
			}
		}
		if p.ForecastRub != nil {
			t.ForecastRub = round2(t.ForecastRub + *p.ForecastRub)
		}
		if p.FactInvestmentsRub != nil {
			t.FactInvestmentsRub = round2(t.FactInvestmentsRub + *p.FactInvestmentsRub)
			t.FactInvestmentsRubNet = round2(t.FactInvestmentsRubNet +
				NetRub(*p.FactInvestmentsRub, period.VATIncluded, period.VATRate))
		}

		if p.InvestmentsPct == nil {
			continue
		}
		if p.PlanRub != nil {
			gross, net := investmentsFor(*p.PlanRub, *p.InvestmentsPct, period.VATIncluded, period.VATRate)
			t.InvestmentsRub = round2(t.InvestmentsRub + gross)
			t.InvestmentsRubNet = round2(t.InvestmentsRubNet + net)
		}
		if p.ForecastRub != nil {
			gross, net := investmentsFor(*p.ForecastRub, *p.InvestmentsPct, period.VATIncluded, period.VATRate)
			t.ForecastInvestmentsRub = round2(t.ForecastInvestmentsRub + gross)
			t.ForecastInvestmentsRubNet = round2(t.ForecastInvestmentsRubNet + net)
		}
	}

	for i := range totals {
		t := &totals[i]
		// Остаток есть только там, где пул заведён: без него распределять нечего.
		if t.GrossPoolRub != nil {
			rest := round2(*t.GrossPoolRub - t.GrossBrandsPlan)
			t.Undistributed = &rest
		}
		// Обязательство по контракту: пул считается целиком, даже если бренды
		// разобрали его не полностью; отдельные бренды прибавляются как есть.
		pool := t.GrossBrandsPlan
		if t.GrossPoolRub != nil {
			pool = *t.GrossPoolRub
		}
		t.ContractPlanRub = round2(pool + t.SeparatePlanRub)
	}
	return totals
}

// SumYearTotals складывает итоги кварталов в итог года.
// Поля пула суммируются только по тем кварталам, где пул заведён: год без
// единого пула остаётся без остатка к распределению, а не с нулём.
// Брендов в пуле за год — не сумма, а максимум по кварталу: это состав, а не объём.
func SumYearTotals(totals []NetworkPlanTotals) NetworkPlanTotals {
	year := NetworkPlanTotals{}

	addOptional := func(target **float64, value float64) {
		current := 0.0
		if *target != nil {
			current = **target
		}
		sum := round2(current + value)
		*target = &sum
	}

	for _, t := range totals {
		year.PlanRub = round2(year.PlanRub + t.PlanRub)
		year.GrossBrandsPlan = round2(year.GrossBrandsPlan + t.GrossBrandsPlan)
		year.SeparatePlanRub = round2(year.SeparatePlanRub + t.SeparatePlanRub)
		year.ContractPlanRub = round2(year.ContractPlanRub + t.ContractPlanRub)
		if t.GrossBrandsCount > year.GrossBrandsCount {
			year.GrossBrandsCount = t.GrossBrandsCount
		}
		year.FactRub = round2(year.FactRub + t.FactRub)
		year.GrossPoolFactRub = round2(year.GrossPoolFactRub + t.GrossPoolFactRub)
		year.ForecastRub = round2(year.ForecastRub + t.ForecastRub)
		year.InvestmentsRub = round2(year.InvestmentsRub + t.InvestmentsRub)
		year.InvestmentsRubNet = round2(year.InvestmentsRubNet + t.InvestmentsRubNet)
		year.ForecastInvestmentsRub = round2(year.ForecastInvestmentsRub + t.ForecastInvestmentsRub)
		year.ForecastInvestmentsRubNet = round2(year.ForecastInvestmentsRubNet + t.ForecastInvestmentsRubNet)
		year.FactInvestmentsRub = round2(year.FactInvestmentsRub + t.FactInvestmentsRub)
		year.FactInvestmentsRubNet = round2(year.FactInvestmentsRubNet + t.FactInvestmentsRubNet)

		if t.GrossPoolRub != nil {
			addOptional(&year.GrossPoolRub, *t.GrossPoolRub)
		}
		if t.GrossPoolFcstRub != nil {
			addOptional(&year.GrossPoolFcstRub, *t.GrossPoolFcstRub)
		}
		if t.Undistributed != nil {
			addOptional(&year.Undistributed, *t.Undistributed)
		}
	}
	return year
}

// NetworkPlanDraft — строка сетки, как её ввёл пользователь.
// Факта здесь нет: он приходит загрузкой отгрузок и берётся из сохранённых строк.
type NetworkPlanDraft struct {
	Quarter        int
	BrandAS        *string
	InGross        bool
	PlanRub        *float64
	ForecastRub    *float64
	InvestmentsPct *float64
}

// draftKey — ключ строки внутри года: квартал + бренд (пусто = валовый пул).
func draftKey(quarter int, brand *string) string {
	name := ""
	if brand != nil {
		name = *brand
	}
	return strconv.Itoa(quarter) + "|" + name
}

// PreviewNetworkPlans пересчитывает несохранённый черновик: накладывает
// введённые значения на факт из сохранённых строк и считает то же самое,
// что вернётся после сохранения. В БД ничего не пишется.
//
// Набор строк задаёт черновик: сохранённая строка, которой в нём нет, в итоги
// не попадает — иначе таблица показывала бы суммы по скрытым брендам.
func PreviewNetworkPlans(
	draft []NetworkPlanDraft,
	stored []models.NetworkPlan,
	periods []models.NetworkPeriod,
) ([]models.NetworkPlan, []NetworkPlanTotals, NetworkPlanTotals) {
	factByKey := make(map[string]models.NetworkPlan, len(stored))
	for _, plan := range stored {
		factByKey[draftKey(plan.Quarter, plan.BrandAS)] = plan
	}

	plans := make([]models.NetworkPlan, 0, len(draft))
	for _, row := range draft {
		plan := models.NetworkPlan{
			Quarter:        row.Quarter,
			BrandAS:        row.BrandAS,
			InGross:        row.InGross,
			PlanRub:        row.PlanRub,
			ForecastRub:    row.ForecastRub,
			InvestmentsPct: row.InvestmentsPct,
		}
		if saved, ok := factByKey[draftKey(row.Quarter, row.BrandAS)]; ok {
			plan.ID = saved.ID
			plan.NetworkID = saved.NetworkID
			plan.Year = saved.Year
			plan.PlanUnits = saved.PlanUnits
			plan.FactRub = saved.FactRub
			plan.FactInvestmentsRub = saved.FactInvestmentsRub
			plan.UpdatedBy = saved.UpdatedBy
			plan.UpdatedAt = saved.UpdatedAt
		}
		plans = append(plans, plan)
	}

	plans = EnrichNetworkPlans(plans, periods)
	totals := CalculateNetworkTotals(plans, periods)
	return plans, totals, SumYearTotals(totals)
}
