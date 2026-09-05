package services

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"backend/models"
	"backend/repository"
)

// Витрина интернет-продаж. Слой собирает дашборд из сырых помесячных агрегатов
// репозитория: пересчёт в евро, ранжирование и доли считаются здесь, а не в SQL,
// потому что курс — свойство месяца, а не строки таблицы.

// SalesError — ошибка с готовым HTTP-статусом.
// Позволяет хендлеру не разбирать причину заново.
type SalesError struct {
	Status  int
	Message string
	Details string
}

func (e *SalesError) Error() string { return e.Message }

func salesError(status int, message string) *SalesError {
	return &SalesError{Status: status, Message: message}
}

// UniqueNonEmptyStrings убирает пустые значения и повторы, сохраняя порядок.
// limit > 0 обрезает результат.
func UniqueNonEmptyStrings(values []string, limit int) []string {
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

// salesDimensionValue — значение измерения за текущий и прошлый год.
type salesDimensionValue struct {
	Name     string
	Current  float64
	Previous float64
}

// buildDimensionViews строит две витрины по одному набору значений:
// драйверы изменения (по модулю дельты) и рейтинг текущего года.
func buildDimensionViews(values []salesDimensionValue) ([]models.SalesDashboardDriver, []models.SalesDashboardRankDetail) {
	drivers := make([]models.SalesDashboardDriver, 0, len(values))
	currentTotal := 0.0
	for _, item := range values {
		currentTotal += item.Current
		delta := item.Current - item.Previous
		var deltaPercent *float64
		if item.Previous != 0 {
			value := delta / item.Previous * 100
			deltaPercent = &value
		}
		drivers = append(drivers, models.SalesDashboardDriver{
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

	ranking := make([]models.SalesDashboardRankDetail, 0, 10)
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
		ranking = append(ranking, models.SalesDashboardRankDetail{
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

// SalesDashboardRequest — разобранные параметры запроса дашборда.
// Год приходит строками в том же порядке приоритета, что и в query:
// явный analysisYear, затем границы диапазона, затем последний год с данными.
type SalesDashboardRequest struct {
	AnalysisYearRaw string
	YearFromRaw     string
	YearToRaw       string

	Segments        []string
	Channel         string
	Unit            string
	Months          []string
	Quarters        []string
	BrandNames      []string
	ProductNames    []string
	NetworkNames    []string
	KAMs            []string
	FocusProducts   []string
	FocusNetworks   []string
	CompareChannels []string
}

// Normalize приводит запрос к рабочему виду и проверяет единицу измерения.
func (r *SalesDashboardRequest) Normalize() error {
	r.Segments = UniqueNonEmptyStrings(r.Segments, 0)
	if len(r.Segments) == 0 {
		r.Segments = []string{"OLAP SS"}
	}
	r.Channel = strings.TrimSpace(r.Channel)
	r.Unit = strings.TrimSpace(r.Unit)
	if r.Unit == "" {
		r.Unit = "руб"
	}
	if r.Unit != "руб" && r.Unit != "уп" && r.Unit != "евро" {
		return salesError(http.StatusBadRequest, "unit должен быть 'руб', 'евро' или 'уп'")
	}
	r.FocusProducts = UniqueNonEmptyStrings(r.FocusProducts, 5)
	r.FocusNetworks = UniqueNonEmptyStrings(r.FocusNetworks, 5)
	r.CompareChannels = UniqueNonEmptyStrings(r.CompareChannels, 5)
	return nil
}

// DBUnit — единица, в которой значения лежат в таблице: евро считается из рублей.
func (r *SalesDashboardRequest) DBUnit() string {
	if r.Unit == "евро" {
		return "руб"
	}
	return r.Unit
}

// SalesNetworkOptionsRequest — параметры списка доступных сетей.
type SalesNetworkOptionsRequest struct {
	Unit         string
	Channel      string
	Segments     []string
	YearFromRaw  string
	YearToRaw    string
	Months       []string
	Quarters     []string
	BrandNames   []string
	ProductNames []string
	// KAMs сужает список сетей до закреплённых за выбранными КАМами: фильтр
	// «КАМ → Сеть» иначе предлагал бы сети чужих КАМов.
	KAMs []string
}

// SalesNetworkOptions возвращает сети, по которым есть данные при текущих
// фильтрах. Сами выбранные сети в условие не входят, чтобы список можно было
// пересчитать до их применения.
func SalesNetworkOptions(req SalesNetworkOptionsRequest) ([]string, error) {
	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = "руб"
	}
	if unit != "руб" && unit != "уп" && unit != "евро" {
		return nil, salesError(http.StatusBadRequest, "unit должен быть 'руб', 'евро' или 'уп'")
	}
	dbUnit := unit
	if unit == "евро" {
		dbUnit = "руб"
	}
	segments := UniqueNonEmptyStrings(req.Segments, 0)
	if len(segments) == 0 {
		segments = []string{"OLAP SS"}
	}

	networks, err := repository.SalesNetworkNames(repository.SalesFilter{
		YearFromStr:  req.YearFromRaw,
		YearToStr:    req.YearToRaw,
		Months:       req.Months,
		Quarters:     req.Quarters,
		BrandNames:   req.BrandNames,
		ProductNames: req.ProductNames,
		KAMs:         req.KAMs,
		UnRubs:       []string{dbUnit},
		Segments:     segments,
		Channels:     UniqueNonEmptyStrings([]string{req.Channel}, 0),
	})
	if err != nil {
		return nil, salesError(http.StatusInternalServerError, "Network options query failed")
	}
	return networks, nil
}

// resolveAnalysisYear выбирает год анализа: явный параметр, затем границы
// диапазона, затем последний год с данными.
func resolveAnalysisYear(req SalesDashboardRequest) (int, error) {
	for _, raw := range []string{req.AnalysisYearRaw, req.YearToRaw, req.YearFromRaw} {
		if year, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && year >= 2000 && year <= 2100 {
			return year, nil
		}
	}
	year, err := repository.SalesLatestYear()
	if err != nil {
		return 0, salesError(http.StatusInternalServerError, "Dashboard year query failed")
	}
	return year, nil
}

// dashboardBuilder держит общий контекст сборки: фильтр, курсы и год анализа.
type dashboardBuilder struct {
	req          SalesDashboardRequest
	analysisYear int
	baseFilter   repository.SalesFilter
	eurRates     map[int]map[int]float64
}

// convert переводит рублёвую сумму месяца в выбранную единицу.
func (b *dashboardBuilder) convert(value float64, year, month int) (float64, error) {
	if b.req.Unit != "евро" {
		return value, nil
	}
	rate := b.eurRates[year][month]
	if rate <= 0 {
		return 0, fmt.Errorf("нет курса EUR за %02d.%d", month, year)
	}
	return value / rate, nil
}

// monthlyPoints пересчитывает помесячные строки в точки тренда.
func (b *dashboardBuilder) monthlyPoints(rows []repository.SalesMonthlyRow) ([]models.SalesDashboardPoint, error) {
	points := make([]models.SalesDashboardPoint, 0, len(rows))
	for _, row := range rows {
		value, err := b.convert(row.Value, row.Year, row.Month)
		if err != nil {
			return nil, err
		}
		points = append(points, models.SalesDashboardPoint{Year: row.Year, Month: row.Month, Value: value})
	}
	return points, nil
}

// seriesPoints пересчитывает помесячные строки в именованные серии.
func (b *dashboardBuilder) seriesPoints(rows []repository.SalesMonthlyRow) ([]models.SalesDashboardSeriesPoint, error) {
	points := make([]models.SalesDashboardSeriesPoint, 0, len(rows))
	for _, row := range rows {
		value, err := b.convert(row.Value, row.Year, row.Month)
		if err != nil {
			return nil, err
		}
		points = append(points, models.SalesDashboardSeriesPoint{
			Name: row.Name, Year: row.Year, Month: row.Month, Value: value,
		})
	}
	return points, nil
}

// dimensionValues складывает помесячные строки в пары «текущий/прошлый год».
func (b *dashboardBuilder) dimensionValues(rows []repository.SalesMonthlyRow) ([]salesDimensionValue, error) {
	byName := make(map[string]*salesDimensionValue)
	for _, row := range rows {
		if row.Name == "" {
			continue
		}
		value, err := b.convert(row.Value, row.Year, row.Month)
		if err != nil {
			return nil, err
		}
		item := byName[row.Name]
		if item == nil {
			item = &salesDimensionValue{Name: row.Name}
			byName[row.Name] = item
		}
		switch row.Year {
		case b.analysisYear:
			item.Current += value
		case b.analysisYear - 1:
			item.Previous += value
		}
	}
	result := make([]salesDimensionValue, 0, len(byName))
	for _, item := range byName {
		result = append(result, *item)
	}
	// Порядок карты случаен, а дальше идёт устойчивая сортировка: без этого
	// равные значения выстраивались бы по-разному от запроса к запросу.
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// yearRangeFilter — базовый фильтр, растянутый на прошлый и текущий год.
func (b *dashboardBuilder) yearRangeFilter() repository.SalesFilter {
	filter := b.baseFilter
	filter.YearFromStr = strconv.Itoa(b.analysisYear - 1)
	filter.YearToStr = strconv.Itoa(b.analysisYear)
	return filter
}

// BuildSalesDashboard собирает витрину дашборда интернет-продаж.
func BuildSalesDashboard(req SalesDashboardRequest) (*models.SalesDashboardResponse, error) {
	if err := req.Normalize(); err != nil {
		return nil, err
	}

	analysisYear, err := resolveAnalysisYear(req)
	if err != nil {
		return nil, err
	}

	b := &dashboardBuilder{
		req:          req,
		analysisYear: analysisYear,
		eurRates:     make(map[int]map[int]float64),
		baseFilter: repository.SalesFilter{
			YearFromStr:  strconv.Itoa(analysisYear),
			YearToStr:    strconv.Itoa(analysisYear),
			Months:       req.Months,
			Quarters:     req.Quarters,
			BrandNames:   req.BrandNames,
			ProductNames: req.ProductNames,
			NetworkNames: req.NetworkNames,
			KAMs:         req.KAMs,
			UnRubs:       []string{req.DBUnit()},
			Segments:     req.Segments,
			Channels:     UniqueNonEmptyStrings([]string{req.Channel}, 0),
		},
	}

	if req.Unit == "евро" {
		for _, year := range []int{analysisYear - 1, analysisYear} {
			rates, rateErr := LoadEURMonthlyRates(year)
			if rateErr != nil {
				return nil, &SalesError{
					Status:  http.StatusServiceUnavailable,
					Message: "Не удалось загрузить официальные курсы EUR ЦБ РФ",
					Details: rateErr.Error(),
				}
			}
			b.eurRates[year] = rates
		}
	}

	response := &models.SalesDashboardResponse{
		AnalysisYear: analysisYear,
		Channel:      req.Channel,
		Segment:      strings.Join(req.Segments, ", "),
		Segments:     req.Segments,
		Unit:         req.Unit,
	}
	if req.Unit == "евро" {
		// Подпись отмечает месяцы, курс за которые перенесён с предыдущего:
		// пересчёт в евро не должен выглядеть одинаково достоверным там, где
		// котировка настоящая, и там, где её ещё нет.
		response.CurrencySource = eurCurrencySource([]int{analysisYear - 1, analysisYear})
	}

	summary, err := repository.SalesSummary(b.baseFilter)
	if err != nil {
		return nil, salesError(http.StatusInternalServerError, "Dashboard summary query failed")
	}

	if err := b.fillTrends(response, summary); err != nil {
		return nil, err
	}
	if err := b.fillComparisons(response); err != nil {
		return nil, err
	}
	if err := b.fillDimensions(response); err != nil {
		return nil, err
	}
	if err := b.fillEcomShare(response); err != nil {
		return nil, err
	}
	if err := b.fillSummary(response, summary); err != nil {
		return nil, err
	}
	if err := b.fillFocusTrends(response); err != nil {
		return nil, err
	}
	if err := b.fillRankings(response); err != nil {
		return nil, err
	}
	if err := b.fillSegmentTotals(response); err != nil {
		return nil, err
	}
	if err := b.fillSeriesComparisons(response); err != nil {
		return nil, err
	}
	if err := b.fillNetworkBreakdown(response); err != nil {
		return nil, err
	}
	return response, nil
}

// fillTrends заполняет тренд текущего и прошлого года.
// В евро итог пересобирается из тренда: курс у каждого месяца свой.
func (b *dashboardBuilder) fillTrends(response *models.SalesDashboardResponse, summary repository.SalesSummaryRow) error {
	rows, err := repository.SalesMonthlyTotals(b.baseFilter)
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard trend query failed")
	}
	trend, err := b.monthlyPoints(rows)
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard currency conversion failed")
	}

	previousFilter := b.baseFilter
	previousFilter.YearFromStr = strconv.Itoa(b.analysisYear - 1)
	previousFilter.YearToStr = strconv.Itoa(b.analysisYear - 1)
	previousRows, err := repository.SalesMonthlyTotals(previousFilter)
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard previous year trend query failed")
	}
	previousTrend, err := b.monthlyPoints(previousRows)
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard currency conversion failed")
	}

	response.Trend = trend
	response.PreviousYearTrend = previousTrend
	response.Summary.Total = summary.Total
	if b.req.Unit == "евро" {
		total := 0.0
		for _, point := range trend {
			total += point.Value
		}
		response.Summary.Total = total
	}
	return nil
}

// fillComparisons считает сравнение год к году во всех трёх единицах.
// Рубли и упаковки берутся из БД, евро — из уже пересчитанных трендов.
func (b *dashboardBuilder) fillComparisons(response *models.SalesDashboardResponse) error {
	comparisonFor := func(metricUnit, failure string) (models.SalesDashboardMetricComparison, error) {
		filter := b.baseFilter
		filter.YearFromStr = ""
		filter.YearToStr = ""
		filter.UnRubs = []string{metricUnit}
		result, err := repository.SalesYearComparison(filter, b.analysisYear, b.analysisYear-1)
		if err != nil {
			return result, salesError(http.StatusInternalServerError, failure)
		}
		return result, nil
	}

	rub, err := comparisonFor("руб", "Dashboard rub comparison query failed")
	if err != nil {
		return err
	}
	units, err := comparisonFor("уп", "Dashboard units comparison query failed")
	if err != nil {
		return err
	}

	eur := models.SalesDashboardMetricComparison{}
	accumulate := func(points []models.SalesDashboardPoint, target *float64) {
		for _, point := range points {
			if b.req.Unit == "евро" {
				*target += point.Value
				continue
			}
			if rates := b.eurRates[point.Year]; rates != nil && rates[point.Month] > 0 {
				*target += point.Value / rates[point.Month]
			}
		}
	}
	accumulate(response.Trend, &eur.Current)
	accumulate(response.PreviousYearTrend, &eur.Previous)

	response.MetricComparisons = models.SalesDashboardMetricComparisons{Rub: rub, Eur: eur, Units: units}
	return nil
}

// fillDimensions строит драйверы и рейтинги по сетям, брендам и продуктам.
func (b *dashboardBuilder) fillDimensions(response *models.SalesDashboardResponse) error {
	filter := b.yearRangeFilter()

	load := func(column, failure string) ([]salesDimensionValue, error) {
		rows, err := repository.SalesDimensionMonthly(filter, column)
		if err != nil {
			return nil, salesError(http.StatusInternalServerError, failure)
		}
		values, err := b.dimensionValues(rows)
		if err != nil {
			return nil, salesError(http.StatusInternalServerError, "Dashboard currency conversion failed")
		}
		return values, nil
	}

	networkValues, err := load("networkName", "Dashboard network analytics query failed")
	if err != nil {
		return err
	}
	brandValues, err := load("brandName", "Dashboard brand analytics query failed")
	if err != nil {
		return err
	}
	productValues, err := load("productName", "Dashboard product analytics query failed")
	if err != nil {
		return err
	}

	response.NetworkDrivers, response.NetworkRanking = buildDimensionViews(networkValues)
	// Рейтинг по брендам витрина не показывает, поэтому здесь нужны только
	// драйверы: строить его впрок значило бы отдавать наружу мёртвое поле.
	response.BrandDrivers, _ = buildDimensionViews(brandValues)
	response.ProductDrivers, response.ProductRanking = buildDimensionViews(productValues)
	return nil
}

// fillEcomShare считает долю Ecom внутри семейства сегментов OLAP.
// Для каналов вне семейства блок неприменим и остаётся пустым.
func (b *dashboardBuilder) fillEcomShare(response *models.SalesDashboardResponse) error {
	family := ""
	switch b.req.Channel {
	case "OLAP SS", "OLAP SS wo Ecom":
		family = "OLAP SS"
	case "OLAP NW", "OLAP NW wo Ecom":
		family = "OLAP NW"
	}
	if family == "" {
		return nil
	}

	share := models.SalesDashboardEcomShare{Applicable: true, Family: family}
	withoutEcom := family + " wo Ecom"

	filter := b.yearRangeFilter()
	filter.Segments = []string{family, withoutEcom}
	filter.Channels = nil
	rows, err := repository.SalesDimensionMonthly(filter, "segment")
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard Ecom share query failed")
	}

	for _, row := range rows {
		value, convertErr := b.convert(row.Value, row.Year, row.Month)
		if convertErr != nil {
			return salesError(http.StatusInternalServerError, "Dashboard Ecom currency conversion failed")
		}
		switch row.Year {
		case b.analysisYear:
			if row.Name == family {
				share.Full += value
			} else if row.Name == withoutEcom {
				share.WithoutEcom += value
			}
		case b.analysisYear - 1:
			// Прошлый год копится тем же вычитанием: сначала минус сегмент
			// «wo Ecom», затем к нему прибавляется полный объём семейства.
			if row.Name == family {
				share.PreviousFull += value
			} else if row.Name == withoutEcom {
				share.PreviousEcom -= value
			}
		}
	}

	share.Ecom = share.Full - share.WithoutEcom
	share.PreviousEcom += share.PreviousFull
	if share.Full != 0 {
		value := share.Ecom / share.Full * 100
		share.Share = &value
	}
	if share.PreviousFull != 0 {
		value := share.PreviousEcom / share.PreviousFull * 100
		share.PreviousShare = &value
	}
	response.EcomShare = share
	return nil
}

// fillSummary дозаполняет карточки шапки: охват выборки и последний период
// в сравнении с предыдущим месяцем и тем же месяцем год назад.
func (b *dashboardBuilder) fillSummary(response *models.SalesDashboardResponse, summary repository.SalesSummaryRow) error {
	response.Summary.ActiveNetworks = summary.ActiveNetworks
	response.Summary.ActiveProducts = summary.ActiveProducts
	response.Summary.Periods = summary.Periods
	if summary.Periods > 0 {
		response.Summary.AveragePerMonth = response.Summary.Total / float64(summary.Periods)
	}

	if len(response.Trend) == 0 {
		return nil
	}
	latest := response.Trend[len(response.Trend)-1]
	response.Summary.LatestYear = latest.Year
	response.Summary.LatestMonth = latest.Month
	latestValue := latest.Value
	response.Summary.LatestValue = &latestValue

	// Сравнение с соседними периодами игнорирует выбранные месяцы и кварталы:
	// иначе предыдущего месяца в выборке может просто не оказаться.
	filter := b.baseFilter
	filter.YearFromStr = ""
	filter.YearToStr = ""
	filter.Months = nil
	filter.Quarters = nil

	periodValue := func(year, month int) *float64 {
		value, count, err := repository.SalesPeriodValue(filter, year, month)
		if err != nil || count == 0 {
			return nil
		}
		converted, convertErr := b.convert(value, year, month)
		if convertErr != nil {
			return nil
		}
		return &converted
	}

	previousYear, previousMonth := latest.Year, latest.Month-1
	if previousMonth == 0 {
		previousYear--
		previousMonth = 12
	}
	response.Summary.PreviousValue = periodValue(previousYear, previousMonth)
	response.Summary.YearAgoValue = periodValue(latest.Year-1, latest.Month)
	return nil
}

// fillFocusTrends добавляет тренды выбранных продуктов и сетей.
func (b *dashboardBuilder) fillFocusTrends(response *models.SalesDashboardResponse) error {
	focusTrends := make([]models.SalesDashboardFocusPoint, 0)

	load := func(column, focusType, failure string, names []string) error {
		if len(names) == 0 {
			return nil
		}
		rows, err := repository.SalesDimensionMonthlyIn(b.baseFilter, column, names)
		if err != nil {
			return salesError(http.StatusInternalServerError, failure)
		}
		for _, row := range rows {
			value, convertErr := b.convert(row.Value, row.Year, row.Month)
			if convertErr != nil {
				return salesError(http.StatusInternalServerError, "Dashboard currency conversion failed")
			}
			focusTrends = append(focusTrends, models.SalesDashboardFocusPoint{
				Type: focusType, Name: row.Name, Year: row.Year, Month: row.Month, Value: value,
			})
		}
		return nil
	}

	if err := load("productName", "product", "Dashboard product focus query failed", b.req.FocusProducts); err != nil {
		return err
	}
	if err := load("networkName", "network", "Dashboard network focus query failed", b.req.FocusNetworks); err != nil {
		return err
	}
	response.FocusTrends = focusTrends
	return nil
}

// fillRankings заполняет топы сетей и продуктов.
// В евро топ берётся из уже пересчитанного рейтинга: суммировать рубли в SQL
// и потом делить на курс года нельзя — курс месячный.
func (b *dashboardBuilder) fillRankings(response *models.SalesDashboardResponse) error {
	const topLimit = 8

	if b.req.Unit == "евро" {
		fromRanking := func(details []models.SalesDashboardRankDetail) []models.SalesDashboardRank {
			result := make([]models.SalesDashboardRank, 0, topLimit)
			for index, item := range details {
				if index >= topLimit {
					break
				}
				result = append(result, models.SalesDashboardRank{Name: item.Name, Value: item.Value})
			}
			return result
		}
		response.TopNetworks = fromRanking(response.NetworkRanking)
		response.TopProducts = fromRanking(response.ProductRanking)
		return nil
	}

	networks, err := repository.SalesTopBy(b.baseFilter, "networkName", topLimit)
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard networks query failed")
	}
	products, err := repository.SalesTopBy(b.baseFilter, "productName", topLimit)
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard products query failed")
	}
	response.TopNetworks = networks
	response.TopProducts = products
	return nil
}

