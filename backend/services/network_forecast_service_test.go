package services

import (
	"testing"
	"time"

	"backend/models"
)

func forecastRow(response models.NetworkForecastResponse, brand string, month int, sku *string) *models.NetworkForecastMonth {
	for index := range response.Months {
		row := &response.Months[index]
		if row.BrandAS != brand || row.Month != month {
			continue
		}
		if sku == nil && row.SKU == nil {
			return row
		}
		if sku != nil && row.SKU != nil && *row.SKU == *sku {
			return row
		}
	}
	return nil
}

func TestBuildNetworkForecastAllocatesQuarterPlanWithoutLosingKopecks(t *testing.T) {
	brand := "Альфа"
	response := BuildNetworkForecast(
		models.Network{}, 2027, 1,
		[]models.NetworkPlan{{
			Quarter: 1, BrandAS: &brand, PlanRub: models.PtrFloat(100.01),
			Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
		}}, nil, nil, nil, nil, nil,
		time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	)

	want := map[int]float64{1: 30, 2: 30, 3: 40.01}
	var total float64
	for month, expected := range want {
		row := forecastRow(response, brand, month, nil)
		if row == nil || row.PlanRub == nil || *row.PlanRub != expected {
			t.Fatalf("план месяца %d = %#v, ожидалось %.2f", month, row, expected)
		}
		total += *row.PlanRub
	}
	if round2(total) != 100.01 {
		t.Fatalf("сумма месячных планов = %.2f, ожидалось 100.01", total)
	}
}

func TestBuildNetworkForecastUsesNetworkProfileDistribution(t *testing.T) {
	brand := "Альфа"
	response := BuildNetworkForecast(
		models.Network{Month1Pct: 20, Month2Pct: 30, Month3Pct: 50}, 2027, 1,
		[]models.NetworkPlan{{
			Quarter: 1, BrandAS: &brand, PlanRub: models.PtrFloat(100),
			// Старое распределение строки больше не является источником.
			Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
		}}, nil, nil, nil, nil, nil,
		time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	)

	want := map[int]float64{1: 20, 2: 30, 3: 50}
	for month, expected := range want {
		row := forecastRow(response, brand, month, nil)
		if row == nil || row.PlanRub == nil || *row.PlanRub != expected {
			t.Fatalf("план месяца %d = %#v, ожидалось %.2f из профиля сети", month, row, expected)
		}
	}
}

func TestBuildNetworkForecastRecommendationUsesHistoryAndApprovedPromoNotPlan(t *testing.T) {
	brand := "Альфа"
	facts := []models.NetworkMonthlyFact{
		{Year: 2025, Month: 1, BrandAS: brand, FactRub: models.PtrFloat(100), FactUnits: models.PtrFloat(10)},
		{Year: 2025, Month: 10, BrandAS: brand, FactRub: models.PtrFloat(180), FactUnits: models.PtrFloat(18)},
		{Year: 2025, Month: 11, BrandAS: brand, FactRub: models.PtrFloat(200), FactUnits: models.PtrFloat(20)},
		{Year: 2025, Month: 12, BrandAS: brand, FactRub: models.PtrFloat(220), FactUnits: models.PtrFloat(22)},
	}
	response := BuildNetworkForecast(
		models.Network{}, 2026, 1,
		[]models.NetworkPlan{{
			Quarter: 1, BrandAS: &brand, PlanRub: models.PtrFloat(3000),
			Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
		}}, nil, facts, nil,
		[]models.NetworkPromoIndicator{{Year: 2026, Month: 1, BrandAS: brand, PlanUpliftRub: 20, PlanUpliftUnits: 2}},
		nil, time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
	)

	row := forecastRow(response, brand, 1, nil)
	// Среднее аналогичного месяца (100) и среднего последних трёх (200), плюс uplift 20.
	if row == nil || row.SystemForecastRub == nil || *row.SystemForecastRub != 170 {
		t.Fatalf("системная рекомендация = %#v, ожидалось 170", row)
	}
	if row.Confidence == nil || *row.Confidence != "high" {
		t.Fatalf("confidence = %#v, ожидалось high", row.Confidence)
	}
	if row.SystemForecastUnits == nil || *row.SystemForecastUnits != 17 {
		t.Fatalf("рекомендация в упаковках = %#v, ожидалось 17", row.SystemForecastUnits)
	}
}

