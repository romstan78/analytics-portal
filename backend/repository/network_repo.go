package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"backend/config"
	"backend/models"
)

// ─── Ошибки ─────────────────────────────────────────────────────────────────

var (
	// ErrNetworkNotFound — сети с таким ID нет в реестре.
	ErrNetworkNotFound = errors.New("network not found")
	// ErrNetworkExists — сеть с таким именем уже заведена.
	ErrNetworkExists = errors.New("network already exists")
	// ErrNetworkConflict — данные изменены другим пользователем.
	ErrNetworkConflict = errors.New("network data conflict")
	// ErrNetworkPeriodGroupInvalid — некорректное или неоднозначное правило
	// совместного зачёта кварталов.
	ErrNetworkPeriodGroupInvalid = errors.New("invalid network period group")
)

// ─── Карточка сети ──────────────────────────────────────────────────────────

const networkColumns = `id, name, kam, network_type, is_active,
		vat_included, vat_rate,
		month1_pct, month2_pct, month3_pct, has_annual_investment_cumulative,
		CONVERT(NVARCHAR, created_at, 121), CONVERT(NVARCHAR, updated_at, 121)`

func scanNetwork(scanner interface{ Scan(...interface{}) error }) (models.Network, error) {
	var n models.Network
	err := scanner.Scan(
		&n.ID, &n.Name, &n.KAM, &n.NetworkType, &n.IsActive,
		&n.VATIncluded, &n.VATRate,
		&n.Month1Pct, &n.Month2Pct, &n.Month3Pct, &n.HasAnnualInvestmentCumulative,
		&n.CreatedAt, &n.UpdatedAt,
	)
	return n, err
}

