package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"

	sq "github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func GetFilterOptions(c *gin.Context) {
	getDistinct := func(query string) []string {
		rows, e := config.DB.Query(query)
		if e != nil {
			return []string{}
		}
		defer rows.Close()
		var vals []string
		for rows.Next() {
			var v sql.NullString
			if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
				vals = append(vals, v.String)
			}
		}
		return vals
	}

	result := gin.H{
		"brandName":   getDistinct("SELECT DISTINCT brandName FROM dbo.tbl_EcomSalesNormalized WHERE brandName IS NOT NULL ORDER BY brandName"),
		"productName": getDistinct("SELECT DISTINCT productName FROM dbo.tbl_EcomSalesNormalized WHERE productName IS NOT NULL ORDER BY productName"),
		"networkName": getDistinct("SELECT DISTINCT networkName FROM dbo.tbl_EcomSalesNormalized WHERE networkName IS NOT NULL ORDER BY networkName"),
		"un_rub":      getDistinct("SELECT DISTINCT un_rub FROM dbo.tbl_EcomSalesNormalized WHERE un_rub IS NOT NULL ORDER BY un_rub"),
		"segment":     getDistinct("SELECT DISTINCT segment FROM dbo.tbl_EcomSalesNormalized WHERE segment IS NOT NULL ORDER BY segment"),
		"channel":     getDistinct("SELECT DISTINCT channel FROM dbo.tbl_EcomSalesNormalized WHERE channel IS NOT NULL ORDER BY channel"),
	}

	mappingQuery := `SELECT segment, channel FROM dbo.tbl_ChannelSegmentMapping WHERE segment IS NOT NULL AND channel IS NOT NULL GROUP BY segment, channel ORDER BY segment, channel`
	rows, e := config.DB.Query(mappingQuery)
	if e != nil {
		result["segmentChannelMap"] = make(map[string][]string)
		result["channelSegmentMap"] = make(map[string][]string)
	} else {
		defer rows.Close()
		segChanMap := make(map[string][]string)
		chanSegMap := make(map[string][]string)
		for rows.Next() {
			var seg, chanVal sql.NullString
			if err := rows.Scan(&seg, &chanVal); err == nil {
				if seg.Valid && chanVal.Valid && seg.String != "" && chanVal.String != "" {
					segChanMap[seg.String] = append(segChanMap[seg.String], chanVal.String)
					chanSegMap[chanVal.String] = append(chanSegMap[chanVal.String], seg.String)
				}
			}
		}
		result["segmentChannelMap"] = segChanMap
		result["channelSegmentMap"] = chanSegMap
	}
	c.JSON(http.StatusOK, result)
}

// salesFilter — параметры фильтрации интернет-продаж.
type salesFilter struct {
	YearFromStr  string
	YearToStr    string
	Months       []string
	Quarters     []string
	BrandNames   []string // LIKE-фильтр (для GetData, ExportSalesExcel)
	ProductNames []string
	NetworkNames []string // LIKE-фильтр (для GetData, ExportSalesExcel)
	UnRubs       []string
	Segments     []string
	Channels     []string
	Search       string
	// Точное совпадение (для Drilldown)
	BrandExact   string
	ProductExact string
	NetworkExact string
}