func TestBuildNetworkForecastEACUsesFactForClosedMonthAndOfficialForCurrent(t *testing.T) {
	brand := "Альфа"
	response := BuildNetworkForecast(
		models.Network{}, 2026, 1,
		[]models.NetworkPlan{{
			Quarter: 1, BrandAS: &brand, PlanRub: models.PtrFloat(600),
			Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
			InvestmentsPct: models.PtrFloat(10),
		}}, nil,
		[]models.NetworkMonthlyFact{
			{Year: 2026, Month: 1, BrandAS: brand, FactRub: models.PtrFloat(100)},
			{Year: 2026, Month: 3, BrandAS: brand, FactRub: models.PtrFloat(50)},
		},
		[]models.NetworkForecastLine{
			{Year: 2026, Month: 1, BrandAS: brand, ForecastRub: models.PtrFloat(999)},
			{Year: 2026, Month: 3, BrandAS: brand, ForecastRub: models.PtrFloat(300), ForecastInvestmentsRub: models.PtrFloat(80)},
		}, nil, nil,
		time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
	)

	jan := forecastRow(response, brand, 1, nil)
	mar := forecastRow(response, brand, 3, nil)
	if jan == nil || jan.EACRub == nil || *jan.EACRub != 100 {
		t.Fatalf("EAC закрытого января = %#v, ожидался факт 100", jan)
	}
	if mar == nil || mar.EACRub == nil || *mar.EACRub != 300 {
		t.Fatalf("EAC текущего марта = %#v, ожидался официальный прогноз 300", mar)
	}
	if mar.EACInvestmentsRub == nil || *mar.EACInvestmentsRub != 80 {
		t.Fatalf("EAC инвестиций марта = %#v, ожидался независимый прогноз 80", mar.EACInvestmentsRub)
	}
}

func TestBuildNetworkForecastConvertsSKUUnitsByEffectiveContractPrice(t *testing.T) {
	brand, sku := "Альфа", "SKU-1"
	response := BuildNetworkForecast(
		models.Network{}, 2026, 1,
		[]models.NetworkPlan{{
			Quarter: 1, BrandAS: &brand, PlanRub: models.PtrFloat(100),
			Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
		}}, nil, nil,
		[]models.NetworkForecastLine{{
			Year: 2026, Month: 1, BrandAS: brand, SKU: &sku, ForecastUnits: models.PtrFloat(10),
		}}, nil,
		[]models.NetworkContractPrice{{
			BrandAS: brand, SKU: sku, ContractPrice: 2.5,
			ValidFrom: "2026-01-01", ValidTo: "2026-12-31",
		}},
		time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
	)

	skuRow := forecastRow(response, brand, 1, &sku)
	brandRow := forecastRow(response, brand, 1, nil)
	if skuRow == nil || skuRow.ContractPrice == nil || *skuRow.ContractPrice != 2.5 ||
		skuRow.ForecastRub == nil || *skuRow.ForecastRub != 25 {
		t.Fatalf("SKU-строка = %#v, ожидались цена 2.5 и прогноз 25", skuRow)
	}
	if brandRow == nil || brandRow.EACRub == nil || *brandRow.EACRub != 25 {
		t.Fatalf("брендовый EAC = %#v, ожидалась сумма SKU 25", brandRow)
	}
	if brandRow.EACUnits == nil || *brandRow.EACUnits != 10 || response.Totals.EACUnits != 10 {
		t.Fatalf("EAC в упаковках = %#v, итого %.2f; ожидалось 10", brandRow.EACUnits, response.Totals.EACUnits)
	}
}

func TestBuildNetworkForecastCompletesMissingOfficialMetricFromSKU(t *testing.T) {
	brand, sku := "Альфа", "SKU-1"
	response := BuildNetworkForecast(
		models.Network{}, 2026, 1,
		[]models.NetworkPlan{{
			Quarter: 1, BrandAS: &brand, PlanRub: models.PtrFloat(100),
			Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
		}}, nil, nil,
		[]models.NetworkForecastLine{
			{Year: 2026, Month: 1, BrandAS: brand, ForecastUnits: models.PtrFloat(10)},
			{Year: 2026, Month: 1, BrandAS: brand, SKU: &sku, ForecastUnits: models.PtrFloat(10)},
		}, nil,
		[]models.NetworkContractPrice{{
			BrandAS: brand, SKU: sku, ContractPrice: 2.5,
			ValidFrom: "2026-01-01", ValidTo: "2026-12-31",
		}},
		time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
	)

	brandRow := forecastRow(response, brand, 1, nil)
	if brandRow == nil || brandRow.ForecastRub == nil || *brandRow.ForecastRub != 25 {
		t.Fatalf("рублёвый прогноз бренда = %#v, ожидалось дополнение из SKU до 25", brandRow)
	}
	if brandRow.EACUnits == nil || *brandRow.EACUnits != 10 {
		t.Fatalf("EAC бренда в упаковках = %#v, ожидалось 10", brandRow.EACUnits)
	}
}
