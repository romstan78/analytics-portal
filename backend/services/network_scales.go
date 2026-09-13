package services

import (
	"sort"

	"backend/models"
)

// ─── Ступени контракта ──────────────────────────────────────────────────────
//
// Ступень — порог объёма и процент инвестиций за него. Лестница области —
// пороги T₁ < T₂ < T₃ пула, отдельного бренда или объединённого периода.
// Достигнутая ступень k* — наивысшая, чей порог закрыт объёмом области.
//
// База к оплате считается по каждому SKU бренда (или по бренду целиком, если
// SKU-строк нет) от его плана на ступени:
//   закрытая крышка            → база = min(объём, план);
//   k* не последняя в области  → база = объём (коридор оплачивается полностью);
//   последняя, открытая        → база = объём;
//   последняя, процентная c    → база = min(объём, план × (1 + c/100)).
// Инвестиции = Σ база × процент SKU (или бренда) на ступени k*.
//
// Всё, что ниже, — арифметика без обращения к БД: объёмы SKU приходят на
// строках ступеней сводом помесячного слоя (ApplyForecastRollup).

// planScales — лестница строки плана, нормализованная для расчёта: ступень 1
// всегда есть и берёт порог и процент из самой строки, верхние ступени идут по
// номеру. Крышка и SKU ступени 1 берутся из сохранённой строки ступени, если
// она есть. Лестница обрывается на первой ступени без порога.
func planScales(plan models.NetworkPlan) []models.NetworkPlanScale {
	byNo := make(map[int]models.NetworkPlanScale, len(plan.Scales))
	for _, scale := range plan.Scales {
		byNo[scale.ScaleNo] = scale
	}
	first := byNo[1]
	first.ScaleNo = 1
	first.PlanRub = plan.PlanRub
	first.PlanUnits = plan.PlanUnits
	first.InvestmentsPct = plan.InvestmentsPct
	first.EffectiveInvestmentsPct = plan.InvestmentsPct
	ladder := []models.NetworkPlanScale{first}
	for no := 2; no <= 3; no++ {
		scale, ok := byNo[no]
		if !ok || scale.PlanRub == nil {
			break
		}
		// Ступень без своего процента наследует процент ступени ниже: КАМ,
		// заведший порог, не должен обнулить выплату до ввода процента.
		scale.EffectiveInvestmentsPct = scale.InvestmentsPct
		if scale.EffectiveInvestmentsPct == nil {
			scale.EffectiveInvestmentsPct = ladder[len(ladder)-1].EffectiveInvestmentsPct
		}
		ladder = append(ladder, scale)
	}
	return ladder
}

// thresholdsOf — пороги владельца области по ступеням. Лестница обрывается на
// ступени, порог которой не выше предыдущего: такая ступень не измеряет ничего.
func thresholdsOf(plan models.NetworkPlan) []float64 {
	thresholds := make([]float64, 0, 3)
	for _, scale := range planScales(plan) {
		if scale.PlanRub == nil {
			break
		}
		thresholds = append(thresholds, round2(*scale.PlanRub))
	}
	return normalizeLadder(thresholds)
}

// normalizeLadder обрезает лестницу на первом пороге, который не растёт.
// Нулевой первый порог оставляется: «план не задан» отличают выше по коду.
func normalizeLadder(thresholds []float64) []float64 {
	for i := 1; i < len(thresholds); i++ {
		if thresholds[i] <= thresholds[i-1] {
			return thresholds[:i]
		}
	}
	return thresholds
}

// reachedScale — достигнутая ступень: наивысшая с порогом не выше объёма.
// Ноль означает, что не закрыт даже первый порог, и ноль же — когда лестницы нет.
func reachedScale(thresholds []float64, achieved float64) int {
	reached := 0
	for i, threshold := range thresholds {
		if threshold > 0 && achieved >= threshold {
			reached = i + 1
		}
	}
	return reached
}

// capSetting — крышка владельца порога. Пустой режим читается как открытая:
// строки до миграции 032 крышки не знали, и открытая — это их поведение.
type capSetting struct {
	mode string
	pct  *float64
}

