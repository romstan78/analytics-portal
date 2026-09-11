package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"backend/config"
	"backend/models"
)

// ─── Бюджет OPEX по контракту ───────────────────────────────────────────────
//
// Хранится помесячно, вводится кварталом. Раскладка квартала на месяцы — расчёт,
// и живёт он в services.NetworkOpexMonthlyRows: репозиторий получает готовые
// строки месяцев и только пишет их.

// NetworkOpexInput — квартальная ячейка бюджета, как её ввёл КАМ.
//
// AmountRub == nil означает «значение снято», а не ноль: согласованный нулевой
// бюджет — это тоже решение, и отличать его от незаведённого обязательно.
//
// UpdatedAt — версия ячейки, полученная при чтении: самая свежая из трёх её
// месяцев. Пустая строка означает «ячейки не было»; если она к этому времени
// появилась, запись получает 409, как и всюду в реестре.
type NetworkOpexInput struct {
	Quarter   int      `json:"quarter"`
	BrandAS   string   `json:"brand_as"`
	Article   string   `json:"article"`
	AmountRub *float64 `json:"amount_rub"`
	UpdatedAt string   `json:"updated_at"`
}

// NetworkOpexMonthWrite — строка месяца, готовая к записи.
type NetworkOpexMonthWrite struct {
	Month     int
	AmountRub float64
	AmountNet float64
}

// NetworkOpexCellWrite — что сделать с одной квартальной ячейкой.
// Пустой Months — значение снято: месячные строки ячейки удаляются.
type NetworkOpexCellWrite struct {
	Quarter   int
	BrandAS   string
	Article   string
	UpdatedAt string
	Months    []NetworkOpexMonthWrite
}

type SaveNetworkOpexInput struct {
	NetworkID int
	Year      int
	Cells     []NetworkOpexCellWrite
	UserName  string
}

