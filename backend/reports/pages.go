package reports

import (
	"fmt"
	"strings"

	"backend/models"
)

// ─── Страничная модель ──────────────────────────────────────────────────────
//
// Блок отчёта — одна страница PDF и один слайд PPTX (длинная таблица
// продолжается на следующих). Страница описана данными, а не командами
// рисования: карточки показателей, спецификация графика, таблица с
// оценками ячеек, примечания. Рендеры форматов только раскладывают это по
// макету, а рисуют графики общим кодом из canvas.go.

type cell struct {
	Text string
	Tone tone
	// Bar — доля для полосы в ячейке (0..1+), отрицательное — без полосы.
	Bar  float64
	Bold bool
}

func txt(s string) cell           { return cell{Text: s, Bar: -1} }
func toned(s string, t tone) cell { return cell{Text: s, Tone: t, Bar: -1} }

// pctCell — процент выполнения с оценкой цветом и полосой.
func pctCell(p *float64) cell {
	c := cell{Text: percent(p), Tone: completionTone(p), Bar: -1}
	if p != nil {
		c.Bar = *p / 100
	}
	return c
}

type column struct {
	Title string
	// Weight — доля ширины таблицы.
	Weight float64
	Right  bool
}

type table struct {
	Columns []column
	Rows    [][]cell
	// Total — итоговая строка: жирная, с линией сверху.
	Total []cell
}

type page struct {
	Block    string
	Title    string
	Subtitle string
	// Lines — абзацы текста: титул, методика.
	Lines []string
	Cards []card
	Chart *chartSpec
	Table *table
	Notes []string
}

// maxTableRowsPerSlide — сколько строк таблицы помещается на слайд.
const maxTableRowsPerSlide = 18

// bulletTop — сколько строк попадает на bullet-график разреза.
const bulletTop = 10

// BuildPages превращает снимок в последовательность страниц в порядке блоков
// запроса.
func BuildPages(s *models.ReportSnapshot) ([]page, error) {
	if s == nil || s.Dashboard == nil {
		return nil, fmt.Errorf("снимок отчёта пуст")
	}
	pages := make([]page, 0, len(s.Request.Blocks))
	for _, code := range s.Request.Blocks {
		p, ok := buildPage(code, s)
		if !ok {
			return nil, fmt.Errorf("блок %s: неизвестный блок", code)
		}
		pages = append(pages, p)
	}
	return pages, nil
}

func buildPage(code string, s *models.ReportSnapshot) (page, bool) {
	switch code {
	case models.ReportBlockCover:
		return coverPage(s), true
	case models.ReportBlockSummaryKPI:
		return summaryPage(s), true
	case models.ReportBlockPeriodTrend:
		return periodTrendPage(s), true
	case models.ReportBlockPlanFactEAC:
		return planFactEACPage(s), true
	case models.ReportBlockNetworkTop:
		return breakdownPage(s, models.ReportBlockNetworkTop, "Топ сетей", "Сеть", s.Dashboard.Networks, true), true
	case models.ReportBlockBrandTop:
		return brandTopPage(s), true
	case models.ReportBlockKAMTop:
		return breakdownPage(s, models.ReportBlockKAMTop, "Разрез по КАМам", "КАМ", s.Dashboard.KAMs, false), true
	case models.ReportBlockGTN:
		return gtnPage(s), true
	case models.ReportBlockRegistryOpex:
		return registryOpexPage(s), true
	case models.ReportBlockPromoInvestments:
		return promoInvestmentsPage(s), true
	case models.ReportBlockNetworkQuarters:
		return networkQuartersPage(s), true
	case models.ReportBlockMethodology:
		return methodologyPage(s), true
	}
	return page{}, false
}

func unitNote(unit string) string {
	if unit == models.ReportUnitUnits {
		return "Объём в упаковках, инвестиции в рублях."
	}
	return "Объём и инвестиции в рублях с НДС; сравнимые между сетями суммы инвестиций — в базе без НДС."
}

// volumeScale — шкала объёма страницы по плану среза: самому большому числу.
func volumeScale(s *models.ReportSnapshot) scale {
	m := s.Dashboard.Summary
	return scaleFor(s.Request.Unit, pick(s.Request.Unit, m.PlanRub, m.PlanUnits))
}

