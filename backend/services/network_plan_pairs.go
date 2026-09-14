package services

import (
	"sort"

	"backend/models"
	"backend/repository"
)

// ─── Пара «рубли / упаковки» в плане ────────────────────────────────────────
//
// План вводится в одной единице — по режиму бренда (entry_unit), — а вторая
// считается по ценам контракта и хранится рядом, тем же обещанием, что и пара
// прогноза (network_forecast_pairs.go): колонки читают без пересчёта по прайсу.
//
// Правило считается в рублях: пороги, крышки и объёмы контракта денежные.
// Упаковки — единица ввода. Цена берётся помесячно: квартальный план уже
// раскладывается по месяцам схемой сети, и упаковки месяца × цена этого месяца
// дают рубли квартала. При неизменной цене это «упаковки × цена», при смене
// цены внутри квартала — результат, совпадающий с помесячной картиной «Прогноза».
//
// Без цены пары нет: SKU с планом в упаковках и без цены контракта не может
// участвовать в рублёвом правиле, и выдумывать курс пересчёта незачем.

// monthlyPrice — цена, действующая в месяце квартала.
type monthlyPrice func(month int) *float64

// completePair дополняет пару по введённой метрике. Введённая — та, что задана
// режимом бренда; если её нет, а вторая есть, вторая считается введённой:
// клиент, который про пару не знает, не должен стирать план, сохранённый в
// другой единице. Считать нечем (нет цены) — вторая метрика остаётся, какой
// была: пустой у новой строки, прежней у сохранённой. Стирать сохранённое
// из-за отсутствия прайса значило бы терять данные, заведённые без него.
func completePair(
	rub, units *float64,
	entryUnit string,
	quarter int,
	distribution [3]float64,
	price monthlyPrice,
) (*float64, *float64) {
	unitsEntered := entryUnit == "units" && units != nil || entryUnit != "units" && rub == nil && units != nil
	if unitsEntered {
		if computed := unitsToRub(*units, quarter, distribution, price); computed != nil {
			return computed, units
		}
		return rub, units
	}
	if rub == nil {
		return nil, units
	}
	if computed := rubToUnits(*rub, quarter, distribution, price); computed != nil {
		return rub, computed
	}
	return rub, units
}

// allocateByDistribution раскладывает квартальную величину по трём месяцам
// так же, как allocationForPlan: два первых — по долям, третий — остаток.
func allocateByDistribution(total float64, distribution [3]float64) [3]float64 {
	first := round2(total * distribution[0] / 100)
	second := round2(total * distribution[1] / 100)
	return [3]float64{first, second, round2(total - first - second)}
}

func unitsToRub(units float64, quarter int, distribution [3]float64, price monthlyPrice) *float64 {
	monthFrom := (quarter-1)*3 + 1
	parts := allocateByDistribution(units, distribution)
	sum := 0.0
	for i, part := range parts {
		p := price(monthFrom + i)
		if p == nil || *p <= 0 {
			return nil
		}
		sum = round2(sum + round2(part**p))
	}
	return &sum
}

func rubToUnits(rub float64, quarter int, distribution [3]float64, price monthlyPrice) *float64 {
	monthFrom := (quarter-1)*3 + 1
	parts := allocateByDistribution(rub, distribution)
	sum := 0.0
	for i, part := range parts {
		p := price(monthFrom + i)
		if p == nil || *p <= 0 {
			return nil
		}
		sum = round2(sum + round2(part / *p))
	}
	return &sum
}

// CompleteNetworkPlanPairs дополняет пару у строк плана, их ступеней и SKU.
// Строка бренда и её ступени считаются по цене бренда, взвешенной по миксу
// SKU (как прогноз бренда без детализации); SKU — по своей цене контракта.
// Строка пула ведётся в рублях и пары не имеет.
func CompleteNetworkPlanPairs(
	network models.Network,
	year int,
	plans []models.NetworkPlan,
	prices []models.NetworkContractPrice,
	facts []models.NetworkMonthlyFact,
) []models.NetworkPlan {
	_, skuFacts := aggregateFacts(facts)
	distribution := networkMonthlyDistribution(network)

	skusByBrand := map[string][]string{}
	for _, price := range prices {
		skusByBrand[price.BrandAS] = append(skusByBrand[price.BrandAS], price.SKU)
	}
	for brand, skus := range skusByBrand {
		sort.Strings(skus)
		unique := skus[:0]
		for i, sku := range skus {
			if i == 0 || sku != skus[i-1] {
				unique = append(unique, sku)
			}
		}
		skusByBrand[brand] = unique
	}

	for i := range plans {
		plan := &plans[i]
		if plan.BrandAS == nil {
			continue
		}
		brand := *plan.BrandAS
		_, entryUnit := entryModeOfPlan(*plan, network)
		skuNames := skusByBrand[brand]
		brandPrice := func(month int) *float64 {
			shares := skuMixShares(skuFacts, year, month, brand, skuNames)
			return brandWeightedPrice(prices, brand, year, month, skuNames, shares)
		}

		plan.PlanRub, plan.PlanUnits = completePair(plan.PlanRub, plan.PlanUnits, entryUnit, plan.Quarter, distribution, brandPrice)
		for s := range plan.Scales {
			scale := &plan.Scales[s]
			if scale.ScaleNo == 1 {
				// Ступень 1 — зеркало строки.
				scale.PlanRub, scale.PlanUnits = plan.PlanRub, plan.PlanUnits
			} else {
				scale.PlanRub, scale.PlanUnits = completePair(scale.PlanRub, scale.PlanUnits, entryUnit, plan.Quarter, distribution, brandPrice)
			}
			for k := range scale.SKUs {
				sku := &scale.SKUs[k]
				skuPrice := func(month int) *float64 { return effectiveContractPrice(prices, sku.SKU, year, month) }
				sku.PlanRub, sku.PlanUnits = completePair(sku.PlanRub, sku.PlanUnits, entryUnit, plan.Quarter, distribution, skuPrice)
			}
		}
	}
	return plans
}

// RebuildNetworkPlanPairsYear пересчитывает и закрепляет пару плана сети за
// год. Нужен после сохранения плана и после правки цен: парная величина
// считается по прайсу и вслед за ним обязана меняться.
func RebuildNetworkPlanPairsYear(networkID, year int) (int64, error) {
	network, err := repository.GetNetworkByID(networkID)
	if err != nil {
		return 0, err
	}
	plans, err := repository.GetNetworkPlans(networkID, year)
	if err != nil {
		return 0, err
	}
	prices, err := repository.GetNetworkContractPrices(networkID, year)
	if err != nil {
		return 0, err
	}
	facts, err := repository.GetNetworkMonthlyFacts(networkID, year-1, year)
	if err != nil {
		return 0, err
	}
	completed := CompleteNetworkPlanPairs(network, year, plans, prices, facts)
	return repository.SaveNetworkPlanPairs(completed)
}
