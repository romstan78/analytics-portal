package handlers

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSalesWhereQuarters(t *testing.T) {
	where, args := buildSalesWhere(salesFilter{
		YearFromStr: "2026",
		Quarters:    []string{"2", "4", "0", "bad"},
	})

	if !strings.Contains(where, "((n.[month] - 1) / 3) + 1 IN (?,?)") {
		t.Fatalf("quarter condition is missing from %q", where)
	}
	wantArgs := []interface{}{2026, 2, 4}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildSalesWhereExactFocus(t *testing.T) {
	where, args := buildSalesWhere(salesFilter{
		ProductExact: "SKU 1",
		NetworkExact: "Network 1",
	})

	if !strings.Contains(where, "n.productName = ?") || !strings.Contains(where, "n.networkName = ?") {
		t.Fatalf("focus conditions are missing from %q", where)
	}
	wantArgs := []interface{}{"SKU 1", "Network 1"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestUniqueNonEmptyStrings(t *testing.T) {
	got := uniqueNonEmptyStrings([]string{" A ", "", "B", "A", "C"}, 2)
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueNonEmptyStrings() = %#v, want %#v", got, want)
	}
}

func TestBuildDimensionViews(t *testing.T) {
	drivers, ranking := buildDimensionViews([]salesDimensionValue{
		{Name: "Сеть A", Current: 120, Previous: 100},
		{Name: "Сеть B", Current: 80, Previous: 160},
		{Name: "Сеть C", Current: 100, Previous: 0},
	})

	if len(drivers) != 3 || drivers[0].Name != "Сеть C" && drivers[0].Name != "Сеть B" {
		t.Fatalf("drivers are not ordered by absolute contribution: %#v", drivers)
	}
	if len(ranking) != 3 || ranking[0].Name != "Сеть A" || ranking[0].Rank != 1 {
		t.Fatalf("unexpected ranking: %#v", ranking)
	}
	if ranking[0].Share != 40 {
		t.Fatalf("share = %v, want 40", ranking[0].Share)
	}
	if ranking[1].Name != "Сеть C" || ranking[1].YoYPercent != nil {
		t.Fatalf("new network comparison is incorrect: %#v", ranking[1])
	}
}

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

func TestSalesExportMaxRows(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  int
	}{
		{"по умолчанию", "", defaultSalesExportMaxRows},
		{"значение из окружения", "500", 500},
		{"пробелы обрезаются", "  750  ", 750},
		{"ноль игнорируется", "0", defaultSalesExportMaxRows},
		{"отрицательное игнорируется", "-10", defaultSalesExportMaxRows},
		{"мусор игнорируется", "много", defaultSalesExportMaxRows},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SALES_EXPORT_MAX_ROWS", tc.value)
			if got := salesExportMaxRows(); got != tc.want {
				t.Fatalf("salesExportMaxRows() = %d, want %d", got, tc.want)
			}
		})
	}
}