func capOf(mode string, pct *float64) capSetting {
	if mode == "" {
		mode = models.CapModeOpen
	}
	return capSetting{mode: mode, pct: pct}
}

// cappedBase — база к оплате одного ведра (SKU или бренда целиком) на ступени.
// Закрытая крышка — «сверх плана не платим»: не больше плана ведра, но и не
// больше его объёма. Иначе ведро без объёма (SKU без помесячных данных,
// остаток бренда) получало бы план, и база бренда превышала бы его объём.
func cappedBase(volume, plan float64, isLast bool, cap capSetting) float64 {
	switch {
	case cap.mode == models.CapModeClosed:
		if volume < plan {
			return round2(volume)
		}
		return round2(plan)
	case !isLast, cap.mode == models.CapModeOpen:
		return round2(volume)
	default:
		limit := plan
		if cap.pct != nil {
			limit = plan * (1 + *cap.pct/100)
		}
		if volume < limit {
			return round2(volume)
		}
		return round2(limit)
	}
}

// bucket — единица расчёта базы: SKU со своими планом, объёмом и настройками
// либо «всё остальное» бренда — остаток плана и объёма вне SKU-строк.
type bucket struct {
	sku    *models.NetworkPlanScaleSKU // nil — остаток бренда
	plan   float64
	volume float64
	pct    *float64
	cap    capSetting
}

// scaleBuckets раскладывает ступень бренда на вёдра. Объём SKU берётся из
// строки SKU по выбранной мере (прогноз или факт); всё, что не разложено по
// SKU — и план, и объём, — остаётся ведром бренда с его процентом и крышкой.
func scaleBuckets(
	scale models.NetworkPlanScale,
	brandVolume *float64,
	ownerCap capSetting,
	volumeOf func(models.NetworkPlanScaleSKU) *float64,
) []bucket {
	brandPlan := models.ValFloat(scale.PlanRub)
	restPlan, restVolume := brandPlan, models.ValFloat(brandVolume)
	buckets := make([]bucket, 0, len(scale.SKUs)+1)
	for i := range scale.SKUs {
		row := &scale.SKUs[i]
		pct := row.InvestmentsPct
		if pct == nil {
			pct = scale.EffectiveInvestmentsPct
		}
		cap := ownerCap
		if row.CapMode != "" {
			cap = capOf(row.CapMode, row.CapPct)
		}
		plan := models.ValFloat(row.PlanRub)
		volume := models.ValFloat(volumeOf(*row))
		restPlan = round2(restPlan - plan)
		restVolume = round2(restVolume - volume)
		buckets = append(buckets, bucket{sku: row, plan: plan, volume: volume, pct: pct, cap: cap})
	}
	if restPlan < 0 {
		restPlan = 0
	}
	if restVolume < 0 {
		restVolume = 0
	}
	// Ведро бренда есть всегда: у строки без SKU оно и есть бренд целиком.
	buckets = append(buckets, bucket{plan: restPlan, volume: restVolume, pct: scale.EffectiveInvestmentsPct, cap: ownerCap})
	return buckets
}

// scaleOutcome — результат ступени по одной мере: база и инвестиции в двух
// базах НДС. pctKnown — хоть у одного ведра есть процент; без него результат
// не ноль, а «считать нечем».
type scaleOutcome struct {
	base     float64
	gross    float64
	net      float64
	pctKnown bool
}

// evaluateScale считает ступень бренда «как если бы она была достигнутой»,
// попутно записывая базу и инвестиции в строки SKU (writeSKU).
func evaluateScale(
	scale *models.NetworkPlanScale,
	brandVolume *float64,
	isLast bool,
	ownerCap capSetting,
	period models.NetworkPeriod,
	volumeOf func(models.NetworkPlanScaleSKU) *float64,
	writeSKU func(*models.NetworkPlanScaleSKU, float64, float64, float64),
) scaleOutcome {
	var out scaleOutcome
	for _, b := range scaleBuckets(*scale, brandVolume, ownerCap, volumeOf) {
		base := cappedBase(b.volume, b.plan, isLast, b.cap)
		gross, net := 0.0, 0.0
		if b.pct != nil {
			out.pctKnown = true
			gross, net = investmentsFor(base, *b.pct, period.VATIncluded, period.VATRate)
		}
		out.base = round2(out.base + base)
		out.gross = round2(out.gross + gross)
		out.net = round2(out.net + net)
		if b.sku != nil && writeSKU != nil {
			writeSKU(b.sku, base, gross, net)
		}
	}
	return out
}

