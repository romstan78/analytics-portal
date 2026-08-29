package repository

import (
	"fmt"
	"sort"
	"strings"

	"backend/config"
	"backend/models"
)

// NetworkDashboardFilter — область витрины реестра.
//
// OwnKAM и KAMs различаются намеренно: OwnKAM — закрепление вошедшего
// пользователя, оно применяется всегда и не может быть расширено запросом;
// KAMs — фильтр, который пользователь выбрал сам внутри уже доступной области.
type NetworkDashboardFilter struct {
	Year int
	// Quarters — выбранные кварталы, не обязательно смежные: сравнить Q1 с Q3
	// диапазоном нельзя, а такой разрез нужен не реже полугодия.
	Quarters   []int
	OwnKAM     string
	KAMs       []string
	NetworkIDs []int
}

// NormalizedQuarters приводит выбор к возрастающему набору без повторов.
// Пустой выбор означает весь год: витрина не должна открываться пустой.
func (f NetworkDashboardFilter) NormalizedQuarters() []int {
	seen := map[int]bool{}
	result := make([]int, 0, 4)
	for _, quarter := range f.Quarters {
		if quarter < 1 || quarter > 4 || seen[quarter] {
			continue
		}
		seen[quarter] = true
		result = append(result, quarter)
	}
	if len(result) == 0 {
		return []int{1, 2, 3, 4}
	}
	sort.Ints(result)
	return result
}

// Months — месяцы выбранных кварталов, по возрастанию.
func (f NetworkDashboardFilter) Months() []int {
	quarters := f.NormalizedQuarters()
	months := make([]int, 0, len(quarters)*3)
	for _, quarter := range quarters {
		for index := 0; index < 3; index++ {
			months = append(months, (quarter-1)*3+1+index)
		}
	}
	return months
}

// intArgs превращает набор чисел в условие IN и его аргументы.
func intArgs(values []int) (string, []interface{}) {
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return placeholders(len(values)), args
}

// NetworkDashboardPeriodData — план, факт и прогноз одного года.
//
// Groups — правила совместного зачёта смежных кварталов. Витрине они нужны
// по той же причине, что и карточке: право на инвестиции проверяется в
// границах правила, и без них Q1+Q2 считались бы двумя порогами вместо одного.
type NetworkDashboardPeriodData struct {
	Year      int
	Periods   []models.NetworkPeriod
	Plans     []models.NetworkPlan
	Facts     []models.NetworkMonthlyFact
	Forecasts []models.NetworkForecastLine
	Groups    []models.NetworkPeriodGroup
}

// NetworkDashboardPromoRow — промо, проведённое в срезе. Канал приходит из
// справочника механик, где он размечен как онлайн/оффлайн.
type NetworkDashboardPromoRow struct {
	NetworkName string
	Year        int
	Month       int
	BrandAS     *string
	Mechanics   *string
	Channel     *string
	ShortCode   *string
	PromoCount  int
	PlanRub     float64
	InvestRub   float64
}

// NetworkDashboardData — сырые строки, из которых собирается витрина.
// Наружу они не отдаются: агрегатор превращает их в models.NetworkDashboard*.
//
// Prev — тот же диапазон кварталов прошлого года. Он читается всегда: витрина
// нужна для разговора об итогах, а итог без сравнения с прошлым годом
// половина ответа.
type NetworkDashboardData struct {
	Networks       []models.Network
	Current        NetworkDashboardPeriodData
	Prev           NetworkDashboardPeriodData
	Promos         []NetworkDashboardPromoRow
	AvailableYears []int
}

// networkScope собирает условие видимости сетей. Префикс задаёт алиас таблицы
// сетей в запросе; для таблиц без него ограничение уходит в подзапрос по id.
func (f NetworkDashboardFilter) networkScope(alias string) (string, []interface{}) {
	where := ""
	args := []interface{}{}

	if strings.TrimSpace(f.OwnKAM) != "" {
		where += " AND " + alias + ".kam = ?"
		args = append(args, strings.TrimSpace(f.OwnKAM))
	}
	if len(f.KAMs) > 0 {
		where += " AND " + alias + ".kam IN (" + placeholders(len(f.KAMs)) + ")"
		for _, kam := range f.KAMs {
			args = append(args, kam)
		}
	}
	if len(f.NetworkIDs) > 0 {
		where += " AND " + alias + ".id IN (" + placeholders(len(f.NetworkIDs)) + ")"
		for _, id := range f.NetworkIDs {
			args = append(args, id)
		}
	}
	return where, args
}