// GetNetworkOpexBudgets читает бюджет сети за год целиком: вкладка показывает
// все четыре квартала сразу, и запрос на каждый был бы четырьмя обращениями
// к базе вместо одного.
func GetNetworkOpexBudgets(networkID, year int) ([]models.NetworkOpexBudgetRow, error) {
	rows, err := config.DB.Query(
		`SELECT id, network_id, [year], [month], brand_as, article,
			amount_rub, amount_rub_net, updated_by,
			CONVERT(NVARCHAR, updated_at, 121)
		 FROM dbo.tbl_NetworkOpexBudgets
		 WHERE network_id = ? AND [year] = ?
		 ORDER BY brand_as, article, [month]`,
		networkID, year,
	)
	if err != nil {
		return nil, fmt.Errorf("query network opex budgets: %w", err)
	}
	defer rows.Close()

	result := []models.NetworkOpexBudgetRow{}
	for rows.Next() {
		var row models.NetworkOpexBudgetRow
		if err := rows.Scan(
			&row.ID, &row.NetworkID, &row.Year, &row.Month, &row.BrandAS, &row.Article,
			&row.AmountRub, &row.AmountNet, &row.UpdatedBy, &row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan network opex budget: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// NetworkPlanBrands — бренды, у которых есть строка плана в этом году.
// Строка валового пула (brand_as IS NULL) брендом не считается.
func NetworkPlanBrands(networkID, year int) ([]string, error) {
	rows, err := config.DB.Query(
		`SELECT DISTINCT LTRIM(RTRIM(brand_as)) AS brand
		 FROM dbo.tbl_NetworkPlans
		 WHERE network_id = ? AND [year] = ? AND brand_as IS NOT NULL
		   AND LTRIM(RTRIM(brand_as)) <> ''
		 ORDER BY brand`,
		networkID, year,
	)
	if err != nil {
		return nil, fmt.Errorf("query network plan brands: %w", err)
	}
	defer rows.Close()

	result := []string{}
	for rows.Next() {
		var brand string
		if err := rows.Scan(&brand); err != nil {
			return nil, fmt.Errorf("scan network plan brand: %w", err)
		}
		result = append(result, brand)
	}
	return result, rows.Err()
}

// opexCellVersion — версия квартальной ячейки: самая свежая из её месяцев.
// Пустая строка означает, что ячейки в базе нет.
func opexCellVersion(
	tx *sql.Tx,
	networkID, year, quarter int,
	brand, article string,
) (string, error) {
	monthFrom := (quarter-1)*3 + 1
	var version sql.NullString
	err := tx.QueryRow(
		`SELECT MAX(CONVERT(NVARCHAR, updated_at, 121))
		 FROM dbo.tbl_NetworkOpexBudgets
		 WHERE network_id = ? AND [year] = ? AND [month] BETWEEN ? AND ?
		   AND brand_as = ? AND article = ?`,
		networkID, year, monthFrom, monthFrom+2, brand, article,
	).Scan(&version)
	if err != nil {
		return "", fmt.Errorf("read opex cell version: %w", err)
	}
	return version.String, nil
}

// SaveNetworkOpexBudgets перезаписывает переданные квартальные ячейки одной
// транзакцией. Остальные ячейки не трогаются: вкладка правит то, что изменили,
// а не весь год целиком.
//
// Бренд обязан быть в плане года — иначе бюджет повис бы вне любого разреза
// витрины. Снятие значения проверку проходит всегда: бренд, выведенный из
// плана, должен иметь возможность унести и свой бюджет, а не остаться с суммой,
// которую уже нечем убрать.
func SaveNetworkOpexBudgets(in SaveNetworkOpexInput) error {
	planned, err := NetworkPlanBrands(in.NetworkID, in.Year)
	if err != nil {
		return err
	}
	plannedBrands := make(map[string]bool, len(planned))
	for _, brand := range planned {
		plannedBrands[brand] = true
	}

	tx, err := config.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, cell := range in.Cells {
		brand := strings.TrimSpace(cell.BrandAS)
		if brand == "" {
			return fmt.Errorf("бренд бюджета OPEX не указан")
		}
		if cell.Quarter < 1 || cell.Quarter > 4 {
			return fmt.Errorf("некорректный квартал бюджета OPEX: %d", cell.Quarter)
		}

		current, err := opexCellVersion(tx, in.NetworkID, in.Year, cell.Quarter, brand, cell.Article)
		if err != nil {
			return err
		}
		if cell.UpdatedAt != current {
			return ErrNetworkConflict
		}

		if len(cell.Months) == 0 {
			monthFrom := (cell.Quarter-1)*3 + 1
			if _, err := tx.Exec(
				`DELETE FROM dbo.tbl_NetworkOpexBudgets
				 WHERE network_id = ? AND [year] = ? AND [month] BETWEEN ? AND ?
				   AND brand_as = ? AND article = ?`,
				in.NetworkID, in.Year, monthFrom, monthFrom+2, brand, cell.Article,
			); err != nil {
				return fmt.Errorf("delete opex cell: %w", err)
			}
			continue
		}

		if !plannedBrands[brand] {
			return ErrNetworkBrandNotPlanned
		}

		for _, month := range cell.Months {
			result, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkOpexBudgets
				 SET amount_rub = ?, amount_rub_net = ?, updated_by = ?, updated_at = GETDATE()
				 WHERE network_id = ? AND [year] = ? AND [month] = ?
				   AND brand_as = ? AND article = ?`,
				month.AmountRub, month.AmountNet, in.UserName,
				in.NetworkID, in.Year, month.Month, brand, cell.Article,
			)
			if err != nil {
				return fmt.Errorf("update opex month: %w", err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("update opex month: %w", err)
			}
			if affected > 0 {
				continue
			}
			if _, err := tx.Exec(
				`INSERT INTO dbo.tbl_NetworkOpexBudgets (
					network_id, [year], [month], brand_as, article,
					amount_rub, amount_rub_net, updated_by
				 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				in.NetworkID, in.Year, month.Month, brand, cell.Article,
				month.AmountRub, month.AmountNet, in.UserName,
			); err != nil {
				return fmt.Errorf("insert opex month: %w", err)
			}
		}
	}

	return tx.Commit()
}
