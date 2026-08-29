package services

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"backend/models"
	"backend/repository"
)

// ─── Витрина реестра сетей ──────────────────────────────────────────────────
//
// Витрина обязана сходиться с карточкой сети, поэтому собственной математики
// здесь нет: строки плана дополняются фактом и EAC из помесячных таблиц, после
// чего отдаются тем же EnrichNetworkPlans и CalculateNetworkTotals, что и
// вкладка «План и факт». Всё, что делает этот файл сверх этого, — складывает
// готовые квартальные итоги по сетям, брендам и КАМам.

// dashboardCells — готовность данных одного квартала одной сети.
// Ячейка — это бренд × месяц; брендом считается тот, у кого есть строка плана.
type dashboardCells struct {
	closed              int
	closedWithFact      int
	openWithoutForecast int
}

func (c *dashboardCells) add(other dashboardCells) {
	c.closed += other.closed
	c.closedWithFact += other.closedWithFact
	c.openWithoutForecast += other.openWithoutForecast
}

// unitTotals — те же величины в упаковках. Считаются отдельно от рублёвых
// итогов: CalculateNetworkTotals упаковки не ведёт, а показать их надо тем же
// правилом, включая валовый пул.
type unitTotals struct {
	plan float64
	fact float64
	eac  float64
}

func (u *unitTotals) add(other unitTotals) {
	u.plan = round2(u.plan + other.plan)
	u.fact = round2(u.fact + other.fact)
	u.eac = round2(u.eac + other.eac)
}

// networkDashboardValues — вклад одной строки среза в агрегат.
type networkDashboardValues struct {
	networkID int
	brand     string

	planRub float64
	factRub float64
	eacRub  float64
	units   unitTotals

	planInvest, planInvestNet float64
	factInvest, factInvestNet float64
	eacInvest, eacInvestNet   float64

	undistributed *float64
	cells         dashboardCells

	// Прошлый год, тот же квартал. Факт и план отслеживаются раздельно: в
	// реестре план появляется позже факта, и год с фактом, но без плана —
	// обычное дело. Сравнивать факт с фактом там можно, план с планом — нет.
	hasPrevFact  bool
	hasPrevPlan  bool
	prevPlanRub  float64
	prevFactRub  float64
	prevFactUnit float64

	// Сколько строк плана лежит в валовом пуле, а сколько заведено отдельно.
	// Счётчики, а не флаг: признак стоит на паре «бренд × квартал», и в срезе
	// из нескольких кварталов бренд может быть в пуле не везде.
	grossRows    int
	separateRows int

	promo dashboardPromoTotals
}

// dashboardPromoTotals — промо среза в разбивке по каналу.
type dashboardPromoTotals struct {
	count   int
	online  int
	offline int
	invest  float64
}

func (p *dashboardPromoTotals) add(other dashboardPromoTotals) {
	p.count += other.count
	p.online += other.online
	p.offline += other.offline
	p.invest = round2(p.invest + other.invest)
}

type networkDashboardAccumulator struct {
	networks map[int]struct{}
	brands   map[string]struct{}

	planRub float64
	factRub float64
	eacRub  float64
	units   unitTotals

	planInvest, planInvestNet float64
	factInvest, factInvestNet float64
	eacInvest, eacInvestNet   float64

	undistributed    float64
	undistributedSet bool

	cells dashboardCells

	hasPrevFact  bool
	hasPrevPlan  bool
	prevPlanRub  float64
	prevFactRub  float64
	prevFactUnit float64

	grossRows    int
	separateRows int

	promo dashboardPromoTotals
}

// inGross — лежит ли срез целиком в валовом пуле. nil означает «неоднородно»
// или «строк плана в срезе нет»: и то и другое не «да» и не «нет».
func (a networkDashboardAccumulator) inGross() *bool {
	if a.grossRows == 0 && a.separateRows == 0 {
		return nil
	}
	if a.grossRows > 0 && a.separateRows > 0 {
		return nil
	}
	value := a.separateRows == 0
	return &value
}

func (a *networkDashboardAccumulator) init() {
	if a.networks == nil {
		a.networks = map[int]struct{}{}
		a.brands = map[string]struct{}{}
	}
}

// markBrand отмечает бренд в срезе, не добавляя сумм: в разрезе сетей и КАМов
// бренды считаются по строкам плана, а деньги приходят квартальным итогом.
func (a *networkDashboardAccumulator) markBrand(brand string) {
	a.init()
	if brand != "" {
		a.brands[brand] = struct{}{}
	}
}

func (a *networkDashboardAccumulator) add(v networkDashboardValues) {
	a.init()
	if v.networkID > 0 {
		a.networks[v.networkID] = struct{}{}
	}
	if v.brand != "" {
		a.brands[v.brand] = struct{}{}
	}

	a.planRub = round2(a.planRub + v.planRub)
	a.factRub = round2(a.factRub + v.factRub)
	a.eacRub = round2(a.eacRub + v.eacRub)

	a.planInvest = round2(a.planInvest + v.planInvest)
	a.planInvestNet = round2(a.planInvestNet + v.planInvestNet)
	a.factInvest = round2(a.factInvest + v.factInvest)
	a.factInvestNet = round2(a.factInvestNet + v.factInvestNet)
	a.eacInvest = round2(a.eacInvest + v.eacInvest)
	a.eacInvestNet = round2(a.eacInvestNet + v.eacInvestNet)

	a.units.add(v.units)

	// Остаток появляется только там, где заведён валовый пул: срез без пула
	// остаётся без остатка, а не с нулём.
	if v.undistributed != nil {
		a.undistributed = round2(a.undistributed + *v.undistributed)
		a.undistributedSet = true
	}
	a.cells.add(v.cells)
	a.promo.add(v.promo)
	a.grossRows += v.grossRows
	a.separateRows += v.separateRows

	if v.hasPrevFact {
		a.hasPrevFact = true
		a.prevFactRub = round2(a.prevFactRub + v.prevFactRub)
		a.prevFactUnit = round2(a.prevFactUnit + v.prevFactUnit)
	}
	if v.hasPrevPlan {
		a.hasPrevPlan = true
		a.prevPlanRub = round2(a.prevPlanRub + v.prevPlanRub)
	}
}

func (a networkDashboardAccumulator) metrics() models.NetworkDashboardMetrics {
	metrics := models.NetworkDashboardMetrics{
		NetworkCount: len(a.networks),
		BrandCount:   len(a.brands),

		PlanRub: a.planRub,
		FactRub: a.factRub,
		EACRub:  a.eacRub,

		CompletionPct:    completionPct(a.factRub, a.planRub),
		EACCompletionPct: completionPct(a.eacRub, a.planRub),
		GapRub:           round2(a.eacRub - a.planRub),
		GapUnits:         round2(a.units.eac - a.units.plan),

		PlanInvestmentsRub:    a.planInvest,
		PlanInvestmentsRubNet: a.planInvestNet,
		FactInvestmentsRub:    a.factInvest,
		FactInvestmentsRubNet: a.factInvestNet,
		EACInvestmentsRub:     a.eacInvest,
		EACInvestmentsRubNet:  a.eacInvestNet,

		// Отклонение считается в базе «без НДС»: сети работают с разными
		// ставками, и в валовой базе их разница означала бы разный НДС,
		// а не разные инвестиции.
		InvestmentVarianceRub: round2(a.eacInvestNet - a.planInvestNet),
		// Во что реально обходится ожидаемый объём.
		EffectiveInvestmentsPct: completionPct(a.eacInvest, a.eacRub),

		ClosedCells:              a.cells.closed,
		ClosedCellsWithFact:      a.cells.closedWithFact,
		FactCoveragePct:          completionPct(float64(a.cells.closedWithFact), float64(a.cells.closed)),
		OpenCellsWithoutForecast: a.cells.openWithoutForecast,

		PlanUnits: a.units.plan,
		FactUnits: a.units.fact,
		EACUnits:  a.units.eac,

		PromoCount:          a.promo.count,
		PromoOnlineCount:    a.promo.online,
		PromoOfflineCount:   a.promo.offline,
		PromoInvestmentsRub: a.promo.invest,
	}
	if a.undistributedSet {
		rest := a.undistributed
		metrics.UndistributedRub = &rest
	}
	// Прошлый год показывается только там, где сопоставимый период вообще был.
	// Иначе «−100%» читалось бы как обвал продаж, а не как отсутствие истории.
	if a.hasPrevFact {
		prevFact, prevUnits := a.prevFactRub, a.prevFactUnit
		metrics.PrevFactRub = &prevFact
		metrics.PrevFactUnits = &prevUnits
		metrics.FactYoYPct = growthPct(a.factRub, prevFact)
	}
	if a.hasPrevPlan {
		prevPlan := a.prevPlanRub
		metrics.PrevPlanRub = &prevPlan
		metrics.PlanYoYPct = growthPct(a.planRub, prevPlan)
	}
	return metrics
}

