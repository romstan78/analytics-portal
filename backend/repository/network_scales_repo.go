package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"backend/config"
	"backend/models"
)

// ─── Ступени контракта: запись ──────────────────────────────────────────────
//
// Ступени и их SKU живут в своих таблицах, но сохраняются только вместе со
// строкой плана — в её транзакции. Ступень 1 есть у каждой строки и зеркалит
// plan_rub / investments_pct самой строки: источник — строка, зеркало — ради
// целостной лестницы для тех, кто читает таблицу ступеней напрямую.

// capLabel — крышка одной строкой для аудит-лога.
func capLabel(mode string, pct *float64) string {
	switch mode {
	case "", models.CapModeOpen:
		return "open"
	case models.CapModeClosed:
		return "closed"
	default:
		if pct == nil {
			return "pct"
		}
		return fmt.Sprintf("pct %.2f", *pct)
	}
}

// skuLabel — SKU ступени одной строкой для аудит-лога: план, процент, крышка.
func skuLabel(planRub, pct *float64, capMode string, capPct *float64) string {
	parts := []string{}
	if planRub != nil {
		parts = append(parts, fmt.Sprintf("plan %.2f", *planRub))
	}
	if pct != nil {
		parts = append(parts, fmt.Sprintf("%.2f%%", *pct))
	}
	if capMode != "" {
		parts = append(parts, capLabel(capMode, capPct))
	}
	if len(parts) == 0 {
		return "как у бренда"
	}
	return strings.Join(parts, " · ")
}

// validatePlanScalesInput проверяет лестницу строки до записи. У владельца
// порога (пул, отдельный бренд) пороги растут; у валового бренда на верхних
// ступенях — его планы, им расти не обязательно. Пул инвестиций и SKU не ведёт.
func validatePlanScalesInput(p NetworkPlanInput) error {
	if p.Scales == nil {
		return nil
	}
	seen := map[int]bool{}
	previous := p.PlanRub
	owner := p.BrandAS == nil || !p.InGross
	for _, scale := range sortedScaleInputs(p.Scales) {
		if scale.ScaleNo < 1 || scale.ScaleNo > 3 {
			return fmt.Errorf("недопустимый номер ступени: %d", scale.ScaleNo)
		}
		if seen[scale.ScaleNo] {
			return fmt.Errorf("ступень %d указана дважды", scale.ScaleNo)
		}
		seen[scale.ScaleNo] = true
		// Ступень 1 подразумевается строкой, ступень 3 без ступени 2 — дыра.
		if scale.ScaleNo == 3 && !seen[2] {
			return errors.New("ступень 3 без ступени 2")
		}
		if scale.InvestmentsPct != nil && (*scale.InvestmentsPct < 0 || *scale.InvestmentsPct > 100) {
			return fmt.Errorf("ступень %d: инвестиции вне диапазона 0–100", scale.ScaleNo)
		}
		if scale.PlanRub != nil && *scale.PlanRub < 0 || scale.PlanUnits != nil && *scale.PlanUnits < 0 {
			return fmt.Errorf("ступень %d: план не может быть отрицательным", scale.ScaleNo)
		}
		if p.BrandAS == nil && (scale.InvestmentsPct != nil || len(scale.SKUs) > 0) {
			return errors.New("пул не ведёт ни процент инвестиций, ни SKU")
		}
		if scale.ScaleNo > 1 {
			if scale.PlanRub == nil && scale.PlanUnits == nil {
				return fmt.Errorf("ступень %d без плана", scale.ScaleNo)
			}
			// Рост порогов проверяется по рублям, когда обе стороны известны:
			// план в упаковках получит рубли после пересчёта по ценам.
			if owner && scale.PlanRub != nil && previous != nil && *scale.PlanRub <= *previous {
				return fmt.Errorf("порог ступени %d должен быть выше ступени %d", scale.ScaleNo, scale.ScaleNo-1)
			}
			previous = scale.PlanRub
		}
		skus := map[string]bool{}
		for _, sku := range scale.SKUs {
			name := strings.TrimSpace(sku.SKU)
			if name == "" {
				return fmt.Errorf("ступень %d: SKU без названия", scale.ScaleNo)
			}
			if skus[name] {
				return fmt.Errorf("ступень %d: SKU %q указан дважды", scale.ScaleNo, name)
			}
			skus[name] = true
			if sku.InvestmentsPct != nil && (*sku.InvestmentsPct < 0 || *sku.InvestmentsPct > 100) {
				return fmt.Errorf("SKU %q: инвестиции вне диапазона 0–100", name)
			}
			if sku.PlanRub != nil && *sku.PlanRub < 0 || sku.PlanUnits != nil && *sku.PlanUnits < 0 {
				return fmt.Errorf("SKU %q: план не может быть отрицательным", name)
			}
			if sku.CapMode != "" {
				if _, ok := oneOfString(sku.CapMode, models.CapModeOpen, models.CapModePct, models.CapModeClosed); !ok {
					return fmt.Errorf("SKU %q: недопустимый режим крышки %q", name, sku.CapMode)
				}
				if sku.CapMode == models.CapModePct && (sku.CapPct == nil || *sku.CapPct < 0) {
					return fmt.Errorf("SKU %q: процентная крышка требует неотрицательный процент", name)
				}
			}
		}
	}
	return nil
}

