package services

import (
	"math"

	"backend/models"
)

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
		if p.InvestmentsPct == nil {
			continue
		}
		period, ok := byQuarter[p.Quarter]
		vatIncluded, vatRate := false, 0.0
		if ok {
			vatIncluded, vatRate = period.VATIncluded, period.VATRate
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