// fillSegmentTotals считает объёмы сегментов выбранного канала.
func (b *dashboardBuilder) fillSegmentTotals(response *models.SalesDashboardResponse) error {
	channelSegments := make([]string, 0)
	if b.req.Channel != "" {
		segments, err := repository.SalesChannelSegments(b.req.Channel, b.req.DBUnit())
		if err != nil {
			return salesError(http.StatusInternalServerError, "Dashboard channel mapping query failed")
		}
		channelSegments = segments
	}
	if len(channelSegments) == 0 {
		channelSegments = append(channelSegments, b.req.Segments...)
	}
	response.ChannelSegments = channelSegments

	filter := b.baseFilter
	filter.Segments = channelSegments
	rows, err := repository.SalesDimensionMonthly(filter, "segment")
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard segment totals query failed")
	}

	values := make(map[string]float64)
	monthly := make([]models.SalesDashboardSeriesPoint, 0, len(rows))
	for _, row := range rows {
		value, convertErr := b.convert(row.Value, row.Year, row.Month)
		if convertErr != nil {
			return salesError(http.StatusInternalServerError, "Dashboard segment currency conversion failed")
		}
		values[row.Name] += value
		monthly = append(monthly, models.SalesDashboardSeriesPoint{Name: row.Name, Year: row.Year, Month: row.Month, Value: value})
	}

	totals := make([]models.SalesDashboardRank, 0, len(values))
	for name, value := range values {
		totals = append(totals, models.SalesDashboardRank{Name: name, Value: value})
	}
	sort.SliceStable(totals, func(i, j int) bool {
		if totals[i].Value == totals[j].Value {
			return totals[i].Name < totals[j].Name
		}
		return totals[i].Value > totals[j].Value
	})
	response.SegmentTotals = totals
	response.SegmentTrends = b.segmentTrends(monthly, totals)
	return nil
}

