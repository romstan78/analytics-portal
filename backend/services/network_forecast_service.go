package services

import (
	"fmt"
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

func allocationForPlan(plan models.NetworkPlan) [3]float64 {
	if plan.PlanRub == nil {
		return [3]float64{}
	}
	first := round2(*plan.PlanRub * plan.Month1Pct / 100)
	second := round2(*plan.PlanRub * plan.Month2Pct / 100)
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
	now time.Time,
) models.NetworkForecastResponse {
	monthFrom := (quarter-1)*3 + 1
	monthTo := monthFrom + 2
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
		planMonths := allocationForPlan(plan)
		investmentMonths := investmentAllocationForPlan(plan, planMonths)
		brandTotal := models.NetworkForecastBrandTotals{BrandAS: brand}

		for month := monthFrom; month <= monthTo; month++ {
			index := month - monthFrom
			key := forecastMonthKey(year, month, brand)
			fact := brandFacts[key]
			promo := promoByKey[key]
			line, hasLine := forecastByBrand[key]

			// SKU-детализация дополняет отсутствующую официальную метрику бренда.
			// Это важно после очистки только рублей или только упаковок: сохранённая
			// вторая метрика продолжает участвовать в прогнозе и пересчёте по цене.
			var skuRub, skuUnits, skuInvest *float64
			for sku := range skusByBrand[brand] {
				skuLine, ok := forecastBySKU[forecastSKUKey(year, month, brand, sku)]
				if !ok {
					continue
				}
				rub := skuLine.ForecastRub
				if rub == nil && skuLine.ForecastUnits != nil {
					if price := effectiveContractPrice(prices, sku, year, month); price != nil {
						value := round2(*skuLine.ForecastUnits * *price)
						rub = &value
					}
				}
				addPtrValue(&skuRub, rub)
				addPtrValue(&skuUnits, skuLine.ForecastUnits)
				addPtrValue(&skuInvest, skuLine.ForecastInvestmentsRub)
			}
			if !hasLine || line.ForecastRub == nil {
				line.ForecastRub = skuRub
			}
			if !hasLine || line.ForecastUnits == nil {
				line.ForecastUnits = skuUnits
			}
			if !hasLine || line.ForecastInvestmentsRub == nil {
				line.ForecastInvestmentsRub = skuInvest
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
			eacInvestments := valueForEAC(closed, fact.investments, line.ForecastInvestmentsRub, nil)
			if eacInvestments == nil && investmentMonths[index] != 0 {
				value := investmentMonths[index]
				eacInvestments = &value
			}

			planRub := planMonths[index]
			planInvestment := investmentMonths[index]
			row := models.NetworkForecastMonth{
				Year: year, Quarter: quarter, Month: month, BrandAS: brand,
				PlanRub: &planRub, PlanInvestmentsRub: &planInvestment,
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
				IsClosed:       closed, IsCurrent: isCurrentForecastMonth(year, month, now),
				UpdatedAt: line.UpdatedAt,
			}
			rows = append(rows, row)

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

		// SKU-строки идут после официальной строки бренда и используются в drawer.
		skuNames := make([]string, 0, len(skusByBrand[brand]))
		for sku := range skusByBrand[brand] {
			skuNames = append(skuNames, sku)
		}
		sort.Strings(skuNames)
		for _, sku := range skuNames {
			for month := monthFrom; month <= monthTo; month++ {
				fact := skuFacts[forecastSKUKey(year, month, brand, sku)]
				line := forecastBySKU[forecastSKUKey(year, month, brand, sku)]
				rub := line.ForecastRub
				contractPrice := effectiveContractPrice(prices, sku, year, month)
				systemUnits, systemConfidence := recommendedSKUUnits(skuFacts, year, month, brand, sku)
				if line.Confidence != nil {
					systemConfidence = line.Confidence
				}
				if rub == nil && line.ForecastUnits != nil {
					if price := contractPrice; price != nil {
						value := round2(*line.ForecastUnits * *price)
						rub = &value
					}
				}
				closed := isClosedForecastMonth(year, month, now)
				rows = append(rows, models.NetworkForecastMonth{
					Year: year, Quarter: quarter, Month: month, BrandAS: brand, SKU: &sku,
					ContractPrice: contractPrice,
					FactRub:       fact.rub, FactUnits: fact.units, FactInvestmentsRub: fact.investments,
					ForecastRub: rub, ForecastUnits: line.ForecastUnits,
					ForecastInvestmentsRub: line.ForecastInvestmentsRub,
					SystemForecastUnits:    systemUnits,
					EACRub:                 valueForEAC(closed, fact.rub, rub, nil),
					EACUnits:               valueForEAC(closed, fact.units, line.ForecastUnits, systemUnits),
					EACInvestmentsRub:      valueForEAC(closed, fact.investments, line.ForecastInvestmentsRub, nil),
					Confidence:             systemConfidence, AdjustmentReason: line.AdjustmentReason,
					IsClosed: closed, IsCurrent: isCurrentForecastMonth(year, month, now),
					UpdatedAt: line.UpdatedAt,
				})
			}
		}
	}

	planTotals := CalculateNetworkTotals(plans, periods)[quarter-1]
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