// monthSeries — факт и EAC по месяцам для спарклайнов.
func monthSeries(s *models.ReportSnapshot) (fact, eac []float64) {
	unit := s.Request.Unit
	for _, m := range s.Dashboard.Months {
		fact = append(fact, pick(unit, m.FactRub, m.FactUnits))
		eac = append(eac, pick(unit, m.EACRub, m.EACUnits))
	}
	return fact, eac
}

// ─── Титул ──────────────────────────────────────────────────────────────────

func coverPage(s *models.ReportSnapshot) page {
	m := s.Dashboard.Summary
	unit := s.Request.Unit
	sc := volumeScale(s)
	// Период стоит в подзаголовке, остальные подписи области — абзацами.
	lines := append([]string{}, s.FilterLabels[1:]...)
	lines = append(lines, fmt.Sprintf("Автор: %s · Сформировано: %s", s.Owner, s.CreatedAt), unitNote(unit))
	if s.OwnKAM != "" {
		lines = append(lines, "Область отчёта ограничена закреплением автора за КАМом.")
	}
	fact, eac := monthSeries(s)
	return page{
		Block:    models.ReportBlockCover,
		Title:    s.Title,
		Subtitle: s.FilterLabels[0],
		Lines:    lines,
		Cards: []card{
			{Label: "План, " + sc.label, Value: sc.format(pick(unit, m.PlanRub, m.PlanUnits)),
				Sub: fmt.Sprintf("%d сетей · %d брендов", m.NetworkCount, m.BrandCount)},
			{Label: "Факт, " + sc.label, Value: sc.format(pick(unit, m.FactRub, m.FactUnits)),
				Delta: "выполнение " + percent(m.CompletionPct), DeltaTone: toneNeutral,
				Sub: "покрытие фактом " + percent(m.FactCoveragePct), Spark: fact},
			{Label: "Ожидаемый итог (EAC), " + sc.label, Value: sc.format(pick(unit, m.EACRub, m.EACUnits)),
				Delta: percent(m.EACCompletionPct) + " плана", DeltaTone: completionTone(m.EACCompletionPct),
				Sub: "разрыв " + sc.signed(pick(unit, m.GapRub, m.GapUnits)), Spark: eac},
		},
	}
}

// ─── Ключевые показатели ────────────────────────────────────────────────────

func summaryPage(s *models.ReportSnapshot) page {
	m := s.Dashboard.Summary
	unit := s.Request.Unit
	sc := volumeScale(s)
	fact, eac := monthSeries(s)
	cards := []card{
		{Label: "План, " + sc.label, Value: sc.format(pick(unit, m.PlanRub, m.PlanUnits)),
			Delta: yoyDelta(m.PlanYoYPct), DeltaTone: toneNeutral,
			Sub: fmt.Sprintf("%d сетей · %d брендов · обязательство по контракту", m.NetworkCount, m.BrandCount)},
		{Label: "Факт, " + sc.label, Value: sc.format(pick(unit, m.FactRub, m.FactUnits)),
			Delta: yoyDelta(m.FactYoYPct), DeltaTone: yoyTone(m.FactYoYPct),
			Sub: "выполнение плана " + percent(m.CompletionPct), Spark: fact},
		{Label: "Ожидаемый итог (EAC), " + sc.label, Value: sc.format(pick(unit, m.EACRub, m.EACUnits)),
			Delta: percent(m.EACCompletionPct) + " плана", DeltaTone: completionTone(m.EACCompletionPct),
			Sub: "разрыв к плану " + sc.signed(pick(unit, m.GapRub, m.GapUnits)), Spark: eac},
		{Label: "Факт прошлого года, " + sc.label, Value: sc.optional(pickPtr(unit, m.PrevFactRub, m.PrevFactUnits)),
			Sub: "тот же набор кварталов; «—» — сопоставимого периода нет"},
		{Label: "Покрытие фактом", Value: percent(m.FactCoveragePct),
			Delta: fmt.Sprintf("%d из %d закрытых ячеек", m.ClosedCellsWithFact, m.ClosedCells), DeltaTone: toneNeutral,
			Sub: "ячейка — сеть × бренд × месяц"},
		{Label: "Открытых ячеек без прогноза", Value: integer(float64(m.OpenCellsWithoutForecast)),
			Delta: openCellsDelta(m.OpenCellsWithoutForecast), DeltaTone: openCellsTone(m.OpenCellsWithoutForecast),
			Sub: "их EAC содержит только уже отгруженное"},
	}
	notes := []string{
		"EAC = факт закрытых месяцев + официальный прогноз открытых; месяц без прогноза планом не достраивается.",
	}
	if m.UndistributedRub != nil {
		notes = append(notes, "Нераспределённый остаток валового пула: "+money(*m.UndistributedRub)+" ₽ — входит в план, но не разобран брендами.")
	}
	return page{Block: models.ReportBlockSummaryKPI, Title: "Ключевые показатели", Subtitle: sc.label, Cards: cards, Notes: notes}
}

