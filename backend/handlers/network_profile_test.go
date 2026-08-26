package handlers

import (
	"testing"

	"backend/models"
)

func TestNetworkProfileSettingsUsesFallbackAndValidatesDistribution(t *testing.T) {
	fallback := models.Network{
		Month1Pct: 25, Month2Pct: 35, Month3Pct: 40,
		HasAnnualInvestmentCumulative: true,
	}
	profile, err := networkProfileSettings(networkInput{}, fallback)
	if err != nil || profile.Month1Pct != 25 || profile.Month2Pct != 35 ||
		profile.Month3Pct != 40 || !profile.HasAnnualInvestmentCumulative {
		t.Fatalf("профиль по умолчанию = %#v err=%v", profile, err)
	}
	// Сеть, заведённая до появления режима ведения, открывается в рублях по бренду.
	if profile.DefaultEntryLevel != "brand" || profile.DefaultEntryUnit != "rub" {
		t.Fatalf("режим по умолчанию = %q/%q, ожидалось brand/rub",
			profile.DefaultEntryLevel, profile.DefaultEntryUnit)
	}

	invalid := 20.0
	if _, err := networkProfileSettings(networkInput{Month3Pct: &invalid}, fallback); err == nil {
		t.Fatal("профиль с суммой не 100% должен отклоняться")
	}
}

func TestNetworkProfileSettingsKeepsAndValidatesEntryMode(t *testing.T) {
	fallback := models.Network{
		Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
		DefaultEntryLevel: "sku", DefaultEntryUnit: "units",
	}
	// Запрос без режима не переключает сеть на чужой способ ведения.
	profile, err := networkProfileSettings(networkInput{}, fallback)
	if err != nil || profile.DefaultEntryLevel != "sku" || profile.DefaultEntryUnit != "units" {
		t.Fatalf("режим сети = %q/%q err=%v, ожидалось sku/units",
			profile.DefaultEntryLevel, profile.DefaultEntryUnit, err)
	}

	level, unit := "brand", "rub"
	profile, err = networkProfileSettings(networkInput{
		DefaultEntryLevel: &level, DefaultEntryUnit: &unit,
	}, fallback)
	if err != nil || profile.DefaultEntryLevel != "brand" || profile.DefaultEntryUnit != "rub" {
		t.Fatalf("режим из запроса = %q/%q err=%v, ожидалось brand/rub",
			profile.DefaultEntryLevel, profile.DefaultEntryUnit, err)
	}

	wrong := "package"
	if _, err := networkProfileSettings(networkInput{DefaultEntryUnit: &wrong}, fallback); err == nil {
		t.Fatal("неизвестная единица ведения должна отклоняться")
	}
}

func TestNetworkVATSettingsUsesFallbackAndValidatesRate(t *testing.T) {
	fallback := models.Network{VATIncluded: true, VATRate: 20}
	included, rate, err := networkVATSettings(networkInput{}, fallback)
	if err != nil || !included || rate != 20 {
		t.Fatalf("НДС профиля по умолчанию = included=%v rate=%.2f err=%v", included, rate, err)
	}

	withoutVAT := false
	zeroRate := 0.0
	included, rate, err = networkVATSettings(networkInput{
		VATIncluded: &withoutVAT,
		VATRate:     &zeroRate,
	}, fallback)
	if err != nil || included || rate != 0 {
		t.Fatalf("профиль без НДС = included=%v rate=%.2f err=%v", included, rate, err)
	}

	invalidRate := 100.0
	if _, _, err := networkVATSettings(networkInput{VATRate: &invalidRate}, fallback); err == nil {
		t.Fatal("ставка НДС 100% должна отклоняться")
	}
}

func TestNetworkPeriodsWithDefaultsPreservesQuarterVAT(t *testing.T) {
	network := models.Network{ID: 7, VATIncluded: true, VATRate: 22}
	persisted := []models.NetworkPeriod{{
		ID: 13, NetworkID: 7, Year: 2026, Quarter: 2,
		VATIncluded: false, VATRate: 10, UpdatedAt: "version",
	}}

	periods := networkPeriodsWithDefaults(network, 2027, persisted)
	if len(periods) != 4 {
		t.Fatalf("кварталов = %d, ожидалось 4", len(periods))
	}
	for index, period := range periods {
		if period.NetworkID != 7 || period.Year != 2027 || period.Quarter != index+1 {
			t.Fatalf("неверные реквизиты Q%d: %#v", index+1, period)
		}
		if period.Quarter != 2 && (!period.VATIncluded || period.VATRate != 22) {
			t.Fatalf("новый Q%d не получил значения по умолчанию: %#v", index+1, period)
		}
	}
	if periods[1].ID != 13 || periods[1].UpdatedAt != "version" || periods[1].VATIncluded || periods[1].VATRate != 10 {
		t.Fatalf("настройки сохранённого квартала потеряны: %#v", periods[1])
	}
}

func TestNetworkPeriodsFromInputAppliesVATPerQuarter(t *testing.T) {
	network := models.Network{ID: 7, VATIncluded: true, VATRate: 20}
	periods, err := networkPeriodsFromInput(network, 2027, nil, []networkPeriodInput{
		{Quarter: 1, VATIncluded: false, VATRate: 10},
		{Quarter: 2, VATIncluded: true, VATRate: 18},
		{Quarter: 3, VATIncluded: true, VATRate: 20},
		{Quarter: 4, VATIncluded: false, VATRate: 22},
	})
	if err != nil {
		t.Fatalf("поквартальные настройки отклонены: %v", err)
	}
	wantIncluded := []bool{false, true, true, false}
	wantRates := []float64{10, 18, 20, 22}
	for index, period := range periods {
		if period.VATIncluded != wantIncluded[index] || period.VATRate != wantRates[index] {
			t.Fatalf("неверный НДС Q%d: %#v", index+1, period)
		}
	}

	if _, err := networkPeriodsFromInput(network, 2027, nil, []networkPeriodInput{
		{Quarter: 1, VATIncluded: true, VATRate: 20},
		{Quarter: 1, VATIncluded: false, VATRate: 10},
	}); err == nil {
		t.Fatal("дублирующийся квартал должен отклоняться")
	}
	if _, err := networkPeriodsFromInput(network, 2027, nil, []networkPeriodInput{
		{Quarter: 4, VATIncluded: true, VATRate: 100},
	}); err == nil {
		t.Fatal("ставка НДС 100% должна отклоняться")
	}
}
