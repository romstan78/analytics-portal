package handlers

import (
	"bytes"
	"testing"

	"backend/models"

	"github.com/xuri/excelize/v2"
)

func forecastImportWorkbook(t *testing.T, rows [][]interface{}) *bytes.Reader {
	t.Helper()
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for index, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, index+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := book.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buffer bytes.Buffer
	if err := book.Write(&buffer); err != nil {
		t.Fatal(err)
	}
	if err := book.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buffer.Bytes())
}

func TestParseForecastImportWorkbookSupportsRubAndUnits(t *testing.T) {
	source := forecastImportWorkbook(t, [][]interface{}{
		{"Бренд", "SKU", "Месяц", "Прогноз ТО, руб", "Прогноз ТО, уп.", "Комментарий"},
		{"Альфа", "SKU-1", "Январь", "1 250,50", "", "рубли"},
		{"Альфа", "SKU-2", 2, "", 10, "упаковки"},
	})

	rows, issues, err := parseForecastImportWorkbook(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 || len(rows) != 2 {
		t.Fatalf("строки=%#v проблемы=%#v", rows, issues)
	}
	if rows[0].Rub == nil || *rows[0].Rub != 1250.5 || rows[0].Units != nil {
		t.Fatalf("первая строка = %#v", rows[0])
	}
	if rows[1].Units == nil || *rows[1].Units != 10 || rows[1].Rub != nil {
		t.Fatalf("вторая строка = %#v", rows[1])
	}
}

func TestParseForecastImportWorkbookAllowsSingleMetricColumn(t *testing.T) {
	source := forecastImportWorkbook(t, [][]interface{}{
		{"Бренд", "SKU", "Месяц", "Прогноз ТО, руб"},
		{"Альфа", "SKU-1", 1, 100},
	})

	rows, issues, err := parseForecastImportWorkbook(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 || len(rows) != 1 || rows[0].Units != nil || rows[0].Rub == nil || *rows[0].Rub != 100 {
		t.Fatalf("строки=%#v проблемы=%#v", rows, issues)
	}
}

func TestParseForecastImportWorkbookRejectsDuplicateSKUInMonth(t *testing.T) {
	source := forecastImportWorkbook(t, [][]interface{}{
		{"Бренд", "SKU", "Месяц", "Прогноз ТО, руб"},
		{"Альфа", "SKU-1", 1, 100},
		{"альфа", "sku-1", 1, 200},
	})

	rows, issues, err := parseForecastImportWorkbook(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(issues) != 1 || issues[0].Row != 3 {
		t.Fatalf("строки=%#v проблемы=%#v", rows, issues)
	}
}

func TestPrepareForecastImportConvertsUnitsAndBuildsBrandTotal(t *testing.T) {
	brand, sku := "Альфа", "SKU-1"
	current := models.NetworkForecastResponse{
		Year: 2026, Quarter: 1,
		Months: []models.NetworkForecastMonth{
			{
				Year: 2026, Quarter: 1, Month: 1, BrandAS: brand,
				ForecastInvestmentsRub: models.PtrFloat(20), UpdatedAt: "brand-version",
			},
			{
				Year: 2026, Quarter: 1, Month: 1, BrandAS: brand, SKU: &sku,
				ContractPrice: models.PtrFloat(2.5), ForecastInvestmentsRub: models.PtrFloat(7),
				UpdatedAt: "sku-version",
			},
		},
	}

	prepared := prepareForecastImport("forecast.xlsx", []forecastImportRow{{
		Row: 2, Month: 1, BrandAS: brand, SKU: sku, Units: models.PtrFloat(10),
	}}, nil, current)
	if len(prepared.Preview.Errors) != 0 || prepared.Preview.ValidRows != 1 || len(prepared.Lines) != 2 {
		t.Fatalf("preview=%#v lines=%#v", prepared.Preview, prepared.Lines)
	}
	skuLine := prepared.Lines[0]
	if skuLine.ForecastRub == nil || *skuLine.ForecastRub != 25 ||
		skuLine.ForecastUnits == nil || *skuLine.ForecastUnits != 10 ||
		skuLine.ForecastInvestmentsRub == nil || *skuLine.ForecastInvestmentsRub != 7 {
		t.Fatalf("SKU-строка = %#v", skuLine)
	}
	brandLine := prepared.Lines[1]
	if brandLine.ForecastRub == nil || *brandLine.ForecastRub != 25 ||
		brandLine.ForecastUnits == nil || *brandLine.ForecastUnits != 10 ||
		brandLine.ForecastInvestmentsRub == nil || *brandLine.ForecastInvestmentsRub != 20 {
		t.Fatalf("итог бренда = %#v", brandLine)
	}
}

func TestPrepareForecastImportRejectsUnitsWithoutPrice(t *testing.T) {
	brand, sku := "Альфа", "SKU-1"
	current := models.NetworkForecastResponse{
		Year: 2026, Quarter: 1,
		Months: []models.NetworkForecastMonth{{
			Year: 2026, Quarter: 1, Month: 1, BrandAS: brand, SKU: &sku,
		}},
	}

	prepared := prepareForecastImport("forecast.xlsx", []forecastImportRow{{
		Row: 2, Month: 1, BrandAS: brand, SKU: sku, Units: models.PtrFloat(10),
	}}, nil, current)
	if len(prepared.Preview.Errors) != 1 || len(prepared.Lines) != 0 {
		t.Fatalf("preview=%#v lines=%#v", prepared.Preview, prepared.Lines)
	}
}
