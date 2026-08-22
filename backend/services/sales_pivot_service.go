package services

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"backend/models"
	"backend/repository"
)

// SalesPivotRequest — фильтры и форма колонок иерархической сводной.
type SalesPivotRequest struct {
	AnalysisYearRaw string
	YearFromRaw     string
	YearToRaw       string
	Unit            string
	Granularity     string
	Months          []string
	Quarters        []string
	BrandNames      []string
	ProductNames    []string
	NetworkNames    []string
	Segments        []string
	Channel         string
}

func (r *SalesPivotRequest) Normalize() error {
	r.Unit = strings.TrimSpace(r.Unit)
	if r.Unit == "" {
		r.Unit = "руб"
	}
	if r.Unit != "руб" && r.Unit != "уп" && r.Unit != "евро" {
		return salesError(http.StatusBadRequest, "unit должен быть 'руб', 'евро' или 'уп'")
	}
	r.Granularity = strings.ToLower(strings.TrimSpace(r.Granularity))
	if r.Granularity == "" {
		r.Granularity = "year"
	}
	if r.Granularity != "year" && r.Granularity != "quarter" && r.Granularity != "month" {
		return salesError(http.StatusBadRequest, "granularity должен быть year, quarter или month")
	}
	r.Segments = UniqueNonEmptyStrings(r.Segments, 0)
	if len(r.Segments) == 0 {
		r.Segments = []string{"OLAP SS"}
	}
	r.Channel = strings.TrimSpace(r.Channel)
	return nil
}

func (r SalesPivotRequest) DBUnit() string {
	if r.Unit == "евро" {
		return "руб"
	}
	return r.Unit
}