// segmentTrends раскладывает общий ряд на выбранные сегменты канала: у каналов
// вроде PURE их несколько, и сумма сама по себе не показывает, чей это рост.
//
// Один сегмент повторил бы общий ряд линия в линию, поэтому разбивки нет.
// Порядок берётся из totals: крупные сегменты идут первыми, и цвет линии не
// прыгает между сегментами при смене периода.
func (b *dashboardBuilder) segmentTrends(monthly []models.SalesDashboardSeriesPoint, totals []models.SalesDashboardRank) []models.SalesDashboardSeriesPoint {
	points := make([]models.SalesDashboardSeriesPoint, 0)
	if len(b.req.Segments) < 2 {
		return points
	}

	selected := make(map[string]bool, len(b.req.Segments))
	for _, segment := range b.req.Segments {
		selected[segment] = true
	}
	order := make(map[string]int, len(totals))
	for index, item := range totals {
		order[item.Name] = index
	}

	for _, point := range monthly {
		if selected[point.Name] {
			points = append(points, point)
		}
	}
	sort.SliceStable(points, func(i, j int) bool {
		if points[i].Name != points[j].Name {
			return order[points[i].Name] < order[points[j].Name]
		}
		if points[i].Year != points[j].Year {
			return points[i].Year < points[j].Year
		}
		return points[i].Month < points[j].Month
	})
	return points
}

