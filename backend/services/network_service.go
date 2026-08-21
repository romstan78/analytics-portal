package services

import (
	"math"

	"backend/models"
)

// NetworkPlanTotals — итоги квартала для шапки сетки планов.
// НДС применяется только к инвестициям: план остаётся тем, что ввёл КАМ.
type NetworkPlanTotals struct {
	Quarter           int      `json:"quarter"`
	PlanRub           float64  `json:"plan_rub"`            // сумма плана по брендам
	GrossPlanRub      *float64 `json:"gross_plan_rub"`      // общий объём валового контракта
	Undistributed     *float64 `json:"undistributed"`       // остаток к распределению
	InvestmentsRub    float64  `json:"investments_rub"`     // инвестиции до вычета НДС
	InvestmentsRubNet float64  `json:"investments_rub_net"` // они же с вычетом НДС
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

// EnrichNetworkPlans заполняет расчётные поля строк плана: рубли инвестиций
// от процента — до вычета НДС и с вычетом, если сеть работает с НДС.
func EnrichNetworkPlans(plans []models.NetworkPlan, periods []models.NetworkPeriod) []models.NetworkPlan {
	byQuarter := make(map[int]models.NetworkPeriod, len(periods))
	for _, p := range periods {
		byQuarter[p.Quarter] = p
	}

	for i := range plans {
		p := &plans[i]
		if p.PlanRub == nil || p.InvestmentsPct == nil {
			continue
		}
		period, ok := byQuarter[p.Quarter]
		vatIncluded, vatRate := false, 0.0
		if ok {
			vatIncluded, vatRate = period.VATIncluded, period.VATRate
		}

		gross := round2(*p.PlanRub * *p.InvestmentsPct / 100)
		net := NetRub(gross, vatIncluded, vatRate)
		p.InvestmentsRub = &gross
		p.InvestmentsNet = &net
	}
	return plans
}

// CalculateNetworkTotals считает итоги по кварталам.
// Для валового контракта строка с пустым брендом — общий объём, а бренды
// распределяют его; остаток показывается отдельно.
func CalculateNetworkTotals(plans []models.NetworkPlan, periods []models.NetworkPeriod) []NetworkPlanTotals {
	byQuarter := make(map[int]models.NetworkPeriod, len(periods))
	for _, p := range periods {
		byQuarter[p.Quarter] = p
	}

	totals := make([]NetworkPlanTotals, 4)
	for i := range totals {
		totals[i].Quarter = i + 1
	}

	for _, p := range plans {
		if p.Quarter < 1 || p.Quarter > 4 || p.PlanRub == nil {
			continue
		}
		t := &totals[p.Quarter-1]
		period := byQuarter[p.Quarter]

		if p.BrandAS == nil {
			gross := round2(*p.PlanRub)
			t.GrossPlanRub = &gross
			continue
		}

		t.PlanRub = round2(t.PlanRub + *p.PlanRub)
		if p.InvestmentsPct != nil {
			investments := round2(*p.PlanRub * *p.InvestmentsPct / 100)
			t.InvestmentsRub = round2(t.InvestmentsRub + investments)
			t.InvestmentsRubNet = round2(t.InvestmentsRubNet + NetRub(investments, period.VATIncluded, period.VATRate))
		}
	}

	for i := range totals {
		t := &totals[i]
		if t.GrossPlanRub != nil {
			rest := round2(*t.GrossPlanRub - t.PlanRub)
			t.Undistributed = &rest
		}
	}
	return totals
}
