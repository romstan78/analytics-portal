package handlers

import (
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"backend/config"
	"backend/models"
	"backend/repository"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

const forecastImportMaxFileSize = 10 << 20

type forecastImportIssue struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type forecastImportPreviewResponse struct {
	FileName       string                `json:"file_name"`
	Rows           int                   `json:"rows"`
	ValidRows      int                   `json:"valid_rows"`
	AddedRows      int                   `json:"added_rows"`
	UpdatedRows    int                   `json:"updated_rows"`
	UnchangedRows  int                   `json:"unchanged_rows"`
	AffectedBrands int                   `json:"affected_brands"`
	Errors         []forecastImportIssue `json:"errors"`
	Warnings       []forecastImportIssue `json:"warnings"`
}

type forecastImportSaveResponse struct {
	Message      string                         `json:"message"`
	ImportedRows int                            `json:"imported_rows"`
	Data         models.NetworkForecastResponse `json:"data"`
}

type forecastClearResponse struct {
	Message     string                         `json:"message"`
	ClearedRows int64                          `json:"cleared_rows"`
	Data        models.NetworkForecastResponse `json:"data"`
}

type forecastImportRow struct {
	Row     int
	Month   int
	BrandAS string
	SKU     string
	Rub     *float64
	Units   *float64
	Comment string
}

type preparedForecastImport struct {
	Preview forecastImportPreviewResponse
	Lines   []repository.NetworkForecastInput
}

var forecastImportHeaders = map[string]string{
	"бренд":                "brand_as",
	"бренддляас":           "brand_as",
	"брендляас":            "brand_as",
	"sku":                  "sku",
	"скю":                  "sku",
	"артикул":              "sku",
	"месяц":                "month",
	"прогнозруб":           "rub",
	"прогнозторуб":         "rub",
	"прогнозторубли":       "rub",
	"прогнозврублях":       "rub",
	"прогнозуп":            "units",
	"прогнозтоуп":          "units",
	"прогнозтоупаковки":    "units",
	"прогнозупаковки":      "units",
	"прогнозвупаковках":    "units",
	"комментарий":          "comment",
	"причинакорректировки": "comment",
}

var forecastImportMonths = map[string]int{
	"январь": 1, "февраль": 2, "март": 3, "апрель": 4,
	"май": 5, "июнь": 6, "июль": 7, "август": 8,
	"сентябрь": 9, "октябрь": 10, "ноябрь": 11, "декабрь": 12,
}

func normalizeForecastImportHeader(value string) string {
	var result strings.Builder
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func forecastImportCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func parseForecastImportNumber(value string) (*float64, error) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\u00a0", "")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, ",", ".")
	if value == "" || value == "-" || value == "—" {
		return nil, nil
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, fmt.Errorf("не число: %q", value)
	}
	if number < 0 {
		return nil, errors.New("значение не может быть отрицательным")
	}
	return &number, nil
}

func parseForecastImportMonth(value string) (int, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if month, ok := forecastImportMonths[value]; ok {
		return month, nil
	}
	number, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil || number != math.Trunc(number) || number < 1 || number > 12 {
		return 0, fmt.Errorf("некорректный месяц: %q", value)
	}
	return int(number), nil
}

func parseForecastImportWorkbook(source io.Reader) ([]forecastImportRow, []forecastImportIssue, error) {
	book, err := excelize.OpenReader(source, excelize.Options{
		UnzipSizeLimit:    64 << 20,
		UnzipXMLSizeLimit: 16 << 20,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось прочитать Excel-файл: %w", err)
	}
	defer book.Close()

	sheet := book.GetSheetName(0)
	if sheet == "" {
		return nil, nil, errors.New("в Excel-файле нет листов")
	}
	rows, err := book.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось прочитать лист %q: %w", sheet, err)
	}
	if len(rows) == 0 {
		return nil, []forecastImportIssue{{Row: 1, Message: "файл пуст"}}, nil
	}

	columns := map[string]int{}
	for index, value := range rows[0] {
		if field := forecastImportHeaders[normalizeForecastImportHeader(value)]; field != "" {
			if _, exists := columns[field]; !exists {
				columns[field] = index
			}
		}
	}

	issues := []forecastImportIssue{}
	for _, required := range []struct{ field, label string }{
		{"brand_as", "Бренд"}, {"sku", "SKU"}, {"month", "Месяц"},
	} {
		if _, ok := columns[required.field]; !ok {
			issues = append(issues, forecastImportIssue{Row: 1, Message: "нет обязательной колонки «" + required.label + "»"})
		}
	}
	if _, rubOK := columns["rub"]; !rubOK {
		if _, unitsOK := columns["units"]; !unitsOK {
			issues = append(issues, forecastImportIssue{Row: 1, Message: "нужна колонка «Прогноз, руб.» или «Прогноз, уп.»"})
		}
	}
	if len(issues) > 0 {
		return nil, issues, nil
	}

	result := []forecastImportRow{}
	seen := map[string]int{}
	rubColumn, rubColumnExists := columns["rub"]
	if !rubColumnExists {
		rubColumn = -1
	}
	unitsColumn, unitsColumnExists := columns["units"]
	if !unitsColumnExists {
		unitsColumn = -1
	}
	commentColumn, commentColumnExists := columns["comment"]
	if !commentColumnExists {
		commentColumn = -1
	}
	for index, cells := range rows[1:] {
		rowNumber := index + 2
		brand := forecastImportCell(cells, columns["brand_as"])
		sku := forecastImportCell(cells, columns["sku"])
		monthText := forecastImportCell(cells, columns["month"])
		rubText := forecastImportCell(cells, rubColumn)
		unitsText := forecastImportCell(cells, unitsColumn)
		comment := forecastImportCell(cells, commentColumn)
		if brand == "" && sku == "" && monthText == "" && rubText == "" && unitsText == "" && comment == "" {
			continue
		}

		rowIssues := []string{}
		if brand == "" {
			rowIssues = append(rowIssues, "не указан бренд")
		}
		if sku == "" {
			rowIssues = append(rowIssues, "не указан SKU")
		}
		month, monthErr := parseForecastImportMonth(monthText)
		if monthErr != nil {
			rowIssues = append(rowIssues, monthErr.Error())
		}
		rub, rubErr := parseForecastImportNumber(rubText)
		if rubErr != nil {
			rowIssues = append(rowIssues, "прогноз в рублях: "+rubErr.Error())
		}
		units, unitsErr := parseForecastImportNumber(unitsText)
		if unitsErr != nil {
			rowIssues = append(rowIssues, "прогноз в упаковках: "+unitsErr.Error())
		}
		if rubErr == nil && unitsErr == nil && rub == nil && units == nil {
			rowIssues = append(rowIssues, "не заполнен прогноз ни в рублях, ни в упаковках")
		}

		key := fmt.Sprintf("%d|%s|%s", month, strings.ToLower(brand), strings.ToLower(sku))
		if previous, duplicate := seen[key]; duplicate && brand != "" && sku != "" && monthErr == nil {
			rowIssues = append(rowIssues, fmt.Sprintf("дубль строки %d", previous))
		} else if brand != "" && sku != "" && monthErr == nil {
			seen[key] = rowNumber
		}
		if len(rowIssues) > 0 {
			for _, message := range rowIssues {
				issues = append(issues, forecastImportIssue{Row: rowNumber, Message: message})
			}
			continue
		}

		result = append(result, forecastImportRow{
			Row: rowNumber, Month: month, BrandAS: brand, SKU: sku,
			Rub: rub, Units: units, Comment: comment,
		})
	}
	if len(result) == 0 && len(issues) == 0 {
		issues = append(issues, forecastImportIssue{Row: 1, Message: "в файле нет строк прогноза"})
	}
	return result, issues, nil
}

func forecastImportRowKey(month int, brand string, sku *string) string {
	skuValue := ""
	if sku != nil {
		skuValue = *sku
	}
	return fmt.Sprintf("%d|%s|%s", month, brand, skuValue)
}

func forecastImportBrandMonthKey(month int, brand string) string {
	return fmt.Sprintf("%d|%s", month, brand)
}

func sameForecastImportNumber(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return math.Abs(*left-*right) < 0.005
}

func roundedForecastImportNumber(value float64) *float64 {
	rounded := math.Round(value*100) / 100
	return &rounded
}

func prepareForecastImport(
	fileName string,
	rawRows []forecastImportRow,
	parseIssues []forecastImportIssue,
	current models.NetworkForecastResponse,
) preparedForecastImport {
	prepared := preparedForecastImport{Preview: forecastImportPreviewResponse{
		FileName: filepath.Base(fileName), Rows: len(rawRows),
		Errors:   append([]forecastImportIssue{}, parseIssues...),
		Warnings: []forecastImportIssue{},
	}}

	currentByKey := make(map[string]models.NetworkForecastMonth, len(current.Months))
	workingSKU := make(map[string]models.NetworkForecastMonth, len(current.Months))
	for _, row := range current.Months {
		key := forecastImportRowKey(row.Month, row.BrandAS, row.SKU)
		currentByKey[key] = row
		if row.SKU != nil {
			workingSKU[key] = row
		}
	}

	monthFrom := (current.Quarter-1)*3 + 1
	monthTo := monthFrom + 2
	affectedGroups := map[string]bool{}
	affectedBrands := map[string]bool{}
	for _, raw := range rawRows {
		if raw.Month < monthFrom || raw.Month > monthTo {
			prepared.Preview.Errors = append(prepared.Preview.Errors, forecastImportIssue{
				Row: raw.Row, Message: fmt.Sprintf("месяц %d не относится к Q%d", raw.Month, current.Quarter),
			})
			continue
		}
		sku := raw.SKU
		key := forecastImportRowKey(raw.Month, raw.BrandAS, &sku)
		currentRow, exists := currentByKey[key]
		if !exists {
			prepared.Preview.Errors = append(prepared.Preview.Errors, forecastImportIssue{
				Row: raw.Row, Message: fmt.Sprintf("пара бренд/SKU не найдена в выбранной сети: %s / %s", raw.BrandAS, raw.SKU),
			})
			continue
		}
		if currentRow.IsClosed {
			prepared.Preview.Errors = append(prepared.Preview.Errors, forecastImportIssue{
				Row: raw.Row, Message: fmt.Sprintf("месяц %d закрыт и недоступен для изменения", raw.Month),
			})
			continue
		}

		rub, units := raw.Rub, raw.Units
		if rub == nil && units != nil {
			if currentRow.ContractPrice == nil || *currentRow.ContractPrice <= 0 {
				prepared.Preview.Errors = append(prepared.Preview.Errors, forecastImportIssue{
					Row: raw.Row, Message: "для прогноза только в упаковках не задана контрактная цена SKU",
				})
				continue
			}
			rub = roundedForecastImportNumber(*units * *currentRow.ContractPrice)
		}
		if units == nil && rub != nil {
			if currentRow.ContractPrice != nil && *currentRow.ContractPrice > 0 {
				units = roundedForecastImportNumber(*rub / *currentRow.ContractPrice)
			} else {
				prepared.Preview.Warnings = append(prepared.Preview.Warnings, forecastImportIssue{
					Row: raw.Row, Message: "нет контрактной цены: прогноз в упаковках рассчитать нельзя",
				})
			}
		}

		reason := strings.TrimSpace(raw.Comment)
		if reason == "" {
			reason = "Импорт из Excel: " + prepared.Preview.FileName
		}
		line := repository.NetworkForecastInput{
			Month: raw.Month, BrandAS: raw.BrandAS, SKU: &sku,
			ForecastRub: rub, ForecastUnits: units,
			ForecastInvestmentsRub: currentRow.ForecastInvestmentsRub,
			AdjustmentReason:       &reason, UpdatedAt: currentRow.UpdatedAt,
		}
		prepared.Lines = append(prepared.Lines, line)
		currentRow.ForecastRub = rub
		currentRow.ForecastUnits = units
		workingSKU[key] = currentRow
		prepared.Preview.ValidRows++
		original := currentByKey[key]
		switch {
		case sameForecastImportNumber(original.ForecastRub, rub) && sameForecastImportNumber(original.ForecastUnits, units):
			prepared.Preview.UnchangedRows++
		case original.UpdatedAt == "" && original.ForecastRub == nil && original.ForecastUnits == nil:
			prepared.Preview.AddedRows++
		default:
			prepared.Preview.UpdatedRows++
		}
		affectedGroups[forecastImportBrandMonthKey(raw.Month, raw.BrandAS)] = true
		affectedBrands[raw.BrandAS] = true
	}

	for group := range affectedGroups {
		monthText, brand, found := strings.Cut(group, "|")
		if !found {
			continue
		}
		month, err := strconv.Atoi(monthText)
		if err != nil {
			continue
		}
		hasForecast := false
		allRub, allUnits := true, true
		rubTotal, unitsTotal := 0.0, 0.0
		for _, row := range workingSKU {
			if row.Month != month || row.BrandAS != brand || (row.ForecastRub == nil && row.ForecastUnits == nil) {
				continue
			}
			hasForecast = true
			if row.ForecastRub == nil {
				allRub = false
			} else {
				rubTotal += *row.ForecastRub
			}
			if row.ForecastUnits == nil {
				allUnits = false
			} else {
				unitsTotal += *row.ForecastUnits
			}
		}
		if !hasForecast {
			continue
		}
		var rub, units *float64
		if allRub {
			rub = roundedForecastImportNumber(rubTotal)
		}
		if allUnits {
			units = roundedForecastImportNumber(unitsTotal)
		}
		brandKey := forecastImportRowKey(month, brand, nil)
		brandRow := currentByKey[brandKey]
		reason := "Итог SKU после импорта из Excel: " + prepared.Preview.FileName
		prepared.Lines = append(prepared.Lines, repository.NetworkForecastInput{
			Month: month, BrandAS: brand,
			ForecastRub: rub, ForecastUnits: units,
			ForecastInvestmentsRub: brandRow.ForecastInvestmentsRub,
			AdjustmentReason:       &reason, UpdatedAt: brandRow.UpdatedAt,
		})
	}
	prepared.Preview.AffectedBrands = len(affectedBrands)
	return prepared
}

func readForecastImportFile(c *gin.Context) ([]forecastImportRow, []forecastImportIssue, string, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, forecastImportMaxFileSize+(1<<20))
	header, err := c.FormFile("file")
	if err != nil {
		return nil, nil, "", errors.New("прикрепите Excel-файл")
	}
	if header.Size <= 0 || header.Size > forecastImportMaxFileSize {
		return nil, nil, "", errors.New("размер Excel-файла должен быть не больше 10 МБ")
	}
	extension := strings.ToLower(filepath.Ext(header.Filename))
	if extension != ".xlsx" && extension != ".xlsm" {
		return nil, nil, "", errors.New("поддерживаются файлы .xlsx и .xlsm")
	}
	source, err := openForecastImportMultipartFile(header)
	if err != nil {
		return nil, nil, "", err
	}
	defer source.Close()
	rows, issues, err := parseForecastImportWorkbook(source)
	return rows, issues, filepath.Base(header.Filename), err
}

