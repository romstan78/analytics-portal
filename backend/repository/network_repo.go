package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"backend/config"
	"backend/models"

	mssql "github.com/microsoft/go-mssqldb"
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
	// ErrNetworkBrandNotPlanned — бренда нет в плане квартала, менять его
	// режим ведения не на чем.
	ErrNetworkBrandNotPlanned = errors.New("network brand is not planned")
)

// ─── Карточка сети ──────────────────────────────────────────────────────────

const networkColumns = `id, name, kam, network_type, is_active,
		vat_included, vat_rate,
		month1_pct, month2_pct, month3_pct, has_annual_investment_cumulative,
		default_entry_level, default_entry_unit,
		default_scales_count, default_cap_mode,
		CONVERT(NVARCHAR, created_at, 121), CONVERT(NVARCHAR, updated_at, 121)`

func scanNetwork(scanner interface{ Scan(...interface{}) error }) (models.Network, error) {
	var n models.Network
	err := scanner.Scan(
		&n.ID, &n.Name, &n.KAM, &n.NetworkType, &n.IsActive,
		&n.VATIncluded, &n.VATRate,
		&n.Month1Pct, &n.Month2Pct, &n.Month3Pct, &n.HasAnnualInvestmentCumulative,
		&n.DefaultEntryLevel, &n.DefaultEntryUnit,
		&n.DefaultScalesCount, &n.DefaultCapMode,
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
	defaultEntryLevel, defaultEntryUnit string,
	defaultScalesCount int, defaultCapMode string,
) (int, error) {
	// Уникальность имени проверяет индекс UQ_Networks_name, а не отдельный
	// SELECT перед вставкой: между проверкой и вставкой помещается чужой INSERT,
	// и такая гонка отдавала клиенту 500 вместо понятного «сеть уже есть».
	var id int
	err := config.DB.QueryRow(
		`INSERT INTO dbo.tbl_Networks (
			name, kam, network_type, vat_included, vat_rate,
			month1_pct, month2_pct, month3_pct,
			has_annual_investment_cumulative, default_entry_level, default_entry_unit,
			default_scales_count, default_cap_mode
		 ) OUTPUT INSERTED.id VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		name, nullIfEmpty(kam), networkType, vatIncluded, vatRate,
		month1Pct, month2Pct, month3Pct,
		hasAnnualInvestmentCumulative, defaultEntryLevel, defaultEntryUnit,
		defaultScalesCount, defaultCapMode,
	).Scan(&id)
	if isUniqueViolation(err) {
		return 0, ErrNetworkExists
	}
	return id, err
}

// isUniqueViolation распознаёт нарушение уникального индекса MSSQL:
// 2627 — нарушение ограничения PRIMARY KEY/UNIQUE, 2601 — уникального индекса.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var mssqlErr mssql.Error
	if errors.As(err, &mssqlErr) {
		return mssqlErr.Number == 2627 || mssqlErr.Number == 2601
	}
	return false
}

// UpdateNetwork правит карточку сети с проверкой updated_at (optimistic locking).
func UpdateNetwork(
	id int,
	name, kam, networkType string,
	isActive bool,
	vatIncluded bool, vatRate float64,
	month1Pct, month2Pct, month3Pct float64,
	hasAnnualInvestmentCumulative bool,
	defaultEntryLevel, defaultEntryUnit string,
	defaultScalesCount int, defaultCapMode string,
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
				has_annual_investment_cumulative = ?,
				default_entry_level = ?, default_entry_unit = ?,
				default_scales_count = ?, default_cap_mode = ?, updated_at = GETDATE()
			WHERE id = ?`
	args := []interface{}{
		name, nullIfEmpty(kam), networkType, isActive,
		vatIncluded, vatRate,
		month1Pct, month2Pct, month3Pct, hasAnnualInvestmentCumulative,
		defaultEntryLevel, defaultEntryUnit,
		defaultScalesCount, defaultCapMode, id,
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
		if err := upsertPeriodTx(tx, id, year, period, defaultScalesCount); err != nil {
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
			scales_count, CONVERT(NVARCHAR, updated_at, 121)
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
		// NULL — «как в профиле сети»; ноль здесь разрешает services.
		var scales sql.NullInt64
		if err := rows.Scan(&p.ID, &p.NetworkID, &p.Year, &p.Quarter, &p.VATIncluded,
			&p.VATRate, &scales, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if scales.Valid {
			p.ScalesCount = int(scales.Int64)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// upsertPeriodTx пишет квартал. Число ступеней хранится только как исключение:
// значение, совпадающее с умолчанием сети (или нулевое), уходит в базу NULL,
// чтобы смена умолчания в профиле распространялась на обычные кварталы.
func upsertPeriodTx(tx *sql.Tx, networkID, year int, p models.NetworkPeriod, defaultScales int) error {
	var scales interface{}
	if p.ScalesCount > 0 && p.ScalesCount != defaultScales {
		scales = p.ScalesCount
	}
	res, err := tx.Exec(
		`UPDATE dbo.tbl_NetworkPeriods
		 SET vat_included = ?, vat_rate = ?, scales_count = ?, updated_at = GETDATE()
		 WHERE network_id = ? AND [year] = ? AND [quarter] = ?`,
		p.VATIncluded, p.VATRate, scales, networkID, year, p.Quarter,
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
		`INSERT INTO dbo.tbl_NetworkPeriods (network_id, [year], [quarter], vat_included, vat_rate, scales_count)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		networkID, year, p.Quarter, p.VATIncluded, p.VATRate, scales,
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

// GetNetworkPlans возвращает строки плана сети за год вместе со ступенями и
// их SKU. Расчётные колонки ступеней не читаются: они пересчитываются на
// каждом чтении, как и инвестиции самой строки.
func GetNetworkPlans(networkID, year int) ([]models.NetworkPlan, error) {
	rows, err := config.DB.Query(
		`SELECT p.id, p.network_id, p.[year], p.[quarter], p.brand_as, p.in_gross, p.plan_rub, p.plan_units,
			n.month1_pct, n.month2_pct, n.month3_pct,
			p.fact_rub, p.forecast_rub, p.investments_pct, p.paid_investments_rub,
			p.pay_investments_from_fact, p.cap_mode, p.cap_pct,
			p.entry_level, p.entry_unit, p.updated_by,
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
			&p.FactRub, &p.ForecastRub, &p.InvestmentsPct, &p.PaidInvestmentsRub,
			&p.PayInvestmentsFromFact, &p.CapMode, &p.CapPct, &p.EntryLevel, &p.EntryUnit,
			&p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachPlanScales(networkID, year, result); err != nil {
		return nil, err
	}
	return result, nil
}

// attachPlanScales дочитывает ступени и SKU строк плана двумя запросами и
// раскладывает их по строкам. Ступень 1 в таблице есть у каждой строки с
// миграции 032; строка без неё (заведена до пересчёта) получит её в services.
func attachPlanScales(networkID, year int, plans []models.NetworkPlan) error {
	if len(plans) == 0 {
		return nil
	}
	byPlanID := make(map[int]int, len(plans))
	for i := range plans {
		byPlanID[plans[i].ID] = i
	}

	scaleRows, err := config.DB.Query(
		`SELECT s.id, s.plan_id, s.scale_no, s.plan_rub, s.plan_units, s.investments_pct,
			s.updated_by, CONVERT(NVARCHAR, s.updated_at, 121)
		 FROM dbo.tbl_NetworkPlanScales s
		 JOIN dbo.tbl_NetworkPlans p ON p.id = s.plan_id
		 WHERE p.network_id = ? AND p.[year] = ?
		 ORDER BY s.plan_id, s.scale_no`,
		networkID, year,
	)
	if err != nil {
		return err
	}
	defer scaleRows.Close()
	scaleIndex := map[int64][2]int{} // id ступени → (строка плана, позиция ступени)
	for scaleRows.Next() {
		var s models.NetworkPlanScale
		var planID int
		if err := scaleRows.Scan(&s.ID, &planID, &s.ScaleNo, &s.PlanRub, &s.PlanUnits,
			&s.InvestmentsPct, &s.UpdatedBy, &s.UpdatedAt); err != nil {
			return err
		}
		i, ok := byPlanID[planID]
		if !ok {
			continue
		}
		s.SKUs = []models.NetworkPlanScaleSKU{}
		plans[i].Scales = append(plans[i].Scales, s)
		scaleIndex[s.ID] = [2]int{i, len(plans[i].Scales) - 1}
	}
	if err := scaleRows.Err(); err != nil {
		return err
	}

	skuRows, err := config.DB.Query(
		`SELECT k.id, k.scale_id, k.sku, k.plan_rub, k.plan_units, k.investments_pct,
			k.cap_mode, k.cap_pct, CONVERT(NVARCHAR, k.updated_at, 121)
		 FROM dbo.tbl_NetworkPlanScaleSKU k
		 JOIN dbo.tbl_NetworkPlanScales s ON s.id = k.scale_id
		 JOIN dbo.tbl_NetworkPlans p ON p.id = s.plan_id
		 WHERE p.network_id = ? AND p.[year] = ?
		 ORDER BY k.scale_id, k.sku`,
		networkID, year,
	)
	if err != nil {
		return err
	}
	defer skuRows.Close()
	for skuRows.Next() {
		var k models.NetworkPlanScaleSKU
		var scaleID int64
		var capMode sql.NullString
		if err := skuRows.Scan(&k.ID, &scaleID, &k.SKU, &k.PlanRub, &k.PlanUnits, &k.InvestmentsPct,
			&capMode, &k.CapPct, &k.UpdatedAt); err != nil {
			return err
		}
		k.CapMode = capMode.String
		at, ok := scaleIndex[scaleID]
		if !ok {
			continue
		}
		scale := &plans[at[0]].Scales[at[1]]
		scale.SKUs = append(scale.SKUs, k)
	}
	return skuRows.Err()
}

// NetworkPlanInput — строка плана из запроса на сохранение.
// UpdatedAt — версия строки, полученная клиентом при чтении (для 409).
// Ни факта, ни прогноза в запросе нет: факт приходит загрузкой отгрузок, прогноз
// ведётся помесячно во вкладке «Прогноз». Квартальные fact_rub и forecast_rub —
// денормализованное зеркало помесячного слоя, и форма планов их не правит.
type NetworkPlanInput struct {
	Quarter        int      `json:"quarter"`
	BrandAS        *string  `json:"brand_as"`
	InGross        bool     `json:"in_gross"`
	PlanRub        *float64 `json:"plan_rub"`
	PlanUnits      *float64 `json:"plan_units,omitempty"`
	InvestmentsPct *float64 `json:"investments_pct"`
	// Режим ведения бренда. Пустые значения означают клиента, который про
	// режим ещё не знает: сохранённый режим строки в этом случае не меняется.
	EntryLevel string `json:"entry_level"`
	EntryUnit  string `json:"entry_unit"`
	// Крышка перевыполнения владельца порога. Пустой режим — клиент про крышку
	// не знает: сохранённая остаётся, новая строка берёт умолчание сети.
	CapMode string   `json:"cap_mode,omitempty"`
	CapPct  *float64 `json:"cap_pct,omitempty"`
	// Ступени строки. Поле отсутствует — клиент про ступени не знает,
	// сохранённые верхние ступени и SKU остаются; пустой массив — у строки нет
	// ничего, кроме ступени 1. Ступень 1 в массиве несёт только свои SKU: её
	// порог и процент — это plan_rub и investments_pct самой строки.
	Scales    []NetworkPlanScaleInput `json:"scales,omitempty"`
	UpdatedAt string                  `json:"updated_at"`
}

// NetworkPlanScaleInput — ступень из запроса на сохранение.
type NetworkPlanScaleInput struct {
	ScaleNo        int                        `json:"scale_no"`
	PlanRub        *float64                   `json:"plan_rub"`
	PlanUnits      *float64                   `json:"plan_units"`
	InvestmentsPct *float64                   `json:"investments_pct"`
	SKUs           []NetworkPlanScaleSKUInput `json:"skus"`
	UpdatedAt      string                     `json:"updated_at"`
}

// NetworkPlanScaleSKUInput — SKU на ступени из запроса. Пустые процент и
// крышка означают «как у бренда».
type NetworkPlanScaleSKUInput struct {
	SKU            string   `json:"sku"`
	PlanRub        *float64 `json:"plan_rub"`
	PlanUnits      *float64 `json:"plan_units"`
	InvestmentsPct *float64 `json:"investments_pct"`
	CapMode        string   `json:"cap_mode"`
	CapPct         *float64 `json:"cap_pct"`
}

// entryModeValue выбирает режим ведения строки плана. Пустое значение в запросе —
// это клиент, который про режим ещё не знает: сохранённая строка остаётся в своём
// режиме, а новая заводится в режиме по умолчанию из профиля сети. Так обновление
// клиента не переключает бренды на чужой способ ведения.
func entryModeValue(incoming, saved, fallback string, allowed ...string) (string, error) {
	value := strings.TrimSpace(incoming)
	for _, candidate := range []string{saved, fallback, allowed[0]} {
		if value != "" {
			break
		}
		value = candidate
	}
	for _, option := range allowed {
		if value == option {
			return value, nil
		}
	}
	return "", fmt.Errorf("недопустимый режим ведения: %q", value)
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
			// Пустые ступени, а не nil: бренд из плана убрали, его лестница
			// уходит вместе с планом, факт остаётся.
			write = append(write, NetworkPlanInput{
				Quarter: old.Quarter, BrandAS: old.BrandAS, UpdatedAt: old.UpdatedAt,
				Scales: []NetworkPlanScaleInput{},
			})
			continue
		}
		remove = append(remove, old)
	}
	return write, remove
}

// UpdateNetworkPlanEntryMode переключает режим ведения бренда в квартале.
//
// Режим живёт на строке плана, но переключать его нужно и из формы прогноза:
// именно там видно, что бренд удобнее вести иначе. Значения при этом не
// трогаются — меняется только то, какой уровень считается введённым.
func UpdateNetworkPlanEntryMode(networkID, year, quarter int, brand, level, unit, userName string) error {
	if _, ok := oneOfString(level, "brand", "sku"); !ok {
		return fmt.Errorf("уровень ведения: brand или sku")
	}
	if _, ok := oneOfString(unit, "rub", "units"); !ok {
		return fmt.Errorf("единица ведения: rub или units")
	}
	brand = strings.TrimSpace(brand)
	if brand == "" {
		return errors.New("бренд не указан")
	}

	result, err := config.DB.Exec(
		`UPDATE dbo.tbl_NetworkPlans
		    SET entry_level = ?, entry_unit = ?, updated_by = ?, updated_at = GETDATE()
		  WHERE network_id = ? AND [year] = ? AND [quarter] = ? AND brand_as = ?`,
		level, unit, userName, networkID, year, quarter, brand,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNetworkBrandNotPlanned
	}
	return nil
}

// NetworkInvestmentPaymentModeInput — одна ячейка матрицы «бренд × квартал»
// из профиля сети. UpdatedAt защищает настройку от параллельной правки плана.
type NetworkInvestmentPaymentModeInput struct {
	Quarter                int    `json:"quarter"`
	BrandAS                string `json:"brand_as"`
	PayInvestmentsFromFact bool   `json:"pay_investments_from_fact"`
	UpdatedAt              string `json:"updated_at"`
}

// UpdateNetworkInvestmentPaymentModes меняет только режим оплаты, не
// переписывая планы, проценты и прогнозы из другой вкладки.
func UpdateNetworkInvestmentPaymentModes(
	networkID, year int,
	inputs []NetworkInvestmentPaymentModeInput,
	userName string,
) (string, error) {
	existing, err := GetNetworkPlans(networkID, year)
	if err != nil {
		return "", err
	}
	byKey := make(map[string]models.NetworkPlan, len(existing))
	for _, plan := range existing {
		if plan.BrandAS != nil {
			byKey[planKey(plan.Quarter, plan.BrandAS)] = plan
		}
	}

	seen := make(map[string]bool, len(inputs))
	tx, err := config.DB.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	changes := make([]planChange, 0)
	for _, input := range inputs {
		brand := strings.TrimSpace(input.BrandAS)
		if input.Quarter < 1 || input.Quarter > 4 || brand == "" {
			return "", errors.New("некорректная настройка оплаты инвестиций")
		}
		brandCopy := brand
		key := planKey(input.Quarter, &brandCopy)
		if seen[key] {
			return "", errors.New("настройка оплаты инвестиций задана дважды")
		}
		seen[key] = true
		old, ok := byKey[key]
		if !ok {
			return "", ErrNetworkBrandNotPlanned
		}
		if input.UpdatedAt != "" && input.UpdatedAt != old.UpdatedAt {
			return "", ErrNetworkConflict
		}
		if old.PayInvestmentsFromFact == input.PayInvestmentsFromFact {
			continue
		}
		changes = append(changes, planChange{
			Quarter: input.Quarter, Brand: brand, Field: "pay_investments_from_fact",
			Old: old.PayInvestmentsFromFact, New: input.PayInvestmentsFromFact,
		})
		if _, err := tx.Exec(
			`UPDATE dbo.tbl_NetworkPlans
			    SET pay_investments_from_fact = ?, updated_by = ?, updated_at = GETDATE()
			  WHERE id = ?`,
			input.PayInvestmentsFromFact, userName, old.ID,
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
	payload, err := json.Marshal(map[string]interface{}{"year": year, "changes": changes})
	if err != nil {
		return "", nil
	}
	return string(payload), nil
}

// oneOfString возвращает значение, если оно есть в списке допустимых.
func oneOfString(value string, allowed ...string) (string, bool) {
	for _, option := range allowed {
		if value == option {
			return value, true
		}
	}
	return "", false
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
		if err := upsertPeriodTx(tx, in.NetworkID, in.Year, p, network.DefaultScalesCount); err != nil {
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
		if p.PlanUnits != nil && *p.PlanUnits < 0 {
			return "", fmt.Errorf("план в упаковках не может быть отрицательным: %.2f", *p.PlanUnits)
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

		// Крышка: пустой режим — клиент про неё не знает; сохранённая
		// остаётся, новая строка берёт умолчание сети. У валового бренда
		// крышки нет — её несёт пул, — но хранить «open» безвредно.
		capMode, capPct := p.CapMode, p.CapPct
		if capMode == "" {
			if exists {
				capMode, capPct = old.CapMode, old.CapPct
			} else {
				capMode = network.DefaultCapMode
			}
		}
		if capMode == "" {
			capMode = models.CapModeOpen
		}
		if _, ok := oneOfString(capMode, models.CapModeOpen, models.CapModePct, models.CapModeClosed); !ok {
			return "", fmt.Errorf("недопустимый режим крышки: %q", capMode)
		}
		if capMode != models.CapModePct {
			capPct = nil
		} else if capPct == nil || *capPct < 0 {
			return "", errors.New("процентная крышка требует неотрицательный процент")
		}
		if err := validatePlanScalesInput(p); err != nil {
			return "", err
		}

		// Режим ведения — свойство бренда; у строки пула его нет.
		savedLevel, savedUnit := "", ""
		if exists {
			savedLevel, savedUnit = old.EntryLevel, old.EntryUnit
		}
		entryLevel, err := entryModeValue(p.EntryLevel, savedLevel, network.DefaultEntryLevel, "brand", "sku")
		if err != nil {
			return "", err
		}
		entryUnit, err := entryModeValue(p.EntryUnit, savedUnit, network.DefaultEntryUnit, "rub", "units")
		if err != nil {
			return "", err
		}
		if p.BrandAS == nil {
			entryLevel, entryUnit = "brand", "rub"
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
			if old.InGross != p.InGross {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "in_gross", Old: old.InGross, New: p.InGross})
			}
			if old.EntryLevel != entryLevel {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "entry_level", Old: old.EntryLevel, New: entryLevel})
			}
			if old.EntryUnit != entryUnit {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "entry_unit", Old: old.EntryUnit, New: entryUnit})
			}
			if !floatPtrEqual(old.PlanUnits, p.PlanUnits) && p.PlanUnits != nil {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "plan_units", Old: floatPtrValue(old.PlanUnits), New: floatPtrValue(p.PlanUnits)})
			}
			if old.CapMode != capMode || !floatPtrEqual(old.CapPct, capPct) {
				changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "cap", Old: capLabel(old.CapMode, old.CapPct), New: capLabel(capMode, capPct)})
			}
			// forecast_rub не в списке намеренно: прогноз ведётся помесячно, а
			// в этой колонке живёт его свод. Сохранение плана его не трогает.
			// plan_units пишется, только если клиент его прислал: старый клиент
			// не должен стирать пару, которую закрепил пересчёт по ценам.
			planUnits := p.PlanUnits
			if planUnits == nil {
				planUnits = old.PlanUnits
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkPlans
				 SET plan_rub = ?, plan_units = ?, investments_pct = ?, in_gross = ?,
					 month1_pct = ?, month2_pct = ?, month3_pct = ?,
					 entry_level = ?, entry_unit = ?, cap_mode = ?, cap_pct = ?,
					 updated_by = ?, updated_at = GETDATE()
				 WHERE id = ?`,
				p.PlanRub, planUnits, p.InvestmentsPct, p.InGross,
				month1Pct, month2Pct, month3Pct, entryLevel, entryUnit, capMode, capPct,
				in.UserName, old.ID,
			); err != nil {
				return "", err
			}
			scaleChanges, err := syncPlanScalesTx(tx, old.ID, p, planUnits, old.Scales, in.UserName)
			if err != nil {
				return "", err
			}
			changes = append(changes, scaleChanges...)
			continue
		}

		// Строка бренда заводится и пустой: её наличие и есть признак того, что
		// бренд ведут в плане года. Иначе бренд, добавленный до того, как в нём
		// появились суммы, пропадал бы из формы после сохранения.
		// Пул без сумм не заводим: общий объём контракта — это и есть сумма.
		if p.BrandAS == nil && p.PlanRub == nil && p.InvestmentsPct == nil {
			continue
		}
		if p.BrandAS != nil {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "brand", Old: nil, New: brandLabel})
		}
		if p.PlanRub != nil {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "plan_rub", Old: nil, New: floatPtrValue(p.PlanRub)})
		}
		if p.InvestmentsPct != nil {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "investments_pct", Old: nil, New: floatPtrValue(p.InvestmentsPct)})
		}
		if p.InGross {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "in_gross", Old: false, New: true})
		}
		if capMode != models.CapModeOpen {
			changes = append(changes, planChange{Quarter: p.Quarter, Brand: brandLabel, Field: "cap", Old: nil, New: capLabel(capMode, capPct)})
		}
		var planID int
		if err := tx.QueryRow(
			`INSERT INTO dbo.tbl_NetworkPlans (network_id, [year], [quarter], brand_as, in_gross,
				plan_rub, plan_units, investments_pct, month1_pct, month2_pct, month3_pct,
				entry_level, entry_unit, cap_mode, cap_pct, updated_by)
			 OUTPUT INSERTED.id
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.NetworkID, in.Year, p.Quarter, p.BrandAS, p.InGross,
			p.PlanRub, p.PlanUnits, p.InvestmentsPct, month1Pct, month2Pct, month3Pct,
			entryLevel, entryUnit, capMode, capPct, in.UserName,
		).Scan(&planID); err != nil {
			return "", err
		}
		scaleChanges, err := syncPlanScalesTx(tx, planID, p, p.PlanUnits, nil, in.UserName)
		if err != nil {
			return "", err
		}
		changes = append(changes, scaleChanges...)
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
		 WHERE entity_id = ? AND entity_type IN ('network', 'network_plan', 'network_forecast', 'network_price', 'network_opex')
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
// GetKAMOptions возвращает КАМов для фильтра реестра.
//
// Основной источник — справочник tbl_KAMNetworkMapping. К нему добавляются
// КАМы, проставленные прямо в карточках сетей: фильтр применяется к
// tbl_Networks.kam, и без объединения сеть с КАМом вне справочника нельзя было
// бы отобрать ни одним значением списка.
func GetKAMOptions() ([]string, error) {
	rows, err := config.DB.Query(
		`SELECT kam FROM (
		     SELECT DISTINCT LTRIM(RTRIM(kam)) AS kam FROM dbo.tbl_KAMNetworkMapping
		     WHERE kam IS NOT NULL AND LTRIM(RTRIM(kam)) <> ''
		     UNION
		     SELECT DISTINCT LTRIM(RTRIM(kam)) AS kam FROM dbo.tbl_Networks
		     WHERE kam IS NOT NULL AND LTRIM(RTRIM(kam)) <> ''
		 ) options ORDER BY kam`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []string{}
	for rows.Next() {
		var kam string
		if err := rows.Scan(&kam); err != nil {
			return nil, err
		}
		result = append(result, kam)
	}
	return result, rows.Err()
}

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

// ─── Расчётные колонки инвестиций ───────────────────────────────────────────

// SaveNetworkInvestmentColumns записывает шесть расчётных сумм и платёжный
// факт в квартальные строки года.
//
// Колонки — денормализованное зеркало для внешних потребителей: ежедневной
// выгрузки, BI и интеграций. Экраны портала продолжают считать на лету, поэтому
// пропущенная запись не может показать пользователю устаревшие деньги — она
// может только задержать их снаружи до следующего пересчёта.
func SaveNetworkInvestmentColumns(plans []models.NetworkPlan) error {
	if len(plans) == 0 {
		return nil
	}
	tx, err := config.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, plan := range plans {
		if plan.ID == 0 {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE dbo.tbl_NetworkPlans
			    SET plan_investments_rub = ?, plan_investments_rub_net = ?,
			        forecast_investments_rub = ?, forecast_investments_rub_net = ?,
			        fact_investments_rub = ?, fact_investments_rub_net = ?,
			        forecast_scale = ?, fact_scale = ?,
			        forecast_base_rub = ?, fact_base_rub = ?
			  WHERE id = ?`,
			plan.InvestmentsRub, plan.InvestmentsNet,
			plan.ForecastInvestmentsRub, plan.ForecastInvestmentsNet,
			plan.FactInvestmentsRub, plan.FactInvestmentsNet,
			plan.ForecastScale, plan.FactScale,
			plan.ForecastBaseRub, plan.FactBaseRub,
			plan.ID,
		); err != nil {
			return fmt.Errorf("save investment columns for plan %d: %w", plan.ID, err)
		}
		// Ступени и SKU — тем же зеркалом: «сколько было бы» на каждой ступени
		// и что достигнуто, для тех, кто читает таблицы напрямую.
		if err := saveScaleColumnsTx(tx, plan); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// NetworkYear — сеть и год, за которые надо пересчитать инвестиции.
type NetworkYear struct {
	NetworkID int
	Year      int
}

// NetworkYearsWithPlans перечисляет пары «сеть × год», где вообще есть планы.
// Нужен пакетному пересчёту после ежедневной заливки факта: обходить сети,
// которых нет в реестре, незачем.
func NetworkYearsWithPlans(year int) ([]NetworkYear, error) {
	query := `SELECT DISTINCT p.network_id, p.[year]
		FROM dbo.tbl_NetworkPlans p
		JOIN dbo.tbl_Networks n ON n.id = p.network_id
		WHERE n.is_active = 1`
	args := []interface{}{}
	if year > 0 {
		query += " AND p.[year] = ?"
		args = append(args, year)
	}
	query += " ORDER BY p.network_id, p.[year]"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query network years: %w", err)
	}
	defer rows.Close()

	result := []NetworkYear{}
	for rows.Next() {
		var item NetworkYear
		if err := rows.Scan(&item.NetworkID, &item.Year); err != nil {
			return nil, fmt.Errorf("scan network year: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
