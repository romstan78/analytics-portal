package handlers

import (
	"testing"
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