func openForecastImportMultipartFile(header *multipart.FileHeader) (multipart.File, error) {
	source, err := header.Open()
	if err != nil {
		return nil, errors.New("не удалось открыть загруженный файл")
	}
	return source, nil
}

func forecastImportContext(c *gin.Context) (int, int, int, models.NetworkForecastResponse, bool) {
	id, ok := networkIDParam(c)
	if !ok {
		return 0, 0, 0, models.NetworkForecastResponse{}, false
	}
	year, ok := planYear(c)
	if !ok {
		return 0, 0, 0, models.NetworkForecastResponse{}, false
	}
	quarter, ok := planQuarter(c)
	if !ok {
		return 0, 0, 0, models.NetworkForecastResponse{}, false
	}
	current, err := loadNetworkForecast(id, year, quarter)
	if err != nil {
		respondNetworkError(c, err, "network_forecast_import_context_failed")
		return 0, 0, 0, models.NetworkForecastResponse{}, false
	}
	return id, year, quarter, current, true
}

// PreviewNetworkForecastImport полностью проверяет файл, но не пишет в БД.
func PreviewNetworkForecastImport(c *gin.Context) {
	_, _, _, current, ok := forecastImportContext(c)
	if !ok {
		return
	}
	rows, issues, fileName, err := readForecastImportFile(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	prepared := prepareForecastImport(fileName, rows, issues, current)
	c.JSON(http.StatusOK, prepared.Preview)
}

// ImportNetworkForecast повторно валидирует тот же файл и сохраняет все
// подготовленные SKU-строки вместе с официальными итогами брендов.
func ImportNetworkForecast(c *gin.Context) {
	id, year, quarter, current, ok := forecastImportContext(c)
	if !ok {
		return
	}
	rows, issues, fileName, err := readForecastImportFile(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	prepared := prepareForecastImport(fileName, rows, issues, current)
	if len(prepared.Preview.Errors) > 0 {
		first := prepared.Preview.Errors[0]
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("строка %d: %s", first.Row, first.Message)})
		return
	}
	if prepared.Preview.ValidRows == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "в файле нет строк для импорта"})
		return
	}

	username, _ := currentUser(c)
	if err := repository.SaveNetworkForecastLines(repository.SaveNetworkForecastInput{
		NetworkID: id, Year: year, Quarter: quarter,
		Lines: prepared.Lines, UserName: username,
	}); err != nil {
		respondNetworkError(c, err, "network_forecast_import_failed")
		return
	}
	response, err := loadNetworkForecast(id, year, quarter)
	if err != nil {
		respondNetworkError(c, err, "network_forecast_import_refetch_failed")
		return
	}
	if err := repository.UpdateNetworkPlanForecastRollup(id, year, quarter, response.Brands); err != nil {
		config.Logger.Error("network_forecast_import_rollup_failed", "error", err.Error(), "network_id", id)
	}
	_ = repository.InsertEntityAuditLog(
		"network_forecast", id, username, "IMPORT",
		jsonString(map[string]interface{}{
			"year": year, "quarter": quarter, "file": filepath.Base(fileName),
			"rows": prepared.Preview.ValidRows,
		}),
	)
	c.JSON(http.StatusOK, forecastImportSaveResponse{
		Message: "Прогноз импортирован", ImportedRows: prepared.Preview.ValidRows, Data: response,
	})
}

