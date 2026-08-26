package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"backend/config"
	"backend/models"
)

var (
	ErrNetworkPriceOverlap         = errors.New("network contract price periods overlap")
	ErrNetworkPriceDeleteForbidden = errors.New("only manual network contract prices can be deleted")
	ErrNetworkClosedMonth          = errors.New("network forecast month is closed")
)

// GetNetworkMonthlyFacts возвращает атомарные месячные факты за диапазон лет.
func GetNetworkMonthlyFacts(networkID, yearFrom, yearTo int) ([]models.NetworkMonthlyFact, error) {
	rows, err := config.DB.Query(
		`SELECT id, network_id, [year], [month], brand_as, sku,
			fact_rub, fact_units, fact_investments_rub, is_final, source_name,
			CONVERT(NVARCHAR, updated_at, 121)
		 FROM dbo.tbl_NetworkMonthlyFacts
		 WHERE network_id = ? AND [year] BETWEEN ? AND ?
		 ORDER BY [year], [month], brand_as, sku`,
		networkID, yearFrom, yearTo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkMonthlyFact{}
	for rows.Next() {
		var row models.NetworkMonthlyFact
		if err := rows.Scan(
			&row.ID, &row.NetworkID, &row.Year, &row.Month, &row.BrandAS, &row.SKU,
			&row.FactRub, &row.FactUnits, &row.FactInvestmentsRub, &row.IsFinal,
			&row.SourceName, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetNetworkForecastLines читает официальный и SKU-прогноз выбранного квартала.
func GetNetworkForecastLines(networkID, year, quarter int) ([]models.NetworkForecastLine, error) {
	monthFrom := (quarter-1)*3 + 1
	monthTo := monthFrom + 2
	rows, err := config.DB.Query(
		`SELECT id, network_id, [year], [month], brand_as, sku,
			forecast_rub, forecast_units, forecast_investments_rub,
			system_forecast_rub, system_forecast_units, confidence,
			adjustment_reason, updated_by, CONVERT(NVARCHAR, updated_at, 121)
		 FROM dbo.tbl_NetworkForecasts
		 WHERE network_id = ? AND [year] = ? AND [month] BETWEEN ? AND ?
		 ORDER BY brand_as, sku, [month]`,
		networkID, year, monthFrom, monthTo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkForecastLine{}
	for rows.Next() {
		var row models.NetworkForecastLine
		if err := rows.Scan(
			&row.ID, &row.NetworkID, &row.Year, &row.Month, &row.BrandAS, &row.SKU,
			&row.ForecastRub, &row.ForecastUnits, &row.ForecastInvestmentsRub,
			&row.SystemForecastRub, &row.SystemForecastUnits, &row.Confidence,
			&row.AdjustmentReason, &row.UpdatedBy, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetNetworkPromoIndicators собирает только компактные признаки прогноза.
// В суммы входят согласованные на обоих этапах промо; черновики видны счётчиком.
func GetNetworkPromoIndicators(networkName string, year, quarter int) ([]models.NetworkPromoIndicator, error) {
	monthFrom := (quarter-1)*3 + 1
	monthTo := monthFrom + 2
	rows, err := config.DB.Query(
		`SELECT p.[year], p.[month], p.brand_as,
			COUNT(*) AS promo_count,
			SUM(CASE WHEN p.agreement1_status = 'approved' AND p.agreement2_status = 'approved' THEN 1 ELSE 0 END),
			SUM(CASE WHEN ISNULL(p.agreement1_status, '') <> 'approved' OR ISNULL(p.agreement2_status, '') <> 'approved' THEN 1 ELSE 0 END),
			SUM(CASE WHEN p.agreement1_status = 'approved' AND p.agreement2_status = 'approved' THEN ISNULL(p.plan_promo_units, 0) ELSE 0 END),
			SUM(CASE WHEN p.agreement1_status = 'approved' AND p.agreement2_status = 'approved' THEN ISNULL(p.plan_promo_rub, 0) ELSE 0 END),
			SUM(CASE WHEN p.agreement1_status = 'approved' AND p.agreement2_status = 'approved' THEN ISNULL(p.plan_investments_rub, 0) ELSE 0 END),
			SUM(CASE WHEN p.agreement1_status = 'approved' AND p.agreement2_status = 'approved' THEN ISNULL(p.plan_promo_uplift_rub, 0) ELSE 0 END),
			SUM(CASE WHEN p.agreement1_status = 'approved' AND p.agreement2_status = 'approved' THEN ISNULL(p.plan_promo_uplift_units, 0) ELSE 0 END)
		 FROM dbo.tbl_PromoActivities p
		 WHERE p.deleted_at IS NULL
		   AND p.network_name = ?
		   AND p.[year] = ?
		   AND p.[month] BETWEEN ? AND ?
		   AND p.brand_as IS NOT NULL
		   AND ISNULL(p.status, '') <> 'cancelled'
		 GROUP BY p.[year], p.[month], p.brand_as
		 ORDER BY p.[month], p.brand_as`,
		networkName, year, monthFrom, monthTo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkPromoIndicator{}
	for rows.Next() {
		var row models.NetworkPromoIndicator
		if err := rows.Scan(
			&row.Year, &row.Month, &row.BrandAS, &row.PromoCount,
			&row.ApprovedCount, &row.DraftCount, &row.PlanPromoUnits,
			&row.PlanPromoRub, &row.PlanInvestmentsRub, &row.PlanUpliftRub, &row.PlanUpliftUnits,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// NetworkForecastInput — строка сохранения официального или SKU-прогноза.
type NetworkForecastInput struct {
	Month                  int      `json:"month"`
	BrandAS                string   `json:"brand_as"`
	SKU                    *string  `json:"sku"`
	ForecastRub            *float64 `json:"forecast_rub"`
	ForecastUnits          *float64 `json:"forecast_units"`
	ForecastInvestmentsRub *float64 `json:"forecast_investments_rub"`
	AdjustmentReason       *string  `json:"adjustment_reason"`
	UpdatedAt              string   `json:"updated_at"`
}

type SaveNetworkForecastInput struct {
	NetworkID int
	Year      int
	Quarter   int
	Lines     []NetworkForecastInput
	UserName  string
}

type ClearNetworkForecastInput struct {
	NetworkID int
	Year      int
	Month     int
	Scope     string
	UserName  string
}

func validateForecastValue(name string, value *float64) error {
	if value != nil && *value < 0 {
		return fmt.Errorf("%s не может быть отрицательным", name)
	}
	return nil
}

// forecastEntryLevels — уровень ведения каждого бренда квартала. Бренда,
// которого в плане нет, здесь тоже нет: его уровень определит профиль сети.
func forecastEntryLevels(networkID, year, quarter int) (map[string]string, string, error) {
	network, err := GetNetworkByID(networkID)
	if err != nil {
		return nil, "", err
	}
	plans, err := GetNetworkPlans(networkID, year)
	if err != nil {
		return nil, "", err
	}
	levels := make(map[string]string, len(plans))
	for _, plan := range plans {
		if plan.Quarter != quarter || plan.BrandAS == nil {
			continue
		}
		level := plan.EntryLevel
		if level == "" {
			level = network.DefaultEntryLevel
		}
		if level != "sku" {
			level = "brand"
		}
		levels[*plan.BrandAS] = level
	}
	fallback := network.DefaultEntryLevel
	if fallback != "sku" {
		fallback = "brand"
	}
	return levels, fallback, nil
}

// SaveNetworkForecastLines сохраняет переданные строки без удаления остальных:
// внутри своего уровня бренд и его SKU заполняются независимо.
//
// Уровень ведения бренда при этом соблюдается: писать значение на уровне,
// который считается расчётным, нельзя. Иначе в базе появилась бы вторая версия
// той же величины — та самая рассинхронизация, ради которой уровень и заведён.
func SaveNetworkForecastLines(in SaveNetworkForecastInput) error {
	monthFrom := (in.Quarter-1)*3 + 1
	monthTo := monthFrom + 2
	now := time.Now()
	entryLevels, defaultLevel, err := forecastEntryLevels(in.NetworkID, in.Year, in.Quarter)
	if err != nil {
		return err
	}
	tx, err := config.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, line := range in.Lines {
		line.BrandAS = strings.TrimSpace(line.BrandAS)
		if line.Month < monthFrom || line.Month > monthTo {
			return fmt.Errorf("месяц %d не относится к Q%d", line.Month, in.Quarter)
		}
		if in.Year < now.Year() || (in.Year == now.Year() && line.Month < int(now.Month())) {
			return fmt.Errorf("%w: %02d.%d", ErrNetworkClosedMonth, line.Month, in.Year)
		}
		if line.BrandAS == "" {
			return errors.New("бренд прогноза не указан")
		}
		if err := validateForecastValue("прогноз", line.ForecastRub); err != nil {
			return err
		}
		if err := validateForecastValue("прогноз упаковок", line.ForecastUnits); err != nil {
			return err
		}
		if err := validateForecastValue("прогноз инвестиций", line.ForecastInvestmentsRub); err != nil {
			return err
		}

		level, known := entryLevels[line.BrandAS]
		if !known {
			level = defaultLevel
		}
		hasSKU := line.SKU != nil && strings.TrimSpace(*line.SKU) != ""
		if level == "sku" && !hasSKU && (line.ForecastRub != nil || line.ForecastUnits != nil) {
			return fmt.Errorf(
				"%s ведётся по SKU: объём вносится в SKU-строки, строка бренда считается суммой",
				line.BrandAS,
			)
		}
		if level == "brand" && hasSKU {
			return fmt.Errorf(
				"%s ведётся на уровне бренда: чтобы вносить SKU, переключите бренд на детализацию",
				line.BrandAS,
			)
		}

		var id int64
		var updatedAt string
		var lookupErr error
		if line.SKU == nil || strings.TrimSpace(*line.SKU) == "" {
			line.SKU = nil
			lookupErr = tx.QueryRow(
				`SELECT id, CONVERT(NVARCHAR, updated_at, 121)
				 FROM dbo.tbl_NetworkForecasts
				 WHERE network_id = ? AND [year] = ? AND [month] = ? AND brand_as = ? AND sku IS NULL`,
				in.NetworkID, in.Year, line.Month, line.BrandAS,
			).Scan(&id, &updatedAt)
		} else {
			sku := strings.TrimSpace(*line.SKU)
			line.SKU = &sku
			lookupErr = tx.QueryRow(
				`SELECT id, CONVERT(NVARCHAR, updated_at, 121)
				 FROM dbo.tbl_NetworkForecasts
				 WHERE network_id = ? AND [year] = ? AND [month] = ? AND brand_as = ? AND sku = ?`,
				in.NetworkID, in.Year, line.Month, line.BrandAS, sku,
			).Scan(&id, &updatedAt)
		}

		switch {
		case lookupErr == nil:
			if line.UpdatedAt != "" && line.UpdatedAt != updatedAt {
				return ErrNetworkConflict
			}
			_, err = tx.Exec(
				`UPDATE dbo.tbl_NetworkForecasts
				 SET forecast_rub = ?, forecast_units = ?, forecast_investments_rub = ?,
				     adjustment_reason = ?, updated_by = ?, updated_at = GETDATE()
				 WHERE id = ?`,
				line.ForecastRub, line.ForecastUnits, line.ForecastInvestmentsRub,
				line.AdjustmentReason, in.UserName, id,
			)
		case errors.Is(lookupErr, sql.ErrNoRows):
			_, err = tx.Exec(
				`INSERT INTO dbo.tbl_NetworkForecasts (
					network_id, [year], [month], brand_as, sku,
					forecast_rub, forecast_units, forecast_investments_rub,
					adjustment_reason, updated_by
				 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				in.NetworkID, in.Year, line.Month, line.BrandAS, line.SKU,
				line.ForecastRub, line.ForecastUnits, line.ForecastInvestmentsRub,
				line.AdjustmentReason, in.UserName,
			)
		default:
			return lookupErr
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ClearNetworkForecastMonth очищает только внесённый прогноз объёма. Строки
// сохраняются для аудита и для независимого прогноза инвестиций; системная
// рекомендация после очистки снова становится fallback для EAC.
func ClearNetworkForecastMonth(in ClearNetworkForecastInput) (int64, error) {
	now := time.Now()
	if in.Year < now.Year() || (in.Year == now.Year() && in.Month < int(now.Month())) {
		return 0, fmt.Errorf("%w: %02d.%d", ErrNetworkClosedMonth, in.Month, in.Year)
	}
	if in.Month < 1 || in.Month > 12 {
		return 0, errors.New("некорректный месяц прогноза")
	}

	var query string
	switch in.Scope {
	case "rub":
		query = `UPDATE dbo.tbl_NetworkForecasts
			SET forecast_rub = NULL, adjustment_reason = NULL,
				updated_by = ?, updated_at = GETDATE()
			WHERE network_id = ? AND [year] = ? AND [month] = ?
			  AND forecast_rub IS NOT NULL`
	case "units":
		query = `UPDATE dbo.tbl_NetworkForecasts
			SET forecast_units = NULL, adjustment_reason = NULL,
				updated_by = ?, updated_at = GETDATE()
			WHERE network_id = ? AND [year] = ? AND [month] = ?
			  AND forecast_units IS NOT NULL`
	case "all":
		query = `UPDATE dbo.tbl_NetworkForecasts
			SET forecast_rub = NULL, forecast_units = NULL, adjustment_reason = NULL,
				updated_by = ?, updated_at = GETDATE()
			WHERE network_id = ? AND [year] = ? AND [month] = ?
			  AND (forecast_rub IS NOT NULL OR forecast_units IS NOT NULL)`
	default:
		return 0, errors.New("область очистки: rub, units или all")
	}

	result, err := config.DB.Exec(query, in.UserName, in.NetworkID, in.Year, in.Month)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// UpdateNetworkPlanForecastRollup поддерживает квартальную форму совместимой с
// новым месячным прогнозом: в старой строке сохраняется именно EAC квартала.
func UpdateNetworkPlanForecastRollup(networkID, year, quarter int, totals []models.NetworkForecastBrandTotals) error {
	tx, err := config.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, total := range totals {
		if _, err := tx.Exec(
			`UPDATE dbo.tbl_NetworkPlans
			 SET forecast_rub = ?, forecast_investments_rub = ?, updated_at = GETDATE()
			 WHERE network_id = ? AND [year] = ? AND [quarter] = ? AND brand_as = ?`,
			total.EACRub, total.EACInvestmentsRub, networkID, year, quarter, total.BrandAS,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// NetworkContractPriceInput — редактируемые поля строки цены.
type NetworkContractPriceInput struct {
	ID            int64   `json:"id"`
	BrandAS       string  `json:"brand_as"`
	SKU           string  `json:"sku"`
	ContractPrice float64 `json:"contract_price"`
	ValidFrom     string  `json:"valid_from"`
	ValidTo       string  `json:"valid_to"`
	IsConfirmed   bool    `json:"is_confirmed"`
	UpdatedAt     string  `json:"updated_at"`
}

type NetworkContractPriceDeleteInput struct {
	ID        int64  `json:"id"`
	UpdatedAt string `json:"updated_at"`
}

type SaveNetworkPricesInput struct {
	NetworkID   int
	Rows        []NetworkContractPriceInput
	DeletedRows []NetworkContractPriceDeleteInput
	UserName    string
}

type olapSKUPrice struct {
	BrandAS string
	SKU     string
	Price   float64
	Year    int
	Month   int
}

// Цена OLAP SS едина для SKU: название сети намеренно не участвует ни в
// фильтрации, ни в группировке. Месяц выбирается один для всего среза года,
// после чего цена считается как SUM(руб) / SUM(уп) по всем сетям.
const globalOlapSSSKUPricesQuery = `WITH latest_month AS (
	SELECT TOP 1 n.[month]
	FROM dbo.tbl_EcomSalesNormalized n
	WHERE n.[year] = ? AND n.segment = N'OLAP SS'
	GROUP BY n.[month]
	HAVING SUM(CASE WHEN n.un_rub = N'руб' THEN n.metric_value ELSE 0 END) > 0
	   AND SUM(CASE WHEN n.un_rub = N'уп' THEN n.metric_value ELSE 0 END) > 0
	ORDER BY n.[month] DESC
)
SELECT
	COALESCE(
		MAX(NULLIF(LTRIM(RTRIM(sm.brand_as)), N'')),
		MAX(NULLIF(LTRIM(RTRIM(n.brandName)), N'')),
		N'Без бренда'
	) AS brand_as,
	LTRIM(RTRIM(n.productName)) AS sku,
	SUM(CASE WHEN n.un_rub = N'руб' THEN n.metric_value ELSE 0 END)
		/ NULLIF(SUM(CASE WHEN n.un_rub = N'уп' THEN n.metric_value ELSE 0 END), 0) AS price,
	n.[year], n.[month]
FROM dbo.tbl_EcomSalesNormalized n
JOIN latest_month lm ON lm.[month] = n.[month]
LEFT JOIN dbo.tbl_SKUMapping sm ON LTRIM(RTRIM(sm.sku)) = LTRIM(RTRIM(n.productName))
WHERE n.[year] = ?
  AND n.segment = N'OLAP SS'
  AND n.productName IS NOT NULL
  AND LTRIM(RTRIM(n.productName)) <> N''
GROUP BY LTRIM(RTRIM(n.productName)), n.[year], n.[month]
HAVING SUM(CASE WHEN n.un_rub = N'руб' THEN n.metric_value ELSE 0 END) > 0
   AND SUM(CASE WHEN n.un_rub = N'уп' THEN n.metric_value ELSE 0 END) > 0
ORDER BY brand_as, sku`

func getGlobalOlapSSSKUPrices(year int) ([]olapSKUPrice, error) {
	rows, err := config.DB.Query(globalOlapSSSKUPricesQuery, year, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []olapSKUPrice{}
	for rows.Next() {
		var row olapSKUPrice
		if err := rows.Scan(&row.BrandAS, &row.SKU, &row.Price, &row.Year, &row.Month); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetNetworkPriceSKUOptions возвращает тот же общий срез OLAP SS, который
// используется для первичной подстановки цен. В ответ входят и ранее
// исключённые из конкретной сети SKU, чтобы пользователь мог добавить их
// обратно явно.
func GetNetworkPriceSKUOptions() ([]models.NetworkPriceSKUOption, error) {
	rows, err := getGlobalOlapSSSKUPrices(2026)
	if err != nil {
		return nil, err
	}
	result := make([]models.NetworkPriceSKUOption, 0, len(rows))
	for _, row := range rows {
		result = append(result, models.NetworkPriceSKUOption{
			BrandAS:     row.BrandAS,
			SKU:         row.SKU,
			Price:       row.Price,
			SourceYear:  row.Year,
			SourceMonth: row.Month,
		})
	}
	return result, nil
}

func contractPriceSKUKey(sku string) string {
	return strings.ToLower(strings.TrimSpace(sku))
}

func intPointer(value int) *int {
	return &value
}

// mergeNetworkContractPrices добавляет отсутствующие строки из OLAP SS и
// освежает только неподтверждённые автозаполненные цены. Ручные и
// подтверждённые значения всегда имеют приоритет.
func mergeNetworkContractPrices(
	networkID, year int,
	persisted []models.NetworkContractPrice,
	defaults, comparisons []olapSKUPrice,
	excluded map[string]bool,
) []models.NetworkContractPrice {
	defaultBySKU := make(map[string]olapSKUPrice, len(defaults))
	for _, row := range defaults {
		defaultBySKU[contractPriceSKUKey(row.SKU)] = row
	}
	comparisonBySKU := make(map[string]olapSKUPrice, len(comparisons))
	for _, row := range comparisons {
		comparisonBySKU[contractPriceSKUKey(row.SKU)] = row
	}

	result := make([]models.NetworkContractPrice, 0, len(persisted)+len(defaults))
	persistedSKU := make(map[string]bool, len(persisted))
	for _, row := range persisted {
		key := contractPriceSKUKey(row.SKU)
		persistedSKU[key] = true

		if source, ok := defaultBySKU[key]; ok && row.SourceType == "olap_seed" && !row.IsConfirmed {
			row.BrandAS = source.BrandAS
			row.ContractPrice = source.Price
			row.SourceYear = intPointer(source.Year)
			row.SourceMonth = intPointer(source.Month)
		}
		if comparison, ok := comparisonBySKU[key]; ok {
			row.OlapPrice = &comparison.Price
			row.OlapYear = intPointer(comparison.Year)
			row.OlapMonth = intPointer(comparison.Month)
		}
		result = append(result, row)
	}

	for _, source := range defaults {
		key := contractPriceSKUKey(source.SKU)
		if persistedSKU[key] || excluded[key] {
			continue
		}
		row := models.NetworkContractPrice{
			NetworkID:     networkID,
			BrandAS:       source.BrandAS,
			SKU:           source.SKU,
			ContractPrice: source.Price,
			ValidFrom:     fmt.Sprintf("%04d-01-01", year),
			ValidTo:       fmt.Sprintf("%04d-12-31", year),
			SourceType:    "olap_seed",
			SourceYear:    intPointer(source.Year),
			SourceMonth:   intPointer(source.Month),
		}
		if comparison, ok := comparisonBySKU[key]; ok {
			row.OlapPrice = &comparison.Price
			row.OlapYear = intPointer(comparison.Year)
			row.OlapMonth = intPointer(comparison.Month)
		}
		result = append(result, row)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].BrandAS != result[j].BrandAS {
			return result[i].BrandAS < result[j].BrandAS
		}
		if result[i].SKU != result[j].SKU {
			return result[i].SKU < result[j].SKU
		}
		return result[i].ValidFrom < result[j].ValidFrom
	})
	return result
}

func getNetworkContractPriceExclusions(networkID int) (map[string]bool, error) {
	rows, err := config.DB.Query(
		`SELECT sku FROM dbo.tbl_NetworkContractPriceExclusions WHERE network_id = ?`,
		networkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	excluded := make(map[string]bool)
	for rows.Next() {
		var sku string
		if err := rows.Scan(&sku); err != nil {
			return nil, err
		}
		excluded[contractPriceSKUKey(sku)] = true
	}
	return excluded, rows.Err()
}

// GetNetworkContractPrices возвращает цены, пересекающие выбранный год.
// Для отсутствующих SKU подставляется общая для всех сетей цена OLAP SS из
// последнего доступного месяца 2026 года. OLAP-сравнение также не зависит от
// сети и берётся из последнего доступного месяца выбранного года.
func GetNetworkContractPrices(networkID, year int) ([]models.NetworkContractPrice, error) {
	start := fmt.Sprintf("%04d-01-01", year)
	end := fmt.Sprintf("%04d-12-31", year)
	rows, err := config.DB.Query(
		`SELECT p.id, p.network_id, p.brand_as, p.sku, p.contract_price,
			CONVERT(NVARCHAR(10), p.valid_from, 23), CONVERT(NVARCHAR(10), p.valid_to, 23),
			p.source_type, p.source_year, p.source_month, p.is_confirmed, p.updated_by,
			CONVERT(NVARCHAR, p.updated_at, 121)
		 FROM dbo.tbl_NetworkContractPrices p
		 WHERE p.network_id = ? AND p.valid_from <= ? AND p.valid_to >= ?
		 ORDER BY p.brand_as, p.sku, p.valid_from`,
		networkID, end, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkContractPrice{}
	for rows.Next() {
		var row models.NetworkContractPrice
		if err := rows.Scan(
			&row.ID, &row.NetworkID, &row.BrandAS, &row.SKU, &row.ContractPrice,
			&row.ValidFrom, &row.ValidTo, &row.SourceType, &row.SourceYear, &row.SourceMonth,
			&row.IsConfirmed, &row.UpdatedBy, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	defaults, err := getGlobalOlapSSSKUPrices(2026)
	if err != nil {
		return nil, err
	}
	comparisons := defaults
	if year != 2026 {
		comparisons, err = getGlobalOlapSSSKUPrices(year)
		if err != nil {
			return nil, err
		}
	}
	excluded, err := getNetworkContractPriceExclusions(networkID)
	if err != nil {
		return nil, err
	}
	return mergeNetworkContractPrices(networkID, year, result, defaults, comparisons, excluded), nil
}

func parsePriceDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

// SaveNetworkContractPrices подтверждает предзаполненные цены и добавляет новые
// периоды. Пересекающиеся интервалы одного SKU запрещены.
func SaveNetworkContractPrices(in SaveNetworkPricesInput) error {
	tx, err := config.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	deletedIDs := make(map[int64]bool, len(in.DeletedRows))
	deletedSKUs := make(map[string]string, len(in.DeletedRows))
	activeSKUs := make(map[string]bool, len(in.Rows))
	for _, row := range in.Rows {
		if sku := strings.TrimSpace(row.SKU); sku != "" {
			activeSKUs[contractPriceSKUKey(sku)] = true
		}
	}
	for _, row := range in.DeletedRows {
		if row.ID <= 0 || deletedIDs[row.ID] {
			continue
		}
		deletedIDs[row.ID] = true

		var updatedAt, sourceType, sku string
		if err := tx.QueryRow(
			`SELECT CONVERT(NVARCHAR, updated_at, 121), source_type, sku
			 FROM dbo.tbl_NetworkContractPrices WHERE id = ? AND network_id = ?`,
			row.ID, in.NetworkID,
		).Scan(&updatedAt, &sourceType, &sku); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNetworkNotFound
			}
			return err
		}
		if row.UpdatedAt != "" && row.UpdatedAt != updatedAt {
			return ErrNetworkConflict
		}
		if sourceType != "manual" {
			return ErrNetworkPriceDeleteForbidden
		}
		deletedSKUs[contractPriceSKUKey(sku)] = strings.TrimSpace(sku)
		if _, err := tx.Exec(
			`DELETE FROM dbo.tbl_NetworkContractPrices WHERE id = ? AND network_id = ?`,
			row.ID, in.NetworkID,
		); err != nil {
			return err
		}
	}

	// Удалённый из формы SKU больше не должен автоматически возвращаться из
	// OLAP SS. Если тот же SKU остаётся среди сохраняемых строк, исключение не
	// ставим: это частичная нормализация периодов, а не удаление SKU целиком.
	for key, sku := range deletedSKUs {
		if activeSKUs[key] {
			continue
		}
		res, err := tx.Exec(
			`UPDATE dbo.tbl_NetworkContractPriceExclusions
			 SET sku = ?, excluded_by = ?, excluded_at = GETDATE()
			 WHERE network_id = ? AND sku = ?`,
			sku, nullIfEmpty(in.UserName), in.NetworkID, sku,
		)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			if _, err := tx.Exec(
				`INSERT INTO dbo.tbl_NetworkContractPriceExclusions (network_id, sku, excluded_by)
				 VALUES (?, ?, ?)`,
				in.NetworkID, sku, nullIfEmpty(in.UserName),
			); err != nil {
				return err
			}
		}
	}
	for key := range activeSKUs {
		if _, err := tx.Exec(
			`DELETE FROM dbo.tbl_NetworkContractPriceExclusions
			 WHERE network_id = ? AND LOWER(LTRIM(RTRIM(sku))) = ?`,
			in.NetworkID, key,
		); err != nil {
			return err
		}
	}

	for _, row := range in.Rows {
		row.BrandAS = strings.TrimSpace(row.BrandAS)
		row.SKU = strings.TrimSpace(row.SKU)
		from, fromErr := parsePriceDate(row.ValidFrom)
		to, toErr := parsePriceDate(row.ValidTo)
		if row.BrandAS == "" || row.SKU == "" || fromErr != nil || toErr != nil || from.After(to) {
			return errors.New("некорректная строка цены контракта")
		}
		if row.ContractPrice <= 0 {
			return errors.New("цена контракта должна быть больше нуля")
		}

		var overlaps int
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM dbo.tbl_NetworkContractPrices
			 WHERE network_id = ? AND sku = ? AND id <> ?
			   AND valid_from <= ? AND valid_to >= ?`,
			in.NetworkID, row.SKU, row.ID, row.ValidTo, row.ValidFrom,
		).Scan(&overlaps); err != nil {
			return err
		}
		if overlaps > 0 {
			return fmt.Errorf("%w: %s", ErrNetworkPriceOverlap, row.SKU)
		}

		if row.ID > 0 {
			var updatedAt, sourceType string
			var existingPrice float64
			var sourceYear, sourceMonth sql.NullInt64
			if err := tx.QueryRow(
				`SELECT CONVERT(NVARCHAR, updated_at, 121), contract_price,
				        source_type, source_year, source_month
				 FROM dbo.tbl_NetworkContractPrices WHERE id = ? AND network_id = ?`,
				row.ID, in.NetworkID,
			).Scan(&updatedAt, &existingPrice, &sourceType, &sourceYear, &sourceMonth); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrNetworkNotFound
				}
				return err
			}
			if row.UpdatedAt != "" && row.UpdatedAt != updatedAt {
				return ErrNetworkConflict
			}
			// Явно изменённая КАМ цена больше не является автозначением OLAP:
			// иначе следующий GET снова подставит поверх неё свежий default.
			if math.Abs(existingPrice-row.ContractPrice) > 0.00005 {
				sourceType = "manual"
				sourceYear = sql.NullInt64{}
				sourceMonth = sql.NullInt64{}
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkContractPrices
				 SET brand_as = ?, sku = ?, contract_price = ?, valid_from = ?, valid_to = ?,
				     source_type = ?, source_year = ?, source_month = ?,
				     is_confirmed = ?, updated_by = ?, updated_at = GETDATE()
				 WHERE id = ? AND network_id = ?`,
				row.BrandAS, row.SKU, row.ContractPrice, row.ValidFrom, row.ValidTo,
				sourceType, sourceYear, sourceMonth, row.IsConfirmed, in.UserName,
				row.ID, in.NetworkID,
			); err != nil {
				return err
			}
			continue
		}

		if _, err := tx.Exec(
			`INSERT INTO dbo.tbl_NetworkContractPrices (
				network_id, brand_as, sku, contract_price, valid_from, valid_to,
				source_type, is_confirmed, updated_by
			 ) VALUES (?, ?, ?, ?, ?, ?, 'manual', ?, ?)`,
			in.NetworkID, row.BrandAS, row.SKU, row.ContractPrice,
			row.ValidFrom, row.ValidTo, row.IsConfirmed, in.UserName,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