// sortedScaleInputs — ступени по номеру, вход не меняется.
func sortedScaleInputs(scales []NetworkPlanScaleInput) []NetworkPlanScaleInput {
	sorted := append(make([]NetworkPlanScaleInput, 0, len(scales)), scales...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].ScaleNo < sorted[j-1].ScaleNo; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

// syncPlanScalesTx приводит ступени строки плана к запросу.
//
// Ступень 1 зеркалит строку всегда. nil в Scales — клиент, который про
// ступени не знает: верхние ступени и SKU остаются. Иначе лестница
// переписывается целиком: ступени, которых нет в запросе, удаляются вместе с
// SKU; SKU ступени — тоже полный список.
func syncPlanScalesTx(
	tx *sql.Tx,
	planID int,
	p NetworkPlanInput,
	planUnits *float64,
	old []models.NetworkPlanScale,
	userName string,
) ([]planChange, error) {
	oldByNo := make(map[int]models.NetworkPlanScale, len(old))
	for _, scale := range old {
		oldByNo[scale.ScaleNo] = scale
	}
	brandLabel := ""
	if p.BrandAS != nil {
		brandLabel = *p.BrandAS
	}
	var changes []planChange

	// Ступень 1 — зеркало строки.
	firstID, err := upsertScaleTx(tx, planID, oldByNo[1], 1, p.PlanRub, planUnits, p.InvestmentsPct, userName)
	if err != nil {
		return nil, err
	}

	if p.Scales == nil {
		return nil, nil
	}

	wanted := map[int]NetworkPlanScaleInput{}
	for _, scale := range p.Scales {
		wanted[scale.ScaleNo] = scale
	}

	// SKU ступени 1 — из запроса; без ступени 1 в запросе их нет.
	skuChanges, err := syncScaleSKUsTx(tx, firstID, 1, brandLabel, p.Quarter, oldByNo[1].SKUs, wanted[1].SKUs)
	if err != nil {
		return nil, err
	}
	changes = append(changes, skuChanges...)

	for no := 2; no <= 3; no++ {
		scale, ok := wanted[no]
		oldScale, existed := oldByNo[no]
		if !ok {
			if existed {
				changes = append(changes, planChange{
					Quarter: p.Quarter, Brand: brandLabel, Field: fmt.Sprintf("scale%d", no),
					Old: floatPtrValue(oldScale.PlanRub), New: nil,
				})
				if _, err := tx.Exec(`DELETE FROM dbo.tbl_NetworkPlanScales WHERE id = ?`, oldScale.ID); err != nil {
					return nil, err
				}
			}
			continue
		}
		if existed && scale.UpdatedAt != "" && scale.UpdatedAt != oldScale.UpdatedAt {
			return nil, ErrNetworkConflict
		}
		if !existed || !floatPtrEqual(oldScale.PlanRub, scale.PlanRub) {
			changes = append(changes, planChange{
				Quarter: p.Quarter, Brand: brandLabel, Field: fmt.Sprintf("scale%d_plan_rub", no),
				Old: floatPtrValue(oldScale.PlanRub), New: floatPtrValue(scale.PlanRub),
			})
		}
		if !existed && scale.InvestmentsPct != nil || existed && !floatPtrEqual(oldScale.InvestmentsPct, scale.InvestmentsPct) {
			changes = append(changes, planChange{
				Quarter: p.Quarter, Brand: brandLabel, Field: fmt.Sprintf("scale%d_investments_pct", no),
				Old: floatPtrValue(oldScale.InvestmentsPct), New: floatPtrValue(scale.InvestmentsPct),
			})
		}
		scaleID, err := upsertScaleTx(tx, planID, oldScale, no, scale.PlanRub, scale.PlanUnits, scale.InvestmentsPct, userName)
		if err != nil {
			return nil, err
		}
		skuChanges, err := syncScaleSKUsTx(tx, scaleID, no, brandLabel, p.Quarter, oldScale.SKUs, scale.SKUs)
		if err != nil {
			return nil, err
		}
		changes = append(changes, skuChanges...)
	}
	return changes, nil
}

// upsertScaleTx пишет ступень и возвращает её id.
func upsertScaleTx(
	tx *sql.Tx,
	planID int,
	old models.NetworkPlanScale,
	no int,
	planRub, planUnits, pct *float64,
	userName string,
) (int64, error) {
	if old.ID != 0 {
		_, err := tx.Exec(
			`UPDATE dbo.tbl_NetworkPlanScales
			 SET plan_rub = ?, plan_units = ?, investments_pct = ?, updated_by = ?, updated_at = GETDATE()
			 WHERE id = ?`,
			planRub, planUnits, pct, userName, old.ID,
		)
		return old.ID, err
	}
	var id int64
	err := tx.QueryRow(
		`INSERT INTO dbo.tbl_NetworkPlanScales (plan_id, scale_no, plan_rub, plan_units, investments_pct, updated_by)
		 OUTPUT INSERTED.id VALUES (?, ?, ?, ?, ?, ?)`,
		planID, no, planRub, planUnits, pct, userName,
	).Scan(&id)
	return id, err
}

// syncScaleSKUsTx приводит SKU ступени к списку из запроса: лишние удаляются,
// остальные обновляются или заводятся.
func syncScaleSKUsTx(
	tx *sql.Tx,
	scaleID int64,
	no int,
	brandLabel string,
	quarter int,
	old []models.NetworkPlanScaleSKU,
	wanted []NetworkPlanScaleSKUInput,
) ([]planChange, error) {
	oldBySKU := make(map[string]models.NetworkPlanScaleSKU, len(old))
	for _, sku := range old {
		oldBySKU[sku.SKU] = sku
	}
	var changes []planChange
	field := fmt.Sprintf("scale%d_sku", no)
	kept := map[string]bool{}
	for _, sku := range wanted {
		name := strings.TrimSpace(sku.SKU)
		kept[name] = true
		capMode := sku.CapMode
		capPct := sku.CapPct
		if capMode != models.CapModePct {
			capPct = nil
		}
		var capModeValue interface{}
		if capMode != "" {
			capModeValue = capMode
		}
		label := skuLabel(sku.PlanRub, sku.InvestmentsPct, capMode, capPct)
		if existing, ok := oldBySKU[name]; ok {
			oldLabel := skuLabel(existing.PlanRub, existing.InvestmentsPct, existing.CapMode, existing.CapPct)
			if oldLabel != label || !floatPtrEqual(existing.PlanUnits, sku.PlanUnits) {
				changes = append(changes, planChange{Quarter: quarter, Brand: brandLabel + " / " + name, Field: field, Old: oldLabel, New: label})
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkPlanScaleSKU
				 SET plan_rub = ?, plan_units = ?, investments_pct = ?, cap_mode = ?, cap_pct = ?, updated_at = GETDATE()
				 WHERE id = ?`,
				sku.PlanRub, sku.PlanUnits, sku.InvestmentsPct, capModeValue, capPct, existing.ID,
			); err != nil {
				return nil, err
			}
			continue
		}
		changes = append(changes, planChange{Quarter: quarter, Brand: brandLabel + " / " + name, Field: field, Old: nil, New: label})
		if _, err := tx.Exec(
			`INSERT INTO dbo.tbl_NetworkPlanScaleSKU (scale_id, sku, plan_rub, plan_units, investments_pct, cap_mode, cap_pct)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			scaleID, name, sku.PlanRub, sku.PlanUnits, sku.InvestmentsPct, capModeValue, capPct,
		); err != nil {
			return nil, err
		}
	}
	for _, existing := range old {
		if kept[existing.SKU] {
			continue
		}
		changes = append(changes, planChange{
			Quarter: quarter, Brand: brandLabel + " / " + existing.SKU, Field: field,
			Old: skuLabel(existing.PlanRub, existing.InvestmentsPct, existing.CapMode, existing.CapPct), New: nil,
		})
		if _, err := tx.Exec(`DELETE FROM dbo.tbl_NetworkPlanScaleSKU WHERE id = ?`, existing.ID); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

// ─── Расчётные колонки ступеней ─────────────────────────────────────────────

// saveScaleColumnsTx записывает итог правила по ступеням и SKU строки плана.
//
// Ступень 1 без id — строка плана заведена мимо API (сидер, загрузчик) и в
// таблице ступеней её ещё нет: пересчёт заводит зеркало сам, иначе таблица
// ступеней отставала бы от плана до первого сохранения формы. Верхние
// ступени без id бывают только в черновике, и их пропускаем.
func saveScaleColumnsTx(tx *sql.Tx, plan models.NetworkPlan) error {
	for _, scale := range plan.Scales {
		if scale.ID == 0 && scale.ScaleNo == 1 && plan.ID != 0 {
			user := ""
			if plan.UpdatedBy != nil {
				user = *plan.UpdatedBy
			}
			id, err := upsertScaleTx(tx, plan.ID, models.NetworkPlanScale{}, 1, plan.PlanRub, plan.PlanUnits, plan.InvestmentsPct, user)
			if err != nil {
				return fmt.Errorf("mirror scale 1 for plan %d: %w", plan.ID, err)
			}
			scale.ID = id
		}
		if scale.ID == 0 {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE dbo.tbl_NetworkPlanScales
			    SET plan_investments_rub = ?, plan_investments_rub_net = ?,
			        forecast_rub = ?, forecast_base_rub = ?,
			        forecast_investments_rub = ?, forecast_investments_rub_net = ?, forecast_reached = ?,
			        fact_rub = ?, fact_base_rub = ?,
			        fact_investments_rub = ?, fact_investments_rub_net = ?, fact_reached = ?
			  WHERE id = ?`,
			scale.PlanInvestmentsRub, scale.PlanInvestmentsNet,
			scale.ForecastRub, scale.ForecastBaseRub,
			scale.ForecastInvestmentsRub, scale.ForecastInvestmentsNet, scale.ForecastReached,
			scale.FactRub, scale.FactBaseRub,
			scale.FactInvestmentsRub, scale.FactInvestmentsNet, scale.FactReached,
			scale.ID,
		); err != nil {
			return fmt.Errorf("save scale columns for scale %d: %w", scale.ID, err)
		}
		for _, sku := range scale.SKUs {
			if sku.ID == 0 {
				continue
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkPlanScaleSKU
				    SET plan_investments_rub = ?, plan_investments_rub_net = ?,
				        forecast_rub = ?, forecast_base_rub = ?,
				        forecast_investments_rub = ?, forecast_investments_rub_net = ?,
				        fact_rub = ?, fact_base_rub = ?,
				        fact_investments_rub = ?, fact_investments_rub_net = ?
				  WHERE id = ?`,
				sku.PlanInvestmentsRub, sku.PlanInvestmentsNet,
				sku.ForecastRub, sku.ForecastBaseRub,
				sku.ForecastInvestmentsRub, sku.ForecastInvestmentsNet,
				sku.FactRub, sku.FactBaseRub,
				sku.FactInvestmentsRub, sku.FactInvestmentsNet,
				sku.ID,
			); err != nil {
				return fmt.Errorf("save sku columns for sku %d: %w", sku.ID, err)
			}
		}
	}
	return nil
}

// ─── Пара «рубли / упаковки» плана ──────────────────────────────────────────

// SaveNetworkPlanPairs закрепляет обе метрики пары у строк плана, ступеней и
// SKU. Записываются только строки с id: план, которого в базе нет, пары не
// имеет. Возвращает число обновлённых строк плана.
func SaveNetworkPlanPairs(plans []models.NetworkPlan) (int64, error) {
	if len(plans) == 0 {
		return 0, nil
	}
	tx, err := config.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var written int64
	for _, plan := range plans {
		if plan.ID == 0 {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE dbo.tbl_NetworkPlans SET plan_rub = ?, plan_units = ? WHERE id = ?`,
			plan.PlanRub, plan.PlanUnits, plan.ID,
		); err != nil {
			return written, fmt.Errorf("save plan pair for plan %d: %w", plan.ID, err)
		}
		written++
		for _, scale := range plan.Scales {
			if scale.ID == 0 {
				continue
			}
			if _, err := tx.Exec(
				`UPDATE dbo.tbl_NetworkPlanScales SET plan_rub = ?, plan_units = ? WHERE id = ?`,
				scale.PlanRub, scale.PlanUnits, scale.ID,
			); err != nil {
				return written, fmt.Errorf("save scale pair for scale %d: %w", scale.ID, err)
			}
			for _, sku := range scale.SKUs {
				if sku.ID == 0 {
					continue
				}
				if _, err := tx.Exec(
					`UPDATE dbo.tbl_NetworkPlanScaleSKU SET plan_rub = ?, plan_units = ? WHERE id = ?`,
					sku.PlanRub, sku.PlanUnits, sku.ID,
				); err != nil {
					return written, fmt.Errorf("save sku pair for sku %d: %w", sku.ID, err)
				}
			}
		}
	}
	return written, tx.Commit()
}