// yoyDelta — подпись сравнения с прошлым годом; без данных — пусто, а не «—».
func yoyDelta(p *float64) string {
	if p == nil {
		return ""
	}
	return signedPercent(p) + " к прошлому году"
}

func yoyTone(p *float64) tone {
	if p == nil {
		return toneNeutral
	}
	return deltaTone(*p)
}

func openCellsDelta(n int) string {
	if n == 0 {
		return "прогноз заполнен полностью"
	}
	return "требуют официального прогноза"
}

func openCellsTone(n int) tone {
	if n == 0 {
		return toneGood
	}
	return toneWarn
}

// ─── Динамика по месяцам ────────────────────────────────────────────────────

func periodTrendPage(s *models.ReportSnapshot) page {
	unit := s.Request.Unit
	months := s.Dashboard.Months
	values := make([]float64, 0, len(months)*2)
	for _, m := range months {
		values = append(values, pick(unit, m.PlanRub, m.PlanUnits), pick(unit, m.EACRub, m.EACUnits))
	}
	sc := scaleFor(unit, values...)
	points := make([]monthPoint, 0, len(months))
	rows := make([][]cell, 0, len(months))
	for _, m := range months {
		prev := pickPtr(unit, m.PrevFactRub, m.PrevFactUnits)
		points = append(points, monthPoint{
			Label: monthName(m.Month), Plan: pick(unit, m.PlanRub, m.PlanUnits),
			Fact: pick(unit, m.FactRub, m.FactUnits), EAC: pick(unit, m.EACRub, m.EACUnits),
			Prev: prev, Closed: m.Closed,
		})
		status := "открыт"
		if m.Closed {
			status = "закрыт"
		}
		without := txt(integer(float64(m.CellsWithoutForecast)))
		if m.CellsWithoutForecast > 0 {
			without.Tone = toneWarn
		}
		rows = append(rows, []cell{
			txt(monthName(m.Month)), txt(fmt.Sprintf("Q%d", m.Quarter)), txt(status),
			txt(sc.format(pick(unit, m.PlanRub, m.PlanUnits))),
			txt(sc.format(pick(unit, m.FactRub, m.FactUnits))),
			txt(sc.format(pick(unit, m.EACRub, m.EACUnits))),
			txt(sc.optional(prev)),
			without,
		})
	}
	return page{
		Block: models.ReportBlockPeriodTrend, Title: "Динамика по месяцам", Subtitle: sc.label,
		Chart: &chartSpec{Title: "Факт, ожидаемый итог и план по месяцам", Subtitle: sc.label, Scale: sc, Months: points},
		Table: &table{
			Columns: []column{{"Месяц", 1, false}, {"Кв.", 0.6, false}, {"Статус", 1, false},
				{"План, " + sc.label, 1.4, true}, {"Факт, " + sc.label, 1.4, true}, {"EAC, " + sc.label, 1.4, true},
				{"Прошлый год, " + sc.label, 1.4, true}, {"Без прогноза", 1.2, true}},
			Rows: rows,
		},
		Notes: []string{
			"План месяца — раскладка квартального обязательства по схеме распределения из профиля сети; помесячных планов в реестре нет.",
			"«Без прогноза» — открытые ячейки бренда без официального прогноза: их EAC содержит только уже отгруженное.",
		},
	}
}

// ─── План, факт и EAC по кварталам ──────────────────────────────────────────