// GetNetworkDashboardData читает всё, что нужно витрине, пятью запросами.
//
// Помесячные факт и прогноз читаются целиком за период: квартальные колонки
// tbl_NetworkPlans для витрины не годятся — они заполняются только загрузкой
// отгрузок и сохранением прогноза, поэтому по нетронутой сети пусты.
func GetNetworkDashboardData(filter NetworkDashboardFilter) (NetworkDashboardData, error) {
	var data NetworkDashboardData

	scope, scopeArgs := filter.networkScope("n")
	months := filter.Months()

	networks, err := dashboardNetworks(scope, scopeArgs)
	if err != nil {
		return data, err
	}
	data.Networks = networks
	if len(networks) == 0 {
		years, err := dashboardYears(scope, scopeArgs)
		if err != nil {
			return data, err
		}
		data.AvailableYears = years
		return data, nil
	}

	if data.Current, err = dashboardPeriodData(filter.Year, scope, scopeArgs); err != nil {
		return data, err
	}
	if data.Prev, err = dashboardPeriodData(filter.Year-1, scope, scopeArgs); err != nil {
		return data, err
	}
	if data.Promos, err = dashboardPromos(filter, scope, scopeArgs, months); err != nil {
		return data, err
	}
	if data.AvailableYears, err = dashboardYears(scope, scopeArgs); err != nil {
		return data, err
	}
	return data, nil
}

// dashboardPeriodData читает один год целиком: настройки кварталов, строки
// плана и помесячные факт с прогнозом.
func dashboardPeriodData(
	year int,
	scope string,
	scopeArgs []interface{},
) (NetworkDashboardPeriodData, error) {
	result := NetworkDashboardPeriodData{Year: year}
	var err error
	if result.Periods, err = dashboardPeriods(year, scope, scopeArgs); err != nil {
		return result, err
	}
	if result.Plans, err = dashboardPlans(year, scope, scopeArgs); err != nil {
		return result, err
	}
	if result.Facts, err = dashboardFacts(year, scope, scopeArgs); err != nil {
		return result, err
	}
	if result.Forecasts, err = dashboardForecasts(year, scope, scopeArgs); err != nil {
		return result, err
	}
	if result.Groups, err = dashboardPeriodGroups(year, scope, scopeArgs); err != nil {
		return result, err
	}
	return result, nil
}

// dashboardPeriodGroups читает правила совместного зачёта всех сетей области.
// Год берётся целиком, а не по выбранным кварталам: правило описывает свой
// диапазон само, и обрезать его фильтром значило бы менять условие зачёта.
func dashboardPeriodGroups(year int, scope string, scopeArgs []interface{}) ([]models.NetworkPeriodGroup, error) {
	query := `SELECT g.id, g.network_id, g.[year], g.start_quarter, g.end_quarter, g.brand_as,
			g.updated_by, CONVERT(NVARCHAR, g.updated_at, 121)
		FROM dbo.tbl_NetworkPeriodGroups g
		JOIN dbo.tbl_Networks n ON n.id = g.network_id
		WHERE g.[year] = ? AND n.is_active = 1` + scope + `
		ORDER BY g.network_id, g.start_quarter, g.end_quarter`

	args := append([]interface{}{year}, scopeArgs...)
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard period groups: %w", err)
	}
	defer rows.Close()

	result := []models.NetworkPeriodGroup{}
	for rows.Next() {
		var group models.NetworkPeriodGroup
		if err := rows.Scan(&group.ID, &group.NetworkID, &group.Year,
			&group.StartQuarter, &group.EndQuarter, &group.BrandAS,
			&group.UpdatedBy, &group.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dashboard period group: %w", err)
		}
		result = append(result, group)
	}
	return result, rows.Err()
}

func dashboardNetworks(scope string, scopeArgs []interface{}) ([]models.Network, error) {
	query := "SELECT " + networkColumns + ` FROM dbo.tbl_Networks n
		WHERE n.is_active = 1` + scope + " ORDER BY n.name ASC"

	rows, err := config.DB.Query(query, scopeArgs...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard networks: %w", err)
	}
	defer rows.Close()

	result := []models.Network{}
	for rows.Next() {
		network, err := scanNetwork(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dashboard network: %w", err)
		}
		result = append(result, network)
	}
	return result, rows.Err()
}

