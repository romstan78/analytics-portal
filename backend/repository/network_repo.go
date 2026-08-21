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
)

// ─── Карточка сети ──────────────────────────────────────────────────────────

const networkColumns = `id, name, kam, network_type, is_active,
	CONVERT(NVARCHAR, created_at, 121), CONVERT(NVARCHAR, updated_at, 121)`

func scanNetwork(scanner interface{ Scan(...interface{}) error }) (models.Network, error) {
	var n models.Network
	err := scanner.Scan(&n.ID, &n.Name, &n.KAM, &n.NetworkType, &n.IsActive, &n.CreatedAt, &n.UpdatedAt)
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
func InsertNetwork(name, kam, networkType string) (int, error) {
	var exists int
	if err := config.DB.QueryRow("SELECT COUNT(*) FROM dbo.tbl_Networks WHERE name = ?", name).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrNetworkExists
	}

	var id int
	err := config.DB.QueryRow(
		`INSERT INTO dbo.tbl_Networks (name, kam, network_type)
		 OUTPUT INSERTED.id VALUES (?, ?, ?)`,
		name, nullIfEmpty(kam), networkType,
	).Scan(&id)
	return id, err
}

// UpdateNetwork правит карточку сети с проверкой updated_at (optimistic locking).
func UpdateNetwork(id int, name, kam, networkType string, isActive bool, updatedAt string) error {
	query := `UPDATE dbo.tbl_Networks
		SET name = ?, kam = ?, network_type = ?, is_active = ?, updated_at = GETDATE()
		WHERE id = ?`
	args := []interface{}{name, nullIfEmpty(kam), networkType, isActive, id}
	if updatedAt != "" {
		query += " AND CONVERT(NVARCHAR, updated_at, 121) = ?"
		args = append(args, updatedAt)
	}

	res, err := config.DB.Exec(query, args...)
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
		if err := config.DB.QueryRow("SELECT COUNT(*) FROM dbo.tbl_Networks WHERE id = ?", id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNetworkNotFound
		}
		return ErrNetworkConflict
	}
	return nil
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
		`SELECT id, network_id, [year], [quarter], vat_included, vat_rate, contract_type,
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
			&p.VATRate, &p.ContractType, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func upsertPeriodTx(tx *sql.Tx, networkID, year int, p models.NetworkPeriod) error {
	res, err := tx.Exec(
		`UPDATE dbo.tbl_NetworkPeriods
		 SET vat_included = ?, vat_rate = ?, contract_type = ?, updated_at = GETDATE()
		 WHERE network_id = ? AND [year] = ? AND [quarter] = ?`,
		p.VATIncluded, p.VATRate, p.ContractType, networkID, year, p.Quarter,
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
		`INSERT INTO dbo.tbl_NetworkPeriods (network_id, [year], [quarter], vat_included, vat_rate, contract_type)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		networkID, year, p.Quarter, p.VATIncluded, p.VATRate, p.ContractType,
	)
	return err
}

// ─── Планы ──────────────────────────────────────────────────────────────────

// GetNetworkPlans возвращает строки плана сети за год.
func GetNetworkPlans(networkID, year int) ([]models.NetworkPlan, error) {
	rows, err := config.DB.Query(
		`SELECT id, network_id, [year], [quarter], brand_as, plan_rub, plan_units,
			investments_pct, updated_by, CONVERT(NVARCHAR, updated_at, 121)
		 FROM dbo.tbl_NetworkPlans WHERE network_id = ? AND [year] = ?
		 ORDER BY [quarter], brand_as`,
		networkID, year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.NetworkPlan{}
	for rows.Next() {
		var p models.NetworkPlan
		if err := rows.Scan(&p.ID, &p.NetworkID, &p.Year, &p.Quarter, &p.BrandAS,
			&p.PlanRub, &p.PlanUnits, &p.InvestmentsPct, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// NetworkPlanInput — строка плана из запроса на сохранение.
// UpdatedAt — версия строки, полученная клиентом при чтении (для 409).
type NetworkPlanInput struct {
	Quarter        int      `json:"quarter"`
	BrandAS        *string  `json:"brand_as"`
	PlanRub        *float64 `json:"plan_rub"`
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
	NetworkID int
	Year      int
	Periods   []models.NetworkPeriod
	Plans     []NetworkPlanInput
	UserName  string
}

// planChange — одно изменение для аудит-лога.
type planChange struct {
	Quarter int         `json:"quarter"`
	Brand   string      `json:"brand,omitempty"`
	Field   string      `json:"field"`
	Old     interface{} `json:"old"`
	New     interface{} `json:"new"`
}

// SaveNetworkPlan сохраняет периоды и строки плана одной транзакцией
// и возвращает JSON изменений для аудит-лога (пустая строка — изменений нет).
func SaveNetworkPlan(in SaveNetworkPlanInput) (string, error) {
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

	tx, err := config.DB.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	var changes []planChange

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
			if old.ContractType != p.ContractType {
				changes = append(changes, planChange{Quarter: p.Quarter, Field: "contract_type", Old: old.ContractType, New: p.ContractType})
			}
		} else {
			changes = append(changes, planChange{Quarter: p.Quarter, Field: "period", Old: nil, New: p.ContractType})
		}
		if err := upsertPeriodTx(tx, in.NetworkID, in.Year, p); err != nil {
			return "", err
		}
	}

	for _, p := range in.Plans {
		if p.Quarter < 1 || p.Quarter > 4 {
			return "", fmt.Errorf("некорректный квартал: %d", p.Quarter)
		}
		if p.InvestmentsPct != nil && (*p.InvestmentsPct < 0 || *p.InvestmentsPct > 100) {
			return "", fmt.Errorf("инвестиции вне диапазона 0–100: %.2f", *p.InvestmentsPct)
		}
		if p.PlanRub != nil && *p.PlanRub < 0 {
			return "", fmt.Errorf("план не может быть отрицательным: %.2f", *p.PlanRub)
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
			if !floatPtrEqual(old.InvestmentsPct, p.InvestmentsPct) {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "investments_pct", Old: floatPtrValue(old.InvestmentsPct), New: floatPtrValue(p.InvestmentsPct)})
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkPlans
				 SET plan_rub = ?, investments_pct = ?, updated_by = ?, updated_at = GETDATE()
				 WHERE id = ?`,
				p.PlanRub, p.InvestmentsPct, in.UserName, old.ID,
			); err != nil {
				return "", err
			}
			continue
		}

		// Пустую строку не заводим — так удаление значения не плодит мусор.
		if p.PlanRub == nil && p.InvestmentsPct == nil {
			continue
		}
		changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "plan_rub", Old: nil, New: floatPtrValue(p.PlanRub)})
		if _, err := tx.Exec(
			`INSERT INTO dbo.tbl_NetworkPlans (network_id, [year], [quarter], brand_as, plan_rub, investments_pct, updated_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			in.NetworkID, in.Year, p.Quarter, p.BrandAS, p.PlanRub, p.InvestmentsPct, in.UserName,
		); err != nil {
			return "", err
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
		 WHERE entity_id = ? AND entity_type IN ('network', 'network_plan')
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

// GetBrandOptions — список брендов для планирования (планы ведутся по брендам, не по СКЮ).
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
