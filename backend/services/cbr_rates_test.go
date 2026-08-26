package services

import (
	"strings"
	"testing"
)

func TestParseEURMonthlyRates(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="windows-1251"?>
		<ValCurs><Record Date="10.01.2026"><Nominal>1</Nominal><Value>90,0000</Value></Record>
		<Record Date="20.01.2026"><Nominal>1</Nominal><Value>92,0000</Value></Record>
		<Record Date="05.02.2026"><Nominal>10</Nominal><Value>930,0000</Value></Record></ValCurs>`
	rates, err := parseEURMonthlyRates(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("parseEURMonthlyRates() error = %v", err)
	}
	if rates[1] != 91 || rates[2] != 93 {
		t.Fatalf("rates = %#v, want January 91 and February 93", rates)
	}
}

func TestParseEURMonthlyRatesEmpty(t *testing.T) {
	if _, err := parseEURMonthlyRates(strings.NewReader(`<ValCurs></ValCurs>`)); err == nil {
		t.Fatal("parseEURMonthlyRates() accepted a response without quotes")
	}
}

func TestFillMissingMonthsCarriesLastRate(t *testing.T) {
	// Год ещё не закрыт: котировок после августа нет, но суммы за эти месяцы
	// в витрине есть, и пересчёт в евро обязан остаться возможным.
	filled := fillMissingMonths(map[int]float64{1: 90, 2: 92, 8: 96})
	for month := 1; month <= 12; month++ {
		if filled[month] <= 0 {
			t.Fatalf("месяц %d остался без курса: %#v", month, filled)
		}
	}
	if filled[2] != 92 {
		t.Fatalf("filled[2] = %v, опубликованный курс изменён", filled[2])
	}
	if filled[7] != 92 {
		t.Fatalf("filled[7] = %v, ожидался перенос курса за февраль", filled[7])
	}
	if filled[12] != 96 {
		t.Fatalf("filled[12] = %v, ожидался перенос курса за август", filled[12])
	}
}

func TestFillMissingMonthsBackfillsLeadingGap(t *testing.T) {
	filled := fillMissingMonths(map[int]float64{5: 95})
	if filled[1] != 95 || filled[4] != 95 || filled[12] != 95 {
		t.Fatalf("filled = %#v, ожидался единый курс за весь год", filled)
	}
}

func TestFillMissingMonthsWithoutRatesStaysEmpty(t *testing.T) {
	if filled := fillMissingMonths(map[int]float64{}); len(filled) != 0 {
		t.Fatalf("filled = %#v, ожидалась пустая карта", filled)
	}
}