func dashboardPeriods(year int, scope string, scopeArgs []interface{}) ([]models.NetworkPeriod, error) {
	query := `SELECT p.id, p.network_id, p.[year], p.[quarter], p.vat_included, p.vat_rate,
			CONVERT(NVARCHAR, p.updated_at, 121)
		FROM dbo.tbl_NetworkPeriods p
		JOIN dbo.tbl_Networks n ON n.id = p.network_id
		WHERE p.[year] = ? AND n.is_active = 1` + scope

	args := append([]interface{}{year}, scopeArgs...)
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard periods: %w", err)
	}
	defer rows.Close()

	result := []models.NetworkPeriod{}
	for rows.Next() {
		var period models.NetworkPeriod
		if err := rows.Scan(&period.ID, &period.NetworkID, &period.Year, &period.Quarter,
			&period.VATIncluded, &period.VATRate, &period.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dashboard period: %w", err)
		}
		result = append(result, period)
	}
	return result, rows.Err()
}

// dashboardPlans читает строки плана вместе со строкой валового пула
// (brand_as IS NULL): без неё обязательство по контракту посчитать нельзя.
//
// pay_investments_from_fact читается наравне с процентом: этот режим отменяет
// порог выполнения и меняет базу начисления на факт, и без него витрина
// показала бы такому бренду ноль к выплате там, где выплата положена.
//
// Год читается целиком, без фильтра по выбранным кварталам: правило
// совместного зачёта охватывает соседние кварталы, и порог, посчитанный на
// половине своего периода, дал бы неверное право на выплату. Срез применяет
// агрегатор — наружу лишние кварталы не попадают.
func dashboardPlans(year int, scope string, scopeArgs []interface{}) ([]models.NetworkPlan, error) {
	query := `SELECT p.id, p.network_id, p.[year], p.[quarter], p.brand_as, p.in_gross,
			p.plan_rub, p.plan_units, n.month1_pct, n.month2_pct, n.month3_pct,
			p.investments_pct, p.entry_level, p.entry_unit,
			p.pay_investments_from_fact, p.paid_investments_rub,
			CONVERT(NVARCHAR, p.updated_at, 121)
		FROM dbo.tbl_NetworkPlans p
		JOIN dbo.tbl_Networks n ON n.id = p.network_id
		WHERE p.[year] = ? AND n.is_active = 1` + scope + `
		ORDER BY p.network_id, p.[quarter], p.brand_as`

	args := append([]interface{}{year}, scopeArgs...)
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard plans: %w", err)
	}
	defer rows.Close()

	result := []models.NetworkPlan{}
	for rows.Next() {
		var plan models.NetworkPlan
		if err := rows.Scan(&plan.ID, &plan.NetworkID, &plan.Year, &plan.Quarter,
			&plan.BrandAS, &plan.InGross, &plan.PlanRub, &plan.PlanUnits,
			&plan.Month1Pct, &plan.Month2Pct, &plan.Month3Pct,
			&plan.InvestmentsPct, &plan.EntryLevel, &plan.EntryUnit,
			&plan.PayInvestmentsFromFact, &plan.PaidInvestmentsRub,
			&plan.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dashboard plan: %w", err)
		}
		result = append(result, plan)
	}
	return result, rows.Err()
}

// dashboardFacts читает факт за весь год по той же причине, что и планы:
// право на выплату проверяется в границах правила зачёта, а оно может
// выходить за выбранные кварталы. Срез накладывает агрегатор.
func dashboardFacts(
	year int,
	scope string,
	scopeArgs []interface{},
) ([]models.NetworkMonthlyFact, error) {
	query := `SELECT f.network_id, f.[year], f.[month], f.brand_as, f.sku,
			f.fact_rub, f.fact_units, f.fact_investments_rub
		FROM dbo.tbl_NetworkMonthlyFacts f
		JOIN dbo.tbl_Networks n ON n.id = f.network_id
		WHERE f.[year] = ? AND n.is_active = 1` + scope

	args := append([]interface{}{year}, scopeArgs...)
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard facts: %w", err)
	}
	defer rows.Close()

	result := []models.NetworkMonthlyFact{}
	for rows.Next() {
		var fact models.NetworkMonthlyFact
		if err := rows.Scan(&fact.NetworkID, &fact.Year, &fact.Month, &fact.BrandAS, &fact.SKU,
			&fact.FactRub, &fact.FactUnits, &fact.FactInvestmentsRub); err != nil {
			return nil, fmt.Errorf("scan dashboard fact: %w", err)
		}
		result = append(result, fact)
	}
	return result, rows.Err()
}