// fillSeriesComparisons строит тренды топовых сетей и сравниваемых каналов.
func (b *dashboardBuilder) fillSeriesComparisons(response *models.SalesDashboardResponse) error {
	response.NetworkTrends = make([]models.SalesDashboardSeriesPoint, 0)
	response.ChannelTrends = make([]models.SalesDashboardSeriesPoint, 0)

	if len(response.TopNetworks) > 0 {
		names := make([]string, 0, len(response.TopNetworks))
		for _, item := range response.TopNetworks {
			names = append(names, item.Name)
		}
		rows, err := repository.SalesDimensionMonthlyIn(b.baseFilter, "networkName", names)
		if err != nil {
			return salesError(http.StatusInternalServerError, "Dashboard network trends query failed")
		}
		points, convertErr := b.seriesPoints(rows)
		if convertErr != nil {
			return salesError(http.StatusInternalServerError, "Dashboard network currency conversion failed")
		}
		response.NetworkTrends = points
	}

	if len(b.req.CompareChannels) > 0 {
		// Канал определяется справочником, поэтому фильтр по сегментам снимается.
		filter := b.baseFilter
		filter.Segments = nil
		filter.Channels = nil
		rows, err := repository.SalesChannelMonthly(filter, b.req.CompareChannels)
		if err != nil {
			return salesError(http.StatusInternalServerError, "Dashboard channel comparison query failed")
		}
		points, convertErr := b.seriesPoints(rows)
		if convertErr != nil {
			return salesError(http.StatusInternalServerError, "Dashboard channel currency conversion failed")
		}
		response.ChannelTrends = points
	}
	return nil
}

