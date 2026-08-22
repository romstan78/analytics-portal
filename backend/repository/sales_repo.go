package repository

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"backend/config"
	"backend/models"

	sq "github.com/Masterminds/squirrel"
)

// Доступ к dbo.tbl_EcomSalesNormalized. Слой возвращает сырые агрегаты по
// периодам: пересчёт в валюту и построение витрин — забота services.

const salesTable = "dbo.tbl_EcomSalesNormalized"

// salesRowColumns — порядок колонок, под который написан scanSalesRow.
const salesRowColumns = `n.id, n.[year], n.[month], n.brandName, n.productName,
	n.networkName, n.metric_type, n.metric_value, n.un_rub, n.segment, n.channel, n.updated_at`

// salesRowOrder — порядок выборки строк интернет-продаж.
// Тай-брейк по первичному ключу n.id обязателен: year/month/metric_type
// совпадают у множества строк, и без него SQL Server волен возвращать их
// в произвольном, меняющемся между запросами порядке. При постраничном
// просмотре OFFSET/FETCH применяется к каждый раз новому порядку, поэтому
// одна строка может попасть на две страницы, а другая — ни на одну.
const salesRowOrder = " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type, n.id"

// salesDrilldownGroupBy — группировка агрегатов drilldown.
const salesDrilldownGroupBy = " GROUP BY n.[year], n.[month], n.metric_type, n.un_rub, n.segment, n.channel"

// salesDrilldownOrder — порядок выборки агрегатов drilldown.
// n.id здесь неприменим: запрос агрегирующий, в ORDER BY допустимы только
// колонки из GROUP BY (salesDrilldownGroupBy). Полный набор колонок
// группировки уникален для каждой строки результата и задаёт стабильный
// порядок без обращения к первичному ключу.
const salesDrilldownOrder = " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type, n.un_rub, n.segment, n.channel"

// salesDimensionColumns — колонки, которые разрешено подставлять в SQL как имя
// измерения. Значение приходит из query-параметров, поэтому список закрытый.
var salesDimensionColumns = map[string]struct{}{
	"networkName": {},
	"productName": {},
	"brandName":   {},
	"segment":     {},
	"channel":     {},
}

func checkDimension(column string) error {
	if _, ok := salesDimensionColumns[column]; !ok {
		return fmt.Errorf("недопустимое измерение %q", column)
	}
	return nil
}

// SalesFilter — параметры фильтрации интернет-продаж.
type SalesFilter struct {
	YearFromStr  string
	YearToStr    string
	Months       []string
	Quarters     []string
	BrandNames   []string // LIKE-фильтр (список строк, GetData/экспорт)
	ProductNames []string
	NetworkNames []string // LIKE-фильтр
	UnRubs       []string
	Segments     []string
	Channels     []string
	Search       string
	// Точное совпадение (Drilldown)
	BrandExact   string
	ProductExact string
	NetworkExact string
}

// SalesMonthlyRow — сумма по измерению за конкретный месяц.
// Имя пустое там, где группировка идёт только по периоду.
type SalesMonthlyRow struct {
	Name  string
	Year  int
	Month int
	Value float64
}

// SalesBreakdownRow — сумма сети в разрезе канала и сегмента за месяц.
type SalesBreakdownRow struct {
	Network string
	Channel string
	Segment string
	Year    int
	Month   int
	Value   float64
}

// SalesSummaryRow — агрегаты шапки дашборда.
type SalesSummaryRow struct {
	Total          float64
	ActiveNetworks int
	ActiveProducts int
	Periods        int
}