// ListNetworks возвращает сети реестра с фильтром по названию и КАМу.
func ListNetworks(search, kam string, includeInactive bool) ([]models.Network, error) {
	query := "SELECT " + networkColumns + " FROM dbo.tbl_Networks WHERE 1=1"
	var args []interface{}

	if !includeInactive {
		query += " AND is_active = 1"
	}
	if search != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+search+"%")
	}
	if kam != "" {
		query += " AND kam = ?"
		args = append(args, kam)
	}
	query += " ORDER BY name ASC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.Network{}
	for rows.Next() {
		n, err := scanNetwork(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

// GetNetworkByID возвращает карточку сети.
func GetNetworkByID(id int) (models.Network, error) {
	row := config.DB.QueryRow("SELECT "+networkColumns+" FROM dbo.tbl_Networks WHERE id = ?", id)
	n, err := scanNetwork(row)
	if errors.Is(err, sql.ErrNoRows) {
		return n, ErrNetworkNotFound
	}
	return n, err
}

// InsertNetwork заводит новую сеть и возвращает её ID.
func InsertNetwork(
	name, kam, networkType string,
	vatIncluded bool, vatRate float64,
	month1Pct, month2Pct, month3Pct float64,
	hasAnnualInvestmentCumulative bool,
) (int, error) {
	var exists int
	if err := config.DB.QueryRow("SELECT COUNT(*) FROM dbo.tbl_Networks WHERE name = ?", name).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrNetworkExists
	}

	var id int
	err := config.DB.QueryRow(
		`INSERT INTO dbo.tbl_Networks (
			name, kam, network_type, vat_included, vat_rate,
			month1_pct, month2_pct, month3_pct,
			has_annual_investment_cumulative
		 ) OUTPUT INSERTED.id VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		name, nullIfEmpty(kam), networkType, vatIncluded, vatRate,
		month1Pct, month2Pct, month3Pct,
		hasAnnualInvestmentCumulative,
	).Scan(&id)
	return id, err
}

// UpdateNetwork правит карточку сети с проверкой updated_at (optimistic locking).
func UpdateNetwork(
	id int,
	name, kam, networkType string,
	isActive bool,
	vatIncluded bool, vatRate float64,
	month1Pct, month2Pct, month3Pct float64,
	hasAnnualInvestmentCumulative bool,
	year int, periods []models.NetworkPeriod,
	updatedAt string,
) error {
	tx, err := config.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := `UPDATE dbo.tbl_Networks
			SET name = ?, kam = ?, network_type = ?, is_active = ?,
				vat_included = ?, vat_rate = ?,
				month1_pct = ?, month2_pct = ?, month3_pct = ?,
				has_annual_investment_cumulative = ?, updated_at = GETDATE()
			WHERE id = ?`
	args := []interface{}{
		name, nullIfEmpty(kam), networkType, isActive,
		vatIncluded, vatRate,
		month1Pct, month2Pct, month3Pct, hasAnnualInvestmentCumulative, id,
	}
	if updatedAt != "" {
		query += " AND CONVERT(NVARCHAR, updated_at, 121) = ?"
		args = append(args, updatedAt)
	}

	res, err := tx.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Строка есть, но версия устарела — иначе сети просто нет.
		var exists int
		if err := tx.QueryRow("SELECT COUNT(*) FROM dbo.tbl_Networks WHERE id = ?", id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNetworkNotFound
		}
		return ErrNetworkConflict
	}
	for _, period := range periods {
		if err := upsertPeriodTx(tx, id, year, period); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nullIfEmpty(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

// ─── Периоды ────────────────────────────────────────────────────────────────

// GetNetworkPeriods возвращает квартальные настройки сети за год.
func GetNetworkPeriods(networkID, year int) ([]models.NetworkPeriod, error) {
	rows, err := config.DB.Query(
		`SELECT id, network_id, [year], [quarter], vat_included, vat_rate,
			CONVERT(NVARCHAR, updated_at, 121)
		 FROM dbo.tbl_NetworkPeriods WHERE network_id = ? AND [year] = ? ORDER BY [quarter]`,
		networkID, year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkPeriod{}
	for rows.Next() {
		var p models.NetworkPeriod
		if err := rows.Scan(&p.ID, &p.NetworkID, &p.Year, &p.Quarter, &p.VATIncluded,
			&p.VATRate, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func upsertPeriodTx(tx *sql.Tx, networkID, year int, p models.NetworkPeriod) error {
	res, err := tx.Exec(
		`UPDATE dbo.tbl_NetworkPeriods
		 SET vat_included = ?, vat_rate = ?, updated_at = GETDATE()
		 WHERE network_id = ? AND [year] = ? AND [quarter] = ?`,
		p.VATIncluded, p.VATRate, networkID, year, p.Quarter,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	_, err = tx.Exec(
		`INSERT INTO dbo.tbl_NetworkPeriods (network_id, [year], [quarter], vat_included, vat_rate)
		 VALUES (?, ?, ?, ?, ?)`,
		networkID, year, p.Quarter, p.VATIncluded, p.VATRate,
	)
	return err
}

// ─── Объединение кварталов ──────────────────────────────────────────────────

// NetworkPeriodGroupInput — правило из полного запроса вкладки «Планы».
// UpdatedAt защищает сохранённое правило от незаметной параллельной правки.
type NetworkPeriodGroupInput struct {
	StartQuarter int     `json:"start_quarter"`
	EndQuarter   int     `json:"end_quarter"`
	BrandAS      *string `json:"brand_as"`
	UpdatedAt    string  `json:"updated_at"`
}

func periodGroupKey(startQuarter, endQuarter int, brand *string) string {
	scope := "*"
	if brand != nil {
		scope = *brand
	}
	return fmt.Sprintf("%d|%d|%s", startQuarter, endQuarter, scope)
}

func periodGroupsOverlap(a, b NetworkPeriodGroupInput) bool {
	return a.StartQuarter <= b.EndQuarter && b.StartQuarter <= a.EndQuarter
}

// NormalizeNetworkPeriodGroups проверяет, что каждый диапазон содержит хотя
// бы два смежных квартала и что правила не дают двум областям неоднозначный
// зачёт. Портфельное правило конфликтует с любым пересекающимся правилом;
// брендовые правила могут пересекаться только у разных брендов.
func NormalizeNetworkPeriodGroups(
	groups []NetworkPeriodGroupInput,
	allowedBrands map[string]bool,
) ([]NetworkPeriodGroupInput, error) {
	normalized := make([]NetworkPeriodGroupInput, len(groups))
	copy(normalized, groups)

	seen := make(map[string]bool, len(normalized))
	for i := range normalized {
		group := &normalized[i]
		if group.StartQuarter < 1 || group.StartQuarter > 4 ||
			group.EndQuarter < 1 || group.EndQuarter > 4 ||
			group.StartQuarter >= group.EndQuarter {
			return nil, fmt.Errorf("%w: диапазон должен содержать от двух смежных кварталов в пределах года", ErrNetworkPeriodGroupInvalid)
		}
		if group.BrandAS != nil {
			brand := strings.TrimSpace(*group.BrandAS)
			if brand == "" || !allowedBrands[brand] {
				return nil, fmt.Errorf("%w: бренд %q отсутствует в плане года", ErrNetworkPeriodGroupInvalid, brand)
			}
			group.BrandAS = &brand
		}
		key := periodGroupKey(group.StartQuarter, group.EndQuarter, group.BrandAS)
		if seen[key] {
			return nil, fmt.Errorf("%w: правило Q%d–Q%d задано дважды", ErrNetworkPeriodGroupInvalid, group.StartQuarter, group.EndQuarter)
		}
		seen[key] = true
	}

	for i := range normalized {
		for j := i + 1; j < len(normalized); j++ {
			left, right := normalized[i], normalized[j]
			if !periodGroupsOverlap(left, right) {
				continue
			}
			sameBrand := left.BrandAS != nil && right.BrandAS != nil && *left.BrandAS == *right.BrandAS
			if left.BrandAS == nil || right.BrandAS == nil || sameBrand {
				return nil, fmt.Errorf(
					"%w: пересекающиеся правила Q%d–Q%d и Q%d–Q%d имеют общую область действия",
					ErrNetworkPeriodGroupInvalid,
					left.StartQuarter, left.EndQuarter, right.StartQuarter, right.EndQuarter,
				)
			}
		}
	}
	return normalized, nil
}

// GetNetworkPeriodGroups возвращает правила совместного зачёта за год.
func GetNetworkPeriodGroups(networkID, year int) ([]models.NetworkPeriodGroup, error) {
	rows, err := config.DB.Query(
		`SELECT id, network_id, [year], start_quarter, end_quarter, brand_as,
			updated_by, CONVERT(NVARCHAR, updated_at, 121)
		 FROM dbo.tbl_NetworkPeriodGroups
		 WHERE network_id = ? AND [year] = ?
		 ORDER BY start_quarter, end_quarter, CASE WHEN brand_as IS NULL THEN 0 ELSE 1 END, brand_as`,
		networkID, year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkPeriodGroup{}
	for rows.Next() {
		var group models.NetworkPeriodGroup
		if err := rows.Scan(
			&group.ID, &group.NetworkID, &group.Year, &group.StartQuarter,
			&group.EndQuarter, &group.BrandAS, &group.UpdatedBy, &group.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, group)
	}
	return result, rows.Err()
}

// ─── Планы ──────────────────────────────────────────────────────────────────

// GetNetworkPlans возвращает строки плана сети за год.
func GetNetworkPlans(networkID, year int) ([]models.NetworkPlan, error) {
	rows, err := config.DB.Query(
		`SELECT p.id, p.network_id, p.[year], p.[quarter], p.brand_as, p.in_gross, p.plan_rub, p.plan_units,
			n.month1_pct, n.month2_pct, n.month3_pct,
			p.fact_rub, p.forecast_rub, p.investments_pct, p.fact_investments_rub,
			p.forecast_investments_rub, p.updated_by,
			CONVERT(NVARCHAR, p.updated_at, 121)
		 FROM dbo.tbl_NetworkPlans p
		 JOIN dbo.tbl_Networks n ON n.id = p.network_id
		 WHERE p.network_id = ? AND p.[year] = ?
		 ORDER BY p.[quarter], p.brand_as`,
		networkID, year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkPlan{}
	for rows.Next() {
		var p models.NetworkPlan
		if err := rows.Scan(&p.ID, &p.NetworkID, &p.Year, &p.Quarter, &p.BrandAS, &p.InGross,
			&p.PlanRub, &p.PlanUnits, &p.Month1Pct, &p.Month2Pct, &p.Month3Pct,
			&p.FactRub, &p.ForecastRub, &p.InvestmentsPct, &p.FactInvestmentsRub,
			&p.ForecastInvestmentsRub, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// NetworkPlanInput — строка плана из запроса на сохранение.
// UpdatedAt — версия строки, полученная клиентом при чтении (для 409).
// FactRub в запросе нет: факт приходит загрузкой отгрузок, интерфейс его не правит.
type NetworkPlanInput struct {
	Quarter        int      `json:"quarter"`
	BrandAS        *string  `json:"brand_as"`
	InGross        bool     `json:"in_gross"`
	PlanRub        *float64 `json:"plan_rub"`
	ForecastRub    *float64 `json:"forecast_rub"`
	InvestmentsPct *float64 `json:"investments_pct"`
	UpdatedAt      string   `json:"updated_at"`
}

// planKey — ключ строки плана внутри года: квартал + бренд (пусто = валовый итог).
func planKey(quarter int, brand *string) string {
	b := ""
	if brand != nil {
		b = *brand
	}
	return fmt.Sprintf("%d|%s", quarter, b)
}

// SaveNetworkPlanInput — полный пакет сохранения вкладки «Планы».
type SaveNetworkPlanInput struct {
	NetworkID    int
	Year         int
	Periods      []models.NetworkPeriod
	Plans        []NetworkPlanInput
	PeriodGroups []NetworkPeriodGroupInput
	UserName     string
}

// planChange — одно изменение для аудит-лога.
type planChange struct {
	Quarter int         `json:"quarter"`
	Brand   string      `json:"brand,omitempty"`
	Field   string      `json:"field"`
	Old     interface{} `json:"old"`
	New     interface{} `json:"new"`
}

// planRowsToWrite раскладывает пришедшую сетку на строки для записи и строки,
// которые из плана года убраны.
//
// Запрос — это вся сетка года, поэтому бренда нет в запросе ровно тогда, когда
// его убрали из плана. Строку с фактом при этом не удаляем: факт приходит
// загрузкой отгрузок, а не из формы, — такую строку дописываем в запрос пустой,
// чтобы значения снялись, а факт остался. Пустой запрос не убирает ничего:
// год не переписывается вслепую.
func planRowsToWrite(
	incoming []NetworkPlanInput,
	existing []models.NetworkPlan,
) (write []NetworkPlanInput, remove []models.NetworkPlan) {
	write = append(make([]NetworkPlanInput, 0, len(incoming)), incoming...)
	if len(incoming) == 0 {
		return write, nil
	}

	sent := make(map[string]bool, len(incoming))
	for _, p := range incoming {
		sent[planKey(p.Quarter, p.BrandAS)] = true
	}
	for _, old := range existing {
		if old.BrandAS == nil || sent[planKey(old.Quarter, old.BrandAS)] {
			continue
		}
		if old.FactRub != nil || old.FactInvestmentsRub != nil {
			write = append(write, NetworkPlanInput{
				Quarter: old.Quarter, BrandAS: old.BrandAS, UpdatedAt: old.UpdatedAt,
			})
			continue
		}
		remove = append(remove, old)
	}
	return write, remove
}

// SaveNetworkPlan сохраняет периоды и строки плана одной транзакцией
// и возвращает JSON изменений для аудит-лога (пустая строка — изменений нет).
//
// Строка бренда есть в БД ровно тогда, когда бренд ведут в плане года: поэтому
// пустая строка бренда заводится, а сохранённая строка, которой в запросе нет,
// удаляется.
func SaveNetworkPlan(in SaveNetworkPlanInput) (string, error) {
	network, err := GetNetworkByID(in.NetworkID)
	if err != nil {
		return "", err
	}
	existing, err := GetNetworkPlans(in.NetworkID, in.Year)
	if err != nil {
		return "", err
	}
	existingByKey := make(map[string]models.NetworkPlan, len(existing))
	for _, p := range existing {
		existingByKey[planKey(p.Quarter, p.BrandAS)] = p
	}

	oldPeriods, err := GetNetworkPeriods(in.NetworkID, in.Year)
	if err != nil {
		return "", err
	}
	oldPeriodByQuarter := make(map[int]models.NetworkPeriod, len(oldPeriods))
	for _, p := range oldPeriods {
		oldPeriodByQuarter[p.Quarter] = p
	}

	// nil означает старого клиента, который про правила ещё не знает: такие
	// запросы не должны молча удалить сохранённые объединения. Пустой массив,
	// напротив, является явным удалением всех правил года.
	var normalizedGroups []NetworkPeriodGroupInput
	var existingGroups []models.NetworkPeriodGroup
	if in.PeriodGroups != nil {
		allowedBrands := make(map[string]bool)
		for _, plan := range in.Plans {
			if plan.BrandAS != nil {
				allowedBrands[strings.TrimSpace(*plan.BrandAS)] = true
			}
		}
		normalizedGroups, err = NormalizeNetworkPeriodGroups(in.PeriodGroups, allowedBrands)
		if err != nil {
			return "", err
		}
		existingGroups, err = GetNetworkPeriodGroups(in.NetworkID, in.Year)
		if err != nil {
			return "", err
		}
		existingVersions := make(map[string]bool, len(existingGroups))
		for _, group := range existingGroups {
			existingVersions[group.UpdatedAt] = true
		}
		for _, group := range normalizedGroups {
			if group.UpdatedAt != "" && !existingVersions[group.UpdatedAt] {
				return "", ErrNetworkConflict
			}
		}
	}

	writePlans, removePlans := planRowsToWrite(in.Plans, existing)

	tx, err := config.DB.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	var changes []planChange
	month1Pct, month2Pct, month3Pct := network.Month1Pct, network.Month2Pct, network.Month3Pct

	for _, p := range in.Periods {
		if p.Quarter < 1 || p.Quarter > 4 {
			return "", fmt.Errorf("некорректный квартал: %d", p.Quarter)
		}
		if old, ok := oldPeriodByQuarter[p.Quarter]; ok {
			if old.VATIncluded != p.VATIncluded {
				changes = append(changes, planChange{Quarter: p.Quarter, Field: "vat_included", Old: old.VATIncluded, New: p.VATIncluded})
			}
			if old.VATRate != p.VATRate {
				changes = append(changes, planChange{Quarter: p.Quarter, Field: "vat_rate", Old: old.VATRate, New: p.VATRate})
			}
		} else {
			changes = append(changes, planChange{Quarter: p.Quarter, Field: "period", Old: nil, New: true})
		}
		if err := upsertPeriodTx(tx, in.NetworkID, in.Year, p); err != nil {
			return "", err
		}
	}

	for _, p := range writePlans {
		if p.Quarter < 1 || p.Quarter > 4 {
			return "", fmt.Errorf("некорректный квартал: %d", p.Quarter)
		}
		if p.InvestmentsPct != nil && (*p.InvestmentsPct < 0 || *p.InvestmentsPct > 100) {
			return "", fmt.Errorf("инвестиции вне диапазона 0–100: %.2f", *p.InvestmentsPct)
		}
		if p.PlanRub != nil && *p.PlanRub < 0 {
			return "", fmt.Errorf("план не может быть отрицательным: %.2f", *p.PlanRub)
		}
		if p.ForecastRub != nil && *p.ForecastRub < 0 {
			return "", fmt.Errorf("прогноз не может быть отрицательным: %.2f", *p.ForecastRub)
		}
		// Пул сам в себя не входит: признак валового объёма — только у бренда.
		if p.BrandAS == nil {
			p.InGross = false
		}

		key := planKey(p.Quarter, p.BrandAS)
		old, exists := existingByKey[key]
		brandLabel := ""
		if p.BrandAS != nil {
			brandLabel = *p.BrandAS
		}

		if exists {
			// Версия строки: клиент прислал ту, что читал — иначе кто-то успел раньше.
			if p.UpdatedAt != "" && p.UpdatedAt != old.UpdatedAt {
				return "", ErrNetworkConflict
			}
			if !floatPtrEqual(old.PlanRub, p.PlanRub) {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "plan_rub", Old: floatPtrValue(old.PlanRub), New: floatPtrValue(p.PlanRub)})
			}
			if !floatPtrEqual(old.ForecastRub, p.ForecastRub) {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "forecast_rub", Old: floatPtrValue(old.ForecastRub), New: floatPtrValue(p.ForecastRub)})
			}
			if !floatPtrEqual(old.InvestmentsPct, p.InvestmentsPct) {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "investments_pct", Old: floatPtrValue(old.InvestmentsPct), New: floatPtrValue(p.InvestmentsPct)})
			}
			if old.InGross != p.InGross {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "in_gross", Old: old.InGross, New: p.InGross})
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkPlans
				 SET plan_rub = ?, forecast_rub = ?, investments_pct = ?, in_gross = ?,
					 month1_pct = ?, month2_pct = ?, month3_pct = ?,
					 updated_by = ?, updated_at = GETDATE()
				 WHERE id = ?`,
				p.PlanRub, p.ForecastRub, p.InvestmentsPct, p.InGross,
				month1Pct, month2Pct, month3Pct, in.UserName, old.ID,
			); err != nil {
				return "", err
			}
			continue
		}

		// Строка бренда заводится и пустой: её наличие и есть признак того, что
		// бренд ведут в плане года. Иначе бренд, добавленный до того, как в нём
		// появились суммы, пропадал бы из формы после сохранения.
		// Пул без сумм не заводим: общий объём контракта — это и есть сумма.
		if p.BrandAS == nil && p.PlanRub == nil && p.ForecastRub == nil && p.InvestmentsPct == nil {
			continue
		}
		if p.BrandAS != nil {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "brand", Old: nil, New: brandLabel})
		}
		if p.PlanRub != nil {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "plan_rub", Old: nil, New: floatPtrValue(p.PlanRub)})
		}
		if p.ForecastRub != nil {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "forecast_rub", Old: nil, New: floatPtrValue(p.ForecastRub)})
		}
		if p.InvestmentsPct != nil {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "investments_pct", Old: nil, New: floatPtrValue(p.InvestmentsPct)})
		}
		if p.InGross {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "in_gross", Old: false, New: true})
		}
		if _, err := tx.Exec(
			`INSERT INTO dbo.tbl_NetworkPlans (network_id, [year], [quarter], brand_as, in_gross,
				plan_rub, forecast_rub, investments_pct, month1_pct, month2_pct, month3_pct, updated_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.NetworkID, in.Year, p.Quarter, p.BrandAS, p.InGross,
			p.PlanRub, p.ForecastRub, p.InvestmentsPct, month1Pct, month2Pct, month3Pct, in.UserName,
		); err != nil {
			return "", err
		}
	}

	// Бренд убрали из плана года — строка уходит целиком: пока она есть,
	// форма показывает бренд снова.
	for _, old := range removePlans {
		changes = append(changes, planChange{
			Quarter: old.Quarter, Brand: *old.BrandAS, Field: "brand", Old: *old.BrandAS, New: nil,
		})
		if _, err := tx.Exec(`DELETE FROM dbo.tbl_NetworkPlans WHERE id = ?`, old.ID); err != nil {
			return "", err
		}
	}

	if in.PeriodGroups != nil {
		existingByKey := make(map[string]models.NetworkPeriodGroup, len(existingGroups))
		incomingByKey := make(map[string]NetworkPeriodGroupInput, len(normalizedGroups))
		for _, group := range existingGroups {
			existingByKey[periodGroupKey(group.StartQuarter, group.EndQuarter, group.BrandAS)] = group
		}
		for _, group := range normalizedGroups {
			incomingByKey[periodGroupKey(group.StartQuarter, group.EndQuarter, group.BrandAS)] = group
		}

		for key, old := range existingByKey {
			incoming, kept := incomingByKey[key]
			if kept {
				if incoming.UpdatedAt != "" && incoming.UpdatedAt != old.UpdatedAt {
					return "", ErrNetworkConflict
				}
				continue
			}
			brand := ""
			if old.BrandAS != nil {
				brand = *old.BrandAS
			}
			changes = append(changes, planChange{
				Quarter: old.StartQuarter, Brand: brand, Field: "period_group",
				Old: fmt.Sprintf("Q%d–Q%d", old.StartQuarter, old.EndQuarter), New: nil,
			})
			if _, err := tx.Exec(`DELETE FROM dbo.tbl_NetworkPeriodGroups WHERE id = ?`, old.ID); err != nil {
				return "", err
			}
		}

		for key, group := range incomingByKey {
			if _, exists := existingByKey[key]; exists {
				continue
			}
			brand := ""
			if group.BrandAS != nil {
				brand = *group.BrandAS
			}
			changes = append(changes, planChange{
				Quarter: group.StartQuarter, Brand: brand, Field: "period_group",
				Old: nil, New: fmt.Sprintf("Q%d–Q%d", group.StartQuarter, group.EndQuarter),
			})
			if _, err := tx.Exec(
				`INSERT INTO dbo.tbl_NetworkPeriodGroups
					(network_id, [year], start_quarter, end_quarter, brand_as, updated_by)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				in.NetworkID, in.Year, group.StartQuarter, group.EndQuarter, group.BrandAS, in.UserName,
			); err != nil {
				return "", err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	if len(changes) == 0 {
		return "", nil
	}
	payload := map[string]interface{}{"year": in.Year, "changes": changes}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", nil
	}
	return string(b), nil
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	// Суммы и проценты хранятся с двумя знаками — сравниваем в этой же точности.
	return int64(*a*100+0.5) == int64(*b*100+0.5)
}