func planFactEACPage(s *models.ReportSnapshot) page {
	unit := s.Request.Unit
	qs := s.Dashboard.Quarters
	values := []float64{pick(unit, s.Dashboard.Summary.PlanRub, s.Dashboard.Summary.PlanUnits)}
	for _, q := range qs {
		values = append(values, pick(unit, q.Metrics.PlanRub, q.Metrics.PlanUnits))
	}
	sc := scaleFor(unit, values...)
	bullets := make([]bulletRow, 0, len(qs))
	rows := make([][]cell, 0, len(qs))
	for _, q := range qs {
		m := q.Metrics
		label := fmt.Sprintf("Q%d", q.Quarter)
		bullets = append(bullets, bulletRow{Label: label, Plan: pick(unit, m.PlanRub, m.PlanUnits),
			Fact: pick(unit, m.FactRub, m.FactUnits), EAC: pick(unit, m.EACRub, m.EACUnits), Pct: m.EACCompletionPct})
		rows = append(rows, []cell{
			txt(label),
			txt(sc.format(pick(unit, m.PlanRub, m.PlanUnits))),
			txt(sc.format(pick(unit, m.FactRub, m.FactUnits))),
			txt(sc.format(pick(unit, m.EACRub, m.EACUnits))),
			toned(percent(m.CompletionPct), toneNeutral),
			pctCell(m.EACCompletionPct),
			toned(sc.signed(pick(unit, m.GapRub, m.GapUnits)), deltaTone(pick(unit, m.GapRub, m.GapUnits))),
		})
	}
	sm := s.Dashboard.Summary
	total := []cell{txt("Итого"),
		txt(sc.format(pick(unit, sm.PlanRub, sm.PlanUnits))), txt(sc.format(pick(unit, sm.FactRub, sm.FactUnits))),
		txt(sc.format(pick(unit, sm.EACRub, sm.EACUnits))),
		toned(percent(sm.CompletionPct), toneNeutral), pctCell(sm.EACCompletionPct),
		toned(sc.signed(pick(unit, sm.GapRub, sm.GapUnits)), deltaTone(pick(unit, sm.GapRub, sm.GapUnits)))}
	return page{
		Block: models.ReportBlockPlanFactEAC, Title: "План, факт и EAC по кварталам", Subtitle: sc.label,
		Chart: &chartSpec{Title: "Выполнение обязательства по кварталам", Subtitle: "полоса — план, внутри — факт, риска — ожидаемый итог · " + sc.label, Scale: sc, Bullet: bullets},
		Table: &table{
			Columns: []column{{"Квартал", 0.8, false}, {"План, " + sc.label, 1.3, true}, {"Факт, " + sc.label, 1.3, true},
				{"EAC, " + sc.label, 1.3, true}, {"Факт / план", 1, true}, {"EAC / план", 1.2, true}, {"Разрыв, " + sc.label, 1.3, true}},
			Rows: rows, Total: total,
		},
		Notes: []string{"План — обязательство по контракту: валовый пул входит целиком, даже если бренды разобрали его не полностью."},
	}
}

// ─── Разрезы: сети, КАМы ────────────────────────────────────────────────────

func breakdownPage(s *models.ReportSnapshot, block, title, nameTitle string, items []models.NetworkDashboardBreakdown, withKAM bool) page {
	unit := s.Request.Unit
	sc := breakdownScale(unit, items)
	limit := s.Request.TableLimit
	cols := []column{{"#", 0.3, false}, {nameTitle, 3, false}}
	if withKAM {
		cols = append(cols, column{"КАМ", 1.3, false})
	}
	cols = append(cols, column{"План, " + sc.label, 1.2, true}, column{"Факт, " + sc.label, 1.2, true}, column{"EAC, " + sc.label, 1.2, true},
		column{"EAC / план", 1.4, true}, column{"Покрытие", 0.9, true})
	rows := make([][]cell, 0, limit)
	bullets := make([]bulletRow, 0, bulletTop)
	for i, it := range items {
		if i >= limit {
			break
		}
		m := it.Metrics
		kam := ""
		if it.KAM != nil {
			kam = *it.KAM
		}
		if i < bulletTop {
			bullets = append(bullets, bulletRow{Label: it.Name, Sub: kam, Plan: pick(unit, m.PlanRub, m.PlanUnits),
				Fact: pick(unit, m.FactRub, m.FactUnits), EAC: pick(unit, m.EACRub, m.EACUnits), Pct: m.EACCompletionPct})
		}
		row := []cell{txt(fmt.Sprint(i + 1)), txt(truncate(it.Name, 40))}
		if withKAM {
			if kam == "" {
				kam = "—"
			}
			row = append(row, txt(kam))
		}
		row = append(row, txt(sc.format(pick(unit, m.PlanRub, m.PlanUnits))), txt(sc.format(pick(unit, m.FactRub, m.FactUnits))),
			txt(sc.format(pick(unit, m.EACRub, m.EACUnits))), pctCell(m.EACCompletionPct), txt(percent(m.FactCoveragePct)))
		rows = append(rows, row)
	}
	p := page{
		Block: block, Title: title, Subtitle: sc.label,
		Table: &table{Columns: cols, Rows: rows},
		Notes: []string{fmt.Sprintf("Показано %d из %d, сортировка по плану. Покрытие — доля закрытых ячеек с фактом.", len(rows), len(items))},
	}
	if len(bullets) > 1 {
		p.Chart = &chartSpec{Title: fmt.Sprintf("Топ-%d по плану: выполнение", len(bullets)), Subtitle: sc.label, Scale: sc, Bullet: bullets}
	}
	return p
}

