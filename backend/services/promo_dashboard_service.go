package services

import (
	"sort"
	"strings"

	"backend/models"
	"backend/repository"
)

const promoUnknownDimension = "Не указано"

type promoDashboardAccumulator struct {
	promoCount      int
	factReady       int
	planUnits       float64
	compareUnits    float64
	actualUnits     float64
	planInvest      float64
	compareInvest   float64
	actualInvest    float64
	planUplift      float64
	compareUplift   float64
	actualUplift    float64
	planNet         float64
	planROIDenom    float64
	compareNet      float64
	compareROIDenom float64
	actualNet       float64
	actualROIDenom  float64
}

func floatPointer(value float64) *float64 { return &value }

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func promoDimension(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return promoUnknownDimension
	}
	return strings.TrimSpace(*value)
}

func (a *promoDashboardAccumulator) add(row repository.PromoDashboardRow) {
	a.promoCount++
	planUnits := valueOrZero(row.PlanPromoUnits)
	planInvest := valueOrZero(row.PlanInvestmentsRub)
	planUplift := valueOrZero(row.PlanPromoUpliftUnits)
	planUpliftRub := valueOrZero(row.PlanPromoUpliftRub)
	gm := valueOrZero(row.GM)
	if row.GM == nil || gm == 0 {
		gm = 1
	}

	a.planUnits += planUnits
	a.planInvest += planInvest
	a.planUplift += planUplift
	if planInvest > 0 {
		a.planNet += planUpliftRub * gm
		a.planROIDenom += planInvest
	}

	// Факт сопоставим только когда заполнены продажи и инвестиции. Нулевое
	// значение остаётся валидным фактом и не смешивается с отсутствующим NULL.
	if row.ActualPromoSalesUnits == nil || row.ActualInvestments == nil {
		return
	}
	a.factReady++
	a.compareUnits += planUnits
	a.compareInvest += planInvest
	a.compareUplift += planUplift
	a.actualUnits += valueOrZero(row.ActualPromoSalesUnits)
	a.actualInvest += valueOrZero(row.ActualInvestments)
	a.actualUplift += valueOrZero(row.ActualPromoUpliftUnits)
	if planInvest > 0 {
		a.compareNet += planUpliftRub * gm
		a.compareROIDenom += planInvest
	}
	if *row.ActualInvestments > 0 {
		a.actualNet += valueOrZero(row.ActualPromoUpliftRub) * gm
		a.actualROIDenom += *row.ActualInvestments
	}
}

func ratio(numerator, denominator float64) *float64 {
	if denominator == 0 {
		return nil
	}
	return floatPointer(numerator / denominator * 100)
}

func roi(netUplift, investments float64) *float64 {
	if investments <= 0 {
		return nil
	}
	return floatPointer(netUplift/investments*100 - 100)
}

func (a promoDashboardAccumulator) metrics() models.PromoDashboardMetrics {
	metrics := models.PromoDashboardMetrics{
		PromoCount:                   a.promoCount,
		FactReadyCount:               a.factReady,
		FactCoveragePct:              ratio(float64(a.factReady), float64(a.promoCount)),
		PlanUnits:                    a.planUnits,
		ComparablePlanUnits:          a.compareUnits,
		PlanInvestmentsRub:           a.planInvest,
		ComparablePlanInvestmentsRub: a.compareInvest,
		PlanUpliftUnits:              a.planUplift,
		ComparablePlanUpliftUnits:    a.compareUplift,
		PlanROI:                      roi(a.planNet, a.planROIDenom),
		ComparablePlanROI:            roi(a.compareNet, a.compareROIDenom),
		SalesCompletionPct:           ratio(a.actualUnits, a.compareUnits),
		InvestmentCompletionPct:      ratio(a.actualInvest, a.compareInvest),
	}
	if a.factReady > 0 {
		metrics.ActualUnits = floatPointer(a.actualUnits)
		metrics.ActualInvestmentsRub = floatPointer(a.actualInvest)
		metrics.ActualUpliftUnits = floatPointer(a.actualUplift)
		metrics.ActualROI = roi(a.actualNet, a.actualROIDenom)
		metrics.SalesVarianceUnits = floatPointer(a.actualUnits - a.compareUnits)
		metrics.InvestmentVarianceRub = floatPointer(a.actualInvest - a.compareInvest)
	}
	return metrics
}

type promoPeriodKey struct {
	year  int
	month int
}

type promoCalendarKey struct {
	name  string
	year  int
	month int
}

func addToGroup(groups map[string]*promoDashboardAccumulator, name string, row repository.PromoDashboardRow) {
	accumulator := groups[name]
	if accumulator == nil {
		accumulator = &promoDashboardAccumulator{}
		groups[name] = accumulator
	}
	accumulator.add(row)
}

