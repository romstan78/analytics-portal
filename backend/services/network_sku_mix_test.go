package services

import (
	"testing"

	"backend/models"
)

func TestSKUMixSharesAveragesQuarterAndFallsBackToEqual(t *testing.T) {
	brand := "Альфа"
	facts := []models.NetworkMonthlyFact{
		// Аналогичный квартал прошлого года: A вдвое больше B во всех месяцах.
		{Year: 2025, Month: 4, BrandAS: brand, SKU: models.PtrString("A"), FactUnits: models.PtrFloat(200)},
		{Year: 2025, Month: 4, BrandAS: brand, SKU: models.PtrString("B"), FactUnits: models.PtrFloat(100)},
		{Year: 2025, Month: 5, BrandAS: brand, SKU: models.PtrString("A"), FactUnits: models.PtrFloat(200)},
		{Year: 2025, Month: 5, BrandAS: brand, SKU: models.PtrString("B"), FactUnits: models.PtrFloat(100)},
		{Year: 2025, Month: 6, BrandAS: brand, SKU: models.PtrString("A"), FactUnits: models.PtrFloat(200)},
		{Year: 2025, Month: 6, BrandAS: brand, SKU: models.PtrString("B"), FactUnits: models.PtrFloat(100)},
	}
	prices := []models.NetworkContractPrice{{BrandAS: brand, SKU: "C", ContractPrice: 10, ValidFrom: "2026-01-01", ValidTo: "2026-12-31"}}

	shares := SKUMixShares(2026, 2, brand, facts, prices)
	bySKU := map[string]float64{}
	sum := 0.0
	for _, s := range shares {
		bySKU[s.SKU] = s.Share
		sum += s.Share
	}
	// C есть только в ценах: истории нет, веса нет — доля ноль; A и B делят
	// микс 2:1. Сумма долей — единица.
	if round2(bySKU["A"]) != 0.67 || round2(bySKU["B"]) != 0.33 || bySKU["C"] != 0 {
		t.Errorf("доли = %+v, ожидалось A 0,67 / B 0,33 / C 0", bySKU)
	}
	if round2(sum) != 1 {
		t.Errorf("сумма долей = %v, ожидалась 1", sum)
	}

	// Без истории — поровну между известными SKU.
	equal := SKUMixShares(2026, 2, brand, nil, prices)
	if len(equal) != 1 || equal[0].SKU != "C" || round2(equal[0].Share) != 1 {
		t.Errorf("без истории единственный SKU получает всё: %+v", equal)
	}
	if got := SKUMixShares(2026, 2, "Нет такого", nil, nil); len(got) != 0 {
		t.Errorf("бренд без SKU — пустой список, получено %+v", got)
	}
}