type clearForecastInput struct {
	Year  int    `json:"year"`
	Month int    `json:"month"`
	Scope string `json:"scope"`
}

// ClearNetworkForecast очищает внесённый объём за один открытый месяц. Факт,
// системная рекомендация и прогноз инвестиций не затрагиваются.
func ClearNetworkForecast(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	var input clearForecastInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Year < 2000 || input.Year > 2100 || input.Month < 1 || input.Month > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный период прогноза"})
		return
	}
	username, _ := currentUser(c)
	cleared, err := repository.ClearNetworkForecastMonth(repository.ClearNetworkForecastInput{
		NetworkID: id, Year: input.Year, Month: input.Month,
		Scope: input.Scope, UserName: username,
	})
	if err != nil {
		respondNetworkError(c, err, "network_forecast_clear_failed")
		return
	}
	quarter := (input.Month-1)/3 + 1
	response, err := loadNetworkForecast(id, input.Year, quarter)
	if err != nil {
		respondNetworkError(c, err, "network_forecast_clear_refetch_failed")
		return
	}
	if err := repository.UpdateNetworkPlanForecastRollup(id, input.Year, quarter, response.Brands); err != nil {
		config.Logger.Error("network_forecast_clear_rollup_failed", "error", err.Error(), "network_id", id)
	}
	_ = repository.InsertEntityAuditLog(
		"network_forecast", id, username, "CLEAR",
		jsonString(map[string]interface{}{
			"year": input.Year, "month": input.Month, "scope": input.Scope, "rows": cleared,
		}),
	)
	c.JSON(http.StatusOK, forecastClearResponse{
		Message: "Прогноз месяца очищен", ClearedRows: cleared, Data: response,
	})
}
