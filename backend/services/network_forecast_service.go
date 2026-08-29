package services

import (
	"fmt"
	"math"
	"sort"
	"time"

	"backend/models"
)

type monthlyFactAggregate struct {
	rub         *float64
	units       *float64
	investments *float64
}

func forecastMonthKey(year, month int, brand string) string {
	return fmt.Sprintf("%d|%d|%s", year, month, brand)
}

func forecastSKUKey(year, month int, brand, sku string) string {
	return fmt.Sprintf("%d|%d|%s|%s", year, month, brand, sku)
}

func addPtrValue(target **float64, value *float64) {
	if value == nil {
		return
	}
	current := 0.0
	if *target != nil {
		current = **target
	}
	sum := round2(current + *value)
	*target = &sum
}

// aggregateFacts предпочитает готовую строку бренда; если её нет, складывает SKU.
// Так источник может перейти на SKU постепенно, не удваивая факт.
func aggregateFacts(facts []models.NetworkMonthlyFact) (map[string]monthlyFactAggregate, map[string]monthlyFactAggregate) {
	brandRows := map[string]monthlyFactAggregate{}
	skuSums := map[string]monthlyFactAggregate{}
	skuRows := map[string]monthlyFactAggregate{}
	for _, fact := range facts {
		brandKey := forecastMonthKey(fact.Year, fact.Month, fact.BrandAS)
		if fact.SKU == nil {
			brandRows[brandKey] = monthlyFactAggregate{
				rub: fact.FactRub, units: fact.FactUnits, investments: fact.FactInvestmentsRub,
			}
			continue
		}
		sku := *fact.SKU
		skuKey := forecastSKUKey(fact.Year, fact.Month, fact.BrandAS, sku)
		skuRows[skuKey] = monthlyFactAggregate{
			rub: fact.FactRub, units: fact.FactUnits, investments: fact.FactInvestmentsRub,
		}
		agg := skuSums[brandKey]
		addPtrValue(&agg.rub, fact.FactRub)
		addPtrValue(&agg.units, fact.FactUnits)
		addPtrValue(&agg.investments, fact.FactInvestmentsRub)
		skuSums[brandKey] = agg
	}
	for key, sum := range skuSums {
		if _, exists := brandRows[key]; !exists {
			brandRows[key] = sum
		}
	}
	return brandRows, skuRows
}

func networkMonthlyDistribution(network models.Network) [3]float64 {
	distribution := [3]float64{network.Month1Pct, network.Month2Pct, network.Month3Pct}
	if distribution[0] < 0 || distribution[1] < 0 || distribution[2] < 0 ||
		math.Abs(distribution[0]+distribution[1]+distribution[2]-100) > 0.001 {
		return [3]float64{30, 30, 40}
	}
	return distribution
}

func allocationForPlan(plan models.NetworkPlan, distribution [3]float64) [3]float64 {
	if plan.PlanRub == nil {
		return [3]float64{}
	}
	first := round2(*plan.PlanRub * distribution[0] / 100)
	second := round2(*plan.PlanRub * distribution[1] / 100)
	third := round2(*plan.PlanRub - first - second)
	return [3]float64{first, second, third}
}

func investmentAllocationForPlan(plan models.NetworkPlan, volume [3]float64) [3]float64 {
	if plan.InvestmentsPct == nil {
		return [3]float64{}
	}
	first := round2(volume[0] * *plan.InvestmentsPct / 100)
	second := round2(volume[1] * *plan.InvestmentsPct / 100)
	total := 0.0
	if plan.PlanRub != nil {
		total = round2(*plan.PlanRub * *plan.InvestmentsPct / 100)
	}
	third := round2(total - first - second)
	return [3]float64{first, second, third}
}

func effectiveContractPrice(prices []models.NetworkContractPrice, sku string, year, month int) *float64 {
	target := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	for _, price := range prices {
		if price.SKU != sku {
			continue
		}
		from, errFrom := time.Parse("2006-01-02", price.ValidFrom)
		to, errTo := time.Parse("2006-01-02", price.ValidTo)
		if errFrom == nil && errTo == nil && !target.Before(from) && !target.After(to) {
			value := price.ContractPrice
			return &value
		}
	}
	return nil
}

