package services

import (
	"reflect"
	"testing"

	"backend/repository"
)

func TestBuildSalesPivotFromRowsBuildsHierarchyAndEarlierYearFirst(t *testing.T) {
	req := SalesPivotRequest{Unit: "руб", Granularity: "year", Channel: "PURE", Segments: []string{"A", "B"}}
	if err := req.Normalize(); err != nil {
		t.Fatal(err)
	}
	rows := []repository.SalesPivotLeafRow{
		{Channel: "PURE", Segment: "A", Network: "Сеть 1", Product: "SKU 1", Year: 2025, Month: 1, Value: 100},
		{Channel: "PURE", Segment: "A", Network: "Сеть 1", Product: "SKU 1", Year: 2026, Month: 1, Value: 140},
		{Channel: "PURE", Segment: "A", Network: "Сеть 1", Product: "SKU 2", Year: 2026, Month: 1, Value: 60},
	}

	response, err := buildSalesPivotFromRows(req, 2026, rows, nil)
	if err != nil {
		t.Fatal(err)
	}
	periodKeys := make([]string, 0, len(response.Periods))
	for _, period := range response.Periods {
		periodKeys = append(periodKeys, period.Key)
	}
	if want := []string{"2025-total", "2026-total"}; !reflect.DeepEqual(periodKeys, want) {
		t.Fatalf("periods = %v, want %v", periodKeys, want)
	}
	if response.LeafRows != 2 {
		t.Fatalf("LeafRows = %d, want 2", response.LeafRows)
	}
	if got := response.Totals[response.PreviousTotalKey]; got != 100 {
		t.Fatalf("previous total = %v, want 100", got)
	}
	if got := response.Totals[response.CurrentTotalKey]; got != 200 {
		t.Fatalf("current total = %v, want 200", got)
	}
	if len(response.Rows) != 1 || response.Rows[0].Level != "channel" || response.Rows[0].Name != "PURE" {
		t.Fatalf("unexpected channel hierarchy: %#v", response.Rows)
	}
	network := response.Rows[0].Children[0].Children[0]
	if network.Level != "network" || len(network.Children) != 2 {
		t.Fatalf("unexpected network hierarchy: %#v", network)
	}
}

func TestBuildPivotPeriodsUsesSelectedQuarters(t *testing.T) {
	req := SalesPivotRequest{Unit: "руб", Granularity: "quarter", Quarters: []string{"3", "1", "3"}, Segments: []string{"A"}}
	if err := req.Normalize(); err != nil {
		t.Fatal(err)
	}
	periods, previousKey, currentKey := buildPivotPeriods(req, 2026)
	keys := make([]string, 0, len(periods))
	for _, period := range periods {
		keys = append(keys, period.Key)
	}
	want := []string{"2025-q1", "2025-q3", "2025-total", "2026-q1", "2026-q3", "2026-total"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if previousKey != "2025-total" || currentKey != "2026-total" {
		t.Fatalf("comparison keys = %q/%q", previousKey, currentKey)
	}
}

func TestSalesPivotRequestRejectsUnknownGranularity(t *testing.T) {
	req := SalesPivotRequest{Unit: "руб", Granularity: "week"}
	if err := req.Normalize(); err == nil {
		t.Fatal("Normalize() must reject unknown granularity")
	}
}