// plannedInvestmentsOfScale — плановые инвестиции ступени: план каждого ведра
// на его процент, без порога и крышки. Для ступени 1 без SKU это ровно
// plan_rub × investments_pct — то, что строка плана показывала всегда.
func plannedInvestmentsOfScale(scale *models.NetworkPlanScale, period models.NetworkPeriod) (gross, net *float64) {
	known := false
	sumGross, sumNet := 0.0, 0.0
	for _, b := range scaleBuckets(*scale, nil, capOf("", nil), func(models.NetworkPlanScaleSKU) *float64 { return nil }) {
		if b.pct == nil {
			continue
		}
		known = true
		g, n := investmentsFor(b.plan, *b.pct, period.VATIncluded, period.VATRate)
		sumGross = round2(sumGross + g)
		sumNet = round2(sumNet + n)
		if b.sku != nil {
			b.sku.PlanInvestmentsRub = models.PtrFloat(g)
			b.sku.PlanInvestmentsNet = models.PtrFloat(n)
		}
	}
	if !known {
		return nil, nil
	}
	return &sumGross, &sumNet
}

// enrichPlanScales заполняет плановые инвестиции каждой ступени строки.
// Ступень 1 строки без SKU даёт те же числа, что и plan_rub × investments_pct.
func enrichPlanScales(plan *models.NetworkPlan, period models.NetworkPeriod) {
	ladder := planScales(*plan)
	for i := range ladder {
		scale := &ladder[i]
		if plan.BrandAS == nil {
			// Пул инвестиций не ведёт: у него только пороги.
			scale.PlanInvestmentsRub, scale.PlanInvestmentsNet = nil, nil
			continue
		}
		scale.PlanInvestmentsRub, scale.PlanInvestmentsNet = plannedInvestmentsOfScale(scale, period)
	}
	plan.Scales = ladder
}

// brandEffectiveScale — ступень, по которой платят бренду, когда область
// достигла ступени reached: наивысшая ступень бренда не выше достигнутой.
// Бренд с короткой лестницей в трёхступенчатом пуле получает свою верхнюю.
func brandEffectiveScale(ladder []models.NetworkPlanScale, reached int) int {
	if reached > len(ladder) {
		return len(ladder)
	}
	return reached
}

// applyScaleRule считает по одной мере (прогноз или факт) все ступени строки
// бренда: гипотетический результат каждой и итог по достигнутой. Возвращает
// итог строки; в ladder записываются результаты ступеней и их SKU.
type measure struct {
	volumeOfBrand *float64
	volumeOfSKU   func(models.NetworkPlanScaleSKU) *float64
	reached       int // достигнутая ступень области
	ladderLen     int // число ступеней области
	write         func(scale *models.NetworkPlanScale, out scaleOutcome, reached bool)
	writeSKU      func(*models.NetworkPlanScaleSKU, float64, float64, float64)
}

func applyScaleRule(
	ladder []models.NetworkPlanScale,
	ownerCap capSetting,
	period models.NetworkPeriod,
	m measure,
) (effective int, outcome scaleOutcome) {
	effective = brandEffectiveScale(ladder, m.reached)
	for i := range ladder {
		scale := &ladder[i]
		// Гипотеза «область на этой ступени»: крышка действует, если ступень —
		// последняя в области. Для достигнутой ступени решает не её номер, а
		// область: пул на последней ступени режет крышкой и бренд с короткой
		// лестницей — по его верхнему плану. Итог записывается поверх гипотезы.
		isLast := scale.ScaleNo >= m.ladderLen
		if scale.ScaleNo == effective {
			isLast = m.reached >= m.ladderLen
		}
		out := evaluateScale(scale, m.volumeOfBrand, isLast, ownerCap, period, m.volumeOfSKU, m.writeSKU)
		m.write(scale, out, scale.ScaleNo <= m.reached)
		if scale.ScaleNo == effective {
			outcome = out
		}
	}
	return effective, outcome
}