// growthPct — прирост к прошлому году в процентах. База ноль означает, что
// сравнивать не с чем: рост «с нуля» в процентах не выражается.
func growthPct(current, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	value := round2((current - previous) / previous * 100)
	return &value
}

// forecastAggregate — официальный прогноз месяца по бренду.
type forecastAggregate struct {
	rub         *float64
	units       *float64
	investments *float64
}

// aggregateForecastLines повторяет правило aggregateFacts: готовая строка
// бренда важнее SKU-строк, и только при её отсутствии SKU складываются.
// Иначе бренд, который ведут по SKU и по бренду одновременно, дал бы двойную сумму.
func aggregateForecastLines(lines []models.NetworkForecastLine) map[string]forecastAggregate {
	brandRows := map[string]forecastAggregate{}
	skuSums := map[string]forecastAggregate{}

	for _, line := range lines {
		key := forecastMonthKey(line.Year, line.Month, line.BrandAS)
		if line.SKU == nil {
			brandRows[key] = forecastAggregate{
				rub:         line.ForecastRub,
				units:       line.ForecastUnits,
				investments: line.ForecastInvestmentsRub,
			}
			continue
		}
		agg := skuSums[key]
		addPtrValue(&agg.rub, line.ForecastRub)
		addPtrValue(&agg.units, line.ForecastUnits)
		addPtrValue(&agg.investments, line.ForecastInvestmentsRub)
		skuSums[key] = agg
	}
	for key, sum := range skuSums {
		if _, exists := brandRows[key]; !exists {
			brandRows[key] = sum
		}
	}
	return brandRows
}

// dashboardSKUMonthEAC — ожидаемый итог SKU в детализации dashboard.
// Системная рекомендация считается на уровне бренда и уже входит в его итог;
// раскладывать её по SKU без подтверждённого микса означало бы придумать доли.
func dashboardSKUMonthEAC(closed bool, fact, official *float64) *float64 {
	if closed {
		return fact
	}
	if official != nil {
		return official
	}
	return fact
}

// quarterUnitParts — упаковки квартала в тех же частях, что и рубли, чтобы
// обязательство по контракту считалось одним правилом: валовый пул целиком,
// если он заведён, иначе сумма брендов пула; отдельные бренды прибавляются.
type quarterUnitParts struct {
	poolPlan     float64
	hasPool      bool
	grossPlan    float64
	separatePlan float64
	fact         float64
	eac          float64
}

func (p quarterUnitParts) totals() unitTotals {
	pool := p.grossPlan
	if p.hasPool {
		pool = p.poolPlan
	}
	return unitTotals{plan: round2(pool + p.separatePlan), fact: p.fact, eac: p.eac}
}

// networkSlice — посчитанный вклад одной сети в витрину.
// brandQuarterKey — ключ разреза «бренд × квартал». Это те же строки плана,
// просто не свёрнутые до бренда: в реестре план и заводится на этой паре.
type brandQuarterKey struct {
	brand   string
	quarter int
}

// networkSKUKey — SKU внутри бренда. Один и тот же код у разных брендов
// встречается, поэтому бренд входит в ключ.
type networkSKUKey struct {
	brand string
	sku   string
}

// networkSKUTotals — итоги SKU за срез. Плановых величин здесь нет: план
// в реестре заводится брендом.
type networkSKUTotals struct {
	factRub    float64
	factUnits  float64
	factInvest float64
	eacRub     float64
	eacUnits   float64
}

type networkSlice struct {
	quarterTotals map[int]models.NetworkPlanTotals
	quarterCells  map[int]dashboardCells
	quarterUnits  map[int]unitTotals
	brandValues   map[string]*networkDashboardValues

	// Разрез «бренд × квартал» — основа разбора одной сети. Наружу отдаётся
	// только для неё, но считается всегда: это тот же обход строк плана.
	brandQuarterValues map[brandQuarterKey]*networkDashboardValues
	brandQuarterUnits  map[brandQuarterKey]*unitTotals
	brandQuarterCells  map[brandQuarterKey]dashboardCells

	// SKU внутри бренда: только факт и прогноз. Плана на SKU в реестре нет.
	skuTotals map[networkSKUKey]*networkSKUTotals

	// Месячные ряды. План месяца — квартальное обязательство, разложенное по
	// схеме из профиля сети: помесячных планов в реестре не существует.
	monthPlan  map[int]quarterFact
	monthFact  map[int]quarterFact
	monthEAC   map[int]quarterFact
	monthCells map[int]dashboardCells

	annualInvestmentCumulative *models.NetworkAnnualInvestmentCumulative
}