// breakdownScale — шкала разреза по максимальному плану строк.
func breakdownScale(unit string, items []models.NetworkDashboardBreakdown) scale {
	values := make([]float64, 0, len(items))
	for _, it := range items {
		values = append(values, pick(unit, it.Metrics.PlanRub, it.Metrics.PlanUnits), pick(unit, it.Metrics.EACRub, it.Metrics.EACUnits))
	}
	return scaleFor(unit, values...)
}

func brandTopPage(s *models.ReportSnapshot) page {
	unit := s.Request.Unit
	items := s.Dashboard.Brands
	sc := breakdownScale(unit, items)
	limit := s.Request.TableLimit
	rows := make([][]cell, 0, limit)
	bullets := make([]bulletRow, 0, bulletTop)
	for i, it := range items {
		if i >= limit {
			break
		}
		m := it.Metrics
		inGross := "смешанно"
		switch {
		case it.InGross == nil:
		case *it.InGross:
			inGross = "в пуле"
		default:
			inGross = "отдельно"
		}
		if i < bulletTop {
			bullets = append(bullets, bulletRow{Label: it.Name, Sub: inGross, Plan: pick(unit, m.PlanRub, m.PlanUnits),
				Fact: pick(unit, m.FactRub, m.FactUnits), EAC: pick(unit, m.EACRub, m.EACUnits), Pct: m.EACCompletionPct})
		}
		rows = append(rows, []cell{txt(fmt.Sprint(i + 1)), txt(truncate(it.Name, 40)), txt(inGross),
			txt(sc.format(pick(unit, m.PlanRub, m.PlanUnits))), txt(sc.format(pick(unit, m.FactRub, m.FactUnits))),
			txt(sc.format(pick(unit, m.EACRub, m.EACUnits))), pctCell(m.EACCompletionPct)})
	}
	notes := []string{
		fmt.Sprintf("Показано %d из %d, сортировка по плану. План бренда — собственный план строки; нераспределённый остаток пула в бренды не входит.", len(rows), len(items)),
	}
	if s.Dashboard.Summary.UndistributedRub != nil {
		notes = append(notes, "Нераспределённый остаток валового пула: "+money(*s.Dashboard.Summary.UndistributedRub)+" ₽.")
	}
	p := page{
		Block: models.ReportBlockBrandTop, Title: "Топ брендов", Subtitle: sc.label,
		Table: &table{
			Columns: []column{{"#", 0.3, false}, {"Бренд", 2.6, false}, {"Валовый пул", 1, false},
				{"План, " + sc.label, 1.2, true}, {"Факт, " + sc.label, 1.2, true}, {"EAC, " + sc.label, 1.2, true}, {"EAC / план", 1.4, true}},
			Rows: rows,
		},
		Notes: notes,
	}
	if len(bullets) > 1 {
		p.Chart = &chartSpec{Title: fmt.Sprintf("Топ-%d по плану: выполнение", len(bullets)), Subtitle: sc.label, Scale: sc, Bullet: bullets}
	}
	return p
}