// sortScales — ступени по номеру: репозиторий отдаёт их так, но расчёт
// на порядок вставки полагаться не должен.
func sortScales(scales []models.NetworkPlanScale) {
	sort.Slice(scales, func(i, j int) bool { return scales[i].ScaleNo < scales[j].ScaleNo })
}

// scaleTotalsOf — план квартала по ступеням: порог пула, планы валовых и
// отдельных брендов, остаток и плановые инвестиции каждой ступени.
// Ступень 1 повторяет поля NetworkPlanTotals.
func scaleTotalsOf(plans []models.NetworkPlan, quarter int) []models.NetworkPlanScaleTotals {
	// Ступень 1 есть у любого квартала, даже пустого: она — обязательство, и
	// объединённый период считает её сумму по всем своим кварталам.
	totals := []models.NetworkPlanScaleTotals{{ScaleNo: 1}}
	at := func(no int) *models.NetworkPlanScaleTotals {
		for len(totals) < no {
			totals = append(totals, models.NetworkPlanScaleTotals{ScaleNo: len(totals) + 1})
		}
		return &totals[no-1]
	}
	for _, p := range plans {
		if p.Quarter != quarter {
			continue
		}
		for _, scale := range planScales(p) {
			t := at(scale.ScaleNo)
			if p.BrandAS == nil {
				if scale.PlanRub != nil {
					pool := round2(*scale.PlanRub)
					t.GrossPoolRub = &pool
				}
				continue
			}
			if scale.PlanRub != nil {
				if p.InGross {
					t.GrossBrandsPlan = round2(t.GrossBrandsPlan + *scale.PlanRub)
				} else {
					t.SeparatePlanRub = round2(t.SeparatePlanRub + *scale.PlanRub)
				}
			}
			t.InvestmentsRub = round2(t.InvestmentsRub + models.ValFloat(scale.PlanInvestmentsRub))
			t.InvestmentsRubNet = round2(t.InvestmentsRubNet + models.ValFloat(scale.PlanInvestmentsNet))
		}
	}
	for i := range totals {
		t := &totals[i]
		if t.GrossPoolRub != nil {
			rest := round2(*t.GrossPoolRub - t.GrossBrandsPlan)
			t.Undistributed = &rest
		}
		pool := t.GrossBrandsPlan
		if t.GrossPoolRub != nil {
			pool = *t.GrossPoolRub
		}
		t.ContractPlanRub = round2(pool + t.SeparatePlanRub)
	}
	return totals
}

// scopeThresholdsFromTotals — пороги квартала по ступеням для области: пула
// (порог пула, иначе сумма планов валовых брендов) или всего контракта.
func scopeThresholdsFromTotals(t NetworkPlanTotals, contract bool) []float64 {
	thresholds := make([]float64, 0, len(t.Scales))
	for _, scale := range t.Scales {
		if contract {
			thresholds = append(thresholds, scale.ContractPlanRub)
			continue
		}
		if scale.GrossPoolRub != nil {
			thresholds = append(thresholds, *scale.GrossPoolRub)
		} else {
			thresholds = append(thresholds, scale.GrossBrandsPlan)
		}
	}
	return normalizeLadder(thresholds)
}

// sumLadders складывает лестницы кварталов поступенно: ступень объединённого
// периода есть, только если она есть в каждом квартале диапазона, иначе сумма
// порогов сравнивала бы полный квартал с неполным.
func sumLadders(ladders [][]float64) []float64 {
	if len(ladders) == 0 {
		return nil
	}
	depth := len(ladders[0])
	for _, ladder := range ladders[1:] {
		if len(ladder) < depth {
			depth = len(ladder)
		}
	}
	sum := make([]float64, depth)
	for _, ladder := range ladders {
		for i := 0; i < depth; i++ {
			sum[i] = round2(sum[i] + ladder[i])
		}
	}
	return normalizeLadder(sum)
}
