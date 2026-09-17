package reports

import "backend/models"

// ─── Печатная модель для HTML-рендера ───────────────────────────────────────
//
// Страница печати во фронтенде получает те же страницы, что и векторный
// рендер: BuildPages один раз решает, какие карточки, колонки, шкалы и
// примечания стоят в блоке, а здесь это только переводится в JSON-контракт
// (models.ReportPrint). Так PDF из Chromium и нативные таблицы PPTX собраны
// из одного источника.

// BuildPrint переводит снимок в печатную модель.
func BuildPrint(s *models.ReportSnapshot) (models.ReportPrint, error) {
	pages, err := BuildPages(s)
	if err != nil {
		return models.ReportPrint{}, err
	}
	out := models.ReportPrint{
		Title:        s.Title,
		Owner:        s.Owner,
		CreatedAt:    s.CreatedAt,
		FilterLabels: append([]string{}, s.FilterLabels...),
		Unit:         s.Request.Unit,
		Pages:        make([]models.ReportPrintPage, 0, len(pages)),
	}
	for _, p := range pages {
		out.Pages = append(out.Pages, printPage(p))
	}
	return out, nil
}

func printPage(p page) models.ReportPrintPage {
	out := models.ReportPrintPage{
		Block: p.Block, Title: p.Title, Subtitle: p.Subtitle,
		Lines: p.Lines, Notes: p.Notes,
	}
	for _, c := range p.Cards {
		out.Cards = append(out.Cards, models.ReportPrintCard{
			Label: c.Label, Value: c.Value, Delta: c.Delta, DeltaTone: printTone(c.DeltaTone),
			Sub: c.Sub, Spark: c.Spark,
		})
	}
	if p.Chart != nil {
		out.Chart = printChart(p.Chart)
	}
	if p.Table != nil {
		out.Table = printTable(p.Table)
	}
	return out
}

func printChart(ch *chartSpec) *models.ReportPrintChart {
	out := &models.ReportPrintChart{
		Title: ch.Title, Subtitle: ch.Subtitle,
		Scale: models.ReportPrintScale{Div: ch.Scale.div, Label: ch.Scale.label, Digits: ch.Scale.digits},
	}
	for _, b := range ch.Bullet {
		out.Bullet = append(out.Bullet, models.ReportPrintBullet{
			Label: b.Label, Sub: b.Sub, Plan: b.Plan, Fact: b.Fact, EAC: b.EAC,
			PctLabel: percent(b.Pct), Tone: printTone(completionTone(b.Pct)),
		})
	}
	for _, m := range ch.Months {
		out.Months = append(out.Months, models.ReportPrintMonth{
			Label: m.Label, Plan: m.Plan, Fact: m.Fact, EAC: m.EAC, Prev: m.Prev, Closed: m.Closed,
		})
	}
	for _, st := range ch.Steps {
		out.Steps = append(out.Steps, models.ReportPrintStep{Label: st.Label, Value: st.Value, Total: st.Total})
	}
	return out
}

func printTable(t *table) *models.ReportPrintTable {
	out := &models.ReportPrintTable{
		Columns: make([]models.ReportPrintColumn, 0, len(t.Columns)),
		Rows:    make([][]models.ReportPrintCell, 0, len(t.Rows)),
	}
	for _, c := range t.Columns {
		out.Columns = append(out.Columns, models.ReportPrintColumn{Title: c.Title, Weight: c.Weight, Right: c.Right})
	}
	for _, row := range t.Rows {
		out.Rows = append(out.Rows, printCells(row))
	}
	if len(t.Total) > 0 {
		out.Total = printCells(t.Total)
	}
	return out
}

func printCells(row []cell) []models.ReportPrintCell {
	out := make([]models.ReportPrintCell, 0, len(row))
	for _, c := range row {
		pc := models.ReportPrintCell{Text: c.Text, Tone: printTone(c.Tone), Bold: c.Bold}
		if c.Bar >= 0 {
			bar := c.Bar
			pc.Bar = &bar
		}
		out = append(out, pc)
	}
	return out
}

func printTone(t tone) string {
	switch t {
	case toneGood:
		return models.ReportToneGood
	case toneWarn:
		return models.ReportToneWarn
	case toneBad:
		return models.ReportToneBad
	}
	return models.ReportToneNeutral
}
