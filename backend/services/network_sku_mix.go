package services

import (
	"sort"

	"backend/models"
	"backend/repository"
)

// ─── Микс SKU бренда ────────────────────────────────────────────────────────
//
// Диалог ступени раскладывает план бренда по SKU. Доли берутся из той же
// эвристики, что разлагает прогноз бренда без детализации (skuMixShares):
// аналогичный месяц прошлого года, затем среднее последних месяцев, затем
// поровну. Три месяца квартала усредняются с равным весом — план квартальный,
// и сезонность внутри квартала здесь роли не играет.

// SKUMixShares — доли SKU бренда за квартал по фактам и ценам контракта.
func SKUMixShares(
	year, quarter int,
	brand string,
	facts []models.NetworkMonthlyFact,
	prices []models.NetworkContractPrice,
) []models.NetworkSKUMixShare {
	_, skuFacts := aggregateFacts(facts)
	known := map[string]bool{}
	for _, price := range prices {
		if price.BrandAS == brand {
			known[price.SKU] = true
		}
	}
	for _, fact := range facts {
		if fact.BrandAS == brand && fact.SKU != nil {
			known[*fact.SKU] = true
		}
	}
	skus := make([]string, 0, len(known))
	for sku := range known {
		skus = append(skus, sku)
	}
	sort.Strings(skus)
	if len(skus) == 0 {
		return []models.NetworkSKUMixShare{}
	}

	sum := make(map[string]float64, len(skus))
	monthFrom := (quarter-1)*3 + 1
	for month := monthFrom; month < monthFrom+3; month++ {
		for sku, share := range skuMixShares(skuFacts, year, month, brand, skus) {
			sum[sku] += share / 3
		}
	}
	result := make([]models.NetworkSKUMixShare, 0, len(skus))
	for _, sku := range skus {
		result = append(result, models.NetworkSKUMixShare{SKU: sku, Share: sum[sku]})
	}
	return result
}

// LoadNetworkSKUMix читает вход и считает микс бренда за квартал.
func LoadNetworkSKUMix(networkID, year, quarter int, brand string) (models.NetworkSKUMixResponse, error) {
	facts, err := repository.GetNetworkMonthlyFacts(networkID, year-1, year)
	if err != nil {
		return models.NetworkSKUMixResponse{}, err
	}
	prices, err := repository.GetNetworkContractPrices(networkID, year)
	if err != nil {
		return models.NetworkSKUMixResponse{}, err
	}
	return models.NetworkSKUMixResponse{
		BrandAS: brand, Year: year, Quarter: quarter,
		Data: SKUMixShares(year, quarter, brand, facts, prices),
	}, nil
}