// ─── Инвестиции реестра (GTN) ───────────────────────────────────────────────

func gtnPage(s *models.ReportSnapshot) page {
	m := s.Dashboard.Summary
	sc := scaleFor(models.ReportUnitRub, m.PlanInvestmentsRubNet)
	cards := []card{
		{Label: "План инвестиций, " + sc.label + " без НДС", Value: sc.format(m.PlanInvestmentsRubNet),
			Sub: "с НДС " + sc.format(m.PlanInvestmentsRub)},
		{Label: "Факт, " + sc.label + " без НДС", Value: sc.format(m.FactInvestmentsRubNet),
			Sub: "закрытые месяцы, порог выполнения применён"},
		{Label: "Ожидаемый итог, " + sc.label + " без НДС", Value: sc.format(m.EACInvestmentsRubNet),
			Delta: sc.signed(m.InvestmentVarianceRub) + " к плану", DeltaTone: deltaTone(m.InvestmentVarianceRub),
			Sub: effectiveRate(m.EffectiveInvestmentsPct)},
	}
	steps := []waterfallStep{
		{Label: "План", Value: m.PlanInvestmentsRubNet, Total: true},
		{Label: "Отклонение", Value: m.InvestmentVarianceRub},
		{Label: "EAC", Value: m.EACInvestmentsRubNet, Total: true},
	}
	rows := make([][]cell, 0, len(s.Dashboard.Quarters))
	for _, q := range s.Dashboard.Quarters {
		qm := q.Metrics
		rows = append(rows, []cell{txt(fmt.Sprintf("Q%d", q.Quarter)),
			txt(sc.format(qm.PlanInvestmentsRubNet)), txt(sc.format(qm.FactInvestmentsRubNet)), txt(sc.format(qm.EACInvestmentsRubNet)),
			toned(sc.signed(qm.InvestmentVarianceRub), deltaTone(qm.InvestmentVarianceRub)), txt(percent(qm.EffectiveInvestmentsPct))})
	}
	return page{
		Block: models.ReportBlockGTN, Title: "Инвестиции реестра (GTN)", Subtitle: sc.label + " без НДС",
		Cards: cards,
		Chart: &chartSpec{Title: "От плана к ожидаемому итогу", Subtitle: sc.label + " без НДС", Scale: sc, Steps: steps},
		Table: &table{
			Columns: []column{{"Квартал", 0.8, false}, {"План, " + sc.label, 1.3, true}, {"Факт, " + sc.label, 1.3, true},
				{"EAC, " + sc.label, 1.3, true}, {"Отклонение, " + sc.label, 1.3, true}, {"Ставка", 0.9, true}},
			Rows: rows,
		},
		Notes: []string{
			"GTN — бонус за объём, процент от товарооборота по условиям реестра. Порог выполнения уже применён: бренд, не закрывший план, приносит ноль.",
			"OPEX реестра и инвестиции промо сюда не входят — это отдельные показатели.",
		},
	}
}

func effectiveRate(p *float64) string {
	if p == nil {
		return "ставка не определена: планового объёма нет"
	}
	return "эффективная ставка " + percent(p) + " от планового объёма"
}

// ─── Бюджет OPEX реестра ────────────────────────────────────────────────────

func registryOpexPage(s *models.ReportSnapshot) page {
	m := s.Dashboard.Summary
	sc := scaleFor(models.ReportUnitRub, m.RegistryOpexBudgetRub)
	cards := []card{
		{Label: "Бюджет OPEX, " + sc.label + " с НДС", Value: sc.format(m.RegistryOpexBudgetRub), Sub: "согласованная сумма по статьям договора"},
		{Label: "Бюджет OPEX, " + sc.label + " без НДС", Value: sc.format(m.RegistryOpexBudgetRubNet), Sub: "база для сравнения между сетями"},
	}
	rows := make([][]cell, 0, len(s.Dashboard.Quarters))
	for _, q := range s.Dashboard.Quarters {
		rows = append(rows, []cell{txt(fmt.Sprintf("Q%d", q.Quarter)), txt(sc.format(q.Metrics.RegistryOpexBudgetRub)), txt(sc.format(q.Metrics.RegistryOpexBudgetRubNet))})
	}
	return page{
		Block: models.ReportBlockRegistryOpex, Title: "Бюджет OPEX реестра", Subtitle: sc.label,
		Cards: cards,
		Table: &table{
			Columns: []column{{"Квартал", 0.8, false}, {"Бюджет с НДС, " + sc.label, 1.5, true}, {"Бюджет без НДС, " + sc.label, 1.5, true}},
			Rows:    rows,
		},
		Notes: []string{
			"Пары план/факт у бюджета нет: источника факта по договору в портале не существует.",
			"В инвестиции реестра (GTN) бюджет не входит и ставку не меняет.",
		},
	}
}

