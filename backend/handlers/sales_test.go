package handlers

import (
	"reflect"
	"testing"

	"backend/models"
	"backend/services"
)

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

func TestSelectedSalesExcelColumns(t *testing.T) {
	columns := selectedSalesExcelColumns([]string{"networkName", "metricValue", "networkName", "unknown"})
	fields := make([]string, 0, len(columns))
	for _, column := range columns {
		fields = append(fields, column.Field)
	}
	if want := []string{"networkName", "metricValue"}; !reflect.DeepEqual(fields, want) {
		t.Fatalf("fields = %v, want %v", fields, want)
	}
}

func TestSalesBackgroundExportMaxRows(t *testing.T) {
	t.Setenv("SALES_BACKGROUND_EXPORT_MAX_ROWS", "250000")
	if got := salesBackgroundExportMaxRows(); got != 250000 {
		t.Fatalf("salesBackgroundExportMaxRows() = %d, want 250000", got)
	}
	t.Setenv("SALES_BACKGROUND_EXPORT_MAX_ROWS", "invalid")
	if got := salesBackgroundExportMaxRows(); got != defaultSalesBackgroundExportMaxRows {
		t.Fatalf("invalid value: got %d, want %d", got, defaultSalesBackgroundExportMaxRows)
	}
}

func TestSalesExportJobIsPrivateToOwner(t *testing.T) {
	const id = "job-private-test"
	salesExportJobStore.Lock()
	salesExportJobStore.jobs[id] = &salesExportJob{ID: id, Owner: "alice", Status: "ready"}
	salesExportJobStore.Unlock()
	t.Cleanup(func() {
		salesExportJobStore.Lock()
		delete(salesExportJobStore.jobs, id)
		salesExportJobStore.Unlock()
	})

	if _, ok := salesExportJobForUser(id, "alice"); !ok {
		t.Fatal("owner must be able to read own export job")
	}
	if _, ok := salesExportJobForUser(id, "bob"); ok {
		t.Fatal("another user must not be able to read export job")
	}
}

func TestBuildSalesPivotExcelUsesResponseHierarchy(t *testing.T) {
	response := &models.SalesPivotResponse{
		AnalysisYear: 2026,
		Channel:      "PURE",
		Segments:     []string{"Сегмент"},
		Unit:         "руб",
		Granularity:  "year",
		Periods: []models.SalesPivotPeriod{
			{Key: "2025-total", Label: "Итого", Year: 2025, Kind: "total"},
			{Key: "2026-total", Label: "Итого", Year: 2026, Kind: "total"},
		},
		Rows: []models.SalesPivotNode{{
			ID: "r1", Level: "channel", Name: "PURE",
			Values:   map[string]float64{"2025-total": 100, "2026-total": 120},
			Children: []models.SalesPivotNode{},
		}},
		Totals:           map[string]float64{"2025-total": 100, "2026-total": 120},
		PreviousTotalKey: "2025-total",
		CurrentTotalKey:  "2026-total",
	}

	file, err := buildSalesPivotExcel(response, services.SalesPivotRequest{})
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if value, _ := file.GetCellValue("Сводная", "B1"); value != "2025" {
		t.Fatalf("B1 = %q, want earlier year 2025", value)
	}
	if value, _ := file.GetCellValue("Сводная", "A3"); value != "PURE" {
		t.Fatalf("A3 = %q, want hierarchy root", value)
	}
	if value, _ := file.GetCellValue("Сводная", "A4"); value != "ИТОГО" {
		t.Fatalf("A4 = %q, want grand total", value)
	}
}