func addToCalendar(groups map[promoCalendarKey]*promoDashboardAccumulator, key promoCalendarKey, row repository.PromoDashboardRow) {
	accumulator := groups[key]
	if accumulator == nil {
		accumulator = &promoDashboardAccumulator{}
		groups[key] = accumulator
	}
	accumulator.add(row)
}

func breakdownFrom(groups map[string]*promoDashboardAccumulator) []models.PromoDashboardBreakdown {
	result := make([]models.PromoDashboardBreakdown, 0, len(groups))
	for name, accumulator := range groups {
		result = append(result, models.PromoDashboardBreakdown{Name: name, Metrics: accumulator.metrics()})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Metrics.PlanInvestmentsRub == result[j].Metrics.PlanInvestmentsRub {
			return result[i].Name < result[j].Name
		}
		return result[i].Metrics.PlanInvestmentsRub > result[j].Metrics.PlanInvestmentsRub
	})
	return result
}

func calendarFrom(groups map[promoCalendarKey]*promoDashboardAccumulator) []models.PromoDashboardCalendarPoint {
	result := make([]models.PromoDashboardCalendarPoint, 0, len(groups))
	for key, accumulator := range groups {
		result = append(result, models.PromoDashboardCalendarPoint{
			Name: key.name, Year: key.year, Month: key.month, Metrics: accumulator.metrics(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		if result[i].Year != result[j].Year {
			return result[i].Year < result[j].Year
		}
		return result[i].Month < result[j].Month
	})
	return result
}

// AggregatePromoDashboard — чистая агрегация, единый источник расчётов для
// карточек, трендов, пузырьковой карты и календаря.
func AggregatePromoDashboard(rows []repository.PromoDashboardRow) *models.PromoDashboardResponse {
	summary := &promoDashboardAccumulator{}
	periods := make(map[promoPeriodKey]*promoDashboardAccumulator)
	networks := make(map[string]*promoDashboardAccumulator)
	brands := make(map[string]*promoDashboardAccumulator)
	skus := make(map[string]*promoDashboardAccumulator)
	mechanics := make(map[string]*promoDashboardAccumulator)
	networkCalendar := make(map[promoCalendarKey]*promoDashboardAccumulator)
	brandCalendar := make(map[promoCalendarKey]*promoDashboardAccumulator)
	yearSet := make(map[int]struct{})

	for _, row := range rows {
		if row.Year <= 0 || row.Month < 1 || row.Month > 12 {
			continue
		}
		summary.add(row)
		yearSet[row.Year] = struct{}{}

		periodKey := promoPeriodKey{year: row.Year, month: row.Month}
		period := periods[periodKey]
		if period == nil {
			period = &promoDashboardAccumulator{}
			periods[periodKey] = period
		}
		period.add(row)

		network := promoDimension(row.NetworkName)
		brand := promoDimension(row.BrandAS)
		addToGroup(networks, network, row)
		addToGroup(brands, brand, row)
		addToGroup(skus, promoDimension(row.SKU), row)
		addToGroup(mechanics, promoDimension(row.Mechanics), row)
		addToCalendar(networkCalendar, promoCalendarKey{name: network, year: row.Year, month: row.Month}, row)
		addToCalendar(brandCalendar, promoCalendarKey{name: brand, year: row.Year, month: row.Month}, row)
	}

	years := make([]int, 0, len(yearSet))
	for year := range yearSet {
		years = append(years, year)
	}
	sort.Ints(years)

	trend := make([]models.PromoDashboardTrendPoint, 0, len(periods))
	for key, accumulator := range periods {
		trend = append(trend, models.PromoDashboardTrendPoint{
			Year: key.year, Month: key.month, Metrics: accumulator.metrics(),
		})
	}
	sort.Slice(trend, func(i, j int) bool {
		if trend[i].Year != trend[j].Year {
			return trend[i].Year < trend[j].Year
		}
		return trend[i].Month < trend[j].Month
	})

	return &models.PromoDashboardResponse{
		AvailableYears:  years,
		Summary:         summary.metrics(),
		Trend:           trend,
		Networks:        breakdownFrom(networks),
		Brands:          breakdownFrom(brands),
		SKUs:            breakdownFrom(skus),
		Mechanics:       breakdownFrom(mechanics),
		NetworkCalendar: calendarFrom(networkCalendar),
		BrandCalendar:   calendarFrom(brandCalendar),
	}
}

// BuildPromoDashboard читает отфильтрованные строки и собирает витрину.
func BuildPromoDashboard(params repository.PromoFilterParams, channels []string) (*models.PromoDashboardResponse, error) {
	rows, err := repository.GetPromoDashboardRows(params, channels)
	if err != nil {
		return nil, err
	}
	return AggregatePromoDashboard(rows), nil
}