func averageAvailable(values ...*float64) *float64 {
	total := 0.0
	count := 0
	for _, value := range values {
		if value != nil {
			total += *value
			count++
		}
	}
	if count == 0 {
		return nil
	}
	avg := round2(total / float64(count))
	return &avg
}

func trailingAverage(
	facts map[string]monthlyFactAggregate,
	year, month int,
	brand string,
	metric func(monthlyFactAggregate) *float64,
) *float64 {
	values := make([]*float64, 0, 3)
	for offset := 1; offset <= 3; offset++ {
		date := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -offset, 0)
		values = append(values, metric(facts[forecastMonthKey(date.Year(), int(date.Month()), brand)]))
	}
	return averageAvailable(values...)
}

func recommendedForecastMetric(
	facts map[string]monthlyFactAggregate,
	year, month int,
	brand string,
	uplift float64,
	metric func(monthlyFactAggregate) *float64,
) (*float64, *string) {
	lastYear := metric(facts[forecastMonthKey(year-1, month, brand)])
	recent := trailingAverage(facts, year, month, brand, metric)
	base := averageAvailable(lastYear, recent)
	confidence := "low"
	if lastYear != nil && recent != nil {
		confidence = "high"
	} else if lastYear != nil || recent != nil {
		confidence = "medium"
	}
	if base == nil {
		return nil, &confidence
	}
	value := round2(*base + uplift)
	return &value, &confidence
}

func rubMetric(value monthlyFactAggregate) *float64   { return value.rub }
func unitsMetric(value monthlyFactAggregate) *float64 { return value.units }

func trailingSKUAverage(
	facts map[string]monthlyFactAggregate,
	year, month int,
	brand, sku string,
) *float64 {
	values := make([]*float64, 0, 3)
	for offset := 1; offset <= 3; offset++ {
		date := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -offset, 0)
		values = append(values, facts[forecastSKUKey(date.Year(), int(date.Month()), brand, sku)].units)
	}
	return averageAvailable(values...)
}

func recommendedSKUUnits(facts map[string]monthlyFactAggregate, year, month int, brand, sku string) (*float64, *string) {
	lastYear := facts[forecastSKUKey(year-1, month, brand, sku)].units
	recent := trailingSKUAverage(facts, year, month, brand, sku)
	base := averageAvailable(lastYear, recent)
	confidence := "low"
	if lastYear != nil && recent != nil {
		confidence = "high"
	} else if lastYear != nil || recent != nil {
		confidence = "medium"
	}
	return base, &confidence
}

// entryModeOfPlan — режим ведения бренда. У бренда, которого в плане квартала
// ещё нет, режим берётся из профиля сети, а пустые значения старых строк
// означают прежний способ ведения: рубли по бренду.
func entryModeOfPlan(plan models.NetworkPlan, network models.Network) (string, string) {
	level, unit := plan.EntryLevel, plan.EntryUnit
	if level == "" {
		level = network.DefaultEntryLevel
	}
	if unit == "" {
		unit = network.DefaultEntryUnit
	}
	if level != "sku" {
		level = "brand"
	}
	if unit != "units" {
		unit = "rub"
	}
	return level, unit
}

// skuMixShares — доли SKU внутри бренда, по которым разложение бренда на SKU
// остаётся правдоподобным. Микс берётся из факта: сначала аналогичный месяц
// прошлого года, затем среднее последних трёх месяцев, и только если истории
// нет вовсе — поровну. Сумма долей всегда равна единице.
//
// Это единственное место, где разложение опирается на допущение, а не на
// арифметику, поэтому полученные строки помечаются расчётными и не редактируются.
func skuMixShares(
	skuFacts map[string]monthlyFactAggregate,
	year, month int,
	brand string,
	skus []string,
) map[string]float64 {
	if len(skus) == 0 {
		return nil
	}
	weights := make(map[string]float64, len(skus))
	total := 0.0
	for _, sku := range skus {
		weight := skuFacts[forecastSKUKey(year-1, month, brand, sku)].units
		if weight == nil {
			weight = trailingSKUAverage(skuFacts, year, month, brand, sku)
		}
		if weight != nil && *weight > 0 {
			weights[sku] = *weight
			total += *weight
		}
	}
	shares := make(map[string]float64, len(skus))
	if total <= 0 {
		equal := 1 / float64(len(skus))
		for _, sku := range skus {
			shares[sku] = equal
		}
		return shares
	}
	for _, sku := range skus {
		shares[sku] = weights[sku] / total
	}
	return shares
}