// ─── Инвестиции промо ───────────────────────────────────────────────────────

func promoInvestmentsPage(s *models.ReportSnapshot) page {
	m := s.Dashboard.Summary
	sc := scaleFor(models.ReportUnitRub, m.PromoInvestmentsGTN.PlanRubNet, m.PromoInvestmentsOPEX.PlanRubNet, m.PromoInvestmentsRub)
	cards := []card{
		{Label: "Промо в срезе", Value: integer(float64(m.PromoCount)),
			Sub: fmt.Sprintf("онлайн %d · оффлайн %d", m.PromoOnlineCount, m.PromoOfflineCount)},
		{Label: "Промо GTN: EAC, " + sc.label + " без НДС", Value: sc.format(m.PromoInvestmentsGTN.EACRubNet),
			Delta:     sc.signed(m.PromoInvestmentsGTN.EACRubNet-m.PromoInvestmentsGTN.PlanRubNet) + " к плану",
			DeltaTone: deltaTone(m.PromoInvestmentsGTN.EACRubNet - m.PromoInvestmentsGTN.PlanRubNet),
			Sub:       "план " + sc.format(m.PromoInvestmentsGTN.PlanRubNet) + " · факт " + sc.format(m.PromoInvestmentsGTN.FactRubNet)},
		{Label: "Промо OPEX: EAC, " + sc.label + " без НДС", Value: sc.format(m.PromoInvestmentsOPEX.EACRubNet),
			Delta:     sc.signed(m.PromoInvestmentsOPEX.EACRubNet-m.PromoInvestmentsOPEX.PlanRubNet) + " к плану",
			DeltaTone: deltaTone(m.PromoInvestmentsOPEX.EACRubNet - m.PromoInvestmentsOPEX.PlanRubNet),
			Sub:       "план " + sc.format(m.PromoInvestmentsOPEX.PlanRubNet) + " · факт " + sc.format(m.PromoInvestmentsOPEX.FactRubNet)},
	}
	rows := make([][]cell, 0, len(s.Dashboard.Quarters))
	for _, q := range s.Dashboard.Quarters {
		qm := q.Metrics
		rows = append(rows, []cell{txt(fmt.Sprintf("Q%d", q.Quarter)), txt(integer(float64(qm.PromoCount))),
			txt(sc.format(qm.PromoInvestmentsGTN.PlanRubNet)), txt(sc.format(qm.PromoInvestmentsGTN.EACRubNet)),
			txt(sc.format(qm.PromoInvestmentsOPEX.PlanRubNet)), txt(sc.format(qm.PromoInvestmentsOPEX.EACRubNet))})
	}
	return page{
		Block: models.ReportBlockPromoInvestments, Title: "Инвестиции промо", Subtitle: sc.label + " без НДС",
		Cards: cards,
		Table: &table{
			Columns: []column{{"Квартал", 0.8, false}, {"Промо", 0.7, true}, {"GTN план, " + sc.label, 1.3, true}, {"GTN EAC, " + sc.label, 1.3, true},
				{"OPEX план, " + sc.label, 1.3, true}, {"OPEX EAC, " + sc.label, 1.3, true}},
			Rows: rows,
		},
		Notes: []string{
			"Тип промо (GTN/OPEX) ведётся в карточке промо. Суммы приведены к базе без НДС ставкой квартала сети.",
			"Факт — только закрытые деньги; EAC — факт, если он есть, иначе план, по каждому промо отдельно. С инвестициями реестра не суммируются.",
		},
	}
}

// ─── Сети по кварталам (приложение) ─────────────────────────────────────────