// buildNetworkSlice дополняет строки плана фактом и EAC из помесячных таблиц
// и считает по ним те же квартальные итоги, что показывает карточка сети.
func buildNetworkSlice(
	network models.Network,
	year int,
	quarters map[int]bool,
	plans []models.NetworkPlan,
	persistedPeriods []models.NetworkPeriod,
	facts []models.NetworkMonthlyFact,
	recommendationFacts []models.NetworkMonthlyFact,
	forecasts []models.NetworkForecastLine,
	promoUplifts map[promoForecastKey]forecastPromoTotals,
	groups []models.NetworkPeriodGroup,
	now time.Time,
) networkSlice {
	periods := NetworkPeriodsWithDefaults(network, year, persistedPeriods)
	brandFacts, skuFacts := aggregateFacts(facts)
	recommendationBrandFacts, _ := aggregateFacts(recommendationFacts)
	brandForecasts := aggregateForecastLines(forecasts)

	filled := make([]models.NetworkPlan, 0, len(plans))
	plannedBrands := map[string]struct{}{}
	cells := map[int]dashboardCells{}
	units := map[int]*quarterUnitParts{}
	brandUnits := map[string]*unitTotals{}
	brandQuarterUnits := map[brandQuarterKey]*unitTotals{}
	brandQuarterCells := map[brandQuarterKey]dashboardCells{}
	monthFact := map[int]quarterFact{}
	monthEAC := map[int]quarterFact{}
	monthCells := map[int]dashboardCells{}

	unitsOf := func(quarter int) *quarterUnitParts {
		parts := units[quarter]
		if parts == nil {
			parts = &quarterUnitParts{}
			units[quarter] = parts
		}
		return parts
	}

	// Строки года обходятся целиком, а не только по выбранным кварталам:
	// правило совместного зачёта охватывает соседние кварталы, и порог по
	// половине своего периода дал бы неверное право на выплату. Наружу при
	// этом идёт по-прежнему только срез — за это отвечает visible.
	for _, plan := range plans {
		visible := quarters[plan.Quarter]
		// Строка пула объёма по брендам не раскладывается: её план — само
		// обязательство контракта, а факт и прогноз приносят бренды пула.
		if plan.BrandAS == nil {
			if visible && plan.PlanUnits != nil {
				parts := unitsOf(plan.Quarter)
				parts.poolPlan = round2(*plan.PlanUnits)
				parts.hasPool = true
			}
			filled = append(filled, plan)
			continue
		}

		brand := *plan.BrandAS
		if visible {
			plannedBrands[strings.TrimSpace(brand)] = struct{}{}
		}
		quarterCells := cells[plan.Quarter]
		// Готовность данных самой строки: те же три месяца, но посчитанные
		// отдельно — иначе у ячейки «бренд × квартал» пришлось бы показывать
		// готовность всего квартала сети.
		var rowCells dashboardCells
		var factSum, eacSum, factInvestSum, eacInvestSum *float64
		var factUnitsSum, eacUnitsSum *float64
		// Введённая руками сумма инвестиций порогом не гасится, поэтому признак
		// переопределения доходит до строки плана вместе с самой суммой.
		overridden := false

		for index := 0; index < 3; index++ {
			month := (plan.Quarter-1)*3 + 1 + index
			key := forecastMonthKey(year, month, brand)
			fact := brandFacts[key]
			official := brandForecasts[key]
			uplift := promoUplifts[promoForecastKey{
				network: network.Name, brand: strings.TrimSpace(brand), month: month,
			}]
			systemRub, _ := recommendedForecastMetric(
				recommendationBrandFacts, year, month, brand, uplift.rub, rubMetric,
			)
			systemUnits, _ := recommendedForecastMetric(
				recommendationBrandFacts, year, month, brand, uplift.units, unitsMetric,
			)
			closed := isClosedForecastMonth(year, month, now)

			if visible {
				monthCell := monthCells[month]
				if closed {
					quarterCells.closed++
					monthCell.closed++
					rowCells.closed++
					if fact.rub != nil {
						quarterCells.closedWithFact++
						monthCell.closedWithFact++
						rowCells.closedWithFact++
					}
				} else if official.rub == nil {
					quarterCells.openWithoutForecast++
					monthCell.openWithoutForecast++
					rowCells.openWithoutForecast++
				}
				monthCells[month] = monthCell
			}

			eac := valueForEAC(closed, fact.rub, official.rub, systemRub)
			eacInvest, investSource := investmentsForEAC(closed, fact.investments, official.investments, eac, plan.InvestmentsPct)
			if investSource == "override" {
				overridden = true
			}
			eacUnits := valueForEAC(closed, fact.units, official.units, systemUnits)

			if visible {
				factPoint := monthFact[month]
				factPoint.rub = round2(factPoint.rub + valueOrZero(fact.rub))
				factPoint.units = round2(factPoint.units + valueOrZero(fact.units))
				monthFact[month] = factPoint

				eacPoint := monthEAC[month]
				eacPoint.rub = round2(eacPoint.rub + valueOrZero(eac))
				eacPoint.units = round2(eacPoint.units + valueOrZero(eacUnits))
				monthEAC[month] = eacPoint
			}

			addPtrValue(&factSum, fact.rub)
			addPtrValue(&factInvestSum, fact.investments)
			addPtrValue(&eacSum, eac)
			addPtrValue(&eacInvestSum, eacInvest)
			addPtrValue(&factUnitsSum, fact.units)
			addPtrValue(&eacUnitsSum, eacUnits)
		}
		if visible {
			cells[plan.Quarter] = quarterCells

			parts := unitsOf(plan.Quarter)
			if plan.PlanUnits != nil {
				if plan.InGross {
					parts.grossPlan = round2(parts.grossPlan + *plan.PlanUnits)
				} else {
					parts.separatePlan = round2(parts.separatePlan + *plan.PlanUnits)
				}
			}
			parts.fact = round2(parts.fact + valueOrZero(factUnitsSum))
			parts.eac = round2(parts.eac + valueOrZero(eacUnitsSum))

			brandUnit := brandUnits[brand]
			if brandUnit == nil {
				brandUnit = &unitTotals{}
				brandUnits[brand] = brandUnit
			}
			rowUnits := unitTotals{
				plan: valueOrZero(plan.PlanUnits),
				fact: valueOrZero(factUnitsSum),
				eac:  valueOrZero(eacUnitsSum),
			}
			brandUnit.add(rowUnits)

			rowKey := brandQuarterKey{brand: strings.TrimSpace(brand), quarter: plan.Quarter}
			quarterUnit := brandQuarterUnits[rowKey]
			if quarterUnit == nil {
				quarterUnit = &unitTotals{}
				brandQuarterUnits[rowKey] = quarterUnit
			}
			quarterUnit.add(rowUnits)

			rowCellTotals := brandQuarterCells[rowKey]
			rowCellTotals.add(rowCells)
			brandQuarterCells[rowKey] = rowCellTotals
		}

		plan.FactRub = factSum
		plan.PaidInvestmentsRub = factInvestSum
		plan.FactInvestmentsRub = factInvestSum
		plan.ForecastRub = eacSum
		plan.ForecastInvestmentsRub = eacInvestSum
		plan.ForecastInvestmentsOverridden = overridden
		filled = append(filled, plan)
	}

	// Тот же расчёт и в том же порядке, что и в карточке: сначала суммы и EAC,
	// затем право на выплату, затем итоги вместе с ней. Инвестиции к выплате
	// возникают только у брендов, чей прогноз закрывает план, — считать их
	// витрине отдельным упрощённым правилом значило бы завести вторую
	// арифметику, которая разойдётся с карточкой.
	enriched, totals := BuildNetworkPlanCalculations(filled, periods, groups)
	annualInvestmentCumulative := CalculateNetworkAnnualInvestmentCumulativeForNetwork(
		network, enriched, periods, totals,
	)

	// SKU объясняют бренд снизу: плана на них не заводят, поэтому здесь только
	// факт и прогноз, посчитанные тем же правилом закрытого месяца, что и всё
	// остальное. Считаются лишь бренды со строкой плана — прочих нет и в
	// разрезе брендов, и SKU без своей строки повисли бы ни при чём.
	skuForecasts := map[string]forecastAggregate{}
	for _, line := range forecasts {
		if line.SKU == nil {
			continue
		}
		key := forecastSKUKey(line.Year, line.Month, line.BrandAS, *line.SKU)
		aggregate := skuForecasts[key]
		addPtrValue(&aggregate.rub, line.ForecastRub)
		addPtrValue(&aggregate.units, line.ForecastUnits)
		skuForecasts[key] = aggregate
	}

	skuPairs := map[networkSKUKey]struct{}{}
	collectSKU := func(brand string, sku *string, month int) {
		if sku == nil || !quarters[(month-1)/3+1] {
			return
		}
		name := strings.TrimSpace(brand)
		if _, planned := plannedBrands[name]; !planned {
			return
		}
		skuPairs[networkSKUKey{brand: name, sku: *sku}] = struct{}{}
	}
	for _, fact := range facts {
		collectSKU(fact.BrandAS, fact.SKU, fact.Month)
	}
	for _, line := range forecasts {
		collectSKU(line.BrandAS, line.SKU, line.Month)
	}

	skuTotals := map[networkSKUKey]*networkSKUTotals{}
	for pair := range skuPairs {
		totals := &networkSKUTotals{}
		for quarter := 1; quarter <= 4; quarter++ {
			if !quarters[quarter] {
				continue
			}
			for index := 0; index < 3; index++ {
				month := (quarter-1)*3 + 1 + index
				key := forecastSKUKey(year, month, pair.brand, pair.sku)
				fact := skuFacts[key]
				official := skuForecasts[key]
				closed := isClosedForecastMonth(year, month, now)

				totals.factRub = round2(totals.factRub + valueOrZero(fact.rub))
				totals.factUnits = round2(totals.factUnits + valueOrZero(fact.units))
				totals.factInvest = round2(totals.factInvest + valueOrZero(fact.investments))
				totals.eacRub = round2(totals.eacRub + valueOrZero(dashboardSKUMonthEAC(closed, fact.rub, official.rub)))
				totals.eacUnits = round2(totals.eacUnits + valueOrZero(dashboardSKUMonthEAC(closed, fact.units, official.units)))
			}
		}
		skuTotals[pair] = totals
	}

	slice := networkSlice{
		quarterTotals:              map[int]models.NetworkPlanTotals{},
		quarterCells:               cells,
		quarterUnits:               map[int]unitTotals{},
		brandValues:                map[string]*networkDashboardValues{},
		brandQuarterValues:         map[brandQuarterKey]*networkDashboardValues{},
		brandQuarterUnits:          brandQuarterUnits,
		brandQuarterCells:          brandQuarterCells,
		skuTotals:                  skuTotals,
		monthPlan:                  map[int]quarterFact{},
		monthFact:                  monthFact,
		monthEAC:                   monthEAC,
		monthCells:                 monthCells,
		annualInvestmentCumulative: annualInvestmentCumulative,
	}
	for _, total := range totals {
		if !quarters[total.Quarter] {
			continue
		}
		slice.quarterTotals[total.Quarter] = total
	}
	for quarter, parts := range units {
		slice.quarterUnits[quarter] = parts.totals()
	}

	// План раскладывается по месяцам той же схемой, что применяет карточка,
	// и от квартального обязательства, а не от суммы брендов: иначе месяц
	// потерял бы нераспределённый остаток валового пула.
	distribution := networkMonthlyDistribution(network)
	for quarter, total := range slice.quarterTotals {
		quarterUnitsTotal := slice.quarterUnits[quarter]
		for index := 0; index < 3; index++ {
			month := (quarter-1)*3 + 1 + index
			share := distribution[index] / 100
			slice.monthPlan[month] = quarterFact{
				rub:   round2(total.ContractPlanRub * share),
				units: round2(quarterUnitsTotal.plan * share),
			}
		}
	}

	// Разрез по брендам: у строки бренда есть только собственный план, поэтому
	// обязательство по контракту здесь не применимо — нераспределённый остаток
	// пула в бренды не попадает и виден только в разрезе сетей.
	for _, plan := range enriched {
		// Год посчитан целиком ради порога выплаты; в разрезы идут только
		// кварталы среза.
		if plan.BrandAS == nil || !quarters[plan.Quarter] {
			continue
		}
		brand := strings.TrimSpace(*plan.BrandAS)
		if brand == "" {
			continue
		}
		values := slice.brandValues[brand]
		if values == nil {
			values = &networkDashboardValues{networkID: network.ID, brand: brand}
			slice.brandValues[brand] = values
		}
		values.planRub = round2(values.planRub + valueOrZero(plan.PlanRub))
		values.factRub = round2(values.factRub + valueOrZero(plan.FactRub))
		values.eacRub = round2(values.eacRub + valueOrZero(plan.ForecastRub))
		values.planInvest = round2(values.planInvest + valueOrZero(plan.InvestmentsRub))
		values.planInvestNet = round2(values.planInvestNet + valueOrZero(plan.InvestmentsNet))
		values.factInvest = round2(values.factInvest + valueOrZero(plan.FactInvestmentsRub))
		values.factInvestNet = round2(values.factInvestNet + valueOrZero(plan.FactInvestmentsNet))
		values.eacInvest = round2(values.eacInvest + valueOrZero(plan.ForecastInvestmentsRub))
		values.eacInvestNet = round2(values.eacInvestNet + valueOrZero(plan.ForecastInvestmentsNet))
		// Где лежит эта строка плана. Цикл идёт только по кварталам среза,
		// поэтому бренд, выведенный из вала за его пределами, разрез собой
		// не пометит.
		if plan.InGross {
			values.grossRows++
		} else {
			values.separateRows++
		}

		// Тот же вклад, но не свёрнутый по кварталам. Одна строка плана — одна
		// пара «бренд × квартал», поэтому суммирование здесь то же самое.
		rowKey := brandQuarterKey{brand: brand, quarter: plan.Quarter}
		quarterValues := slice.brandQuarterValues[rowKey]
		if quarterValues == nil {
			quarterValues = &networkDashboardValues{networkID: network.ID, brand: brand}
			slice.brandQuarterValues[rowKey] = quarterValues
		}
		if plan.InGross {
			quarterValues.grossRows++
		} else {
			quarterValues.separateRows++
		}
		quarterValues.planRub = round2(quarterValues.planRub + valueOrZero(plan.PlanRub))
		quarterValues.factRub = round2(quarterValues.factRub + valueOrZero(plan.FactRub))
		quarterValues.eacRub = round2(quarterValues.eacRub + valueOrZero(plan.ForecastRub))
		quarterValues.planInvest = round2(quarterValues.planInvest + valueOrZero(plan.InvestmentsRub))
		quarterValues.planInvestNet = round2(quarterValues.planInvestNet + valueOrZero(plan.InvestmentsNet))
		quarterValues.factInvest = round2(quarterValues.factInvest + valueOrZero(plan.FactInvestmentsRub))
		quarterValues.factInvestNet = round2(quarterValues.factInvestNet + valueOrZero(plan.FactInvestmentsNet))
		quarterValues.eacInvest = round2(quarterValues.eacInvest + valueOrZero(plan.ForecastInvestmentsRub))
		quarterValues.eacInvestNet = round2(quarterValues.eacInvestNet + valueOrZero(plan.ForecastInvestmentsNet))
	}
	for brand, totals := range brandUnits {
		if values := slice.brandValues[brand]; values != nil {
			values.units = *totals
		}
	}
	for key, totals := range slice.brandQuarterUnits {
		if values := slice.brandQuarterValues[key]; values != nil {
			values.units = *totals
		}
	}
	for key, totals := range slice.brandQuarterCells {
		if values := slice.brandQuarterValues[key]; values != nil {
			values.cells = totals
		}
	}
	return slice
}