// buildSalesWhere строит WHERE-условие и аргументы для таблицы tbl_EcomSalesNormalized.
// Возвращает строку, начинающуюся с " WHERE ...", и слайс аргументов (в порядке появления).
// Использует squirrel Query Builder для безопасного построения SQL.
func buildSalesWhere(f salesFilter) (string, []interface{}) {
	q := sq.Select("1").PlaceholderFormat(sq.Question)

	// Базовое условие
	q = q.Where("n.metric_value != 0 AND n.metric_value IS NOT NULL")

	if f.YearFromStr != "" {
		if y, err := strconv.Atoi(f.YearFromStr); err == nil {
			q = q.Where("n.[year] >= ?", y)
		}
	}
	if f.YearToStr != "" {
		if y, err := strconv.Atoi(f.YearToStr); err == nil {
			q = q.Where("n.[year] <= ?", y)
		}
	}
	if len(f.Months) > 0 {
		months := make([]interface{}, 0, len(f.Months))
		for _, m := range f.Months {
			if val, err := strconv.Atoi(m); err == nil {
				months = append(months, val)
			}
		}
		if len(months) > 0 {
			q = q.Where(sq.Eq{"n.[month]": months})
		}
	}
	if len(f.Quarters) > 0 {
		quarters := make([]interface{}, 0, len(f.Quarters))
		for _, quarter := range f.Quarters {
			if value, err := strconv.Atoi(quarter); err == nil && value >= 1 && value <= 4 {
				quarters = append(quarters, value)
			}
		}
		if len(quarters) > 0 {
			q = q.Where(sq.Eq{"((n.[month] - 1) / 3) + 1": quarters})
		}
	}

	// LIKE-фильтры (OR между значениями внутри одного поля)
	if len(f.BrandNames) > 0 {
		orConds := sq.Or{}
		for _, v := range f.BrandNames {
			if v != "" {
				orConds = append(orConds, sq.Like{"n.brandName": "%" + v + "%"})
			}
		}
		if len(orConds) > 0 {
			q = q.Where(orConds)
		}
	}
	if len(f.ProductNames) > 0 {
		orConds := sq.Or{}
		for _, v := range f.ProductNames {
			if v != "" {
				orConds = append(orConds, sq.Like{"n.productName": "%" + v + "%"})
			}
		}
		if len(orConds) > 0 {
			q = q.Where(orConds)
		}
	}
	if len(f.NetworkNames) > 0 {
		orConds := sq.Or{}
		for _, v := range f.NetworkNames {
			if v != "" {
				orConds = append(orConds, sq.Like{"n.networkName": "%" + v + "%"})
			}
		}
		if len(orConds) > 0 {
			q = q.Where(orConds)
		}
	}

	// Точное совпадение (приоритетнее LIKE, используется в Drilldown)
	if f.BrandExact != "" {
		q = q.Where("n.brandName = ?", f.BrandExact)
	}
	if f.ProductExact != "" {
		q = q.Where("n.productName = ?", f.ProductExact)
	}
	if f.NetworkExact != "" {
		q = q.Where("n.networkName = ?", f.NetworkExact)
	}

	// IN-фильтры
	if len(f.UnRubs) > 0 {
		vals := filterNonEmpty(f.UnRubs)
		if len(vals) > 0 {
			q = q.Where(sq.Eq{"n.un_rub": vals})
		}
	}
	if len(f.Segments) > 0 {
		vals := filterNonEmpty(f.Segments)
		if len(vals) > 0 {
			q = q.Where(sq.Eq{"n.segment": vals})
		}
	}
	if len(f.Channels) > 0 {
		vals := filterNonEmpty(f.Channels)
		if len(vals) > 0 {
			q = q.Where(sq.Eq{"n.channel": vals})
		}
	}

	if f.Search != "" {
		likeArg := "%" + f.Search + "%"
		q = q.Where(sq.Or{
			sq.Like{"n.brandName": likeArg},
			sq.Like{"n.productName": likeArg},
			sq.Like{"n.networkName": likeArg},
			sq.Like{"n.metric_type": likeArg},
		})
	}

	sql, args, err := q.ToSql()
	if err != nil {
		config.Logger.Error("buildSalesWhere_ToSql_failed", "error", err.Error())
		return "", nil
	}
	// squirrel генерирует "SELECT 1 WHERE ...", нам нужна только WHERE-часть
	whereIdx := strings.Index(sql, "WHERE")
	if whereIdx >= 0 {
		return " " + sql[whereIdx:], args
	}
	return "", args
}

// filterNonEmpty возвращает слайс []interface{} без пустых строк.
func filterNonEmpty(vals []string) []interface{} {
	res := make([]interface{}, 0, len(vals))
	for _, v := range vals {
		if v != "" {
			res = append(res, v)
		}
	}
	return res
}