// brandWeightedPrice — цена бренда, взвешенная по миксу SKU. Нужна бренду,
// который ведут в упаковках без детализации: перевести его объём в рубли
// иначе нечем. Считается только по SKU с известной ценой контракта.
func brandWeightedPrice(
	prices []models.NetworkContractPrice,
	brand string,
	year, month int,
	skus []string,
	shares map[string]float64,
) *float64 {
	weighted, covered := 0.0, 0.0
	for _, sku := range skus {
		price := effectiveContractPrice(prices, sku, year, month)
		if price == nil || shares[sku] <= 0 {
			continue
		}
		weighted += *price * shares[sku]
		covered += shares[sku]
	}
	if covered <= 0 {
		return nil
	}
	value := round2(weighted / covered)
	return &value
}

// ownedMetrics приводит пару «рубли / упаковки» к единице ввода: введённая
// метрика остаётся как есть, вторая пересчитывается по цене контракта. Без
// цены вторая метрика остаётся пустой: выдумывать курс пересчёта незачем.
//
// Посчитанная пара закрепляется в БД (см. SyncNetworkForecastPairs): колонки
// читает не только форма, а витрина реестра и выгрузки пересчитать по прайсу
// не могут. Расходиться сама с собой пара не начнёт — правка цен запускает
// пересчёт тем же путём, что и правка самого прогноза.
func ownedMetrics(rub, units *float64, entryUnit string, price *float64) (*float64, *float64) {
	if entryUnit == "units" {
		rub = nil
	} else {
		units = nil
	}
	if price == nil || *price <= 0 {
		return rub, units
	}
	if rub == nil && units != nil {
		value := round2(*units * *price)
		rub = &value
	}
	if units == nil && rub != nil {
		value := round2(*rub / *price)
		units = &value
	}
	return rub, units
}

// shareOf — доля итога бренда, приходящаяся на один SKU.
func shareOf(total *float64, share float64) *float64 {
	if total == nil || share <= 0 {
		return nil
	}
	value := round2(*total * share)
	return &value
}

func isClosedForecastMonth(year, month int, now time.Time) bool {
	return year < now.Year() || (year == now.Year() && month < int(now.Month()))
}

func isCurrentForecastMonth(year, month int, now time.Time) bool {
	return year == now.Year() && month == int(now.Month())
}

func valueForEAC(closed bool, fact, official, system *float64) *float64 {
	if closed && fact != nil {
		return fact
	}
	if official != nil {
		return official
	}
	if system != nil {
		return system
	}
	return fact
}

// investmentsForEAC: прогноз инвестиций не вводится руками. Закрытый месяц
// берёт факт выплат, иначе сумма считается процентом бренда от EAC объёма —
// тем же процентом, что применяется к плану в квартальной форме. Сохранённое
// переопределение остаётся для разовых выплат вне процента и показывается
// в форме отдельным признаком.
func investmentsForEAC(closed bool, fact, override, eacRub, pct *float64) (*float64, string) {
	if closed && fact != nil {
		return fact, "fact"
	}
	if override != nil {
		return override, "override"
	}
	if eacRub == nil || pct == nil {
		return nil, "none"
	}
	value := round2(*eacRub * *pct / 100)
	return &value, "pct"
}

func completionPct(value, plan float64) *float64 {
	if plan == 0 {
		return nil
	}
	result := round2(value / plan * 100)
	return &result
}