func parsePivotNumbers(values []string, min, max int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, raw := range values {
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || value < min || value > max {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func pivotMonths(req SalesPivotRequest) []int {
	if months := parsePivotNumbers(req.Months, 1, 12); len(months) > 0 {
		return months
	}
	quarters := parsePivotNumbers(req.Quarters, 1, 4)
	if len(quarters) == 0 {
		return []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	}
	months := make([]int, 0, len(quarters)*3)
	for _, quarter := range quarters {
		start := (quarter-1)*3 + 1
		months = append(months, start, start+1, start+2)
	}
	return months
}

func pivotQuarters(req SalesPivotRequest) []int {
	if quarters := parsePivotNumbers(req.Quarters, 1, 4); len(quarters) > 0 {
		return quarters
	}
	months := parsePivotNumbers(req.Months, 1, 12)
	if len(months) == 0 {
		return []int{1, 2, 3, 4}
	}
	seen := make(map[int]struct{}, 4)
	quarters := make([]int, 0, 4)
	for _, month := range months {
		quarter := (month-1)/3 + 1
		if _, ok := seen[quarter]; ok {
			continue
		}
		seen[quarter] = struct{}{}
		quarters = append(quarters, quarter)
	}
	sort.Ints(quarters)
	return quarters
}

var pivotMonthLabels = []string{"Янв", "Фев", "Мар", "Апр", "Май", "Июн", "Июл", "Авг", "Сен", "Окт", "Ноя", "Дек"}

func buildPivotPeriods(req SalesPivotRequest, analysisYear int) ([]models.SalesPivotPeriod, string, string) {
	periods := make([]models.SalesPivotPeriod, 0, 28)
	for _, year := range []int{analysisYear - 1, analysisYear} {
		switch req.Granularity {
		case "month":
			for _, month := range pivotMonths(req) {
				periods = append(periods, models.SalesPivotPeriod{
					Key: fmt.Sprintf("%d-m%02d", year, month), Label: pivotMonthLabels[month-1], Year: year, Kind: "month",
				})
			}
		case "quarter":
			for _, quarter := range pivotQuarters(req) {
				periods = append(periods, models.SalesPivotPeriod{
					Key: fmt.Sprintf("%d-q%d", year, quarter), Label: fmt.Sprintf("Q%d", quarter), Year: year, Kind: "quarter",
				})
			}
		}
		periods = append(periods, models.SalesPivotPeriod{
			Key: fmt.Sprintf("%d-total", year), Label: "Итого", Year: year, Kind: "total",
		})
	}
	return periods, fmt.Sprintf("%d-total", analysisYear-1), fmt.Sprintf("%d-total", analysisYear)
}

func pivotValueKey(granularity string, year, month int) string {
	switch granularity {
	case "month":
		return fmt.Sprintf("%d-m%02d", year, month)
	case "quarter":
		return fmt.Sprintf("%d-q%d", year, (month-1)/3+1)
	default:
		return ""
	}
}

type salesPivotBuilderNode struct {
	level    string
	name     string
	values   map[string]float64
	children map[string]*salesPivotBuilderNode
}

func newSalesPivotBuilderNode(level, name string) *salesPivotBuilderNode {
	return &salesPivotBuilderNode{
		level: level, name: name, values: make(map[string]float64), children: make(map[string]*salesPivotBuilderNode),
	}
}

func addPivotValue(values map[string]float64, periodKey, totalKey string, value float64) {
	if periodKey != "" {
		values[periodKey] += value
	}
	values[totalKey] += value
}

func finalizePivotNodes(builders map[string]*salesPivotBuilderNode, parentID string, leafCount *int) []models.SalesPivotNode {
	names := make([]string, 0, len(builders))
	for name := range builders {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]models.SalesPivotNode, 0, len(names))
	for index, name := range names {
		builder := builders[name]
		id := fmt.Sprintf("%s%d", parentID, index+1)
		children := finalizePivotNodes(builder.children, id+".", leafCount)
		if builder.level == "sku" {
			(*leafCount)++
		}
		result = append(result, models.SalesPivotNode{
			ID: id, Level: builder.level, Name: builder.name, Values: builder.values, Children: children,
		})
	}
	return result
}

type pivotValueConverter func(value float64, year, month int) (float64, error)

func buildSalesPivotFromRows(req SalesPivotRequest, analysisYear int, rows []repository.SalesPivotLeafRow, convert pivotValueConverter) (*models.SalesPivotResponse, error) {
	periods, previousTotalKey, currentTotalKey := buildPivotPeriods(req, analysisYear)
	knownPeriods := make(map[string]struct{}, len(periods))
	for _, period := range periods {
		knownPeriods[period.Key] = struct{}{}
	}

	roots := make(map[string]*salesPivotBuilderNode)
	totals := make(map[string]float64)
	for _, row := range rows {
		value := row.Value
		if convert != nil {
			converted, err := convert(row.Value, row.Year, row.Month)
			if err != nil {
				return nil, err
			}
			value = converted
		}
		periodKey := pivotValueKey(req.Granularity, row.Year, row.Month)
		if periodKey != "" {
			if _, ok := knownPeriods[periodKey]; !ok {
				continue
			}
		}
		totalKey := fmt.Sprintf("%d-total", row.Year)
		if _, ok := knownPeriods[totalKey]; !ok {
			continue
		}
		addPivotValue(totals, periodKey, totalKey, value)

		path := []struct{ level, name string }{
			{"channel", row.Channel}, {"segment", row.Segment}, {"network", row.Network}, {"sku", row.Product},
		}
		children := roots
		for _, item := range path {
			node := children[item.name]
			if node == nil {
				node = newSalesPivotBuilderNode(item.level, item.name)
				children[item.name] = node
			}
			addPivotValue(node.values, periodKey, totalKey, value)
			children = node.children
		}
	}

	leafRows := 0
	response := &models.SalesPivotResponse{
		AnalysisYear:     analysisYear,
		Channel:          req.Channel,
		Segments:         req.Segments,
		Unit:             req.Unit,
		Granularity:      req.Granularity,
		Periods:          periods,
		Rows:             finalizePivotNodes(roots, "r", &leafRows),
		Totals:           totals,
		PreviousTotalKey: previousTotalKey,
		CurrentTotalKey:  currentTotalKey,
		LeafRows:         leafRows,
	}
	if req.Unit == "евро" {
		response.CurrencySource = "ЦБ РФ · средний официальный курс EUR за месяц"
	}
	return response, nil
}

// BuildSalesPivot строит полную иерархическую сводную по всем строкам,
// подходящим под фильтры, а не только по видимой/раскрытой части экрана.
func BuildSalesPivot(req SalesPivotRequest) (*models.SalesPivotResponse, error) {
	if err := req.Normalize(); err != nil {
		return nil, err
	}
	analysisYear, err := resolveAnalysisYear(SalesDashboardRequest{
		AnalysisYearRaw: req.AnalysisYearRaw, YearFromRaw: req.YearFromRaw, YearToRaw: req.YearToRaw,
	})
	if err != nil {
		return nil, err
	}

	filter := repository.SalesFilter{
		YearFromStr:  strconv.Itoa(analysisYear - 1),
		YearToStr:    strconv.Itoa(analysisYear),
		Months:       req.Months,
		Quarters:     req.Quarters,
		BrandNames:   req.BrandNames,
		ProductNames: req.ProductNames,
		NetworkNames: req.NetworkNames,
		UnRubs:       []string{req.DBUnit()},
		Segments:     req.Segments,
		Channels:     UniqueNonEmptyStrings([]string{req.Channel}, 0),
	}
	rows, err := repository.SalesPivotMonthly(filter)
	if err != nil {
		return nil, salesError(http.StatusInternalServerError, "Pivot query failed")
	}

	var convert pivotValueConverter
	if req.Unit == "евро" {
		ratesByYear := make(map[int]map[int]float64, 2)
		for _, year := range []int{analysisYear - 1, analysisYear} {
			rates, rateErr := LoadEURMonthlyRates(year)
			if rateErr != nil {
				return nil, &SalesError{
					Status: http.StatusServiceUnavailable, Message: "Не удалось загрузить официальные курсы EUR ЦБ РФ", Details: rateErr.Error(),
				}
			}
			ratesByYear[year] = rates
		}
		convert = func(value float64, year, month int) (float64, error) {
			rate := ratesByYear[year][month]
			if rate <= 0 {
				return 0, fmt.Errorf("нет курса EUR за %02d.%d", month, year)
			}
			return value / rate, nil
		}
	}

	return buildSalesPivotFromRows(req, analysisYear, rows, convert)
}