func GetData(c *gin.Context) {
	baseWhere, args := buildSalesWhere(salesFilter{
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
	})
	baseSelect := "SELECT n.id, n.[year], n.[month], n.brandName, n.productName, n.networkName, n.metric_type, n.metric_value, n.un_rub, n.segment, n.channel, n.updated_at FROM dbo.tbl_EcomSalesNormalized n"

	all := c.Query("all")

	if all == "true" {
		// Экспорт — возвращаем всё
		query := baseSelect + baseWhere + " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type"
		rows, err := config.DB.Query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
			return
		}
		defer rows.Close()

		var results []models.Row
		for rows.Next() {
			var r models.Row
			if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.BrandName, &r.ProductName, &r.NetworkName, &r.MetricType, &r.MetricValue, &r.UnRub, &r.Segment, &r.Channel, &r.UpdatedAt); err != nil {
				continue
			}
			results = append(results, r)
		}
		if results == nil {
			results = []models.Row{}
		}
		c.JSON(http.StatusOK, gin.H{"data": results})
		return
	}

	// Пагинация
	countQuery := "SELECT COUNT(*) FROM dbo.tbl_EcomSalesNormalized n" + baseWhere
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)

	var totalRows int
	if err := config.DB.QueryRow(countQuery, countArgs...).Scan(&totalRows); err != nil {
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
	offset := page * pageSize

	query := baseSelect + baseWhere + " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
	args = append(args, offset, pageSize)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	defer rows.Close()

	var results []models.Row
	for rows.Next() {
		var r models.Row
		if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.BrandName, &r.ProductName, &r.NetworkName, &r.MetricType, &r.MetricValue, &r.UnRub, &r.Segment, &r.Channel, &r.UpdatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil {
		results = []models.Row{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results, "totalRows": totalRows})
}

type salesDashboardPoint struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Value float64 `json:"value"`
}

type salesDashboardRank struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type salesDashboardSeriesPoint struct {
	Name  string  `json:"name"`
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Value float64 `json:"value"`
}