// valuesFromTotals переводит квартальный итог сети во вклад среза.
// Плановым объёмом берётся обязательство по контракту: валовый пул входит
// целиком, даже если бренды разобрали его не полностью.
func valuesFromTotals(
	networkID int,
	total models.NetworkPlanTotals,
	cells dashboardCells,
	units unitTotals,
) networkDashboardValues {
	return networkDashboardValues{
		networkID:     networkID,
		planRub:       total.ContractPlanRub,
		factRub:       total.FactRub,
		eacRub:        total.ForecastRub,
		units:         units,
		planInvest:    total.InvestmentsRub,
		planInvestNet: total.InvestmentsRubNet,
		factInvest:    total.FactInvestmentsRub,
		factInvestNet: total.FactInvestmentsRubNet,
		eacInvest:     total.ForecastInvestmentsRub,
		eacInvestNet:  total.ForecastInvestmentsRubNet,
		undistributed: total.Undistributed,
		cells:         cells,
	}
}

// promoCodeOf — код механики для плитки. Назначенный в справочнике код важнее
// вычисленного: сокращения согласованы людьми и попадают в презентации,
// поэтому автоматическое правило работает только как запасное — для механик,
// которым код ещё не назначили.
func promoCodeOf(assigned *string, mechanics string) string {
	if code := strings.TrimSpace(valueOrEmpty(assigned)); code != "" {
		return code
	}
	return promoShortCode(mechanics)
}

// promoShortCode — запасное сокращение для механики без назначенного кода.
//
// Префиксы «e-comm» и «pure» отбрасываются намеренно: они означают канал,
// который показывается отдельной меткой, а сама механика при этом та же самая.
// Так «e-comm скидка» и «Скидка» получают один код и различаются каналом —
// это и есть правда о них, а не совпадение.
func promoShortCode(mechanics string) string {
	value := strings.ToLower(strings.TrimSpace(mechanics))
	if value == "" {
		return "—"
	}
	for _, prefix := range []string{"e-comm ", "pure ", "e-comm-", "pure-"} {
		value = strings.TrimPrefix(value, prefix)
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "—"
	}
	head := []rune(fields[0])
	if len(head) > 4 {
		head = head[:4]
	}
	code := strings.ToUpper(string(head))
	if len(fields) > 1 {
		tail := []rune(fields[1])
		code += "·" + strings.ToUpper(string(tail[:1]))
	}
	return code
}

const (
	promoChannelOnline  = "онлайн"
	promoChannelOffline = "оффлайн"
	promoChannelUnknown = "не указан"
)

func promoChannelOf(channel *string) string {
	value := strings.ToLower(strings.TrimSpace(valueOrEmpty(channel)))
	switch value {
	case promoChannelOnline, promoChannelOffline:
		return value
	default:
		return promoChannelUnknown
	}
}

func accumulatorFor(groups map[string]*networkDashboardAccumulator, key string) *networkDashboardAccumulator {
	accumulator := groups[key]
	if accumulator == nil {
		accumulator = &networkDashboardAccumulator{}
		groups[key] = accumulator
	}
	return accumulator
}