// BuildNetworkForecast собирает рабочее место прогноза. План сравнивается с EAC,
// но не используется как системная рекомендация: рекомендация строится только на
// факте прошлых/последних периодов и согласованном промо-uplift.
func BuildNetworkForecast(
	network models.Network,
	year, quarter int,
	plans []models.NetworkPlan,
	periods []models.NetworkPeriod,
	facts []models.NetworkMonthlyFact,
	forecasts []models.NetworkForecastLine,
	promos []models.NetworkPromoIndicator,
	prices []models.NetworkContractPrice,
	groups []models.NetworkPeriodGroup,
	now time.Time,
) models.NetworkForecastResponse {
	monthFrom := (quarter-1)*3 + 1
	monthTo := monthFrom + 2
	monthDistribution := networkMonthlyDistribution(network)
	brandFacts, skuFacts := aggregateFacts(facts)

	planByBrand := map[string]models.NetworkPlan{}
	brands := map[string]bool{}
	for _, plan := range plans {
		if plan.Quarter == quarter && plan.BrandAS != nil {
			planByBrand[*plan.BrandAS] = plan
			brands[*plan.BrandAS] = true
		}
	}
	promoByKey := map[string]models.NetworkPromoIndicator{}
	for _, promo := range promos {
		promoByKey[forecastMonthKey(promo.Year, promo.Month, promo.BrandAS)] = promo
		brands[promo.BrandAS] = true
	}
	forecastByBrand := map[string]models.NetworkForecastLine{}
	forecastBySKU := map[string]models.NetworkForecastLine{}
	skusByBrand := map[string]map[string]bool{}
	for _, line := range forecasts {
		brands[line.BrandAS] = true
		if line.SKU == nil {
			forecastByBrand[forecastMonthKey(line.Year, line.Month, line.BrandAS)] = line
			continue
		}
		forecastBySKU[forecastSKUKey(line.Year, line.Month, line.BrandAS, *line.SKU)] = line
		if skusByBrand[line.BrandAS] == nil {
			skusByBrand[line.BrandAS] = map[string]bool{}
		}
		skusByBrand[line.BrandAS][*line.SKU] = true
	}
	for _, fact := range facts {
		if fact.Year == year && fact.Month >= monthFrom && fact.Month <= monthTo {
			brands[fact.BrandAS] = true
			if fact.SKU != nil {
				if skusByBrand[fact.BrandAS] == nil {
					skusByBrand[fact.BrandAS] = map[string]bool{}
				}
				skusByBrand[fact.BrandAS][*fact.SKU] = true
			}
		}
	}
	for _, price := range prices {
		brands[price.BrandAS] = true
		if skusByBrand[price.BrandAS] == nil {
			skusByBrand[price.BrandAS] = map[string]bool{}
		}
		skusByBrand[price.BrandAS][price.SKU] = true
	}

	brandNames := make([]string, 0, len(brands))
	for brand := range brands {
		brandNames = append(brandNames, brand)
	}
	sort.Slice(brandNames, func(i, j int) bool { return brandNames[i] < brandNames[j] })

	rows := []models.NetworkForecastMonth{}
	brandTotals := make([]models.NetworkForecastBrandTotals, 0, len(brandNames))
	for _, brand := range brandNames {
		plan := planByBrand[brand]
		planMonths := allocationForPlan(plan, monthDistribution)
		investmentMonths := investmentAllocationForPlan(plan, planMonths)
		entryLevel, entryUnit := entryModeOfPlan(plan, network)
		brandTotal := models.NetworkForecastBrandTotals{BrandAS: brand}

		skuNames := make([]string, 0, len(skusByBrand[brand]))
		for sku := range skusByBrand[brand] {
			skuNames = append(skuNames, sku)
		}
		sort.Strings(skuNames)

		// Что показать в SKU-строках бренда, который ведут на уровне бренда:
		// его EAC, разложенный по миксу. Собирается здесь, потому что доли
		// и итог месяца известны только после расчёта строки бренда.
		type brandMonthView struct {
			shares   map[string]float64
			eacRub   *float64
			eacUnits *float64
		}
		views := make(map[int]brandMonthView, monthTo-monthFrom+1)

		for month := monthFrom; month <= monthTo; month++ {
			index := month - monthFrom
			key := forecastMonthKey(year, month, brand)
			fact := brandFacts[key]
			promo := promoByKey[key]
			line := forecastByBrand[key]

			shares := skuMixShares(skuFacts, year, month, brand, skuNames)
			brandPrice := brandWeightedPrice(prices, brand, year, month, skuNames, shares)

			// Правило владения: у бренда ровно один уровень ввода. Детализованный
			// бренд равен сумме своих SKU — сохранённая строка бренда в расчёт не
			// идёт, иначе одно и то же значение жило бы в двух местах и расходилось.
			// Бренд без детализации, наоборот, живёт собственной строкой.
			if entryLevel == "sku" {
				var skuRub, skuUnits *float64
				for _, sku := range skuNames {
					skuLine, ok := forecastBySKU[forecastSKUKey(year, month, brand, sku)]
					if !ok {
						continue
					}
					lineRub, lineUnits := ownedMetrics(
						skuLine.ForecastRub, skuLine.ForecastUnits, entryUnit,
						effectiveContractPrice(prices, sku, year, month),
					)
					addPtrValue(&skuRub, lineRub)
					addPtrValue(&skuUnits, lineUnits)
				}
				line.ForecastRub, line.ForecastUnits = skuRub, skuUnits
			} else {
				line.ForecastRub, line.ForecastUnits = ownedMetrics(
					line.ForecastRub, line.ForecastUnits, entryUnit, brandPrice,
				)
			}

			systemRub, confidence := recommendedForecastMetric(brandFacts, year, month, brand, promo.PlanUpliftRub, rubMetric)
			systemUnits, unitsConfidence := recommendedForecastMetric(brandFacts, year, month, brand, promo.PlanUpliftUnits, unitsMetric)
			if systemRub == nil && systemUnits != nil {
				confidence = unitsConfidence
			}
			if line.Confidence != nil {
				confidence = line.Confidence
			}
			closed := isClosedForecastMonth(year, month, now)
			eacRub := valueForEAC(closed, fact.rub, line.ForecastRub, systemRub)
			eacUnits := valueForEAC(closed, fact.units, line.ForecastUnits, systemUnits)
			eacInvestments, investmentsSource := investmentsForEAC(
				closed, fact.investments, line.ForecastInvestmentsRub, eacRub, plan.InvestmentsPct,
			)

			planRub := planMonths[index]
			planInvestment := investmentMonths[index]
			row := models.NetworkForecastMonth{
				Year: year, Quarter: quarter, Month: month, BrandAS: brand,
				PlanRub: &planRub, PlanInvestmentsRub: &planInvestment,
				InvestmentsPct: plan.InvestmentsPct, InvestmentsSource: investmentsSource,
				FactRub: fact.rub, FactUnits: fact.units, FactInvestmentsRub: fact.investments,
				ForecastRub: line.ForecastRub, ForecastUnits: line.ForecastUnits,
				ForecastInvestmentsRub: line.ForecastInvestmentsRub,
				SystemForecastRub:      systemRub, SystemForecastUnits: systemUnits,
				EACRub: eacRub, EACUnits: eacUnits, EACInvestmentsRub: eacInvestments,
				Confidence: confidence, AdjustmentReason: line.AdjustmentReason,
				PromoCount: promo.PromoCount, ApprovedPromoCount: promo.ApprovedCount,
				DraftPromoCount: promo.DraftCount, PromoPlanUnits: promo.PlanPromoUnits,
				PromoPlanRub: promo.PlanPromoRub, PromoInvestmentsRub: promo.PlanInvestmentsRub,
				PromoUpliftRub: promo.PlanUpliftRub,
				EntryLevel:     entryLevel, EntryUnit: entryUnit,
				IsDerived: entryLevel == "sku",
				IsClosed:  closed, IsCurrent: isCurrentForecastMonth(year, month, now),
				UpdatedAt: line.UpdatedAt,
			}
			rows = append(rows, row)
			views[month] = brandMonthView{shares: shares, eacRub: eacRub, eacUnits: eacUnits}

			brandTotal.PlanRub = round2(brandTotal.PlanRub + planRub)
			brandTotal.PlanInvestmentsRub = round2(brandTotal.PlanInvestmentsRub + planInvestment)
			if fact.rub != nil {
				brandTotal.FactRub = round2(brandTotal.FactRub + *fact.rub)
			}
			if fact.units != nil {
				brandTotal.FactUnits = round2(brandTotal.FactUnits + *fact.units)
			}
			if fact.investments != nil {
				brandTotal.FactInvestmentsRub = round2(brandTotal.FactInvestmentsRub + *fact.investments)
			}
			if eacRub != nil {
				brandTotal.EACRub = round2(brandTotal.EACRub + *eacRub)
			}
			if eacUnits != nil {
				brandTotal.EACUnits = round2(brandTotal.EACUnits + *eacUnits)
			}
			if eacInvestments != nil {
				brandTotal.EACInvestmentsRub = round2(brandTotal.EACInvestmentsRub + *eacInvestments)
			}
			brandTotal.PromoCount += promo.PromoCount
		}
		brandTotal.CompletionPct = completionPct(brandTotal.EACRub, brandTotal.PlanRub)
		brandTotal.GapRub = round2(brandTotal.EACRub - brandTotal.PlanRub)
		brandTotal.InvestmentVarianceRub = round2(brandTotal.EACInvestmentsRub - brandTotal.PlanInvestmentsRub)
		brandTotals = append(brandTotals, brandTotal)

		// SKU-строки идут после строки бренда: в детализованном бренде они и есть
		// введённые значения, в остальных — его разложение по миксу.
		for _, sku := range skuNames {
			for month := monthFrom; month <= monthTo; month++ {
				fact := skuFacts[forecastSKUKey(year, month, brand, sku)]
				line := forecastBySKU[forecastSKUKey(year, month, brand, sku)]
				contractPrice := effectiveContractPrice(prices, sku, year, month)
				systemUnits, systemConfidence := recommendedSKUUnits(skuFacts, year, month, brand, sku)
				if line.Confidence != nil {
					systemConfidence = line.Confidence
				}
				closed := isClosedForecastMonth(year, month, now)

				var forecastRub, forecastUnits, eacRub, eacUnits *float64
				if entryLevel == "sku" {
					forecastRub, forecastUnits = ownedMetrics(
						line.ForecastRub, line.ForecastUnits, entryUnit, contractPrice,
					)
					eacRub = valueForEAC(closed, fact.rub, forecastRub, nil)
					eacUnits = valueForEAC(closed, fact.units, forecastUnits, systemUnits)
				} else {
					// Бренд ведут целиком: SKU-строка показывает его долю по миксу.
					// Это единственная величина здесь, которая опирается на допущение,
					// поэтому строка помечена расчётной и в форме не редактируется.
					view := views[month]
					eacRub = shareOf(view.eacRub, view.shares[sku])
					eacUnits = shareOf(view.eacUnits, view.shares[sku])
					if closed {
						if fact.rub != nil {
							eacRub = fact.rub
						}
						if fact.units != nil {
							eacUnits = fact.units
						}
					}
				}

				rows = append(rows, models.NetworkForecastMonth{
					Year: year, Quarter: quarter, Month: month, BrandAS: brand, SKU: &sku,
					ContractPrice: contractPrice,
					FactRub:       fact.rub, FactUnits: fact.units, FactInvestmentsRub: fact.investments,
					ForecastRub: forecastRub, ForecastUnits: forecastUnits,
					SystemForecastUnits: systemUnits,
					EACRub:              eacRub, EACUnits: eacUnits,
					// Инвестиции считаются процентом бренда от его EAC: по SKU процент
					// не ведётся, поэтому SKU-строка их не показывает и не суммирует.
					InvestmentsSource: "none",
					Confidence:        systemConfidence, AdjustmentReason: line.AdjustmentReason,
					EntryLevel: entryLevel, EntryUnit: entryUnit,
					IsDerived: entryLevel != "sku",
					IsClosed:  closed, IsCurrent: isCurrentForecastMonth(year, month, now),
					UpdatedAt: line.UpdatedAt,
				})
			}
		}
	}

	// Порог выполнения. Считается здесь, а не на входе: инвестиции меряются
	// тем EAC, который эта же функция только что собрала, а квартальные
	// колонки плана могут быть ещё не обновлены сводом.
	applyForecastInvestmentRule(plans, periods, groups, year, quarter, rows, brandTotals)

	planTotals := CalculateNetworkTotals(EnrichNetworkPlans(clonePlans(plans), periods), periods)[quarter-1]
	totals := models.NetworkForecastTotals{
		PlanRub:            planTotals.ContractPlanRub,
		PlanInvestmentsRub: planTotals.InvestmentsRub,
	}
	for _, brand := range brandTotals {
		totals.FactRub = round2(totals.FactRub + brand.FactRub)
		totals.FactUnits = round2(totals.FactUnits + brand.FactUnits)
		totals.EACRub = round2(totals.EACRub + brand.EACRub)
		totals.EACUnits = round2(totals.EACUnits + brand.EACUnits)
		totals.FactInvestmentsRub = round2(totals.FactInvestmentsRub + brand.FactInvestmentsRub)
		totals.EACInvestmentsRub = round2(totals.EACInvestmentsRub + brand.EACInvestmentsRub)
		totals.PromoCount += brand.PromoCount
	}
	totals.CompletionPct = completionPct(totals.EACRub, totals.PlanRub)
	totals.GapRub = round2(totals.EACRub - totals.PlanRub)
	totals.InvestmentVarianceRub = round2(totals.EACInvestmentsRub - totals.PlanInvestmentsRub)

	return models.NetworkForecastResponse{
		Network: network, Year: year, Quarter: quarter,
		Months: rows, Brands: brandTotals, Totals: totals,
	}
}

