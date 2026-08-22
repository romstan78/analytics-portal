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