// sortBreakdown упорядочивает разрез по плановому объёму: наверху те, кто
// весит больше всего, а при равенстве — по алфавиту, чтобы порядок был устойчив.
func sortBreakdown(items []models.NetworkDashboardBreakdown) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Metrics.PlanRub != items[j].Metrics.PlanRub {
			return items[i].Metrics.PlanRub > items[j].Metrics.PlanRub
		}
		return items[i].Name < items[j].Name
	})
}

// periodIndex — строки одного года, разложенные по сетям.
type periodIndex struct {
	plans     map[int][]models.NetworkPlan
	periods   map[int][]models.NetworkPeriod
	facts     map[int][]models.NetworkMonthlyFact
	forecasts map[int][]models.NetworkForecastLine
	groups    map[int][]models.NetworkPeriodGroup
}

func indexPeriodData(data repository.NetworkDashboardPeriodData) periodIndex {
	index := periodIndex{
		plans:     map[int][]models.NetworkPlan{},
		periods:   map[int][]models.NetworkPeriod{},
		facts:     map[int][]models.NetworkMonthlyFact{},
		forecasts: map[int][]models.NetworkForecastLine{},
		groups:    map[int][]models.NetworkPeriodGroup{},
	}
	for _, group := range data.Groups {
		index.groups[group.NetworkID] = append(index.groups[group.NetworkID], group)
	}
	for _, plan := range data.Plans {
		index.plans[plan.NetworkID] = append(index.plans[plan.NetworkID], plan)
	}
	for _, period := range data.Periods {
		index.periods[period.NetworkID] = append(index.periods[period.NetworkID], period)
	}
	for _, fact := range data.Facts {
		index.facts[fact.NetworkID] = append(index.facts[fact.NetworkID], fact)
	}
	for _, line := range data.Forecasts {
		index.forecasts[line.NetworkID] = append(index.forecasts[line.NetworkID], line)
	}
	return index
}

// factByMonth суммирует помесячный факт, минуя строки плана.
//
// Для прошлого года это единственный правильный путь: план появляется в
// реестре позже факта, и год с отгрузками, но без заведённых планов —
// обычное дело. Требовать план ради сравнения факта с фактом значило бы
// молча прятать сравнение именно там, где оно нужнее всего.
func factByMonth(facts []models.NetworkMonthlyFact, quarters map[int]bool) map[int]quarterFact {
	// aggregateFacts сворачивает SKU в бренд по тому же правилу, что и остальная
	// витрина, поэтому дальше достаточно пройти по свёрнутым строкам.
	brandFacts, _ := aggregateFacts(facts)
	byMonth := map[int]quarterFact{}
	for key, aggregate := range brandFacts {
		month, ok := monthOfFactKey(key)
		if !ok {
			continue
		}
		if !quarters[(month-1)/3+1] {
			continue
		}
		totals := byMonth[month]
		totals.rub = round2(totals.rub + valueOrZero(aggregate.rub))
		totals.units = round2(totals.units + valueOrZero(aggregate.units))
		byMonth[month] = totals
	}
	return byMonth
}

// quartersFromMonths складывает месяцы в кварталы. Обратное преобразование
// невозможно, поэтому месяц и остаётся единицей хранения.
func quartersFromMonths(byMonth map[int]quarterFact) map[int]quarterFact {
	byQuarter := map[int]quarterFact{}
	for month, value := range byMonth {
		quarter := (month-1)/3 + 1
		totals := byQuarter[quarter]
		totals.rub = round2(totals.rub + value.rub)
		totals.units = round2(totals.units + value.units)
		byQuarter[quarter] = totals
	}
	return byQuarter
}

// quarterFact — суммы периода в двух единицах.
type quarterFact struct {
	rub   float64
	units float64
}

// factsByBrandQuarter — факт по паре «бренд × квартал», свёрнутый тем же
// правилом, что и весь остальной факт витрины: строка бренда важнее строк SKU.
//
// Читается прямо из отгрузок, минуя строки плана. Прирост к прошлому году
// сравнивает факт с фактом, и заведённый в том году план ему не нужен:
// требовать план значило бы прятать сравнение ровно там, где оно нужнее
// всего — у бренда, которого год назад ещё не планировали, но уже отгружали.
// Тем же путём идёт прошлый год у SKU, см. buildSKUs.
func factsByBrandQuarter(
	facts []models.NetworkMonthlyFact,
	quarters map[int]bool,
) map[brandQuarterKey]quarterFact {
	brandFacts, _ := aggregateFacts(facts)
	result := map[brandQuarterKey]quarterFact{}
	for key, aggregate := range brandFacts {
		month, brand, ok := monthBrandOfFactKey(key)
		if !ok {
			continue
		}
		quarter := (month-1)/3 + 1
		if !quarters[quarter] {
			continue
		}
		// Бренд приводится к тому же виду, что и ключ строки плана: в отгрузках
		// то же имя встречается с висящими пробелами, и без обрезки бренд
		// разошёлся бы сам с собой.
		mapKey := brandQuarterKey{brand: strings.TrimSpace(brand), quarter: quarter}
		totals := result[mapKey]
		totals.rub = round2(totals.rub + valueOrZero(aggregate.rub))
		totals.units = round2(totals.units + valueOrZero(aggregate.units))
		result[mapKey] = totals
	}
	return result
}

// factsByBrand складывает кварталы бренда. Считается из той же карты, что и
// разрез по кварталам: два прохода по отгрузкам однажды разошлись бы.
func factsByBrand(byBrandQuarter map[brandQuarterKey]quarterFact) map[string]quarterFact {
	result := map[string]quarterFact{}
	for key, value := range byBrandQuarter {
		totals := result[key.brand]
		totals.rub = round2(totals.rub + value.rub)
		totals.units = round2(totals.units + value.units)
		result[key.brand] = totals
	}
	return result
}

// monthOfFactKey достаёт месяц из ключа forecastMonthKey. Ключ строится здесь
// же в пакете, поэтому разбор безопасен, но сломанный ключ мы пропускаем,
// а не роняем витрину.
func monthOfFactKey(key string) (int, bool) {
	month, _, ok := monthBrandOfFactKey(key)
	return month, ok
}

// monthBrandOfFactKey достаёт из того же ключа и месяц, и бренд.
func monthBrandOfFactKey(key string) (int, string, bool) {
	parts := strings.SplitN(key, "|", 3)
	if len(parts) < 3 {
		return 0, "", false
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", false
	}
	return month, parts[2], true
}

// monthAccumulator — портфельный итог одного месяца.
type monthAccumulator struct {
	plan    quarterFact
	fact    quarterFact
	eac     quarterFact
	prev    quarterFact
	hasPrev bool
	cells   dashboardCells
}

type promoCellKey struct {
	network string
	quarter int
}

// promoBrandKey — промо бренда в периоде. Месяц и квартал различаются
// назначением: календарь разбора идёт помесячно, ячейка «бренд × квартал» —
// поквартально.
type promoBrandKey struct {
	brand  string
	period int
}

// promoForecastKey связывает согласованный uplift с рекомендацией той же
// сети, бренда и месяца. Названия сети недостаточно держать снаружи: общий
// dashboard рассчитывает много сетей одним проходом.
type promoForecastKey struct {
	network string
	brand   string
	month   int
}

type forecastPromoTotals struct {
	rub   float64
	units float64
}

// promoIndex — промо среза в разрезах, которые нужны экранам: по ячейке
// тепловой карты, по месяцу для тренда и по бренду для разбора одной сети.
type promoIndex struct {
	byCell     map[promoCellKey]dashboardPromoTotals
	tagsByCell map[promoCellKey][]models.NetworkDashboardPromoTag
	byMonth    map[int]dashboardPromoTotals

	byBrandMonth       map[promoBrandKey]dashboardPromoTotals
	tagsByBrandMonth   map[promoBrandKey][]models.NetworkDashboardPromoTag
	byBrandQuarter     map[promoBrandKey]dashboardPromoTotals
	tagsByBrandQuarter map[promoBrandKey][]models.NetworkDashboardPromoTag
	forecastUplifts    map[promoForecastKey]forecastPromoTotals
}

