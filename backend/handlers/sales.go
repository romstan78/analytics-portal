package handlers

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"backend/config"
	"backend/models"

	sq "github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

const cbrEURCurrencyID = "R01239"

type cbrCurrencyRecord struct {
	Date    string `xml:"Date,attr"`
	Nominal string `xml:"Nominal"`
	Value   string `xml:"Value"`
}

type cbrCurrencyResponse struct {
	Records []cbrCurrencyRecord `xml:"Record"`
}

type eurRateCacheEntry struct {
	Rates     map[int]float64
	ExpiresAt time.Time
}

var eurRateCache = struct {
	sync.Mutex
	Items map[int]eurRateCacheEntry
}{Items: make(map[int]eurRateCacheEntry)}

func parseCBRDecimal(value string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
}

func parseEURMonthlyRates(reader io.Reader) (map[int]float64, error) {
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) {
		return charmap.Windows1251.NewDecoder().Reader(input), nil
	}
	var response cbrCurrencyResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	sums := make(map[int]float64)
	counts := make(map[int]int)
	for _, record := range response.Records {
		date, err := time.Parse("02.01.2006", record.Date)
		if err != nil {
			continue
		}
		nominal, err := parseCBRDecimal(record.Nominal)
		if err != nil || nominal == 0 {
			continue
		}
		value, err := parseCBRDecimal(record.Value)
		if err != nil {
			continue
		}
		sums[int(date.Month())] += value / nominal
		counts[int(date.Month())]++
	}
	rates := make(map[int]float64, len(sums))
	for month, sum := range sums {
		if counts[month] > 0 {
			rates[month] = sum / float64(counts[month])
		}
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("ЦБ РФ не вернул курсы EUR")
	}
	return rates, nil
}

func loadEURMonthlyRates(year int) (map[int]float64, error) {
	eurRateCache.Lock()
	if cached, ok := eurRateCache.Items[year]; ok && time.Now().Before(cached.ExpiresAt) {
		eurRateCache.Unlock()
		return cached.Rates, nil
	}
	eurRateCache.Unlock()

	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	if year == now.Year() && now.Before(end) {
		end = now
	}
	if year > now.Year() {
		return nil, fmt.Errorf("курсы EUR за %d год ещё недоступны", year)
	}
	params := url.Values{
		"date_req1": {start.Format("02/01/2006")},
		"date_req2": {end.Format("02/01/2006")},
		"VAL_NM_RQ": {cbrEURCurrencyID},
	}
	request, err := http.NewRequest(http.MethodGet, "https://www.cbr.ru/scripts/XML_dynamic.asp?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "AnalyticsPortal/1.0")
	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ЦБ РФ вернул HTTP %d", response.StatusCode)
	}
	rates, err := parseEURMonthlyRates(response.Body)
	if err != nil {
		return nil, err
	}
	eurRateCache.Lock()
	eurRateCache.Items[year] = eurRateCacheEntry{Rates: rates, ExpiresAt: time.Now().Add(6 * time.Hour)}
	eurRateCache.Unlock()
	return rates, nil
}

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
		"year":        getDistinct("SELECT DISTINCT CONVERT(varchar(4), [year]) FROM dbo.tbl_EcomSalesNormalized WHERE [year] IS NOT NULL ORDER BY CONVERT(varchar(4), [year])"),
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