// dashboardForecasts — прогноз за весь год, см. dashboardFacts.
func dashboardForecasts(
	year int,
	scope string,
	scopeArgs []interface{},
) ([]models.NetworkForecastLine, error) {
	query := `SELECT f.network_id, f.[year], f.[month], f.brand_as, f.sku,
			f.forecast_rub, f.forecast_units, f.forecast_investments_rub
		FROM dbo.tbl_NetworkForecasts f
		JOIN dbo.tbl_Networks n ON n.id = f.network_id
		WHERE f.[year] = ? AND n.is_active = 1` + scope

	args := append([]interface{}{year}, scopeArgs...)
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard forecasts: %w", err)
	}
	defer rows.Close()

	result := []models.NetworkForecastLine{}
	for rows.Next() {
		var line models.NetworkForecastLine
		if err := rows.Scan(&line.NetworkID, &line.Year, &line.Month, &line.BrandAS, &line.SKU,
			&line.ForecastRub, &line.ForecastUnits, &line.ForecastInvestmentsRub); err != nil {
			return nil, fmt.Errorf("scan dashboard forecast: %w", err)
		}
		result = append(result, line)
	}
	return result, rows.Err()
}

// dashboardPromos — промо, проведённые сетями среза, по месяцам.
//
// Группировка помесячная, а не поквартальная: квартал из месяцев складывается,
// обратно — нет, а витрине нужны оба разреза.
//
// Реестр и промо связаны только названием сети: отдельного ключа между ними
// нет, поэтому JOIN идёт по имени. Отменённые и удалённые промо в срез не
// попадают — метка должна означать реально проведённую активность.
//
// Канал и короткий код берутся из справочника механик. Механика без записи
// в справочнике остаётся без канала, а не приписывается к оффлайну по
// умолчанию; код в этом случае соберёт сам сервис.
func dashboardPromos(
	filter NetworkDashboardFilter,
	scope string,
	scopeArgs []interface{},
	months []int,
) ([]NetworkDashboardPromoRow, error) {
	monthPlaceholders, monthArgs := intArgs(months)
	query := `SELECT n.name, p.[year], p.[month],
			p.brand_as, p.mechanics, m.channel, m.short_code,
			COUNT(*) AS promo_count,
			SUM(ISNULL(p.plan_promo_rub, 0)),
			SUM(ISNULL(p.plan_investments_rub, 0))
		FROM dbo.tbl_PromoActivities p
		JOIN dbo.tbl_Networks n ON n.name = p.network_name
		LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON m.mechanics = p.mechanics
		WHERE p.deleted_at IS NULL
		  AND ISNULL(p.status, '') <> 'cancelled'
		  AND p.[year] = ?
		  AND p.[month] IN (` + monthPlaceholders + `)
		  AND n.is_active = 1` + scope + `
		GROUP BY n.name, p.[year], p.[month], p.brand_as, p.mechanics, m.channel, m.short_code`

	args := append(append([]interface{}{filter.Year}, monthArgs...), scopeArgs...)
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard promos: %w", err)
	}
	defer rows.Close()

	result := []NetworkDashboardPromoRow{}
	for rows.Next() {
		var row NetworkDashboardPromoRow
		if err := rows.Scan(&row.NetworkName, &row.Year, &row.Month, &row.BrandAS,
			&row.Mechanics, &row.Channel, &row.ShortCode, &row.PromoCount, &row.PlanRub, &row.InvestRub); err != nil {
			return nil, fmt.Errorf("scan dashboard promo: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// dashboardYears — годы, за которые в области пользователя вообще есть данные.
// План и факт живут в разных годах (план заводят вперёд, факт приходит назад),
// поэтому список собирается из обеих таблиц.
func dashboardYears(scope string, scopeArgs []interface{}) ([]int, error) {
	query := `SELECT DISTINCT [year] FROM (
			SELECT p.[year] AS [year] FROM dbo.tbl_NetworkPlans p
				JOIN dbo.tbl_Networks n ON n.id = p.network_id
				WHERE n.is_active = 1` + scope + `
			UNION ALL
			SELECT f.[year] AS [year] FROM dbo.tbl_NetworkMonthlyFacts f
				JOIN dbo.tbl_Networks n ON n.id = f.network_id
				WHERE n.is_active = 1` + scope + `
		) years ORDER BY [year]`

	args := append(append([]interface{}{}, scopeArgs...), scopeArgs...)
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboard years: %w", err)
	}
	defer rows.Close()

	result := []int{}
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, fmt.Errorf("scan dashboard year: %w", err)
		}
		result = append(result, year)
	}
	return result, rows.Err()
}
