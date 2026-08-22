package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// HTTP-слой интернет-продаж: разбор query-параметров, коды ответов и выгрузка.
// Запросы к БД живут в repository, расчёты витрин — в services.

// salesFilterFromQuery собирает фильтр выборки из query-параметров.
func salesFilterFromQuery(c *gin.Context) repository.SalesFilter {
	return repository.SalesFilter{
		YearFromStr:  c.Query("yearFrom"),
		YearToStr:    c.Query("yearTo"),
		Months:       c.QueryArray("months"),
		Quarters:     c.QueryArray("quarters"),
		BrandNames:   c.QueryArray("brandName"),
		ProductNames: c.QueryArray("productName"),
		NetworkNames: c.QueryArray("networkName"),
		UnRubs:       c.QueryArray("un_rub"),
		Segments:     c.QueryArray("segment"),
		Channels:     c.QueryArray("channel"),
		Search:       c.Query("search"),
	}
}

// respondSalesError отдаёт статус, выбранный сервисом; на прочих ошибках — 500.
func respondSalesError(c *gin.Context, err error) {
	var salesErr *services.SalesError
	if errors.As(err, &salesErr) {
		body := gin.H{"error": salesErr.Message}
		if salesErr.Details != "" {
			body["details"] = salesErr.Details
		}
		c.JSON(salesErr.Status, body)
		return
	}
	config.Logger.Error("sales_request_failed", "error", err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
}

// GetFilterOptions возвращает справочники панели фильтров.
func GetFilterOptions(c *gin.Context) {
	c.JSON(http.StatusOK, repository.SalesFilterOptions())
}

// GetSalesNetworkOptions возвращает сети, доступные при текущих фильтрах.
func GetSalesNetworkOptions(c *gin.Context) {
	networks, err := services.SalesNetworkOptions(services.SalesNetworkOptionsRequest{
		Unit:         c.DefaultQuery("unit", "руб"),
		Segments:     append(c.QueryArray("focusSegments"), c.Query("focusSegment")),
		YearFromRaw:  c.Query("yearFrom"),
		YearToRaw:    c.Query("yearTo"),
		Months:       c.QueryArray("months"),
		Quarters:     c.QueryArray("quarters"),
		BrandNames:   c.QueryArray("brandName"),
		ProductNames: c.QueryArray("productName"),
	})
	if err != nil {
		respondSalesError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.SalesNetworkOptionsResponse{NetworkName: networks})
}

// defaultSalesExportMaxRows — потолок выгрузки для all=true. Ответ отдаётся
// потоком, поэтому память сервера не растёт, но неограниченная выборка всё
// равно способна занять соединение с БД надолго.
const defaultSalesExportMaxRows = 200000

func salesExportMaxRows() int {
	if raw := strings.TrimSpace(os.Getenv("SALES_EXPORT_MAX_ROWS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultSalesExportMaxRows
}

// streamAllSalesRows отдаёт выборку целиком построчно, не накапливая её в
// памяти. Слишком большая выгрузка отклоняется до начала чтения.
func streamAllSalesRows(c *gin.Context, filter repository.SalesFilter) {
	limit := salesExportMaxRows()

	totalRows, err := repository.SalesRowsCount(filter)
	if err != nil {
		config.Logger.Error("sales_export_count_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	if totalRows > limit {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf(
				"Выгрузка слишком большая: %d строк при лимите %d. Уточните фильтры или используйте выгрузку в Excel.",
				totalRows, limit,
			),
			"total": totalRows,
			"limit": limit,
			"data":  []interface{}{},
		})
		return
	}

	rows, err := repository.SalesRowsCursor(filter)
	if err != nil {
		config.Logger.Error("sales_export_query_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Status(http.StatusOK)
	writer := c.Writer
	encoder := json.NewEncoder(writer)

	if _, err := io.WriteString(writer, `{"data":[`); err != nil {
		return
	}
	first := true
	for rows.Next() {
		r, scanErr := repository.ScanSalesRow(rows)
		if scanErr != nil {
			// Заголовки уже отправлены, поэтому статус не изменить: обрываем
			// ответ незакрытым JSON, чтобы клиент увидел ошибку, а не тишину.
			config.Logger.Error("sales_export_scan_failed", "error", scanErr.Error())
			return
		}
		if !first {
			if _, err := io.WriteString(writer, ","); err != nil {
				return
			}
		}
		first = false
		if err := encoder.Encode(r); err != nil {
			config.Logger.Error("sales_export_encode_failed", "error", err.Error())
			return
		}
	}
	if err := rows.Err(); err != nil {
		config.Logger.Error("sales_export_rows_failed", "error", err.Error())
		return
	}
	if _, err := io.WriteString(writer, "]}"); err != nil {
		config.Logger.Error("sales_export_write_failed", "error", err.Error())
	}
}

// GetData возвращает страницу выборки либо, при all=true, всю её потоком.
func GetData(c *gin.Context) {
	filter := salesFilterFromQuery(c)

	if c.Query("all") == "true" {
		streamAllSalesRows(c, filter)
		return
	}

	totalRows, err := repository.SalesRowsCount(filter)
	if err != nil {
		totalRows = 0
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	rows, err := repository.SalesRowsPage(filter, page*pageSize, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, models.SalesDataResponse{Data: rows, TotalRows: &totalRows})
}

// GetSalesDashboard возвращает витрину выбранных сегментов одного канала
// и одной единицы измерения.
func GetSalesDashboard(c *gin.Context) {
	dashboard, err := services.BuildSalesDashboard(services.SalesDashboardRequest{
		AnalysisYearRaw: c.Query("analysisYear"),
		YearFromRaw:     c.Query("yearFrom"),
		YearToRaw:       c.Query("yearTo"),
		Segments:        append(c.QueryArray("focusSegments"), c.Query("focusSegment")),
		Channel:         c.Query("focusChannel"),
		Unit:            c.DefaultQuery("unit", "руб"),
		Months:          c.QueryArray("months"),
		Quarters:        c.QueryArray("quarters"),
		BrandNames:      c.QueryArray("brandName"),
		ProductNames:    c.QueryArray("productName"),
		NetworkNames:    c.QueryArray("networkName"),
		FocusProducts:   append(c.QueryArray("focusProducts"), c.Query("focusProduct")),
		FocusNetworks:   append(c.QueryArray("focusNetworks"), c.Query("focusNetwork")),
		CompareChannels: c.QueryArray("compareChannels"),
	})
	if err != nil {
		respondSalesError(c, err)
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

// GetDrilldown возвращает разбивку одной пары «бренд × сеть» по периодам.
func GetDrilldown(c *gin.Context) {
	brandName := c.Query("brandName")
	networkName := c.Query("networkName")
	if brandName == "" || networkName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "brandName и networkName обязательны"})
		return
	}

	rows, err := repository.SalesDrilldown(repository.SalesFilter{
		YearFromStr:  c.Query("yearFrom"),
		YearToStr:    c.Query("yearTo"),
		Months:       c.QueryArray("months"),
		Quarters:     c.QueryArray("quarters"),
		Segments:     c.QueryArray("segment"),
		Channels:     c.QueryArray("channel"),
		BrandExact:   brandName,
		NetworkExact: networkName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
		return
	}
	c.JSON(http.StatusOK, models.DrilldownResponse{
		BrandName:   brandName,
		NetworkName: networkName,
		Data:        rows,
	})
}

// ─── Excel Export для интернет-продаж ──────────────────────────────────────

func ExportSalesExcel(c *gin.Context) {
	rows, err := repository.SalesRowsCursor(salesFilterFromQuery(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
		return
	}
	defer rows.Close()

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Интернет-продажи"
	f.SetSheetName("Sheet1", sheet)

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "StreamWriter creation failed"})
		return
	}

	// Заголовки через StreamWriter
	headers := []interface{}{
		"Год", "Месяц", "Бренд", "Продукт", "Сеть",
		"Показатель", "Значение", "Уп/Руб", "Сегмент", "Канал", "Обновлено", "ID",
	}
	if err := sw.SetRow("A1", headers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Header write failed"})
		return
	}

	// Данные — пишем напрямую из курсора БД, строку за строкой
	rowNum := 2
	for rows.Next() {
		r, scanErr := repository.ScanSalesRow(rows)
		if scanErr != nil {
			continue
		}
		vals := []interface{}{
			r.Year, r.Month,
			r.BrandName, r.ProductName, r.NetworkName,
			r.MetricType, r.MetricValue,
			models.ValString(r.UnRub), models.ValString(r.Segment), models.ValString(r.Channel), models.ValString(r.UpdatedAt),
			r.ID,
		}
		cell, _ := excelize.CoordinatesToCellName(1, rowNum)
		if err := sw.SetRow(cell, vals); err != nil {
			continue
		}
		rowNum++
	}

	if err := sw.Flush(); err != nil {
		config.Logger.Error("excel_stream_flush_failed", "error", err.Error())
	}

	// Стиль заголовка
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"6366F1"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)

	// Ширина колонок
	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		f.SetColWidth(sheet, col, col, 18)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=internet-sales_%s.xlsx", time.Now().Format("2006-01-02")))
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		config.Logger.Error("excel_export_sales_failed", "error", err.Error())
	}
}