// GetSalesDashboard возвращает только агрегаты одного сегмента и одной единицы
// измерения. Это не позволяет случайно сложить пересекающиеся итоги OLAP.
func GetSalesDashboard(c *gin.Context) {
	segment := strings.TrimSpace(c.DefaultQuery("focusSegment", "OLAP SS"))
	channel := strings.TrimSpace(c.Query("focusChannel"))
	unit := strings.TrimSpace(c.DefaultQuery("unit", "руб"))
	focusProduct := strings.TrimSpace(c.Query("focusProduct"))
	focusNetwork := strings.TrimSpace(c.Query("focusNetwork"))
	if unit != "руб" && unit != "уп" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit должен быть 'руб' или 'уп'"})
		return
	}

	baseFilter := salesFilter{
		YearFromStr:  c.Query("yearFrom"),
		YearToStr:    c.Query("yearTo"),
		Months:       c.QueryArray("months"),
		Quarters:     c.QueryArray("quarters"),
		BrandNames:   c.QueryArray("brandName"),
		ProductNames: c.QueryArray("productName"),
		NetworkNames: c.QueryArray("networkName"),
		UnRubs:       []string{unit},
		Segments:     []string{segment},
	}
	where, args := buildSalesWhere(baseFilter)

	var total float64
	var activeNetworks, activeProducts, periods int
	summaryQuery := "SELECT COALESCE(SUM(n.metric_value), 0), COUNT(DISTINCT n.networkName), COUNT(DISTINCT n.productName), COUNT(DISTINCT CONCAT(n.[year], '-', n.[month])) FROM dbo.tbl_EcomSalesNormalized n" + where
	if err := config.DB.QueryRow(summaryQuery, args...).Scan(&total, &activeNetworks, &activeProducts, &periods); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard summary query failed"})
		return
	}

	loadTrend := func(trendWhere string, trendArgs []interface{}) ([]salesDashboardPoint, error) {
		rows, err := config.DB.Query("SELECT n.[year], n.[month], SUM(n.metric_value) FROM dbo.tbl_EcomSalesNormalized n"+trendWhere+" GROUP BY n.[year], n.[month] ORDER BY n.[year], n.[month]", trendArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		result := make([]salesDashboardPoint, 0)
		for rows.Next() {
			var point salesDashboardPoint
			if err := rows.Scan(&point.Year, &point.Month, &point.Value); err == nil {
				result = append(result, point)
			}
		}
		return result, rows.Err()
	}

	trend, err := loadTrend(where, args)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard trend query failed"})
		return
	}

	var latestValue, previousPeriodValue, yearAgoValue *float64
	latestYear, latestMonth := 0, 0
	if len(trend) > 0 {
		latest := trend[len(trend)-1]
		latestYear, latestMonth = latest.Year, latest.Month
		latestValue = &latest.Value

		comparisonFilter := baseFilter
		comparisonFilter.YearFromStr = ""
		comparisonFilter.YearToStr = ""
		comparisonFilter.Months = nil
		comparisonFilter.Quarters = nil
		comparisonWhere, comparisonArgs := buildSalesWhere(comparisonFilter)
		loadPeriodValue := func(year, month int) *float64 {
			queryArgs := append([]interface{}{}, comparisonArgs...)
			queryArgs = append(queryArgs, year, month)
			var value float64
			var count int
			query := "SELECT COALESCE(SUM(n.metric_value), 0), COUNT(*) FROM dbo.tbl_EcomSalesNormalized n" + comparisonWhere + " AND n.[year] = ? AND n.[month] = ?"
			if queryErr := config.DB.QueryRow(query, queryArgs...).Scan(&value, &count); queryErr != nil || count == 0 {
				return nil
			}
			return &value
		}

		previousYear, previousMonth := latestYear, latestMonth-1
		if previousMonth == 0 {
			previousYear--
			previousMonth = 12
		}
		previousPeriodValue = loadPeriodValue(previousYear, previousMonth)
		yearAgoValue = loadPeriodValue(latestYear-1, latestMonth)
	}

	focusTrend := make([]salesDashboardPoint, 0)
	if focusProduct != "" || focusNetwork != "" {
		focusFilter := baseFilter
		focusFilter.ProductExact = focusProduct
		focusFilter.NetworkExact = focusNetwork
		focusWhere, focusArgs := buildSalesWhere(focusFilter)
		focusTrend, err = loadTrend(focusWhere, focusArgs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard focus trend query failed"})
			return
		}
	}

	loadRank := func(column string) ([]salesDashboardRank, error) {
		query := "SELECT TOP 8 n." + column + ", SUM(n.metric_value) AS total_value FROM dbo.tbl_EcomSalesNormalized n" + where + " GROUP BY n." + column + " ORDER BY total_value DESC"
		rows, err := config.DB.Query(query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		result := make([]salesDashboardRank, 0)
		for rows.Next() {
			var item salesDashboardRank
			if err := rows.Scan(&item.Name, &item.Value); err == nil {
				result = append(result, item)
			}
		}
		return result, rows.Err()
	}

	topNetworks, err := loadRank("networkName")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard networks query failed"})
		return
	}
	topProducts, err := loadRank("productName")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard products query failed"})
		return
	}

	channelSegments := make([]string, 0)
	if channel != "" {
		segmentRows, queryErr := config.DB.Query("SELECT DISTINCT segment FROM dbo.tbl_ChannelSegmentMapping WHERE channel = ? AND un_rub = ? AND segment IS NOT NULL ORDER BY segment", channel, unit)
		if queryErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard channel mapping query failed"})
			return
		}
		for segmentRows.Next() {
			var mappedSegment string
			if scanErr := segmentRows.Scan(&mappedSegment); scanErr == nil && mappedSegment != "" {
				channelSegments = append(channelSegments, mappedSegment)
			}
		}
		segmentRows.Close()
	}
	if len(channelSegments) == 0 {
		channelSegments = append(channelSegments, segment)
	}

	segmentFilter := baseFilter
	segmentFilter.Segments = channelSegments
	segmentWhere, segmentArgs := buildSalesWhere(segmentFilter)
	segmentRows, err := config.DB.Query("SELECT n.segment, SUM(n.metric_value) AS total_value FROM dbo.tbl_EcomSalesNormalized n"+segmentWhere+" GROUP BY n.segment ORDER BY total_value DESC", segmentArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard segment totals query failed"})
		return
	}
	segmentTotals := make([]salesDashboardRank, 0)
	for segmentRows.Next() {
		var item salesDashboardRank
		if scanErr := segmentRows.Scan(&item.Name, &item.Value); scanErr == nil {
			segmentTotals = append(segmentTotals, item)
		}
	}
	segmentRows.Close()

	networkTrends := make([]salesDashboardSeriesPoint, 0)
	if len(topNetworks) > 0 {
		limit := len(topNetworks)
		if limit > 8 {
			limit = 8
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", limit), ",")
		networkArgs := append([]interface{}{}, args...)
		for _, item := range topNetworks[:limit] {
			networkArgs = append(networkArgs, item.Name)
		}
		networkQuery := "SELECT n.networkName, n.[year], n.[month], SUM(n.metric_value) FROM dbo.tbl_EcomSalesNormalized n" + where + " AND n.networkName IN (" + placeholders + ") GROUP BY n.networkName, n.[year], n.[month] ORDER BY n.[year], n.[month], n.networkName"
		networkRows, queryErr := config.DB.Query(networkQuery, networkArgs...)
		if queryErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard network trends query failed"})
			return
		}
		for networkRows.Next() {
			var point salesDashboardSeriesPoint
			if scanErr := networkRows.Scan(&point.Name, &point.Year, &point.Month, &point.Value); scanErr == nil {
				networkTrends = append(networkTrends, point)
			}
		}
		networkRows.Close()
	}

	average := 0.0
	if periods > 0 {
		average = total / float64(periods)
	}
	c.JSON(http.StatusOK, gin.H{
		"channel":         channel,
		"channelSegments": channelSegments,
		"segment":         segment,
		"unit":            unit,
		"summary": gin.H{
			"total":           total,
			"averagePerMonth": average,
			"activeNetworks":  activeNetworks,
			"activeProducts":  activeProducts,
			"periods":         periods,
			"latestYear":      latestYear,
			"latestMonth":     latestMonth,
			"latestValue":     latestValue,
			"previousValue":   previousPeriodValue,
			"yearAgoValue":    yearAgoValue,
		},
		"trend":         trend,
		"focusTrend":    focusTrend,
		"topNetworks":   topNetworks,
		"topProducts":   topProducts,
		"segmentTotals": segmentTotals,
		"networkTrends": networkTrends,
	})
}