func networkQuartersPage(s *models.ReportSnapshot) page {
	unit := s.Request.Unit
	u := unitLabel(unit)
	limit := s.Request.TableLimit
	// Приложение — единственное место с точными суммами: здесь шкала не
	// применяется, копейки на месте.
	full := func(rub, units float64) string {
		if unit == models.ReportUnitUnits {
			return integer(units)
		}
		return money(rub)
	}
	seen := map[string]bool{}
	rows := make([][]cell, 0, limit*4)
	totalNetworks := 0
	for _, c := range s.Dashboard.NetworkQuarters {
		if !seen[c.Name] {
			seen[c.Name] = true
			totalNetworks++
		}
		if totalNetworks > limit {
			continue
		}
		m := c.Metrics
		without := txt(integer(float64(m.OpenCellsWithoutForecast)))
		if m.OpenCellsWithoutForecast > 0 {
			without.Tone = toneWarn
		}
		rows = append(rows, []cell{txt(truncate(c.Name, 44)), txt(fmt.Sprintf("Q%d", c.Quarter)),
			txt(full(m.PlanRub, m.PlanUnits)), txt(full(m.FactRub, m.FactUnits)), txt(full(m.EACRub, m.EACUnits)),
			pctCell(m.EACCompletionPct), without})
	}
	shown := totalNetworks
	if shown > limit {
		shown = limit
	}
	return page{
		Block: models.ReportBlockNetworkQuarters, Title: "Сети по кварталам", Subtitle: "точные суммы, " + u,
		Table: &table{
			Columns: []column{{"Сеть", 2.6, false}, {"Кв.", 0.5, false}, {"План, " + u, 1.5, true}, {"Факт, " + u, 1.5, true},
				{"EAC, " + u, 1.5, true}, {"EAC / план", 1.3, true}, {"Без прогноза", 0.9, true}},
			Rows: rows,
		},
		Notes: []string{fmt.Sprintf("Показано сетей: %d из %d, по алфавиту.", shown, totalNetworks)},
	}
}

// ─── Методика ───────────────────────────────────────────────────────────────

func methodologyPage(s *models.ReportSnapshot) page {
	lines := []string{
		"Источник — витрина «Реестр сетей» Analytics Portal; отчёт собран из того же серверного ответа, что видит пользователь на экране, по снимку фильтров на момент формирования.",
		"План — обязательство по контракту (квартальные строки реестра); валовый пул входит целиком. Факт — отгрузки закрытых месяцев из помесячных таблиц. EAC — факт закрытых месяцев плюс официальный прогноз открытых; месяц без прогноза планом не достраивается.",
		"Инвестиции реестра (GTN) — процент от товарооборота по условиям реестра с уже применённым порогом выполнения. Бюджет OPEX реестра — согласованная сумма по статьям договора, без факта. Инвестиции промо — деньги конкретных активностей. Три показателя не суммируются.",
		"Суммы инвестиций сопоставимы между сетями только в базе без НДС; в этой базе считается отклонение.",
		"Числа на страницах округлены до шкалы, указанной в подзаголовке (млрд, млн, тыс.); точные суммы с копейками — в приложении «Сети по кварталам». Прочерк «—» означает отсутствие данных за сопоставимый период и не равен нулю.",
		"Оценка выполнения: зелёный — 100 % и выше, жёлтый — от 90 %, красный — ниже 90 %.",
		unitNote(s.Request.Unit),
		fmt.Sprintf("Версия шаблона: %s. Сформировано: %s, автор: %s.", s.TemplateVersion, s.CreatedAt, s.Owner),
	}
	return page{Block: models.ReportBlockMethodology, Title: "Методика и оговорки", Lines: lines}
}

// splitRows делит строки таблицы на порции для слайдов.
func splitRows(rows [][]cell, perSlide int) [][][]cell {
	if len(rows) == 0 {
		return nil
	}
	var chunks [][][]cell
	for start := 0; start < len(rows); start += perSlide {
		end := start + perSlide
		if end > len(rows) {
			end = len(rows)
		}
		chunks = append(chunks, rows[start:end])
	}
	return chunks
}

// continuationTitle — заголовок продолжения таблицы на следующем слайде.
func continuationTitle(title string, part, total int) string {
	if total <= 1 {
		return title
	}
	return fmt.Sprintf("%s (%d/%d)", strings.TrimSpace(title), part, total)
}
