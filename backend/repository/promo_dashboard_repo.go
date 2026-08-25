package repository

import (
	"fmt"

	"backend/config"
)

// PromoDashboardRow — минимальная строка, необходимая для всех агрегатов
// промо-дашборда. Числовые указатели сохраняют различие между NULL и нулём.
type PromoDashboardRow struct {
	Year                   int
	Month                  int
	NetworkName            *string
	BrandAS                *string
	SKU                    *string
	Mechanics              *string
	Channel                *string
	PlanPromoUnits         *float64
	PlanInvestmentsRub     *float64
	PlanPromoUpliftUnits   *float64
	PlanPromoUpliftRub     *float64
	GM                     *float64
	ActualPromoSalesUnits  *float64
	ActualInvestments      *float64
	ActualPromoUpliftUnits *float64
	ActualPromoUpliftRub   *float64
}

// GetPromoDashboardRows читает только поля, используемые агрегатором. Сырые
// строки остаются внутри backend и не передаются браузеру.
func GetPromoDashboardRows(params PromoFilterParams, channels []string) ([]PromoDashboardRow, error) {
	where, args := buildPromoWhere(params, channels)
	query := `SELECT
		COALESCE(p.[year], 0), COALESCE(p.[month], 0),
		p.network_name, p.brand_as, p.sku, p.mechanics, m.channel,
		p.plan_promo_units, p.plan_investments_rub,
		p.plan_promo_uplift_units, p.plan_promo_uplift_rub, p.gm,
		p.actual_promo_sales_units, p.actual_investments,
		p.actual_promo_uplift_units, p.actual_promo_uplift_rub
	FROM dbo.tbl_PromoActivities p
	LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON p.mechanics = m.mechanics` + where + `
	ORDER BY p.[year], p.[month], p.id`

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query promo dashboard rows: %w", err)
	}
	defer rows.Close()

	result := make([]PromoDashboardRow, 0)
	for rows.Next() {
		var row PromoDashboardRow
		if err := rows.Scan(
			&row.Year, &row.Month,
			&row.NetworkName, &row.BrandAS, &row.SKU, &row.Mechanics, &row.Channel,
			&row.PlanPromoUnits, &row.PlanInvestmentsRub,
			&row.PlanPromoUpliftUnits, &row.PlanPromoUpliftRub, &row.GM,
			&row.ActualPromoSalesUnits, &row.ActualInvestments,
			&row.ActualPromoUpliftUnits, &row.ActualPromoUpliftRub,
		); err != nil {
			return nil, fmt.Errorf("scan promo dashboard row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promo dashboard rows: %w", err)
	}
	return result, nil
}