// GetSalesNetworkOptions возвращает только сети, для которых есть данные при
// текущем наборе остальных фильтров. Выбранные сети намеренно не включаются в
// WHERE, чтобы список можно было безопасно пересчитать до их применения.
func GetSalesNetworkOptions(c *gin.Context) {
	unit := strings.TrimSpace(c.DefaultQuery("unit", "руб"))
	segments := append(c.QueryArray("focusSegments"), c.Query("focusSegment"))
	segments = uniqueNonEmptyStrings(segments, 0)
	if len(segments) == 0 {
		segments = []string{"OLAP SS"}
	}
	if unit != "руб" && unit != "уп" && unit != "евро" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit должен быть 'руб', 'евро' или 'уп'"})
		return
	}
	dbUnit := unit
	if unit == "евро" {
		dbUnit = "руб"
	}

	where, args := buildSalesWhere(salesFilter{
		YearFromStr:  c.Query("yearFrom"),
		YearToStr:    c.Query("yearTo"),
		Months:       c.QueryArray("months"),
		Quarters:     c.QueryArray("quarters"),
		BrandNames:   c.QueryArray("brandName"),
		ProductNames: c.QueryArray("productName"),
		UnRubs:       []string{dbUnit},
		Segments:     segments,
	})
	rows, err := config.DB.Query("SELECT DISTINCT n.networkName FROM dbo.tbl_EcomSalesNormalized n"+where+" AND n.networkName IS NOT NULL ORDER BY n.networkName", args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Network options query failed"})
		return
	}
	defer rows.Close()

	networks := make([]string, 0)
	for rows.Next() {
		var network string
		if scanErr := rows.Scan(&network); scanErr == nil && network != "" {
			networks = append(networks, network)
		}
	}
	c.JSON(http.StatusOK, gin.H{"networkName": networks})
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

type salesDashboardFocusPoint struct {
	Type  string  `json:"type"`
	Name  string  `json:"name"`
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Value float64 `json:"value"`
}

type salesDashboardNetworkBreakdown struct {
	Network string  `json:"network"`
	Channel string  `json:"channel"`
	Segment string  `json:"segment"`
	Value   float64 `json:"value"`
}

type salesDashboardMetricComparison struct {
	Current  float64 `json:"current"`
	Previous float64 `json:"previous"`
}

type salesDashboardDriver struct {
	Name         string   `json:"name"`
	Current      float64  `json:"current"`
	Previous     float64  `json:"previous"`
	Delta        float64  `json:"delta"`
	DeltaPercent *float64 `json:"deltaPercent"`
}

type salesDashboardRankDetail struct {
	Name       string   `json:"name"`
	Value      float64  `json:"value"`
	Previous   float64  `json:"previous"`
	YoYPercent *float64 `json:"yoyPercent"`
	Share      float64  `json:"share"`
	Rank       int      `json:"rank"`
	RankChange int      `json:"rankChange"`
}

type salesDimensionValue struct {
	Name     string
	Current  float64
	Previous float64
}

type salesDashboardEcomShare struct {
	Applicable    bool     `json:"applicable"`
	Family        string   `json:"family"`
	Full          float64  `json:"full"`
	WithoutEcom   float64  `json:"withoutEcom"`
	Ecom          float64  `json:"ecom"`
	Share         *float64 `json:"share"`
	PreviousFull  float64  `json:"previousFull"`
	PreviousEcom  float64  `json:"previousEcom"`
	PreviousShare *float64 `json:"previousShare"`
}

func uniqueNonEmptyStrings(values []string, limit int) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func buildDimensionViews(values []salesDimensionValue) ([]salesDashboardDriver, []salesDashboardRankDetail) {
	drivers := make([]salesDashboardDriver, 0, len(values))
	currentTotal := 0.0
	for _, item := range values {
		currentTotal += item.Current
		delta := item.Current - item.Previous
		var deltaPercent *float64
		if item.Previous != 0 {
			value := delta / item.Previous * 100
			deltaPercent = &value
		}
		drivers = append(drivers, salesDashboardDriver{
			Name:         item.Name,
			Current:      item.Current,
			Previous:     item.Previous,
			Delta:        delta,
			DeltaPercent: deltaPercent,
		})
	}
	sort.SliceStable(drivers, func(i, j int) bool {
		return math.Abs(drivers[i].Delta) > math.Abs(drivers[j].Delta)
	})
	if len(drivers) > 12 {
		drivers = drivers[:12]
	}

	currentOrder := append([]salesDimensionValue(nil), values...)
	sort.SliceStable(currentOrder, func(i, j int) bool { return currentOrder[i].Current > currentOrder[j].Current })
	previousOrder := append([]salesDimensionValue(nil), values...)
	sort.SliceStable(previousOrder, func(i, j int) bool { return previousOrder[i].Previous > previousOrder[j].Previous })
	previousRanks := make(map[string]int, len(previousOrder))
	for index, item := range previousOrder {
		if item.Previous > 0 {
			previousRanks[item.Name] = index + 1
		}
	}

	ranking := make([]salesDashboardRankDetail, 0, 10)
	for index, item := range currentOrder {
		if item.Current <= 0 || len(ranking) >= 10 {
			continue
		}
		var yoyPercent *float64
		if item.Previous != 0 {
			value := (item.Current - item.Previous) / item.Previous * 100
			yoyPercent = &value
		}
		share := 0.0
		if currentTotal != 0 {
			share = item.Current / currentTotal * 100
		}
		previousRank := previousRanks[item.Name]
		rankChange := 0
		if previousRank > 0 {
			rankChange = previousRank - (index + 1)
		}
		ranking = append(ranking, salesDashboardRankDetail{
			Name:       item.Name,
			Value:      item.Current,
			Previous:   item.Previous,
			YoYPercent: yoyPercent,
			Share:      share,
			Rank:       index + 1,
			RankChange: rankChange,
		})
	}
	return drivers, ranking
}