// brandHasOverride — в квартале есть месяц с введённой руками суммой инвестиций.
// Такую сумму порог не гасит, поэтому свод обязан донести признак до строки плана.
func brandHasOverride(rows []models.NetworkForecastMonth, brand string) bool {
	for _, row := range rows {
		if row.BrandAS == brand && row.SKU == nil && row.InvestmentsSource == "override" {
			return true
		}
	}
	return false
}

// clonePlans — копия строк для расчёта, который не должен трогать вход.
func clonePlans(plans []models.NetworkPlan) []models.NetworkPlan {
	result := make([]models.NetworkPlan, len(plans))
	copy(result, plans)
	return result
}

// applyForecastInvestmentRule приводит вкладку «Прогноз» к общему правилу:
// прогнозные инвестиции бренда, не закрывшего план своей области, обнуляются.
//
// Гасится весь квартал бренда целиком, а не отдельные месяцы. Порог — величина
// квартальная (а с правилом зачёта и шире), помесячной доли у него нет, и
// обнуление части месяцев развело бы сумму месяцев с итогом бренда.
func applyForecastInvestmentRule(
	plans []models.NetworkPlan,
	periods []models.NetworkPeriod,
	groups []models.NetworkPeriodGroup,
	year, quarter int,
	rows []models.NetworkForecastMonth,
	brandTotals []models.NetworkForecastBrandTotals,
) {
	// Свежие EAC и факт этого квартала кладутся в копию строк плана: правило
	// должно мерить то, что показано на экране, а не то, что успело сохраниться.
	fresh := clonePlans(plans)
	byBrand := make(map[string]models.NetworkForecastBrandTotals, len(brandTotals))
	for _, brand := range brandTotals {
		byBrand[brand.BrandAS] = brand
	}
	for i := range fresh {
		row := &fresh[i]
		if row.Quarter != quarter || row.BrandAS == nil {
			continue
		}
		total, ok := byBrand[*row.BrandAS]
		if !ok {
			continue
		}
		row.FactRub = rollupValue(total.FactRub)
		row.ForecastRub = rollupValue(total.EACRub)
	}

	calculated, _ := BuildNetworkPlanCalculations(fresh, periods, groups)
	earned := make(map[string]bool, len(brandTotals))
	for _, row := range calculated {
		if row.Quarter != quarter || row.BrandAS == nil {
			continue
		}
		earned[*row.BrandAS] = row.ForecastInvestmentsEarned
	}

	zero := 0.0
	for i := range rows {
		row := &rows[i]
		if row.EACInvestmentsRub == nil || earned[row.BrandAS] {
			continue
		}
		// Введённое человеком переопределение порогом не отменяется.
		if row.InvestmentsSource == "override" {
			continue
		}
		row.EACInvestmentsRub = &zero
		row.InvestmentsSource = "unearned"
	}
	for i := range brandTotals {
		total := &brandTotals[i]
		if earned[total.BrandAS] {
			continue
		}
		total.EACInvestmentsRub = 0
		for _, row := range rows {
			if row.BrandAS == total.BrandAS && row.SKU == nil && row.InvestmentsSource == "override" &&
				row.EACInvestmentsRub != nil {
				total.EACInvestmentsRub = round2(total.EACInvestmentsRub + *row.EACInvestmentsRub)
			}
		}
		total.InvestmentVarianceRub = round2(total.EACInvestmentsRub - total.PlanInvestmentsRub)
	}
}