// BuildSalesWhere строит WHERE-условие и аргументы для tbl_EcomSalesNormalized.
// Возвращает строку, начинающуюся с " WHERE ...", и аргументы в порядке появления.
func BuildSalesWhere(f SalesFilter) (string, []interface{}) {
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
	likeAny := func(column string, values []string) {
		if len(values) == 0 {
			return
		}
		orConds := sq.Or{}
		for _, v := range values {
			if v != "" {
				orConds = append(orConds, sq.Like{column: "%" + v + "%"})
			}
		}
		if len(orConds) > 0 {
			q = q.Where(orConds)
		}
	}
	likeAny("n.brandName", f.BrandNames)
	likeAny("n.productName", f.ProductNames)
	likeAny("n.networkName", f.NetworkNames)

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
	inAny := func(column string, values []string) {
		if len(values) == 0 {
			return
		}
		if vals := filterNonEmpty(values); len(vals) > 0 {
			q = q.Where(sq.Eq{column: vals})
		}
	}
	inAny("n.un_rub", f.UnRubs)
	inAny("n.segment", f.Segments)
	inAny("n.channel", f.Channels)

	if f.Search != "" {
		likeArg := "%" + f.Search + "%"
		q = q.Where(sq.Or{
			sq.Like{"n.brandName": likeArg},
			sq.Like{"n.productName": likeArg},
			sq.Like{"n.networkName": likeArg},
			sq.Like{"n.metric_type": likeArg},
		})
	}

	sqlText, args, err := q.ToSql()
	if err != nil {
		config.Logger.Error("buildSalesWhere_ToSql_failed", "error", err.Error())
		return "", nil
	}
	// squirrel генерирует "SELECT 1 WHERE ...", нам нужна только WHERE-часть
	whereIdx := strings.Index(sqlText, "WHERE")
	if whereIdx >= 0 {
		return " " + sqlText[whereIdx:], args
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

// placeholders возвращает "?,?,?" нужной длины.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// ─── Справочники фильтров ───────────────────────────────────────────────────

// salesDistinct возвращает непустые значения колонки; ошибка запроса даёт
// пустой список — панель фильтров должна открываться и без части справочников.
func salesDistinct(query string) []string {
	rows, err := config.DB.Query(query)
	if err != nil {
		config.Logger.Error("sales_distinct_failed", "error", err.Error())
		return []string{}
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
			values = append(values, v.String)
		}
	}
	return values
}

// SalesFilterOptions возвращает справочники для панели фильтров.
func SalesFilterOptions() models.SalesFilterOptions {
	distinctOf := func(expr string) string {
		return "SELECT DISTINCT " + expr + " FROM " + salesTable +
			" WHERE " + expr + " IS NOT NULL ORDER BY " + expr
	}
	options := models.SalesFilterOptions{
		Year:        salesDistinct(distinctOf("CONVERT(varchar(4), [year])")),
		BrandName:   salesDistinct(distinctOf("brandName")),
		ProductName: salesDistinct(distinctOf("productName")),
		NetworkName: salesDistinct(distinctOf("networkName")),
		UnRub:       salesDistinct(distinctOf("un_rub")),
		Segment:     salesDistinct(distinctOf("segment")),
		Channel:     salesDistinct(distinctOf("channel")),
	}
	options.SegmentChannelMap, options.ChannelSegmentMap = salesChannelSegmentMaps()
	return options
}

// salesChannelSegmentMaps возвращает связку сегмент↔канал в обе стороны.
func salesChannelSegmentMaps() (map[string][]string, map[string][]string) {
	segChan := make(map[string][]string)
	chanSeg := make(map[string][]string)

	rows, err := config.DB.Query(`SELECT segment, channel FROM dbo.tbl_ChannelSegmentMapping
		WHERE segment IS NOT NULL AND channel IS NOT NULL
		GROUP BY segment, channel ORDER BY segment, channel`)
	if err != nil {
		config.Logger.Error("sales_channel_map_failed", "error", err.Error())
		return segChan, chanSeg
	}
	defer rows.Close()

	for rows.Next() {
		var segment, channel sql.NullString
		if err := rows.Scan(&segment, &channel); err != nil {
			continue
		}
		if segment.Valid && channel.Valid && segment.String != "" && channel.String != "" {
			segChan[segment.String] = append(segChan[segment.String], channel.String)
			chanSeg[channel.String] = append(chanSeg[channel.String], segment.String)
		}
	}
	return segChan, chanSeg
}

// SalesNetworkNames возвращает сети, по которым есть данные при заданном фильтре.
func SalesNetworkNames(f SalesFilter) ([]string, error) {
	where, args := BuildSalesWhere(f)
	rows, err := config.DB.Query("SELECT DISTINCT n.networkName FROM "+salesTable+" n"+
		where+" AND n.networkName IS NOT NULL ORDER BY n.networkName", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	networks := make([]string, 0)
	for rows.Next() {
		var network string
		if err := rows.Scan(&network); err == nil && network != "" {
			networks = append(networks, network)
		}
	}
	return networks, rows.Err()
}

// SalesChannelSegments возвращает сегменты канала для выбранной единицы измерения.
func SalesChannelSegments(channel, unit string) ([]string, error) {
	rows, err := config.DB.Query(`SELECT DISTINCT segment FROM dbo.tbl_ChannelSegmentMapping
		WHERE channel = ? AND un_rub = ? AND segment IS NOT NULL ORDER BY segment`, channel, unit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	segments := make([]string, 0)
	for rows.Next() {
		var segment string
		if err := rows.Scan(&segment); err == nil && segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments, rows.Err()
}

// ─── Строки ─────────────────────────────────────────────────────────────────

// scanSalesRow читает строку в порядке salesRowColumns.
func scanSalesRow(rows *sql.Rows) (models.Row, error) {
	var r models.Row
	err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.BrandName, &r.ProductName, &r.NetworkName,
		&r.MetricType, &r.MetricValue, &r.UnRub, &r.Segment, &r.Channel, &r.UpdatedAt)
	return r, err
}

// SalesRowsCount возвращает число строк, попадающих под фильтр.
func SalesRowsCount(f SalesFilter) (int, error) {
	where, args := BuildSalesWhere(f)
	var total int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM "+salesTable+" n"+where, args...).Scan(&total)
	return total, err
}

// SalesRowsPage возвращает страницу выборки.
func SalesRowsPage(f SalesFilter, offset, limit int) ([]models.Row, error) {
	where, args := BuildSalesWhere(f)
	query := "SELECT " + salesRowColumns + " FROM " + salesTable + " n" + where +
		salesRowOrder + " OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
	args = append(args, offset, limit)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []models.Row{}
	for rows.Next() {
		r, scanErr := scanSalesRow(rows)
		if scanErr != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SalesRowsCursor открывает курсор по всей выборке. Вызывающий обязан закрыть
// результат и читать его через ScanSalesRow: выгрузки не помещаются в память.
func SalesRowsCursor(f SalesFilter) (*sql.Rows, error) {
	where, args := BuildSalesWhere(f)
	return config.DB.Query("SELECT "+salesRowColumns+" FROM "+salesTable+" n"+where+salesRowOrder, args...)
}

// ScanSalesRow — публичная обёртка над scanSalesRow для потоковой выдачи.
func ScanSalesRow(rows *sql.Rows) (models.Row, error) {
	return scanSalesRow(rows)
}

// SalesDrilldown возвращает разбивку по периодам и метрикам.
func SalesDrilldown(f SalesFilter) ([]models.DrilldownRow, error) {
	where, args := BuildSalesWhere(f)
	query := `SELECT n.[year], n.[month], n.metric_type, SUM(n.metric_value) AS total_value,
		n.un_rub, n.segment, n.channel FROM ` + salesTable + ` n` + where +
		salesDrilldownGroupBy + salesDrilldownOrder

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []models.DrilldownRow{}
	for rows.Next() {
		var r models.DrilldownRow
		if err := rows.Scan(&r.Year, &r.Month, &r.MetricType, &r.TotalValue, &r.UnRub, &r.Segment, &r.Channel); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ─── Агрегаты дашборда ──────────────────────────────────────────────────────

// SalesLatestYear возвращает последний год с данными; при пустой таблице — текущий.
func SalesLatestYear() (int, error) {
	var year int
	err := config.DB.QueryRow("SELECT COALESCE(MAX([year]), YEAR(GETDATE())) FROM " + salesTable).Scan(&year)
	return year, err
}

// SalesSummary возвращает сумму и охват выборки.
func SalesSummary(f SalesFilter) (SalesSummaryRow, error) {
	where, args := BuildSalesWhere(f)
	var s SalesSummaryRow
	err := config.DB.QueryRow(`SELECT COALESCE(SUM(n.metric_value), 0),
		COUNT(DISTINCT n.networkName), COUNT(DISTINCT n.productName),
		COUNT(DISTINCT CONCAT(n.[year], '-', n.[month]))
		FROM `+salesTable+` n`+where, args...).
		Scan(&s.Total, &s.ActiveNetworks, &s.ActiveProducts, &s.Periods)
	return s, err
}

// SalesMonthlyTotals возвращает помесячные суммы без разбивки по измерению.
func SalesMonthlyTotals(f SalesFilter) ([]SalesMonthlyRow, error) {
	where, args := BuildSalesWhere(f)
	rows, err := config.DB.Query("SELECT n.[year], n.[month], SUM(n.metric_value) FROM "+salesTable+" n"+
		where+" GROUP BY n.[year], n.[month] ORDER BY n.[year], n.[month]", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SalesMonthlyRow, 0)
	for rows.Next() {
		var row SalesMonthlyRow
		if err := rows.Scan(&row.Year, &row.Month, &row.Value); err == nil {
			result = append(result, row)
		}
	}
	return result, rows.Err()
}

// SalesYearComparison сравнивает два года одним проходом.
func SalesYearComparison(f SalesFilter, currentYear, previousYear int) (models.SalesDashboardMetricComparison, error) {
	where, args := BuildSalesWhere(f)
	queryArgs := append([]interface{}{currentYear, previousYear}, args...)

	var result models.SalesDashboardMetricComparison
	err := config.DB.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN n.[year] = ? THEN n.metric_value ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN n.[year] = ? THEN n.metric_value ELSE 0 END), 0)
		FROM `+salesTable+` n`+where, queryArgs...).Scan(&result.Current, &result.Previous)
	return result, err
}

// SalesPeriodValue возвращает сумму за конкретный месяц и число строк в нём.
// count = 0 означает, что данных за период нет (это не то же самое, что ноль).
func SalesPeriodValue(f SalesFilter, year, month int) (float64, int, error) {
	where, args := BuildSalesWhere(f)
	queryArgs := append(append([]interface{}{}, args...), year, month)

	var value float64
	var count int
	err := config.DB.QueryRow("SELECT COALESCE(SUM(n.metric_value), 0), COUNT(*) FROM "+salesTable+" n"+
		where+" AND n.[year] = ? AND n.[month] = ?", queryArgs...).Scan(&value, &count)
	return value, count, err
}

// SalesDimensionMonthly возвращает помесячные суммы в разрезе одного измерения.
func SalesDimensionMonthly(f SalesFilter, column string) ([]SalesMonthlyRow, error) {
	if err := checkDimension(column); err != nil {
		return nil, err
	}
	where, args := BuildSalesWhere(f)
	query := `SELECT n.` + column + `, n.[year], n.[month], SUM(n.metric_value)
		FROM ` + salesTable + ` n` + where + ` AND n.` + column + ` IS NOT NULL
		GROUP BY n.` + column + `, n.[year], n.[month]`
	return querySalesMonthlyRows(query, args)
}

// SalesDimensionMonthlyIn — то же, но только по перечисленным значениям измерения.
func SalesDimensionMonthlyIn(f SalesFilter, column string, names []string) ([]SalesMonthlyRow, error) {
	if err := checkDimension(column); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return []SalesMonthlyRow{}, nil
	}
	where, args := BuildSalesWhere(f)
	for _, name := range names {
		args = append(args, name)
	}
	query := "SELECT n." + column + ", n.[year], n.[month], SUM(n.metric_value) FROM " + salesTable + " n" +
		where + " AND n." + column + " IN (" + placeholders(len(names)) + ")" +
		" GROUP BY n." + column + ", n.[year], n.[month] ORDER BY n.[year], n.[month], n." + column
	return querySalesMonthlyRows(query, args)
}

// SalesChannelMonthly возвращает помесячные суммы по каналам из справочника.
func SalesChannelMonthly(f SalesFilter, channels []string) ([]SalesMonthlyRow, error) {
	if len(channels) == 0 {
		return []SalesMonthlyRow{}, nil
	}
	where, args := BuildSalesWhere(f)
	for _, channel := range channels {
		args = append(args, channel)
	}
	query := `SELECT m.channel, n.[year], n.[month], SUM(n.metric_value)
		FROM ` + salesTable + ` n
		INNER JOIN dbo.tbl_ChannelSegmentMapping m ON m.segment = n.segment AND m.un_rub = n.un_rub` +
		where + " AND m.channel IN (" + placeholders(len(channels)) + ")" +
		" GROUP BY m.channel, n.[year], n.[month] ORDER BY n.[year], n.[month], m.channel"
	return querySalesMonthlyRows(query, args)
}

func querySalesMonthlyRows(query string, args []interface{}) ([]SalesMonthlyRow, error) {
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SalesMonthlyRow, 0)
	for rows.Next() {
		var row SalesMonthlyRow
		if err := rows.Scan(&row.Name, &row.Year, &row.Month, &row.Value); err == nil {
			result = append(result, row)
		}
	}
	return result, rows.Err()
}

// SalesTopBy возвращает верхушку рейтинга по измерению.
func SalesTopBy(f SalesFilter, column string, top int) ([]models.SalesDashboardRank, error) {
	if err := checkDimension(column); err != nil {
		return nil, err
	}
	where, args := BuildSalesWhere(f)
	query := "SELECT TOP " + strconv.Itoa(top) + " n." + column + ", SUM(n.metric_value) AS total_value FROM " +
		salesTable + " n" + where + " GROUP BY n." + column + " ORDER BY total_value DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.SalesDashboardRank, 0, top)
	for rows.Next() {
		var item models.SalesDashboardRank
		if err := rows.Scan(&item.Name, &item.Value); err == nil {
			result = append(result, item)
		}
	}
	return result, rows.Err()
}

// SalesNetworkBreakdownMonthly возвращает помесячные суммы сети по каналам и сегментам.
// Пустой focusNetworks оставляет фильтр как есть.
func SalesNetworkBreakdownMonthly(f SalesFilter, focusNetworks []string) ([]SalesBreakdownRow, error) {
	where, args := BuildSalesWhere(f)
	condition := ""
	if len(focusNetworks) > 0 {
		condition = " AND n.networkName IN (" + placeholders(len(focusNetworks)) + ")"
		for _, network := range focusNetworks {
			args = append(args, network)
		}
	}
	query := `SELECT n.networkName, n.channel, n.segment, n.[year], n.[month], SUM(n.metric_value) AS total_value
		FROM ` + salesTable + ` n` + where + condition +
		" GROUP BY n.networkName, n.channel, n.segment, n.[year], n.[month]"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SalesBreakdownRow, 0)
	for rows.Next() {
		var row SalesBreakdownRow
		if err := rows.Scan(&row.Network, &row.Channel, &row.Segment, &row.Year, &row.Month, &row.Value); err == nil {
			result = append(result, row)
		}
	}
	return result, rows.Err()
}