// indexPromos сворачивает промо в счётчики и в набор меток.
// Метки склеиваются по коду и каналу: в одном квартале одна и та же механика
// идёт много раз, и десять одинаковых плиток ничего не добавляют.
func indexPromos(rows []repository.NetworkDashboardPromoRow) promoIndex {
	totals := map[promoCellKey]dashboardPromoTotals{}
	byMonth := map[int]dashboardPromoTotals{}
	byBrandMonth := map[promoBrandKey]dashboardPromoTotals{}
	byBrandQuarter := map[promoBrandKey]dashboardPromoTotals{}
	forecastUplifts := map[promoForecastKey]forecastPromoTotals{}

	cellTags := map[promoCellKey]map[string]*models.NetworkDashboardPromoTag{}
	brandMonthTags := map[promoBrandKey]map[string]*models.NetworkDashboardPromoTag{}
	brandQuarterTags := map[promoBrandKey]map[string]*models.NetworkDashboardPromoTag{}

	for _, row := range rows {
		quarter := (row.Month-1)/3 + 1
		cellKey := promoCellKey{network: row.NetworkName, quarter: quarter}
		channel := promoChannelOf(row.Channel)
		brand := strings.TrimSpace(valueOrEmpty(row.BrandAS))
		monthKey := promoBrandKey{brand: brand, period: row.Month}
		quarterKey := promoBrandKey{brand: brand, period: quarter}
		if brand != "" {
			forecastKey := promoForecastKey{network: row.NetworkName, brand: brand, month: row.Month}
			uplift := forecastUplifts[forecastKey]
			uplift.rub = round2(uplift.rub + row.PlanUpliftRub)
			uplift.units = round2(uplift.units + row.PlanUpliftUnits)
			forecastUplifts[forecastKey] = uplift
		}

		// Один и тот же вклад ложится в четыре разреза, поэтому счёт сведён в
		// одно место: разойтись между разрезами он не должен.
		countIn := func(totals dashboardPromoTotals) dashboardPromoTotals {
			totals.count += row.PromoCount
			totals.invest = round2(totals.invest + row.InvestRub)
			switch channel {
			case promoChannelOnline:
				totals.online += row.PromoCount
			case promoChannelOffline:
				totals.offline += row.PromoCount
			}
			return totals
		}

		byMonth[row.Month] = countIn(byMonth[row.Month])
		totals[cellKey] = countIn(totals[cellKey])
		if brand != "" {
			byBrandMonth[monthKey] = countIn(byBrandMonth[monthKey])
			byBrandQuarter[quarterKey] = countIn(byBrandQuarter[quarterKey])
		}

		mechanics := strings.TrimSpace(valueOrEmpty(row.Mechanics))
		if mechanics == "" {
			continue
		}
		code := promoCodeOf(row.ShortCode, mechanics)
		tagKey := code + "|" + channel
		bumpTag := func(bucket map[string]*models.NetworkDashboardPromoTag) {
			tag := bucket[tagKey]
			if tag == nil {
				tag = &models.NetworkDashboardPromoTag{Code: code, Mechanics: mechanics, Channel: channel}
				bucket[tagKey] = tag
			}
			tag.Count += row.PromoCount
			tag.PlanRub = round2(tag.PlanRub + row.PlanRub)
		}

		if cellTags[cellKey] == nil {
			cellTags[cellKey] = map[string]*models.NetworkDashboardPromoTag{}
		}
		bumpTag(cellTags[cellKey])
		if brand != "" {
			if brandMonthTags[monthKey] == nil {
				brandMonthTags[monthKey] = map[string]*models.NetworkDashboardPromoTag{}
			}
			bumpTag(brandMonthTags[monthKey])
			if brandQuarterTags[quarterKey] == nil {
				brandQuarterTags[quarterKey] = map[string]*models.NetworkDashboardPromoTag{}
			}
			bumpTag(brandQuarterTags[quarterKey])
		}
	}

	return promoIndex{
		byCell:             totals,
		tagsByCell:         sortedPromoTags(cellTags),
		byMonth:            byMonth,
		byBrandMonth:       byBrandMonth,
		tagsByBrandMonth:   sortedPromoTags(brandMonthTags),
		byBrandQuarter:     byBrandQuarter,
		tagsByBrandQuarter: sortedPromoTags(brandQuarterTags),
		forecastUplifts:    forecastUplifts,
	}
}

// promoTagsOrEmpty — пустой список вместо nil.
//
// nil-срез уходит в JSON как null, а сгенерированный тип обещает клиенту
// массив: первое же обращение к длине роняет экран. Ячейка без промо — это
// пустой список меток, а не отсутствие поля.
func promoTagsOrEmpty(tags []models.NetworkDashboardPromoTag) []models.NetworkDashboardPromoTag {
	if tags == nil {
		return []models.NetworkDashboardPromoTag{}
	}
	return tags
}

// sortedPromoTags разворачивает накопленные метки в список: сначала частые,
// при равенстве — по коду, чтобы порядок не зависел от обхода карты.
func sortedPromoTags[K comparable](
	source map[K]map[string]*models.NetworkDashboardPromoTag,
) map[K][]models.NetworkDashboardPromoTag {
	result := map[K][]models.NetworkDashboardPromoTag{}
	for key, byCode := range source {
		list := make([]models.NetworkDashboardPromoTag, 0, len(byCode))
		for _, tag := range byCode {
			list = append(list, *tag)
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Count != list[j].Count {
				return list[i].Count > list[j].Count
			}
			return list[i].Code < list[j].Code
		})
		result[key] = list
	}
	return result
}