// ─── Свод помесячного слоя в квартальную сетку ─────────────────────────────

// Строки одного квартала. BuildNetworkForecast читает
// только свои месяцы, но состав брендов и SKU он собирает по всему, что ему
// передали, — поэтому отбор делается до вызова, а не внутри него.
func forecastLinesOfQuarter(lines []models.NetworkForecastLine, quarter int) []models.NetworkForecastLine {
	monthFrom := (quarter-1)*3 + 1
	result := make([]models.NetworkForecastLine, 0, len(lines))
	for _, line := range lines {
		if line.Month >= monthFrom && line.Month <= monthFrom+2 {
			result = append(result, line)
		}
	}
	return result
}

func promoIndicatorsOfQuarter(rows []models.NetworkPromoIndicator, quarter int) []models.NetworkPromoIndicator {
	monthFrom := (quarter-1)*3 + 1
	result := make([]models.NetworkPromoIndicator, 0, len(rows))
	for _, row := range rows {
		if row.Month >= monthFrom && row.Month <= monthFrom+2 {
			result = append(result, row)
		}
	}
	return result
}

// rollupValue переводит квартальный итог в поле строки плана. Ноль означает
// отсутствие величины: итог не различает «не отгружали» и «данных ещё нет», а
// прочерк в не наступившем квартале честнее объявленного нуля.
func rollupValue(value float64) *float64 {
	if value == 0 {
		return nil
	}
	result := value
	return &result
}

