package services

import (
	"math"
	"sort"
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

// NetworkPeriodsWithDefaults достраивает год до четырёх кварталов: незаведённый
// квартал берёт НДС из карточки сети. Правило должно быть одним для карточки и
// для витрины реестра — иначе одна и та же сеть считалась бы по разным ставкам.
func NetworkPeriodsWithDefaults(
	network models.Network,
	year int,
	persisted []models.NetworkPeriod,
) []models.NetworkPeriod {
	byQuarter := make(map[int]models.NetworkPeriod, len(persisted))
	for _, period := range persisted {
		byQuarter[period.Quarter] = period
	}
	periods := make([]models.NetworkPeriod, 0, 4)
	for quarter := 1; quarter <= 4; quarter++ {
		period := byQuarter[quarter]
		period.NetworkID = network.ID
		period.Year = year
		period.Quarter = quarter
		if period.ID == 0 {
			period.VATIncluded = network.VATIncluded
			period.VATRate = network.VATRate
		}
		periods = append(periods, period)
	}
	return periods
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

// paymentEACRub — фактический объём закрытой части плюс прогноз оставшейся.
// Квартальный forecast_rub уже хранит этот EAC; для старых строк используем факт.
func paymentEACRub(plan models.NetworkPlan) *float64 {
	if plan.ForecastRub != nil {
		return plan.ForecastRub
	}
	return plan.FactRub
}

// EnrichNetworkPlans заполняет плановые инвестиции: процент от планового
// объёма, в обеих базах НДС. Порога здесь нет и быть не может — план равен
// плану по определению.
//
// Прогнозные и фактические инвестиции эта функция намеренно не трогает: они
// зависят от выполнения плана в области, которая шире одной строки, и потому
// считаются в ApplyNetworkInvestmentRule, когда квартальные итоги уже известны.
func EnrichNetworkPlans(plans []models.NetworkPlan, periods []models.NetworkPeriod) []models.NetworkPlan {
	byQuarter := periodsByQuarter(periods)

	for i := range plans {
		p := &plans[i]
		period := byQuarter[p.Quarter]

		if p.InvestmentsPct == nil || p.PlanRub == nil {
			p.InvestmentsRub = nil
			p.InvestmentsNet = nil
			continue
		}
		gross, net := investmentsFor(*p.PlanRub, *p.InvestmentsPct, period.VATIncluded, period.VATRate)
		p.InvestmentsRub = &gross
		p.InvestmentsNet = &net
	}
	return plans
}

// ApplyNetworkInvestmentRule считает прогнозные и фактические инвестиции по
// одному правилу: объём × процент, если объём закрыл план своей области, иначе
// ноль. Прогнозные меряются прогнозом, фактические — фактом; всё остальное в
// правиле общее, поэтому и код общий.
//
// Ноль и «не считается» — разные вещи: строка без процента остаётся с nil,
// строка с процентом, не закрывшая план, получает явный ноль. Иначе потребитель
// не отличил бы незаполненную ставку от заработанного ничего.
func ApplyNetworkInvestmentRule(
	plans []models.NetworkPlan,
	periods []models.NetworkPeriod,
	totals []NetworkPlanTotals,
	groups []models.NetworkPeriodGroup,
) []models.NetworkPlan {
	byQuarter := periodsByQuarter(periods)

	for i := range plans {
		row := &plans[i]
		row.InvestmentScope = ""
		row.InvestmentPeriodStartQuarter = 0
		row.InvestmentPeriodEndQuarter = 0
		row.ForecastCompletionPct = nil
		row.FactCompletionPct = nil
		row.ForecastInvestmentsEarned = false
		row.FactInvestmentsEarned = false
		// Строка пула инвестиций не ведёт: процент задаётся бренду.
		if row.BrandAS == nil {
			continue
		}

		period := byQuarter[row.Quarter]

		forecastPlan, forecastGot, startQuarter, endQuarter, scope :=
			investmentEvaluation(*row, groups, plans, totals, achievedByForecast)
		factPlan, factGot, _, _, _ :=
			investmentEvaluation(*row, groups, plans, totals, achievedByFact)

		row.InvestmentScope = scope
		row.InvestmentPeriodStartQuarter = startQuarter
		row.InvestmentPeriodEndQuarter = endQuarter
		row.ForecastCompletionPct = completionPct(forecastGot, forecastPlan)
		row.FactCompletionPct = completionPct(factGot, factPlan)
		row.ForecastInvestmentsEarned = forecastPlan > 0 && forecastGot >= forecastPlan
		row.FactInvestmentsEarned = factPlan > 0 && factGot >= factPlan

		// «Оплата от факта» — договор без порога: процент начисляется с любого
		// отгруженного рубля. Область при этом всегда собственная строка.
		if row.PayInvestmentsFromFact {
			row.InvestmentScope = "fact"
			row.InvestmentPeriodStartQuarter = row.Quarter
			row.InvestmentPeriodEndQuarter = row.Quarter
			row.ForecastInvestmentsEarned = true
			row.FactInvestmentsEarned = true
		}

		// Прогнозные. Введённое человеком переопределение порог не отменяет:
		// разовая выплата вне процента — осознанное решение, а не расчёт.
		if !row.ForecastInvestmentsOverridden {
			row.ForecastInvestmentsRub, row.ForecastInvestmentsNet = investmentsOrZero(
				row.ForecastInvestmentsEarned, paymentEACRub(*row), row.InvestmentsPct, period,
			)
		} else if row.ForecastInvestmentsRub != nil {
			net := NetRub(*row.ForecastInvestmentsRub, period.VATIncluded, period.VATRate)
			row.ForecastInvestmentsNet = &net
		}

		// Фактические. База — только отгруженное: инвестиции по факту
		// появляются по закрытии периода, а не по ожиданию.
		row.FactInvestmentsRub, row.FactInvestmentsNet = investmentsOrZero(
			row.FactInvestmentsEarned, row.FactRub, row.InvestmentsPct, period,
		)
	}
	return plans
}

// investmentsOrZero — общий хвост правила: процент от объёма при пройденном
// пороге, явный ноль при непройденном, nil когда считать нечем.
func investmentsOrZero(
	earned bool,
	volume, pct *float64,
	period models.NetworkPeriod,
) (gross, net *float64) {
	if pct == nil || volume == nil {
		return nil, nil
	}
	if !earned {
		zero, zeroNet := 0.0, 0.0
		return &zero, &zeroNet
	}
	grossValue, netValue := investmentsFor(*volume, *pct, period.VATIncluded, period.VATRate)
	return &grossValue, &netValue
}

// CalculateNetworkTotals считает итоги по кварталам.
// Строка без бренда — общий объём валового контракта; бренды с in_gross его
// распределяют, остаток показывается отдельно. Бренды без in_gross к пулу
// отношения не имеют и в остаток не попадают.
func CalculateNetworkTotals(plans []models.NetworkPlan, _ []models.NetworkPeriod) []NetworkPlanTotals {
	totals := make([]NetworkPlanTotals, 4)
	for i := range totals {
		totals[i].Quarter = i + 1
	}

	for _, p := range plans {
		if p.Quarter < 1 || p.Quarter > 4 {
			continue
		}
		t := &totals[p.Quarter-1]

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
		if eac := paymentEACRub(p); eac != nil {
			t.EACRub = round2(t.EACRub + *eac)
		}
		// Инвестиции уже посчитаны по строке — здесь только сложение. Своей
		// арифметики у итога нет намеренно: иначе появилась бы вторая, которая
		// однажды разойдётся со строкой.
		t.InvestmentsRub = round2(t.InvestmentsRub + models.ValFloat(p.InvestmentsRub))
		t.InvestmentsRubNet = round2(t.InvestmentsRubNet + models.ValFloat(p.InvestmentsNet))
		t.ForecastInvestmentsRub = round2(t.ForecastInvestmentsRub + models.ValFloat(p.ForecastInvestmentsRub))
		t.ForecastInvestmentsRubNet = round2(t.ForecastInvestmentsRubNet + models.ValFloat(p.ForecastInvestmentsNet))
		t.FactInvestmentsRub = round2(t.FactInvestmentsRub + models.ValFloat(p.FactInvestmentsRub))
		t.FactInvestmentsRubNet = round2(t.FactInvestmentsRubNet + models.ValFloat(p.FactInvestmentsNet))
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
		t.CompletionPct = completionPct(t.EACRub, t.ContractPlanRub)
		t.Completed = t.ContractPlanRub > 0 && t.EACRub >= t.ContractPlanRub
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
		year.EACRub = round2(year.EACRub + t.EACRub)

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
	year.CompletionPct = completionPct(year.EACRub, year.ContractPlanRub)
	year.Completed = year.ContractPlanRub > 0 && year.EACRub >= year.ContractPlanRub
	return year
}

// CalculateNetworkPeriodGroupTotals считает совместный зачёт смежных
// кварталов. Квартальные строки остаются источником истины: здесь они только
// складываются по диапазону и области правила.
//
// Для всего портфеля план — обязательство по контракту, чтобы валовый пул не
// потерял нераспределённый остаток. Для бренда берётся его собственный план.
func CalculateNetworkPeriodGroupTotals(
	groups []models.NetworkPeriodGroup,
	plans []models.NetworkPlan,
	totals []NetworkPlanTotals,
) []models.NetworkPeriodGroupTotals {
	result := make([]models.NetworkPeriodGroupTotals, 0, len(groups))

	for _, group := range groups {
		combined := models.NetworkPeriodGroupTotals{
			StartQuarter: group.StartQuarter,
			EndQuarter:   group.EndQuarter,
			BrandAS:      group.BrandAS,
		}

		if group.BrandAS == nil {
			for quarter := group.StartQuarter; quarter <= group.EndQuarter; quarter++ {
				if quarter < 1 || quarter > len(totals) {
					continue
				}
				t := totals[quarter-1]
				combined.PlanRub = round2(combined.PlanRub + t.ContractPlanRub)
				combined.FactRub = round2(combined.FactRub + t.FactRub)
				combined.ForecastRub = round2(combined.ForecastRub + t.ForecastRub)
				combined.InvestmentsRub = round2(combined.InvestmentsRub + t.InvestmentsRub)
				combined.InvestmentsRubNet = round2(combined.InvestmentsRubNet + t.InvestmentsRubNet)
				combined.ForecastInvestmentsRub = round2(combined.ForecastInvestmentsRub + t.ForecastInvestmentsRub)
				combined.ForecastInvestmentsRubNet = round2(combined.ForecastInvestmentsRubNet + t.ForecastInvestmentsRubNet)
				combined.FactInvestmentsRub = round2(combined.FactInvestmentsRub + t.FactInvestmentsRub)
				combined.FactInvestmentsRubNet = round2(combined.FactInvestmentsRubNet + t.FactInvestmentsRubNet)
				combined.EACRub = round2(combined.EACRub + t.EACRub)
			}
			combined.CompletionPct = completionPct(combined.EACRub, combined.PlanRub)
			combined.Completed = combined.PlanRub > 0 && combined.EACRub >= combined.PlanRub
			result = append(result, combined)
			continue
		}

		for _, plan := range plans {
			if plan.BrandAS == nil || *plan.BrandAS != *group.BrandAS ||
				plan.Quarter < group.StartQuarter || plan.Quarter > group.EndQuarter {
				continue
			}
			combined.PlanRub = round2(combined.PlanRub + models.ValFloat(plan.PlanRub))
			combined.FactRub = round2(combined.FactRub + models.ValFloat(plan.FactRub))
			combined.ForecastRub = round2(combined.ForecastRub + models.ValFloat(plan.ForecastRub))
			combined.InvestmentsRub = round2(combined.InvestmentsRub + models.ValFloat(plan.InvestmentsRub))
			combined.InvestmentsRubNet = round2(combined.InvestmentsRubNet + models.ValFloat(plan.InvestmentsNet))
			combined.ForecastInvestmentsRub = round2(combined.ForecastInvestmentsRub + models.ValFloat(plan.ForecastInvestmentsRub))
			combined.ForecastInvestmentsRubNet = round2(combined.ForecastInvestmentsRubNet + models.ValFloat(plan.ForecastInvestmentsNet))
			combined.FactInvestmentsRub = round2(combined.FactInvestmentsRub + models.ValFloat(plan.FactInvestmentsRub))
			combined.FactInvestmentsRubNet = round2(combined.FactInvestmentsRubNet + models.ValFloat(plan.FactInvestmentsNet))
			if eac := paymentEACRub(plan); eac != nil {
				combined.EACRub = round2(combined.EACRub + *eac)
			}
		}
		combined.CompletionPct = completionPct(combined.EACRub, combined.PlanRub)
		combined.Completed = combined.PlanRub > 0 && combined.EACRub >= combined.PlanRub
		result = append(result, combined)
	}

	return result
}

// achievedRub — мера достижения, по которой проверяется порог. Их две:
// прогнозная (EAC) для прогнозных инвестиций и фактическая для фактических.
// Правило порога от выбора не зависит — зависит только то, чем меряют.
type achievedRub func(models.NetworkPlan) *float64

// achievedByForecast — EAC строки: факт закрытых месяцев плюс прогноз открытых.
func achievedByForecast(plan models.NetworkPlan) *float64 { return paymentEACRub(plan) }

// achievedByFact — только отгруженное. Прогноз сюда не входит намеренно:
// фактические инвестиции появляются по закрытии периода, а не по ожиданию.
func achievedByFact(plan models.NetworkPlan) *float64 { return plan.FactRub }

// investmentEvaluation возвращает план и достигнутое по области, в которой
// проверяется порог 100% для конкретной строки. Портфельное объединение
// действует на все бренды диапазона, брендовое — только на выбранный бренд.
// Без объединения валовые бренды оцениваются вместе по пулу квартала,
// отдельные — по собственной строке.
func investmentEvaluation(
	plan models.NetworkPlan,
	groups []models.NetworkPeriodGroup,
	plans []models.NetworkPlan,
	totals []NetworkPlanTotals,
	achieved achievedRub,
) (planRub, eacRub float64, startQuarter, endQuarter int, scope string) {
	startQuarter, endQuarter = plan.Quarter, plan.Quarter
	var selected *models.NetworkPeriodGroup
	for i := range groups {
		group := &groups[i]
		if plan.Quarter < group.StartQuarter || plan.Quarter > group.EndQuarter {
			continue
		}
		if group.BrandAS == nil {
			selected = group
			break
		}
		if plan.BrandAS != nil && *group.BrandAS == *plan.BrandAS {
			selected = group
			break
		}
	}

	if selected != nil {
		startQuarter, endQuarter = selected.StartQuarter, selected.EndQuarter
		if selected.BrandAS == nil {
			scope = "portfolio"
			for quarter := startQuarter; quarter <= endQuarter; quarter++ {
				if quarter >= 1 && quarter <= len(totals) {
					planRub = round2(planRub + totals[quarter-1].ContractPlanRub)
				}
			}
			for _, candidate := range plans {
				if candidate.BrandAS == nil || candidate.Quarter < startQuarter || candidate.Quarter > endQuarter {
					continue
				}
				if value := achieved(candidate); value != nil {
					eacRub = round2(eacRub + *value)
				}
			}
			return
		}

		scope = "brand"
		for _, candidate := range plans {
			if candidate.BrandAS == nil || *candidate.BrandAS != *selected.BrandAS ||
				candidate.Quarter < startQuarter || candidate.Quarter > endQuarter {
				continue
			}
			planRub = round2(planRub + models.ValFloat(candidate.PlanRub))
			if eac := paymentEACRub(candidate); eac != nil {
				eacRub = round2(eacRub + *eac)
			}
		}
		return
	}

	if plan.InGross {
		scope = "gross"
		if plan.Quarter >= 1 && plan.Quarter <= len(totals) {
			total := totals[plan.Quarter-1]
			planRub = total.GrossBrandsPlan
			if total.GrossPoolRub != nil {
				planRub = *total.GrossPoolRub
			}
		}
		for _, candidate := range plans {
			if candidate.BrandAS == nil || !candidate.InGross || candidate.Quarter != plan.Quarter {
				continue
			}
			if value := achieved(candidate); value != nil {
				eacRub = round2(eacRub + *value)
			}
		}
		return
	}

	scope = "brand"
	planRub = models.ValFloat(plan.PlanRub)
	if value := achieved(plan); value != nil {
		eacRub = *value
	}
	return
}

// BuildNetworkPlanCalculations выполняет расчёты в правильном порядке и
// является единственным входом в них: плановые инвестиции, затем квартальные
// итоги объёма (по ним проверяется порог валового пула), затем правило
// инвестиций, затем окончательные итоги.
//
// Порядок здесь не деталь реализации: порог смотрит шире одной строки, поэтому
// посчитать инвестиции до итогов объёма нельзя, а сложить итоги до инвестиций —
// значит сложить не то.
func BuildNetworkPlanCalculations(
	plans []models.NetworkPlan,
	periods []models.NetworkPeriod,
	groups []models.NetworkPeriodGroup,
) ([]models.NetworkPlan, []NetworkPlanTotals) {
	plans = EnrichNetworkPlans(plans, periods)
	volumeTotals := CalculateNetworkTotals(plans, periods)
	plans = ApplyNetworkInvestmentRule(plans, periods, volumeTotals, groups)
	return plans, CalculateNetworkTotals(plans, periods)
}

// annualEACRub возвращает лучший доступный итог объёма строки за квартал.
// forecast_rub хранит квартальный EAC (факт закрытых месяцев + прогноз
// остальных); до появления прогноза используем уже загруженный факт.
func annualEACRub(plan models.NetworkPlan) float64 {
	if value := paymentEACRub(plan); value != nil {
		return *value
	}
	return 0
}

// CalculateNetworkAnnualInvestmentCumulative считает годовой кумулятив
// инвестиций. Начисление каждой области складывается из квартальных EAC,
// умноженных на процент инвестиций соответствующего квартала. Из начисления
// вычитаются фактические выплаты Q1-Q3 и официальный прогноз инвестиций Q4.
// Отрицательная доплата не создаётся.
func CalculateNetworkAnnualInvestmentCumulative(
	plans []models.NetworkPlan,
	periods []models.NetworkPeriod,
	totals []NetworkPlanTotals,
) models.NetworkAnnualInvestmentCumulative {
	result := models.NetworkAnnualInvestmentCumulative{Rows: []models.NetworkAnnualInvestmentRow{}}
	byQuarter := periodsByQuarter(periods)

	var gross models.NetworkAnnualInvestmentRow
	gross.ScopeType = "gross"
	grossExists := false
	brands := make(map[string]*models.NetworkAnnualInvestmentRow)

	for _, total := range totals {
		if total.GrossPoolRub != nil || total.GrossBrandsCount > 0 {
			grossExists = true
			grossPlan := total.GrossBrandsPlan
			if total.GrossPoolRub != nil {
				grossPlan = *total.GrossPoolRub
			}
			gross.PlanRub = round2(gross.PlanRub + grossPlan)
		}
		result.PortfolioPlanRub = round2(result.PortfolioPlanRub + total.ContractPlanRub)
	}

	addPlan := func(row *models.NetworkAnnualInvestmentRow, plan models.NetworkPlan) {
		eac := annualEACRub(plan)
		row.EACRub = round2(row.EACRub + eac)

		period := byQuarter[plan.Quarter]
		if plan.InvestmentsPct != nil {
			investmentBase := eac
			if plan.PayInvestmentsFromFact {
				investmentBase = models.ValFloat(plan.FactRub)
			}
			grossInvestment, netInvestment := investmentsFor(
				investmentBase, *plan.InvestmentsPct, period.VATIncluded, period.VATRate,
			)
			if plan.PayInvestmentsFromFact {
				row.FactBasedAccruedInvestmentsRub = round2(row.FactBasedAccruedInvestmentsRub + grossInvestment)
				row.FactBasedAccruedInvestmentsRubNet = round2(row.FactBasedAccruedInvestmentsRubNet + netInvestment)
			} else {
				row.AccruedInvestmentsRub = round2(row.AccruedInvestmentsRub + grossInvestment)
				row.AccruedInvestmentsRubNet = round2(row.AccruedInvestmentsRubNet + netInvestment)
			}
		}

		// Вычитается именно перечисленное по документам, а не посчитанное
		// правилом: иначе доплата всегда сходилась бы в ноль сама с собой.
		if plan.Quarter < 4 && plan.PaidInvestmentsRub != nil {
			row.PaidInvestmentsRub = round2(row.PaidInvestmentsRub + *plan.PaidInvestmentsRub)
			row.PaidInvestmentsRubNet = round2(row.PaidInvestmentsRubNet +
				NetRub(*plan.PaidInvestmentsRub, period.VATIncluded, period.VATRate))
		}
		if plan.Quarter == 4 && plan.ForecastInvestmentsRub != nil {
			row.Q4ForecastInvestmentsRub = round2(row.Q4ForecastInvestmentsRub + *plan.ForecastInvestmentsRub)
			row.Q4ForecastInvestmentsNet = round2(row.Q4ForecastInvestmentsNet +
				NetRub(*plan.ForecastInvestmentsRub, period.VATIncluded, period.VATRate))
		}
	}

	for _, plan := range plans {
		if plan.BrandAS == nil {
			continue
		}
		eac := annualEACRub(plan)
		result.PortfolioEACRub = round2(result.PortfolioEACRub + eac)

		if plan.InGross {
			grossExists = true
			addPlan(&gross, plan)
			continue
		}

		brand := *plan.BrandAS
		row := brands[brand]
		if row == nil {
			brandCopy := brand
			row = &models.NetworkAnnualInvestmentRow{ScopeType: "brand", BrandAS: &brandCopy}
			brands[brand] = row
		}
		row.PlanRub = round2(row.PlanRub + models.ValFloat(plan.PlanRub))
		addPlan(row, plan)
	}

	result.PortfolioCompletionPct = completionPct(result.PortfolioEACRub, result.PortfolioPlanRub)
	result.PortfolioCompleted = result.PortfolioPlanRub > 0 && result.PortfolioEACRub >= result.PortfolioPlanRub

	finishRow := func(row models.NetworkAnnualInvestmentRow) models.NetworkAnnualInvestmentRow {
		row.CompletionPct = completionPct(row.EACRub, row.PlanRub)
		row.Completed = row.PlanRub > 0 && row.EACRub >= row.PlanRub
		row.Eligible = result.PortfolioCompleted && row.Completed
		eligibleAccrued := row.FactBasedAccruedInvestmentsRub
		eligibleAccruedNet := row.FactBasedAccruedInvestmentsRubNet
		if row.Eligible {
			eligibleAccrued = round2(eligibleAccrued + row.AccruedInvestmentsRub)
			eligibleAccruedNet = round2(eligibleAccruedNet + row.AccruedInvestmentsRubNet)
		}
		row.SupplementRub = round2(math.Max(0,
			eligibleAccrued-row.PaidInvestmentsRub-row.Q4ForecastInvestmentsRub))
		row.SupplementRubNet = round2(math.Max(0,
			eligibleAccruedNet-row.PaidInvestmentsRubNet-row.Q4ForecastInvestmentsNet))
		result.TotalSupplementRub = round2(result.TotalSupplementRub + row.SupplementRub)
		result.TotalSupplementRubNet = round2(result.TotalSupplementRubNet + row.SupplementRubNet)
		return row
	}

	if grossExists {
		result.Rows = append(result.Rows, finishRow(gross))
	}
	brandNames := make([]string, 0, len(brands))
	for brand := range brands {
		brandNames = append(brandNames, brand)
	}
	sort.Strings(brandNames)
	for _, brand := range brandNames {
		row := brands[brand]
		if row.PlanRub == 0 && row.EACRub == 0 && row.AccruedInvestmentsRub == 0 &&
			row.FactBasedAccruedInvestmentsRub == 0 &&
			row.PaidInvestmentsRub == 0 && row.Q4ForecastInvestmentsRub == 0 {
			continue
		}
		result.Rows = append(result.Rows, finishRow(*row))
	}

	return result
}

// CalculateNetworkAnnualInvestmentCumulativeForNetwork не отдаёт показатель,
// пока он не включён в профиле сети. Так API и интерфейс используют один флаг,
// а скрытая карточка кумулятива не продолжает передаваться клиенту.
func CalculateNetworkAnnualInvestmentCumulativeForNetwork(
	network models.Network,
	plans []models.NetworkPlan,
	periods []models.NetworkPeriod,
	totals []NetworkPlanTotals,
) *models.NetworkAnnualInvestmentCumulative {
	if !network.HasAnnualInvestmentCumulative {
		return nil
	}
	result := CalculateNetworkAnnualInvestmentCumulative(plans, periods, totals)
	return &result
}

// NetworkPlanDraft — строка сетки, как её ввёл пользователь.
// Ни факта, ни прогноза здесь нет: факт приходит загрузкой отгрузок, прогноз
// ведётся помесячно во вкладке «Прогноз», и в квартальную строку оба попадают
// сводом из сохранённых строк, а не из черновика.
type NetworkPlanDraft struct {
	Quarter        int
	BrandAS        *string
	InGross        bool
	PlanRub        *float64
	InvestmentsPct *float64
	Month1Pct      float64
	Month2Pct      float64
	Month3Pct      float64
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
			InvestmentsPct: row.InvestmentsPct,
			Month1Pct:      row.Month1Pct,
			Month2Pct:      row.Month2Pct,
			Month3Pct:      row.Month3Pct,
		}
		if saved, ok := factByKey[draftKey(row.Quarter, row.BrandAS)]; ok {
			plan.ID = saved.ID
			plan.NetworkID = saved.NetworkID
			plan.Year = saved.Year
			plan.PlanUnits = saved.PlanUnits
			plan.FactRub = saved.FactRub
			plan.ForecastRub = saved.ForecastRub
			plan.FactInvestmentsRub = saved.FactInvestmentsRub
			plan.PaidInvestmentsRub = saved.PaidInvestmentsRub
			plan.ForecastInvestmentsRub = saved.ForecastInvestmentsRub
			plan.ForecastInvestmentsOverridden = saved.ForecastInvestmentsOverridden
			plan.PayInvestmentsFromFact = saved.PayInvestmentsFromFact
			plan.UpdatedBy = saved.UpdatedBy
			plan.UpdatedAt = saved.UpdatedAt
		}
		plans = append(plans, plan)
	}

	// Тот же вход в расчёт, что и после сохранения: черновик обязан показывать
	// ровно те инвестиции, которые получатся, а не приблизительные.
	plans, totals := BuildNetworkPlanCalculations(plans, periods, nil)
	return plans, totals, SumYearTotals(totals)
}