// GetSalesDashboard возвращает агрегаты выбранных сегментов одного канала и
// одной единицы измерения. Набор сегментов формируется из справочника канала.
func GetSalesDashboard(c *gin.Context) {
	segments := append(c.QueryArray("focusSegments"), c.Query("focusSegment"))
	segments = uniqueNonEmptyStrings(segments, 0)
	if len(segments) == 0 {
		segments = []string{"OLAP SS"}
	}
	channel := strings.TrimSpace(c.Query("focusChannel"))
	unit := strings.TrimSpace(c.DefaultQuery("unit", "руб"))
	focusProducts := append(c.QueryArray("focusProducts"), c.Query("focusProduct"))
	focusNetworks := append(c.QueryArray("focusNetworks"), c.Query("focusNetwork"))
	focusProducts = uniqueNonEmptyStrings(focusProducts, 5)
	focusNetworks = uniqueNonEmptyStrings(focusNetworks, 5)
	compareChannels := uniqueNonEmptyStrings(c.QueryArray("compareChannels"), 5)
	if unit != "руб" && unit != "уп" && unit != "евро" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit должен быть 'руб', 'евро' или 'уп'"})
		return
	}
	dbUnit := unit
	if unit == "евро" {
		dbUnit = "руб"
	}

	analysisYear, parseErr := strconv.Atoi(strings.TrimSpace(c.Query("analysisYear")))
	if parseErr != nil || analysisYear < 2000 || analysisYear > 2100 {
		analysisYear, parseErr = strconv.Atoi(strings.TrimSpace(c.Query("yearTo")))
	}
	if parseErr != nil || analysisYear < 2000 || analysisYear > 2100 {
		analysisYear, parseErr = strconv.Atoi(strings.TrimSpace(c.Query("yearFrom")))
	}
	if parseErr != nil || analysisYear < 2000 || analysisYear > 2100 {
		if err := config.DB.QueryRow("SELECT COALESCE(MAX([year]), YEAR(GETDATE())) FROM dbo.tbl_EcomSalesNormalized").Scan(&analysisYear); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard year query failed"})
			return
		}
	}

	eurRates := make(map[int]map[int]float64)
	if unit == "евро" {
		for _, year := range []int{analysisYear - 1, analysisYear} {
			rates, err := loadEURMonthlyRates(year)
			if err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Не удалось загрузить официальные курсы EUR ЦБ РФ", "details": err.Error()})
				return
			}
			eurRates[year] = rates
		}
	}
	convertValue := func(value float64, year, month int) (float64, error) {
		if unit != "евро" {
			return value, nil
		}
		rate := eurRates[year][month]
		if rate <= 0 {
			return 0, fmt.Errorf("нет курса EUR за %02d.%d", month, year)
		}
		return value / rate, nil
	}

	baseFilter := salesFilter{
		YearFromStr:  strconv.Itoa(analysisYear),
		YearToStr:    strconv.Itoa(analysisYear),
		Months:       c.QueryArray("months"),
		Quarters:     c.QueryArray("quarters"),
		BrandNames:   c.QueryArray("brandName"),
		ProductNames: c.QueryArray("productName"),
		NetworkNames: c.QueryArray("networkName"),
		UnRubs:       []string{dbUnit},
		Segments:     segments,
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
				point.Value, err = convertValue(point.Value, point.Year, point.Month)
				if err != nil {
					return nil, err
				}
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
	previousYearFilter := baseFilter
	previousYearFilter.YearFromStr = strconv.Itoa(analysisYear - 1)
	previousYearFilter.YearToStr = strconv.Itoa(analysisYear - 1)
	previousYearWhere, previousYearArgs := buildSalesWhere(previousYearFilter)
	previousYearTrend, err := loadTrend(previousYearWhere, previousYearArgs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard previous year trend query failed"})
		return
	}
	if unit == "евро" {
		total = 0
		for _, point := range trend {
			total += point.Value
		}
	}

	loadMetricComparison := func(metricUnit string) (salesDashboardMetricComparison, error) {
		metricFilter := baseFilter
		metricFilter.YearFromStr = ""
		metricFilter.YearToStr = ""
		metricFilter.UnRubs = []string{metricUnit}
		metricWhere, metricArgs := buildSalesWhere(metricFilter)
		queryArgs := []interface{}{analysisYear, analysisYear - 1}
		queryArgs = append(queryArgs, metricArgs...)
		var result salesDashboardMetricComparison
		err := config.DB.QueryRow(`SELECT
			COALESCE(SUM(CASE WHEN n.[year] = ? THEN n.metric_value ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.[year] = ? THEN n.metric_value ELSE 0 END), 0)
			FROM dbo.tbl_EcomSalesNormalized n`+metricWhere, queryArgs...).Scan(&result.Current, &result.Previous)
		return result, err
	}
	rubComparison, err := loadMetricComparison("руб")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard rub comparison query failed"})
		return
	}
	unitsComparison, err := loadMetricComparison("уп")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard units comparison query failed"})
		return
	}
	eurComparison := salesDashboardMetricComparison{}
	for _, point := range trend {
		if unit == "евро" {
			eurComparison.Current += point.Value
		} else if rates := eurRates[point.Year]; rates != nil && rates[point.Month] > 0 {
			eurComparison.Current += point.Value / rates[point.Month]
		}
	}
	for _, point := range previousYearTrend {
		if unit == "евро" {
			eurComparison.Previous += point.Value
		} else if rates := eurRates[point.Year]; rates != nil && rates[point.Month] > 0 {
			eurComparison.Previous += point.Value / rates[point.Month]
		}
	}

	loadDimensionValues := func(column string) ([]salesDimensionValue, error) {
		dimensionFilter := baseFilter
		dimensionFilter.YearFromStr = strconv.Itoa(analysisYear - 1)
		dimensionFilter.YearToStr = strconv.Itoa(analysisYear)
		dimensionWhere, dimensionArgs := buildSalesWhere(dimensionFilter)
		query := `SELECT n.` + column + `, n.[year], n.[month], SUM(n.metric_value)
			FROM dbo.tbl_EcomSalesNormalized n` + dimensionWhere +
			` AND n.` + column + ` IS NOT NULL
			GROUP BY n.` + column + `, n.[year], n.[month]`
		rows, queryErr := config.DB.Query(query, dimensionArgs...)
		if queryErr != nil {
			return nil, queryErr
		}
		defer rows.Close()
		values := make(map[string]*salesDimensionValue)
		for rows.Next() {
			var name string
			var year, month int
			var value float64
			if scanErr := rows.Scan(&name, &year, &month, &value); scanErr == nil && name != "" {
				value, scanErr = convertValue(value, year, month)
				if scanErr != nil {
					return nil, scanErr
				}
				item := values[name]
				if item == nil {
					item = &salesDimensionValue{Name: name}
					values[name] = item
				}
				if year == analysisYear {
					item.Current += value
				} else if year == analysisYear-1 {
					item.Previous += value
				}
			}
		}
		result := make([]salesDimensionValue, 0, len(values))
		for _, item := range values {
			result = append(result, *item)
		}
		return result, rows.Err()
	}
	networkValues, err := loadDimensionValues("networkName")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard network analytics query failed"})
		return
	}
	productValues, err := loadDimensionValues("productName")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard product analytics query failed"})
		return
	}
	networkDrivers, networkRanking := buildDimensionViews(networkValues)
	productDrivers, productRanking := buildDimensionViews(productValues)

	ecomShare := salesDashboardEcomShare{}
	ecomFamily := ""
	switch channel {
	case "OLAP SS", "OLAP SS wo Ecom":
		ecomFamily = "OLAP SS"
	case "OLAP NW", "OLAP NW wo Ecom":
		ecomFamily = "OLAP NW"
	}
	if ecomFamily != "" {
		ecomShare.Applicable = true
		ecomShare.Family = ecomFamily
		withoutEcomSegment := ecomFamily + " wo Ecom"
		ecomFilter := baseFilter
		ecomFilter.YearFromStr = strconv.Itoa(analysisYear - 1)
		ecomFilter.YearToStr = strconv.Itoa(analysisYear)
		ecomFilter.Segments = []string{ecomFamily, withoutEcomSegment}
		ecomWhere, ecomArgs := buildSalesWhere(ecomFilter)
		ecomRows, queryErr := config.DB.Query(`SELECT n.segment, n.[year], n.[month], SUM(n.metric_value)
			FROM dbo.tbl_EcomSalesNormalized n`+ecomWhere+`
			GROUP BY n.segment, n.[year], n.[month]`, ecomArgs...)
		if queryErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard Ecom share query failed"})
			return
		}
		for ecomRows.Next() {
			var segmentName string
			var year, month int
			var value float64
			if scanErr := ecomRows.Scan(&segmentName, &year, &month, &value); scanErr == nil {
				value, scanErr = convertValue(value, year, month)
				if scanErr != nil {
					ecomRows.Close()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard Ecom currency conversion failed"})
					return
				}
				if year == analysisYear {
					if segmentName == ecomFamily {
						ecomShare.Full += value
					} else if segmentName == withoutEcomSegment {
						ecomShare.WithoutEcom += value
					}
				} else if year == analysisYear-1 {
					if segmentName == ecomFamily {
						ecomShare.PreviousFull += value
					} else if segmentName == withoutEcomSegment {
						ecomShare.PreviousEcom -= value
					}
				}
			}
		}
		ecomRows.Close()
		ecomShare.Ecom = ecomShare.Full - ecomShare.WithoutEcom
		ecomShare.PreviousEcom += ecomShare.PreviousFull
		if ecomShare.Full != 0 {
			share := ecomShare.Ecom / ecomShare.Full * 100
			ecomShare.Share = &share
		}
		if ecomShare.PreviousFull != 0 {
			share := ecomShare.PreviousEcom / ecomShare.PreviousFull * 100
			ecomShare.PreviousShare = &share
		}
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
			converted, convertErr := convertValue(value, year, month)
			if convertErr != nil {
				return nil
			}
			return &converted
		}

		previousYear, previousMonth := latestYear, latestMonth-1
		if previousMonth == 0 {
			previousYear--
			previousMonth = 12
		}
		previousPeriodValue = loadPeriodValue(previousYear, previousMonth)
		yearAgoValue = loadPeriodValue(latestYear-1, latestMonth)
	}

	focusTrends := make([]salesDashboardFocusPoint, 0)
	loadFocusTrends := func(column, focusType string, names []string) error {
		if len(names) == 0 {
			return nil
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")
		queryArgs := append([]interface{}{}, args...)
		for _, name := range names {
			queryArgs = append(queryArgs, name)
		}
		query := "SELECT n." + column + ", n.[year], n.[month], SUM(n.metric_value) FROM dbo.tbl_EcomSalesNormalized n" + where + " AND n." + column + " IN (" + placeholders + ") GROUP BY n." + column + ", n.[year], n.[month] ORDER BY n.[year], n.[month], n." + column
		rows, queryErr := config.DB.Query(query, queryArgs...)
		if queryErr != nil {
			return queryErr
		}
		defer rows.Close()
		for rows.Next() {
			var point salesDashboardFocusPoint
			point.Type = focusType
			if scanErr := rows.Scan(&point.Name, &point.Year, &point.Month, &point.Value); scanErr == nil {
				point.Value, scanErr = convertValue(point.Value, point.Year, point.Month)
				if scanErr != nil {
					return scanErr
				}
				focusTrends = append(focusTrends, point)
			}
		}
		return rows.Err()
	}
	if err = loadFocusTrends("productName", "product", focusProducts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard product focus query failed"})
		return
	}
	if err = loadFocusTrends("networkName", "network", focusNetworks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard network focus query failed"})
		return
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

	topNetworks := make([]salesDashboardRank, 0, 8)
	topProducts := make([]salesDashboardRank, 0, 8)
	if unit == "евро" {
		for index, item := range networkRanking {
			if index >= 8 {
				break
			}
			topNetworks = append(topNetworks, salesDashboardRank{Name: item.Name, Value: item.Value})
		}
		for index, item := range productRanking {
			if index >= 8 {
				break
			}
			topProducts = append(topProducts, salesDashboardRank{Name: item.Name, Value: item.Value})
		}
	} else {
		topNetworks, err = loadRank("networkName")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard networks query failed"})
			return
		}
		topProducts, err = loadRank("productName")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard products query failed"})
			return
		}
	}

	channelSegments := make([]string, 0)
	if channel != "" {
		segmentRows, queryErr := config.DB.Query("SELECT DISTINCT segment FROM dbo.tbl_ChannelSegmentMapping WHERE channel = ? AND un_rub = ? AND segment IS NOT NULL ORDER BY segment", channel, dbUnit)
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
		channelSegments = append(channelSegments, segments...)
	}

	segmentFilter := baseFilter
	segmentFilter.Segments = channelSegments
	segmentWhere, segmentArgs := buildSalesWhere(segmentFilter)
	segmentRows, err := config.DB.Query("SELECT n.segment, n.[year], n.[month], SUM(n.metric_value) AS total_value FROM dbo.tbl_EcomSalesNormalized n"+segmentWhere+" GROUP BY n.segment, n.[year], n.[month]", segmentArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard segment totals query failed"})
		return
	}
	segmentValues := make(map[string]float64)
	for segmentRows.Next() {
		var name string
		var year, month int
		var value float64
		if scanErr := segmentRows.Scan(&name, &year, &month, &value); scanErr == nil {
			value, scanErr = convertValue(value, year, month)
			if scanErr != nil {
				segmentRows.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard segment currency conversion failed"})
				return
			}
			segmentValues[name] += value
		}
	}
	segmentRows.Close()
	segmentTotals := make([]salesDashboardRank, 0, len(segmentValues))
	for name, value := range segmentValues {
		segmentTotals = append(segmentTotals, salesDashboardRank{Name: name, Value: value})
	}
	sort.SliceStable(segmentTotals, func(i, j int) bool { return segmentTotals[i].Value > segmentTotals[j].Value })

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
				point.Value, scanErr = convertValue(point.Value, point.Year, point.Month)
				if scanErr != nil {
					networkRows.Close()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard network currency conversion failed"})
					return
				}
				networkTrends = append(networkTrends, point)
			}
		}
		networkRows.Close()
	}

	channelTrends := make([]salesDashboardSeriesPoint, 0)
	if len(compareChannels) > 0 {
		channelFilter := baseFilter
		channelFilter.Segments = nil
		channelWhere, channelArgs := buildSalesWhere(channelFilter)
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(compareChannels)), ",")
		for _, compareChannel := range compareChannels {
			channelArgs = append(channelArgs, compareChannel)
		}
		channelQuery := `SELECT m.channel, n.[year], n.[month], SUM(n.metric_value)
			FROM dbo.tbl_EcomSalesNormalized n
			INNER JOIN dbo.tbl_ChannelSegmentMapping m ON m.segment = n.segment AND m.un_rub = n.un_rub` +
			channelWhere + " AND m.channel IN (" + placeholders + ") GROUP BY m.channel, n.[year], n.[month] ORDER BY n.[year], n.[month], m.channel"
		channelRows, queryErr := config.DB.Query(channelQuery, channelArgs...)
		if queryErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard channel comparison query failed"})
			return
		}
		for channelRows.Next() {
			var point salesDashboardSeriesPoint
			if scanErr := channelRows.Scan(&point.Name, &point.Year, &point.Month, &point.Value); scanErr == nil {
				point.Value, scanErr = convertValue(point.Value, point.Year, point.Month)
				if scanErr != nil {
					channelRows.Close()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard channel currency conversion failed"})
					return
				}
				channelTrends = append(channelTrends, point)
			}
		}
		channelRows.Close()
	}

	networkBreakdown := make([]salesDashboardNetworkBreakdown, 0)
	if len(focusNetworks) > 0 || len(baseFilter.NetworkNames) > 0 {
		breakdownFilter := baseFilter
		breakdownFilter.Segments = nil
		breakdownWhere, breakdownArgs := buildSalesWhere(breakdownFilter)
		exactNetworkCondition := ""
		if len(focusNetworks) > 0 {
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(focusNetworks)), ",")
			exactNetworkCondition = " AND n.networkName IN (" + placeholders + ")"
			for _, network := range focusNetworks {
				breakdownArgs = append(breakdownArgs, network)
			}
		}
		breakdownQuery := `SELECT n.networkName, n.channel, n.segment, n.[year], n.[month], SUM(n.metric_value) AS total_value
			FROM dbo.tbl_EcomSalesNormalized n` + breakdownWhere + exactNetworkCondition +
			" GROUP BY n.networkName, n.channel, n.segment, n.[year], n.[month]"
		breakdownRows, queryErr := config.DB.Query(breakdownQuery, breakdownArgs...)
		if queryErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard network breakdown query failed"})
			return
		}
		breakdownValues := make(map[string]*salesDashboardNetworkBreakdown)
		for breakdownRows.Next() {
			var network, channelName, segmentName string
			var year, month int
			var value float64
			if scanErr := breakdownRows.Scan(&network, &channelName, &segmentName, &year, &month, &value); scanErr == nil {
				value, scanErr = convertValue(value, year, month)
				if scanErr != nil {
					breakdownRows.Close()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Dashboard breakdown currency conversion failed"})
					return
				}
				key := network + "\x00" + channelName + "\x00" + segmentName
				item := breakdownValues[key]
				if item == nil {
					item = &salesDashboardNetworkBreakdown{Network: network, Channel: channelName, Segment: segmentName}
					breakdownValues[key] = item
				}
				item.Value += value
			}
		}
		breakdownRows.Close()
		for _, item := range breakdownValues {
			networkBreakdown = append(networkBreakdown, *item)
		}
		sort.SliceStable(networkBreakdown, func(i, j int) bool { return networkBreakdown[i].Value > networkBreakdown[j].Value })
		if len(networkBreakdown) > 16 {
			networkBreakdown = networkBreakdown[:16]
		}
	}

	average := 0.0
	if periods > 0 {
		average = total / float64(periods)
	}
	c.JSON(http.StatusOK, gin.H{
		"analysisYear":    analysisYear,
		"channel":         channel,
		"channelSegments": channelSegments,
		"segment":         strings.Join(segments, ", "),
		"segments":        segments,
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
		"trend":             trend,
		"previousYearTrend": previousYearTrend,
		"metricComparisons": gin.H{
			"rub":   rubComparison,
			"eur":   eurComparison,
			"units": unitsComparison,
		},
		"currencySource": func() string {
			if unit == "евро" {
				return "ЦБ РФ · средний официальный курс EUR за месяц"
			}
			return ""
		}(),
		"ecomShare":        ecomShare,
		"networkDrivers":   networkDrivers,
		"productDrivers":   productDrivers,
		"networkRanking":   networkRanking,
		"productRanking":   productRanking,
		"focusTrends":      focusTrends,
		"topNetworks":      topNetworks,
		"topProducts":      topProducts,
		"segmentTotals":    segmentTotals,
		"networkTrends":    networkTrends,
		"channelTrends":    channelTrends,
		"networkBreakdown": networkBreakdown,
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