// AggregateNetworkDashboard — чистая агрегация витрины: единый источник
// расчётов для карточек, тренда, разрезов и тепловой карты.
func AggregateNetworkDashboard(
	data repository.NetworkDashboardData,
	filter repository.NetworkDashboardFilter,
	now time.Time,
) *models.NetworkDashboardResponse {
	selectedQuarters := filter.NormalizedQuarters()
	quarterSet := map[int]bool{}
	for _, quarter := range selectedQuarters {
		quarterSet[quarter] = true
	}

	current := indexPeriodData(data.Current)
	previous := indexPeriodData(data.Prev)
	promos := indexPromos(data.Promos)

	summary := &networkDashboardAccumulator{}
	quarters := map[int]*networkDashboardAccumulator{}
	networkGroups := map[string]*networkDashboardAccumulator{}
	brandGroups := map[string]*networkDashboardAccumulator{}
	kamGroups := map[string]*networkDashboardAccumulator{}

	networkMeta := map[string]models.Network{}
	cellsList := make([]models.NetworkDashboardCell, 0)
	brandQuarters := make([]models.NetworkDashboardBrandQuarter, 0)
	brandMonths := make([]models.NetworkDashboardBrandMonth, 0)
	skus := make([]models.NetworkDashboardSKU, 0)
	var annualInvestmentCumulative *models.NetworkAnnualInvestmentCumulative
	months := map[int]*monthAccumulator{}
	monthOf := func(month int) *monthAccumulator {
		acc := months[month]
		if acc == nil {
			acc = &monthAccumulator{}
			months[month] = acc
		}
		return acc
	}

	for _, network := range data.Networks {
		plans := current.plans[network.ID]
		if len(plans) == 0 {
			continue
		}
		recommendationFacts := make([]models.NetworkMonthlyFact, 0,
			len(previous.facts[network.ID])+len(current.facts[network.ID]))
		recommendationFacts = append(recommendationFacts, previous.facts[network.ID]...)
		recommendationFacts = append(recommendationFacts, current.facts[network.ID]...)
		slice := buildNetworkSlice(
			network, filter.Year, quarterSet,
			plans, current.periods[network.ID],
			current.facts[network.ID], recommendationFacts, current.forecasts[network.ID],
			promos.forecastUplifts,
			current.groups[network.ID], now,
		)
		if len(data.Networks) == 1 && len(selectedQuarters) == 4 {
			annualInvestmentCumulative = slice.annualInvestmentCumulative
		}

		// Факт прошлого года берётся прямо из помесячных строк: план для
		// сравнения факта с фактом не нужен и в прошлом году может отсутствовать.
		prevFactMonths := factByMonth(previous.facts[network.ID], quarterSet)
		prevFacts := quartersFromMonths(prevFactMonths)
		// Прошлый год по брендам — из тех же отгрузок. Строки плана в этом не
		// участвуют: сравнение факта с фактом от них не зависит.
		prevBrandQuarterFacts := factsByBrandQuarter(previous.facts[network.ID], quarterSet)
		prevBrandFacts := factsByBrand(prevBrandQuarterFacts)

		// План прошлого года — только там, где он заведён, и посчитанный тем же
		// кодом: обязательство по контракту иначе не сложить.
		var prevSlice *networkSlice
		if prevPlans := previous.plans[network.ID]; len(prevPlans) > 0 {
			built := buildNetworkSlice(
				network, data.Prev.Year, quarterSet,
				prevPlans, previous.periods[network.ID],
				previous.facts[network.ID], previous.facts[network.ID], previous.forecasts[network.ID],
				nil,
				previous.groups[network.ID], now,
			)
			prevSlice = &built
		}

		networkMeta[network.Name] = network
		networkAccumulator := accumulatorFor(networkGroups, network.Name)
		kam := strings.TrimSpace(valueOrEmpty(network.KAM))
		if kam == "" {
			kam = networkUnassignedKAM
		}
		kamAccumulator := accumulatorFor(kamGroups, kam)

		for _, quarter := range selectedQuarters {
			total, ok := slice.quarterTotals[quarter]
			if !ok {
				continue
			}
			values := valuesFromTotals(network.ID, total, slice.quarterCells[quarter], slice.quarterUnits[quarter])

			if prevFact, exists := prevFacts[quarter]; exists {
				values.hasPrevFact = true
				values.prevFactRub = prevFact.rub
				values.prevFactUnit = prevFact.units
			}
			if prevSlice != nil {
				if prevTotal, exists := prevSlice.quarterTotals[quarter]; exists {
					values.hasPrevPlan = true
					values.prevPlanRub = prevTotal.ContractPlanRub
				}
			}

			cellKey := promoCellKey{network: network.Name, quarter: quarter}
			values.promo = promos.byCell[cellKey]

			summary.add(values)
			networkAccumulator.add(values)
			kamAccumulator.add(values)

			quarterAccumulator := quarters[quarter]
			if quarterAccumulator == nil {
				quarterAccumulator = &networkDashboardAccumulator{}
				quarters[quarter] = quarterAccumulator
			}
			quarterAccumulator.add(values)

			cellAccumulator := &networkDashboardAccumulator{}
			cellAccumulator.add(values)
			cellsList = append(cellsList, models.NetworkDashboardCell{
				NetworkID: network.ID,
				Name:      network.Name,
				Quarter:   quarter,
				Metrics:   cellAccumulator.metrics(),
				PromoTags: promoTagsOrEmpty(promos.tagsByCell[cellKey]),
			})
		}

		// Месячный ряд портфеля. Прошлый год берётся помесячно из того же
		// источника, что и квартальный, — из помесячных строк факта.
		for month, plan := range slice.monthPlan {
			acc := monthOf(month)
			acc.plan.rub = round2(acc.plan.rub + plan.rub)
			acc.plan.units = round2(acc.plan.units + plan.units)
		}
		for month, fact := range slice.monthFact {
			acc := monthOf(month)
			acc.fact.rub = round2(acc.fact.rub + fact.rub)
			acc.fact.units = round2(acc.fact.units + fact.units)
		}
		for month, eac := range slice.monthEAC {
			acc := monthOf(month)
			acc.eac.rub = round2(acc.eac.rub + eac.rub)
			acc.eac.units = round2(acc.eac.units + eac.units)
		}
		for month, cells := range slice.monthCells {
			monthOf(month).cells.add(cells)
		}
		for month, prev := range prevFactMonths {
			acc := monthOf(month)
			acc.hasPrev = true
			acc.prev.rub = round2(acc.prev.rub + prev.rub)
			acc.prev.units = round2(acc.prev.units + prev.units)
		}

		// Бренды считаются отдельно от квартальных итогов: у сети и у бренда
		// разный плановый объём, и складывать их в один агрегат нельзя.
		for brand, values := range slice.brandValues {
			contribution := *values
			// Факт прошлого года — из отгрузок, а не из строк плана: сравнение
			// идёт факт с фактом, и бренд, которого год назад не планировали,
			// но уже отгружали, обязан его показать.
			if prevFact, exists := prevBrandFacts[brand]; exists {
				contribution.hasPrevFact = true
				contribution.prevFactRub = prevFact.rub
				contribution.prevFactUnit = prevFact.units
			}
			// План прошлого года — наоборот, только там, где он заведён:
			// сравнивать план с планом можно лишь там, где план был.
			if prevSlice != nil {
				if prevValues := prevSlice.brandValues[brand]; prevValues != nil {
					contribution.hasPrevPlan = true
					contribution.prevPlanRub = prevValues.planRub
				}
			}
			accumulatorFor(brandGroups, brand).add(contribution)
		}
		// Счётчик брендов в разрезе сетей берётся из тех же строк.
		for brand := range slice.brandValues {
			networkAccumulator.markBrand(brand)
			kamAccumulator.markBrand(brand)
			summary.markBrand(brand)
		}

		// Разрезы разбора считаются только для одной сети в области: на
		// портфеле это бренды × кварталы × сети, и ответ вырос бы в разы.
		if len(data.Networks) == 1 {
			brandQuarters = buildBrandQuarters(slice, prevSlice, prevBrandQuarterFacts, promos)
			brandMonths = buildBrandMonths(promos)
			// Прошлый год для SKU читается прямо из факта, минуя строки плана:
			// год с отгрузками, но без заведённых планов — обычное дело, и
			// требовать план ради сравнения факта с фактом значило бы прятать
			// сравнение там, где оно нужнее всего.
			skus = buildSKUs(slice, skuFactsByBrand(previous.facts[network.ID], quarterSet))
		}
	}

	monthTrend := make([]models.NetworkDashboardMonthPoint, 0, len(months))
	for month, acc := range months {
		point := models.NetworkDashboardMonthPoint{
			Year:                 filter.Year,
			Month:                month,
			Quarter:              (month-1)/3 + 1,
			PlanRub:              acc.plan.rub,
			PlanUnits:            acc.plan.units,
			FactRub:              acc.fact.rub,
			FactUnits:            acc.fact.units,
			EACRub:               acc.eac.rub,
			EACUnits:             acc.eac.units,
			Closed:               isClosedForecastMonth(filter.Year, month, now),
			CellsWithoutForecast: acc.cells.openWithoutForecast,
		}
		if promo, ok := promos.byMonth[month]; ok {
			point.PromoCount = promo.count
			point.PromoOnlineCount = promo.online
			point.PromoOfflineCount = promo.offline
		}
		if acc.hasPrev {
			prevRub, prevUnits := acc.prev.rub, acc.prev.units
			point.PrevFactRub = &prevRub
			point.PrevFactUnits = &prevUnits
		}
		monthTrend = append(monthTrend, point)
	}
	sort.Slice(monthTrend, func(i, j int) bool { return monthTrend[i].Month < monthTrend[j].Month })

	trend := make([]models.NetworkDashboardPeriodPoint, 0, len(quarters))
	for quarter, accumulator := range quarters {
		trend = append(trend, models.NetworkDashboardPeriodPoint{
			Year: filter.Year, Quarter: quarter, Metrics: accumulator.metrics(),
		})
	}
	sort.Slice(trend, func(i, j int) bool { return trend[i].Quarter < trend[j].Quarter })

	networks := make([]models.NetworkDashboardBreakdown, 0, len(networkGroups))
	for name, accumulator := range networkGroups {
		meta := networkMeta[name]
		id := meta.ID
		networks = append(networks, models.NetworkDashboardBreakdown{
			Name: name, NetworkID: &id, KAM: meta.KAM, Metrics: accumulator.metrics(),
		})
	}
	sortBreakdown(networks)

	brands := make([]models.NetworkDashboardBreakdown, 0, len(brandGroups))
	for name, accumulator := range brandGroups {
		brands = append(brands, models.NetworkDashboardBreakdown{
			Name: name, Metrics: accumulator.metrics(), InGross: accumulator.inGross(),
		})
	}
	sortBreakdown(brands)

	kams := make([]models.NetworkDashboardBreakdown, 0, len(kamGroups))
	for name, accumulator := range kamGroups {
		kams = append(kams, models.NetworkDashboardBreakdown{Name: name, Metrics: accumulator.metrics()})
	}
	sortBreakdown(kams)

	sort.Slice(cellsList, func(i, j int) bool {
		if cellsList[i].Name != cellsList[j].Name {
			return cellsList[i].Name < cellsList[j].Name
		}
		return cellsList[i].Quarter < cellsList[j].Quarter
	})

	years := data.AvailableYears
	if years == nil {
		years = []int{}
	}

	return &models.NetworkDashboardResponse{
		Year:                       filter.Year,
		SelectedQuarters:           selectedQuarters,
		AvailableYears:             years,
		Summary:                    summary.metrics(),
		Quarters:                   trend,
		Months:                     monthTrend,
		Networks:                   networks,
		Brands:                     brands,
		KAMs:                       kams,
		NetworkQuarters:            cellsList,
		BrandQuarters:              brandQuarters,
		BrandMonths:                brandMonths,
		SKUs:                       skus,
		AnnualInvestmentCumulative: annualInvestmentCumulative,
	}
}

