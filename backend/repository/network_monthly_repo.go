package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/config"
	"backend/models"
)

var (
	ErrNetworkPriceOverlap = errors.New("network contract price periods overlap")
	ErrNetworkClosedMonth  = errors.New("network forecast month is closed")
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

func validateForecastValue(name string, value *float64) error {
	if value != nil && *value < 0 {
		return fmt.Errorf("%s не может быть отрицательным", name)
	}
	return nil
}

// SaveNetworkForecastLines сохраняет переданные строки без удаления остальных:
// брендовый прогноз и детализация SKU могут заполняться независимо.
func SaveNetworkForecastLines(in SaveNetworkForecastInput) error {
	monthFrom := (in.Quarter-1)*3 + 1
	monthTo := monthFrom + 2
	now := time.Now()
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

type SaveNetworkPricesInput struct {
	NetworkID int
	Rows      []NetworkContractPriceInput
	UserName  string
}

// GetNetworkContractPrices возвращает цены, пересекающие выбранный год, и
// последнюю OLAP-цену того же года по той же сети и SKU.
func GetNetworkContractPrices(networkID, year int) ([]models.NetworkContractPrice, error) {
	start := fmt.Sprintf("%04d-01-01", year)
	end := fmt.Sprintf("%04d-12-31", year)
	rows, err := config.DB.Query(
		`SELECT p.id, p.network_id, p.brand_as, p.sku, p.contract_price,
			CONVERT(NVARCHAR(10), p.valid_from, 23), CONVERT(NVARCHAR(10), p.valid_to, 23),
			p.source_type, p.source_year, p.source_month, p.is_confirmed,
			olap.price, olap.[year], olap.[month], p.updated_by,
			CONVERT(NVARCHAR, p.updated_at, 121)
		 FROM dbo.tbl_NetworkContractPrices p
		 JOIN dbo.tbl_Networks n ON n.id = p.network_id
		 OUTER APPLY (
			SELECT TOP 1
				SUM(s.rub) / NULLIF(SUM(s.qty), 0) AS price,
				s.[year], s.[month]
			FROM dbo.tbl_EcomSalesConsolidated s
			WHERE s.[year] = ?
			  AND LTRIM(RTRIM(s.networkName)) = LTRIM(RTRIM(n.name))
			  AND LTRIM(RTRIM(s.productName)) = LTRIM(RTRIM(p.sku))
			  AND s.rub IS NOT NULL AND s.qty IS NOT NULL
			GROUP BY s.[year], s.[month]
			HAVING SUM(s.qty) > 0
			ORDER BY s.[month] DESC
		 ) olap
		 WHERE p.network_id = ? AND p.valid_from <= ? AND p.valid_to >= ?
		 ORDER BY p.brand_as, p.sku, p.valid_from`,
		year, networkID, end, start,
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
			&row.IsConfirmed, &row.OlapPrice, &row.OlapYear, &row.OlapMonth,
			&row.UpdatedBy, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
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
			var updatedAt string
			if err := tx.QueryRow(
				`SELECT CONVERT(NVARCHAR, updated_at, 121)
				 FROM dbo.tbl_NetworkContractPrices WHERE id = ? AND network_id = ?`,
				row.ID, in.NetworkID,
			).Scan(&updatedAt); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrNetworkNotFound
				}
				return err
			}
			if row.UpdatedAt != "" && row.UpdatedAt != updatedAt {
				return ErrNetworkConflict
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkContractPrices
				 SET brand_as = ?, sku = ?, contract_price = ?, valid_from = ?, valid_to = ?,
				     is_confirmed = ?, updated_by = ?, updated_at = GETDATE()
				 WHERE id = ? AND network_id = ?`,
				row.BrandAS, row.SKU, row.ContractPrice, row.ValidFrom, row.ValidTo,
				row.IsConfirmed, in.UserName, row.ID, in.NetworkID,
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