// fillNetworkBreakdown раскладывает объёмы выбранных сетей по каналам и сегментам.
func (b *dashboardBuilder) fillNetworkBreakdown(response *models.SalesDashboardResponse) error {
	response.NetworkBreakdown = make([]models.SalesDashboardNetworkBreakdown, 0)
	if len(b.req.FocusNetworks) == 0 && len(b.baseFilter.NetworkNames) == 0 {
		return nil
	}

	filter := b.baseFilter
	filter.Segments = nil
	filter.Channels = nil
	rows, err := repository.SalesNetworkBreakdownMonthly(filter, b.req.FocusNetworks)
	if err != nil {
		return salesError(http.StatusInternalServerError, "Dashboard network breakdown query failed")
	}

	byKey := make(map[string]*models.SalesDashboardNetworkBreakdown)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		value, convertErr := b.convert(row.Value, row.Year, row.Month)
		if convertErr != nil {
			return salesError(http.StatusInternalServerError, "Dashboard breakdown currency conversion failed")
		}
		key := row.Network + "\x00" + row.Channel + "\x00" + row.Segment
		item := byKey[key]
		if item == nil {
			item = &models.SalesDashboardNetworkBreakdown{
				Network: row.Network, Channel: row.Channel, Segment: row.Segment,
			}
			byKey[key] = item
			order = append(order, key)
		}
		item.Value += value
	}

	breakdown := make([]models.SalesDashboardNetworkBreakdown, 0, len(order))
	for _, key := range order {
		breakdown = append(breakdown, *byKey[key])
	}
	sort.SliceStable(breakdown, func(i, j int) bool { return breakdown[i].Value > breakdown[j].Value })
	if len(breakdown) > 16 {
		breakdown = breakdown[:16]
	}
	response.NetworkBreakdown = breakdown
	return nil
}