// skuFactsByBrand — факт SKU за срез, собранный прямо из помесячных строк.
// Строки плана здесь не участвуют намеренно: см. factByMonth — для прошлого
// года это единственный путь, не прячущий сравнение.
func skuFactsByBrand(facts []models.NetworkMonthlyFact, quarters map[int]bool) map[networkSKUKey]*networkSKUTotals {
	result := map[networkSKUKey]*networkSKUTotals{}
	for _, fact := range facts {
		if fact.SKU == nil || !quarters[(fact.Month-1)/3+1] {
			continue
		}
		key := networkSKUKey{brand: strings.TrimSpace(fact.BrandAS), sku: *fact.SKU}
		totals := result[key]
		if totals == nil {
			totals = &networkSKUTotals{}
			result[key] = totals
		}
		totals.factRub = round2(totals.factRub + valueOrZero(fact.FactRub))
		totals.factUnits = round2(totals.factUnits + valueOrZero(fact.FactUnits))
		totals.factInvest = round2(totals.factInvest + valueOrZero(fact.FactInvestmentsRub))
	}
	return result
}

// buildSKUs — строки SKU разбора одной сети.
//
// Плановых величин здесь нет: план в реестре заводится брендом. Сумма SKU
// вправе не дотягивать до итога бренда — официальный прогноз ведётся на
// бренде, и месяц, где SKU-строк прогноза нет, в эту сумму не попадает.
// Насколько не дотягивает, видно по доле: она для того и считается.
func buildSKUs(
	slice networkSlice,
	prevFacts map[networkSKUKey]*networkSKUTotals,
) []models.NetworkDashboardSKU {
	result := make([]models.NetworkDashboardSKU, 0, len(slice.skuTotals))
	for key, totals := range slice.skuTotals {
		row := models.NetworkDashboardSKU{
			Brand:              key.brand,
			SKU:                key.sku,
			FactRub:            totals.factRub,
			FactUnits:          totals.factUnits,
			EACRub:             totals.eacRub,
			EACUnits:           totals.eacUnits,
			FactInvestmentsRub: totals.factInvest,
		}
		if prev := prevFacts[key]; prev != nil {
			prevRub, prevUnits := prev.factRub, prev.factUnits
			row.PrevFactRub = &prevRub
			row.PrevFactUnits = &prevUnits
			row.FactYoYPct = growthPct(totals.factRub, prevRub)
		}
		if brand := slice.brandValues[key.brand]; brand != nil && brand.eacRub != 0 {
			share := round2(totals.eacRub / brand.eacRub * 100)
			row.ShareOfBrandPct = &share
		}
		result = append(result, row)
	}
	// Внутри бренда — по убыванию ожидаемого объёма: разговор начинается с
	// того SKU, который делает результат.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Brand != result[j].Brand {
			return result[i].Brand < result[j].Brand
		}
		if result[i].EACRub != result[j].EACRub {
			return result[i].EACRub > result[j].EACRub
		}
		return result[i].SKU < result[j].SKU
	})
	return result
}

// buildBrandQuarters — разрез «бренд × квартал» разбора одной сети.
//
// Метрика собирается тем же аккумулятором, что и все прочие срезы: свои
// правила подсчёта здесь завели бы вторую арифметику, которая однажды
// разойдётся с квартальной строкой сети.
func buildBrandQuarters(
	slice networkSlice,
	prev *networkSlice,
	prevFacts map[brandQuarterKey]quarterFact,
	promos promoIndex,
) []models.NetworkDashboardBrandQuarter {
	result := make([]models.NetworkDashboardBrandQuarter, 0, len(slice.brandQuarterValues))
	for key, values := range slice.brandQuarterValues {
		contribution := *values
		// Факт прошлого года — из отгрузок той же пары, независимо от того,
		// заводили ли тогда план: сравнение идёт факт с фактом.
		if prevFact, exists := prevFacts[key]; exists {
			contribution.hasPrevFact = true
			contribution.prevFactRub = prevFact.rub
			contribution.prevFactUnit = prevFact.units
		}
		// План прошлого года — только там, где строка плана была: сравнивать
		// план с планом можно лишь там, где он существовал.
		if prev != nil {
			if prevValues := prev.brandQuarterValues[key]; prevValues != nil {
				contribution.hasPrevPlan = true
				contribution.prevPlanRub = prevValues.planRub
			}
		}
		promoKey := promoBrandKey{brand: key.brand, period: key.quarter}
		contribution.promo = promos.byBrandQuarter[promoKey]

		accumulator := &networkDashboardAccumulator{}
		accumulator.add(contribution)
		result = append(result, models.NetworkDashboardBrandQuarter{
			Brand:     key.brand,
			Quarter:   key.quarter,
			Metrics:   accumulator.metrics(),
			PromoTags: promoTagsOrEmpty(promos.tagsByBrandQuarter[promoKey]),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Brand != result[j].Brand {
			return result[i].Brand < result[j].Brand
		}
		return result[i].Quarter < result[j].Quarter
	})
	return result
}

// buildBrandMonths — промо-календарь разбора: только те ячейки, где промо
// действительно были. Пустые клетки рисует фронтенд из списка брендов и
// выбранных кварталов — гонять по сети нули незачем.
func buildBrandMonths(promos promoIndex) []models.NetworkDashboardBrandMonth {
	result := make([]models.NetworkDashboardBrandMonth, 0, len(promos.byBrandMonth))
	for key, totals := range promos.byBrandMonth {
		result = append(result, models.NetworkDashboardBrandMonth{
			Brand:               key.brand,
			Month:               key.period,
			PromoCount:          totals.count,
			PromoOnlineCount:    totals.online,
			PromoOfflineCount:   totals.offline,
			PromoInvestmentsRub: totals.invest,
			PromoTags:           promoTagsOrEmpty(promos.tagsByBrandMonth[key]),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Brand != result[j].Brand {
			return result[i].Brand < result[j].Brand
		}
		return result[i].Month < result[j].Month
	})
	return result
}

// networkUnassignedKAM — сети без закреплённого КАМа собираются в одну строку,
// а не растворяются в пустом имени.
const networkUnassignedKAM = "Без КАМ"

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// BuildNetworkDashboard читает срез реестра и собирает витрину.
func BuildNetworkDashboard(filter repository.NetworkDashboardFilter) (*models.NetworkDashboardResponse, error) {
	data, err := repository.GetNetworkDashboardData(filter)
	if err != nil {
		return nil, err
	}
	return AggregateNetworkDashboard(data, filter, time.Now()), nil
}