// ApplyForecastRollup подставляет в квартальные строки плана факт и EAC,
// посчитанные из помесячного слоя тем же кодом, что и вкладка «Прогноз».
//
// Колонки fact_rub и forecast_rub в tbl_NetworkPlans остаются денормализованным
// зеркалом для загрузчика отгрузок и внешних потребителей, но источником истины
// при чтении больше не являются. Наполнялись они только побочным эффектом
// записи — загрузкой факта и сохранением прогноза, — и потому вкладка «План и
// факт» показывала нули там, где факт и прогноз давно есть помесячно, а первое
// же сохранение прогноза по одному бренду проявляло EAC сразу по всем брендам
// квартала. Считать здесь — единственный способ свести «План и факт» с
// «Прогнозом» и витриной до копейки в любой момент, без загрузок и сохранений.
//
// Строки плана не создаются: бренд, по которому идут отгрузки, но плана в году
// нет, в сетке не появится — как и до этой правки.
func ApplyForecastRollup(
	network models.Network,
	year int,
	plans []models.NetworkPlan,
	periods []models.NetworkPeriod,
	facts []models.NetworkMonthlyFact,
	forecasts []models.NetworkForecastLine,
	promos []models.NetworkPromoIndicator,
	prices []models.NetworkContractPrice,
	groups []models.NetworkPeriodGroup,
	now time.Time,
) []models.NetworkPlan {
	for quarter := 1; quarter <= 4; quarter++ {
		// Кварталы считаются по возрастанию и правят только свои строки, а
		// BuildNetworkForecast берёт из плана объём и процент инвестиций —
		// то, чего этот свод не трогает. Порядок кварталов на результат не влияет.
		response := BuildNetworkForecast(
			network, year, quarter, plans, periods, facts,
			forecastLinesOfQuarter(forecasts, quarter),
			promoIndicatorsOfQuarter(promos, quarter),
			prices, groups, now,
		)
		byBrand := make(map[string]models.NetworkForecastBrandTotals, len(response.Brands))
		for _, brand := range response.Brands {
			byBrand[brand.BrandAS] = brand
		}

		for i := range plans {
			plan := &plans[i]
			// Строка пула брендом не ведётся: помесячного факта у неё нет, а её
			// доля собирается из брендов пула в итогах квартала.
			if plan.Quarter != quarter || plan.BrandAS == nil {
				continue
			}
			total, ok := byBrand[*plan.BrandAS]
			if !ok {
				continue
			}
			plan.FactRub = rollupValue(total.FactRub)
			plan.ForecastRub = rollupValue(total.EACRub)
			plan.PaidInvestmentsRub = rollupValue(total.FactInvestmentsRub)
			plan.FactInvestmentsRub = rollupValue(total.FactInvestmentsRub)
			plan.ForecastInvestmentsRub = rollupValue(total.EACInvestmentsRub)
			plan.ForecastInvestmentsOverridden = brandHasOverride(response.Months, *plan.BrandAS)
		}
	}
	return plans
}