func GetDrilldown(c *gin.Context) {
	brandName := c.Query("brandName")
	networkName := c.Query("networkName")
	if brandName == "" || networkName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "brandName и networkName обязательны"})
		return
	}

	where, args := buildSalesWhere(salesFilter{
		YearFromStr:  c.Query("yearFrom"),
		YearToStr:    c.Query("yearTo"),
		Months:       c.QueryArray("months"),
		Quarters:     c.QueryArray("quarters"),
		Segments:     c.QueryArray("segment"),
		Channels:     c.QueryArray("channel"),
		BrandExact:   brandName,
		NetworkExact: networkName,
	})

	query := `SELECT n.[year], n.[month], n.metric_type, SUM(n.metric_value) as total_value, n.un_rub, n.segment, n.channel FROM dbo.tbl_EcomSalesNormalized n` + where +
		" GROUP BY n.[year], n.[month], n.metric_type, n.un_rub, n.segment, n.channel ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
		return
	}
	defer rows.Close()

	var results []models.DrilldownRow
	for rows.Next() {
		var r models.DrilldownRow
		if err := rows.Scan(&r.Year, &r.Month, &r.MetricType, &r.TotalValue, &r.UnRub, &r.Segment, &r.Channel); err != nil {
			continue
		}
		results = append(results, r)
	}
	c.JSON(http.StatusOK, gin.H{"brandName": brandName, "networkName": networkName, "data": results})
}

// ─── Excel Export для интернет-продаж ──────────────────────────────────────
func ExportSalesExcel(c *gin.Context) {
	baseWhere, args := buildSalesWhere(salesFilter{
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
	})
	baseSelect := "SELECT n.id, n.[year], n.[month], n.brandName, n.productName, n.networkName, n.metric_type, n.metric_value, n.un_rub, n.segment, n.channel, n.updated_at FROM dbo.tbl_EcomSalesNormalized n"

	query := baseSelect + baseWhere + " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type"
	rows, err := config.DB.Query(query, args...)
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
		var r models.Row
		if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.BrandName, &r.ProductName, &r.NetworkName, &r.MetricType, &r.MetricValue, &r.UnRub, &r.Segment, &r.Channel, &r.UpdatedAt); err != nil {
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
