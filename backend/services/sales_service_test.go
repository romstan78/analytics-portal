package services

import (
	"net/http"
	"reflect"
	"testing"

	"backend/models"
)

func TestUniqueNonEmptyStrings(t *testing.T) {
	got := UniqueNonEmptyStrings([]string{" A ", "", "B", "A", "C"}, 2)
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UniqueNonEmptyStrings() = %#v, want %#v", got, want)
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

func TestSalesDashboardRequestNormalize(t *testing.T) {
	req := SalesDashboardRequest{
		Segments:      []string{"", "  "},
		Unit:          "",
		FocusNetworks: []string{"A", "B", "C", "D", "E", "F"},
	}
	if err := req.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !reflect.DeepEqual(req.Segments, []string{"OLAP SS"}) {
		t.Fatalf("segments = %#v, want default OLAP SS", req.Segments)
	}
	if req.Unit != "руб" || req.DBUnit() != "руб" {
		t.Fatalf("unit = %q, DBUnit = %q", req.Unit, req.DBUnit())
	}
	if len(req.FocusNetworks) != 5 {
		t.Fatalf("focus networks = %#v, want 5 at most", req.FocusNetworks)
	}
}

// Евро считается из рублёвых строк, поэтому в БД запрос уходит в рублях.
func TestSalesDashboardRequestEuroReadsRubles(t *testing.T) {
	req := SalesDashboardRequest{Unit: "евро"}
	if err := req.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if req.DBUnit() != "руб" {
		t.Fatalf("DBUnit() = %q, want руб", req.DBUnit())
	}
}

func TestSalesDashboardRequestRejectsUnknownUnit(t *testing.T) {
	req := SalesDashboardRequest{Unit: "долл"}
	err := req.Normalize()
	if err == nil {
		t.Fatal("Normalize() accepted an unsupported unit")
	}
	salesErr, ok := err.(*SalesError)
	if !ok || salesErr.Status != http.StatusBadRequest {
		t.Fatalf("error = %#v, want SalesError with status 400", err)
	}
}

func TestSegmentTrendsKeepsSelectedSegmentsInTotalsOrder(t *testing.T) {
	builder := &dashboardBuilder{req: SalesDashboardRequest{Segments: []string{"Аптека.ру", "еаптека"}}}
	monthly := []models.SalesDashboardSeriesPoint{
		{Name: "еаптека", Year: 2026, Month: 2, Value: 5},
		{Name: "Аптека.ру", Year: 2026, Month: 2, Value: 30},
		{Name: "Аптека.ру", Year: 2026, Month: 1, Value: 20},
		{Name: "Здравсити", Year: 2026, Month: 1, Value: 100},
		{Name: "еаптека", Year: 2026, Month: 1, Value: 7},
	}
	totals := []models.SalesDashboardRank{
		{Name: "Здравсити", Value: 100},
		{Name: "Аптека.ру", Value: 50},
		{Name: "еаптека", Value: 12},
	}

	got := builder.segmentTrends(monthly, totals)
	want := []models.SalesDashboardSeriesPoint{
		{Name: "Аптека.ру", Year: 2026, Month: 1, Value: 20},
		{Name: "Аптека.ру", Year: 2026, Month: 2, Value: 30},
		{Name: "еаптека", Year: 2026, Month: 1, Value: 7},
		{Name: "еаптека", Year: 2026, Month: 2, Value: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segmentTrends() = %#v, want %#v", got, want)
	}
}

func TestSegmentTrendsEmptyForSingleSegment(t *testing.T) {
	builder := &dashboardBuilder{req: SalesDashboardRequest{Segments: []string{"OLAP SS"}}}
	monthly := []models.SalesDashboardSeriesPoint{{Name: "OLAP SS", Year: 2026, Month: 1, Value: 20}}
	totals := []models.SalesDashboardRank{{Name: "OLAP SS", Value: 20}}

	if got := builder.segmentTrends(monthly, totals); len(got) != 0 {
		t.Fatalf("segmentTrends() = %#v, want empty", got)
	}
}
