package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"backend/config"
	"backend/models"
	"backend/services"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func salesPivotRequestFromQuery(c *gin.Context) services.SalesPivotRequest {
	return services.SalesPivotRequest{
		AnalysisYearRaw: c.Query("analysisYear"),
		YearFromRaw:     c.Query("yearFrom"),
		YearToRaw:       c.Query("yearTo"),
		Unit:            c.DefaultQuery("unit", "руб"),
		Granularity:     c.DefaultQuery("granularity", "year"),
		Months:          c.QueryArray("months"),
		Quarters:        c.QueryArray("quarters"),
		BrandNames:      c.QueryArray("brandName"),
		ProductNames:    c.QueryArray("productName"),
		NetworkNames:    c.QueryArray("networkName"),
		KAMs:            c.QueryArray("kam"),
		Segments:        append(c.QueryArray("focusSegments"), c.Query("focusSegment")),
		Channel:         c.Query("focusChannel"),
	}
}

// GetSalesPivot возвращает одну и ту же сводную модель для web-таблицы и Excel.
func GetSalesPivot(c *gin.Context) {
	response, err := services.BuildSalesPivot(salesPivotRequestFromQuery(c))
	if err != nil {
		respondSalesError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func pivotComparison(values map[string]float64, response *models.SalesPivotResponse) (float64, interface{}) {
	previous := values[response.PreviousTotalKey]
	current := values[response.CurrentTotalKey]
	delta := current - previous
	if previous == 0 {
		return delta, ""
	}
	return delta, delta / previous
}

func buildSalesPivotExcel(response *models.SalesPivotResponse, request services.SalesPivotRequest) (*excelize.File, error) {
	f := excelize.NewFile()
	const sheet = "Сводная"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"6366F1"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    []excelize.Border{{Type: "left", Color: "D8DEEA", Style: 1}, {Type: "right", Color: "D8DEEA", Style: 1}, {Type: "top", Color: "D8DEEA", Style: 1}, {Type: "bottom", Color: "D8DEEA", Style: 1}},
	})
	numberStyle, _ := f.NewStyle(&excelize.Style{
		NumFmt:    4,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    []excelize.Border{{Type: "bottom", Color: "E5E7EB", Style: 1}},
	})
	percentStyle, _ := f.NewStyle(&excelize.Style{
		NumFmt:    10,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    []excelize.Border{{Type: "bottom", Color: "E5E7EB", Style: 1}},
	})
	levelStyles := make([]int, 4)
	levelColors := []string{"E8EAFB", "F0F2FF", "F8F9FC", "FFFFFF"}
	for level := 0; level < 4; level++ {
		levelStyles[level], _ = f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: level < 3},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{levelColors[level]}},
			Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: level},
			Border:    []excelize.Border{{Type: "bottom", Color: "E5E7EB", Style: 1}},
		})
	}
	totalStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"DDE3F3"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    []excelize.Border{{Type: "top", Color: "94A3B8", Style: 2}, {Type: "bottom", Color: "94A3B8", Style: 1}},
	})

	periodCount := len(response.Periods)
	lastColumn := periodCount + 3
	lastColumnName, _ := excelize.ColumnNumberToName(lastColumn)
	f.SetCellValue(sheet, "A1", "Канал / сегмент / сеть / SKU")
	_ = f.MergeCell(sheet, "A1", "A2")

	for start := 0; start < periodCount; {
		year := response.Periods[start].Year
		end := start
		for end+1 < periodCount && response.Periods[end+1].Year == year {
			end++
		}
		startCell, _ := excelize.CoordinatesToCellName(start+2, 1)
		endCell, _ := excelize.CoordinatesToCellName(end+2, 1)
		f.SetCellValue(sheet, startCell, year)
		if startCell != endCell {
			_ = f.MergeCell(sheet, startCell, endCell)
		}
		start = end + 1
	}
	for index, period := range response.Periods {
		cell, _ := excelize.CoordinatesToCellName(index+2, 2)
		f.SetCellValue(sheet, cell, period.Label)
	}
	deltaColumn := periodCount + 2
	yoyColumn := periodCount + 3
	deltaCell, _ := excelize.CoordinatesToCellName(deltaColumn, 1)
	deltaEndCell, _ := excelize.CoordinatesToCellName(deltaColumn, 2)
	yoyCell, _ := excelize.CoordinatesToCellName(yoyColumn, 1)
	yoyEndCell, _ := excelize.CoordinatesToCellName(yoyColumn, 2)
	f.SetCellValue(sheet, deltaCell, "Отклонение")
	f.SetCellValue(sheet, yoyCell, "YoY")
	_ = f.MergeCell(sheet, deltaCell, deltaEndCell)
	_ = f.MergeCell(sheet, yoyCell, yoyEndCell)
	_ = f.SetCellStyle(sheet, "A1", lastColumnName+"2", headerStyle)
	_ = f.SetRowHeight(sheet, 1, 24)
	_ = f.SetRowHeight(sheet, 2, 24)

	rowNumber := 3
	var writeNodes func([]models.SalesPivotNode, int) error
	writeNodes = func(nodes []models.SalesPivotNode, level int) error {
		for _, node := range nodes {
			values := make([]interface{}, 0, lastColumn)
			values = append(values, node.Name)
			for _, period := range response.Periods {
				values = append(values, node.Values[period.Key])
			}
			delta, yoy := pivotComparison(node.Values, response)
			values = append(values, delta, yoy)
			firstCell, _ := excelize.CoordinatesToCellName(1, rowNumber)
			if err := f.SetSheetRow(sheet, firstCell, &values); err != nil {
				return err
			}
			styleLevel := level
			if styleLevel > 3 {
				styleLevel = 3
			}
			_ = f.SetCellStyle(sheet, firstCell, firstCell, levelStyles[styleLevel])
			if lastColumn > 1 {
				numericStart, _ := excelize.CoordinatesToCellName(2, rowNumber)
				numericEnd, _ := excelize.CoordinatesToCellName(yoyColumn-1, rowNumber)
				_ = f.SetCellStyle(sheet, numericStart, numericEnd, numberStyle)
				yoyRowCell, _ := excelize.CoordinatesToCellName(yoyColumn, rowNumber)
				_ = f.SetCellStyle(sheet, yoyRowCell, yoyRowCell, percentStyle)
			}
			if level > 0 {
				_ = f.SetRowOutlineLevel(sheet, rowNumber, uint8(level))
			}
			rowNumber++
			if err := writeNodes(node.Children, level+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := writeNodes(response.Rows, 0); err != nil {
		f.Close()
		return nil, err
	}

	totalValues := make([]interface{}, 0, lastColumn)
	totalValues = append(totalValues, "ИТОГО")
	for _, period := range response.Periods {
		totalValues = append(totalValues, response.Totals[period.Key])
	}
	delta, yoy := pivotComparison(response.Totals, response)
	totalValues = append(totalValues, delta, yoy)
	totalCell, _ := excelize.CoordinatesToCellName(1, rowNumber)
	if err := f.SetSheetRow(sheet, totalCell, &totalValues); err != nil {
		f.Close()
		return nil, err
	}
	_ = f.SetCellStyle(sheet, totalCell, lastColumnName+fmt.Sprint(rowNumber), totalStyle)
	yoyTotalCell, _ := excelize.CoordinatesToCellName(yoyColumn, rowNumber)
	_ = f.SetCellStyle(sheet, yoyTotalCell, yoyTotalCell, percentStyle)

	_ = f.SetColWidth(sheet, "A", "A", 44)
	for column := 2; column <= lastColumn; column++ {
		name, _ := excelize.ColumnNumberToName(column)
		width := 15.0
		if column == yoyColumn {
			width = 11
		}
		_ = f.SetColWidth(sheet, name, name, width)
	}
	_ = f.SetPanes(sheet, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 1, YSplit: 2, TopLeftCell: "B3", ActivePane: "bottomRight",
	})

	const paramsSheet = "Параметры"
	_, _ = f.NewSheet(paramsSheet)
	parameterRows := [][]interface{}{
		{"Параметр", "Значение"},
		{"Год анализа", response.AnalysisYear},
		{"Детализация", map[string]string{"year": "Год", "quarter": "Квартал", "month": "Месяц"}[response.Granularity]},
		{"Единица", response.Unit},
		{"Канал", response.Channel},
		{"Сегменты", strings.Join(response.Segments, ", ")},
		{"Кварталы", strings.Join(request.Quarters, ", ")},
		{"Месяцы", strings.Join(request.Months, ", ")},
		{"Бренды", strings.Join(request.BrandNames, ", ")},
		{"Сети", strings.Join(request.NetworkNames, ", ")},
		{"SKU", strings.Join(request.ProductNames, ", ")},
		{"Сформировано", time.Now().Format("02.01.2006 15:04")},
	}
	for index, values := range parameterRows {
		cell, _ := excelize.CoordinatesToCellName(1, index+1)
		if err := f.SetSheetRow(paramsSheet, cell, &values); err != nil {
			f.Close()
			return nil, err
		}
	}
	_ = f.SetCellStyle(paramsSheet, "A1", "B1", headerStyle)
	_ = f.SetColWidth(paramsSheet, "A", "A", 24)
	_ = f.SetColWidth(paramsSheet, "B", "B", 70)
	return f, nil
}

func ExportSalesPivotExcel(c *gin.Context) {
	request := salesPivotRequestFromQuery(c)
	response, err := services.BuildSalesPivot(request)
	if err != nil {
		respondSalesError(c, err)
		return
	}
	f, err := buildSalesPivotExcel(response, request)
	if err != nil {
		config.Logger.Error("sales_pivot_excel_build_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Excel export failed"})
		return
	}
	defer f.Close()

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=internet-sales-pivot_%s.xlsx", time.Now().Format("2006-01-02")))
	c.Header("Content-Transfer-Encoding", "binary")
	if err := f.Write(c.Writer); err != nil {
		config.Logger.Error("sales_pivot_excel_write_failed", "error", err.Error())
	}
}