func floatPtrValue(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// ─── Комментарии ────────────────────────────────────────────────────────────

// GetNetworkComments возвращает комментарии сети от старых к новым.
func GetNetworkComments(networkID int) ([]models.NetworkComment, error) {
	rows, err := config.DB.Query(
		`SELECT id, network_id, [year], [quarter], brand_as, user_name, role, comment_text,
			CONVERT(NVARCHAR, created_at, 121)
		 FROM dbo.tbl_NetworkComments WHERE network_id = ? ORDER BY id ASC`,
		networkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkComment{}
	for rows.Next() {
		var c models.NetworkComment
		if err := rows.Scan(&c.ID, &c.NetworkID, &c.Year, &c.Quarter, &c.BrandAS,
			&c.UserName, &c.Role, &c.CommentText, &c.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// InsertNetworkComment добавляет комментарий к сети или к ячейке плана.
func InsertNetworkComment(c models.NetworkComment) error {
	_, err := config.DB.Exec(
		`INSERT INTO dbo.tbl_NetworkComments (network_id, [year], [quarter], brand_as, user_name, role, comment_text)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.NetworkID, c.Year, c.Quarter, c.BrandAS, c.UserName, c.Role, c.CommentText,
	)
	return err
}

// ─── Аудит и справочники ────────────────────────────────────────────────────

// InsertEntityAuditLog пишет событие в tbl_AuditLog с явным типом сущности.
func InsertEntityAuditLog(entityType string, entityID int, userName, actionType, changedFields string) error {
	_, err := config.DB.Exec(
		"INSERT INTO dbo.tbl_AuditLog (entity_type, entity_id, user_name, action_type, changed_fields) VALUES (?, ?, ?, ?, ?)",
		entityType, entityID, userName, actionType, changedFields,
	)
	return err
}

// GetNetworkAuditLog возвращает историю карточки сети и её планов одной лентой.
func GetNetworkAuditLog(networkID int) ([]models.AuditLogRow, error) {
	rows, err := config.DB.Query(
		`SELECT id, entity_type, entity_id, user_name, action_type, changed_fields,
			CONVERT(NVARCHAR, created_at, 121)
		 FROM dbo.tbl_AuditLog
		 WHERE entity_id = ? AND entity_type IN ('network', 'network_plan', 'network_forecast', 'network_price')
		 ORDER BY id DESC`,
		networkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.AuditLogRow{}
	for rows.Next() {
		var r models.AuditLogRow
		if err := rows.Scan(&r.ID, &r.EntityType, &r.EntityID, &r.UserName,
			&r.ActionType, &r.ChangedFields, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetBrandOptions — список брендов для планирования (планы ведутся по брендам, не по SKU).
func GetBrandOptions() ([]string, error) {
	rows, err := config.DB.Query(
		`SELECT DISTINCT brand_as FROM dbo.tbl_SKUMapping
		 WHERE brand_as IS NOT NULL AND LTRIM(RTRIM(brand_as)) <> '' ORDER BY brand_as`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []string{}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
