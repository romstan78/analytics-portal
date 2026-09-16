package reports

import (
	"bytes"
	"fmt"
	"math"
	"strings"

	"codeberg.org/go-pdf/fpdf"

	"backend/models"
)

// ─── PDF ────────────────────────────────────────────────────────────────────
//
// A4 альбомный, страница на блок. Карточки и графики рисуются через холст
// (canvas.go), таблица — здесь: у неё свой перенос на следующую страницу с
// повтором шапки, чего холсту знать не нужно.

const (
	pdfMargin  = 12.0 // мм
	pdfPageW   = 297.0
	pdfPageH   = 210.0
	pdfContent = pdfPageW - 2*pdfMargin
	pdfFooterH = 10.0
	pdfBottom  = pdfPageH - pdfMargin - pdfFooterH
)

// pdfCanvas — холст поверх fpdf.
type pdfCanvas struct{ pdf *fpdf.Fpdf }

func (c pdfCanvas) Rect(b box, fill *rgb, stroke *rgb, strokeW float64) {
	mode := ""
	if fill != nil {
		c.pdf.SetFillColor(int(fill.R), int(fill.G), int(fill.B))
		mode += "F"
	}
	if stroke != nil {
		c.pdf.SetDrawColor(int(stroke.R), int(stroke.G), int(stroke.B))
		c.pdf.SetLineWidth(strokeW)
		mode += "D"
	}
	if mode == "" {
		return
	}
	c.pdf.Rect(b.X, b.Y, b.W, b.H, mode)
}

func (c pdfCanvas) Line(x1, y1, x2, y2 float64, col rgb, w float64) {
	c.pdf.SetDrawColor(int(col.R), int(col.G), int(col.B))
	c.pdf.SetLineWidth(w)
	c.pdf.Line(x1, y1, x2, y2)
}

func (c pdfCanvas) Polyline(pts [][2]float64, col rgb, w float64) {
	c.pdf.SetLineCapStyle("round")
	c.pdf.SetLineJoinStyle("round")
	for i := 1; i < len(pts); i++ {
		c.Line(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], col, w)
	}
}

func (c pdfCanvas) Text(b box, s string, st textStyle) {
	style := ""
	if st.Bold {
		style = "B"
	}
	c.pdf.SetFont("Report", style, st.Size)
	c.pdf.SetTextColor(int(st.Color.R), int(st.Color.G), int(st.Color.B))
	c.pdf.SetXY(b.X, b.Y)
	align := st.Align
	if align == "" {
		align = "L"
	}
	c.pdf.CellFormat(b.W, b.H, s, "", 0, align, false, 0, "")
}

type pdfWriter struct {
	pdf    *fpdf.Fpdf
	canvas pdfCanvas
	y      float64
}

// RenderPDF собирает PDF по снимку.
func RenderPDF(s *models.ReportSnapshot) ([]byte, error) {
	pages, err := BuildPages(s)
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("L", "mm", "A4", "")
	w := &pdfWriter{pdf: pdf, canvas: pdfCanvas{pdf}}
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddUTF8FontFromBytes("Report", "", fontRegular)
	pdf.AddUTF8FontFromBytes("Report", "B", fontBold)
	pdf.SetTitle(s.Title, true)
	pdf.SetAuthor(s.Owner, true)
	pdf.SetCreator("Analytics Portal", true)
	footer := fmt.Sprintf("%s · %s · %s", s.Title, s.Owner, s.CreatedAt)
	pdf.SetFooterFunc(func() {
		w.canvas.Line(pdfMargin, pdfPageH-pdfMargin-5, pdfPageW-pdfMargin, pdfPageH-pdfMargin-5, colorLine, 0.2)
		w.canvas.Text(box{pdfMargin, pdfPageH - pdfMargin - 4.5, pdfContent / 2, 4}, footer, textStyle{Size: 6.5, Color: colorMuted, Align: "L"})
		w.canvas.Text(box{pdfMargin + pdfContent/2, pdfPageH - pdfMargin - 4.5, pdfContent / 2, 4},
			fmt.Sprintf("стр. %d", pdf.PageNo()), textStyle{Size: 6.5, Color: colorMuted, Align: "R"})
	})

	for _, p := range pages {
		w.page(p)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// header рисует заголовок страницы и возвращает y начала содержимого.
func (w *pdfWriter) header(title, subtitle string) {
	w.pdf.AddPage()
	c := w.canvas
	c.Rect(box{pdfMargin, pdfMargin, 8, 1.2}, ptr(colorFact), nil, 0)
	c.Text(box{pdfMargin, pdfMargin + 2.5, pdfContent * 0.7, 8}, title, textStyle{Size: 15, Bold: true, Color: colorInk, Align: "L"})
	if subtitle != "" {
		c.Text(box{pdfMargin + pdfContent*0.7, pdfMargin + 2.5, pdfContent * 0.3, 8}, subtitle, textStyle{Size: 8, Color: colorMuted, Align: "R"})
	}
	c.Line(pdfMargin, pdfMargin+11.5, pdfPageW-pdfMargin, pdfMargin+11.5, colorLine, 0.2)
	w.y = pdfMargin + 15
}

func (w *pdfWriter) page(p page) {
	if p.Block == models.ReportBlockCover {
		w.cover(p)
		return
	}
	w.header(p.Title, p.Subtitle)
	c := w.canvas
	if len(p.Lines) > 0 {
		w.paragraphs(p.Lines, 9.5, colorInk, 5.2)
	}
	if len(p.Cards) > 0 {
		w.y += drawCards(c, pdfMargin, w.y, pdfContent, p.Cards) + 5
	}
	tableH := 0.0
	if p.Table != nil {
		tableH = 6 + float64(len(p.Table.Rows))*5.5
		if p.Table.Total != nil {
			tableH += 5.5
		}
	}
	if p.Chart != nil {
		remaining := pdfBottom - w.y - 8
		h := p.Chart.preferredHeight()
		// Короткая таблица должна поместиться под графиком на той же странице.
		if tableH > 0 && tableH <= 60 {
			h = math.Min(h, remaining-tableH-6)
		}
		h = math.Min(h, remaining)
		if h >= 35 {
			drawChart(c, box{pdfMargin, w.y, pdfContent, h}, p.Chart)
			w.y += h + 5
		}
	}
	if p.Table != nil && len(p.Table.Rows) > 0 {
		w.table(p.Table, p.Title, p.Subtitle)
		w.y += 3
	}
	if len(p.Notes) > 0 {
		w.ensureSpace(4*float64(len(p.Notes))+2, p.Title, p.Subtitle)
		w.paragraphs(p.Notes, 7, colorMuted, 3.8)
	}
}

// paragraphs выводит абзацы с переносом; MultiCell нужен только тексту.
func (w *pdfWriter) paragraphs(lines []string, size float64, col rgb, lineH float64) {
	w.pdf.SetFont("Report", "", size)
	w.pdf.SetTextColor(int(col.R), int(col.G), int(col.B))
	for _, line := range lines {
		w.pdf.SetXY(pdfMargin, w.y)
		w.pdf.MultiCell(pdfContent, lineH, line, "", "L", false)
		w.y = w.pdf.GetY() + 1.5
	}
}

func (w *pdfWriter) cover(p page) {
	w.pdf.AddPage()
	c := w.canvas
	c.Rect(box{0, 0, pdfPageW, 6}, ptr(colorFact), nil, 0)
	c.Text(box{pdfMargin, 34, pdfContent, 6}, "РЕЕСТР СЕТЕЙ · ОТЧЁТ", textStyle{Size: 8, Bold: true, Color: colorMuted, Align: "L"})
	w.pdf.SetFont("Report", "B", 26)
	w.pdf.SetTextColor(int(colorInk.R), int(colorInk.G), int(colorInk.B))
	w.pdf.SetXY(pdfMargin, 42)
	w.pdf.MultiCell(pdfContent, 12, p.Title, "", "L", false)
	c.Text(box{pdfMargin, w.pdf.GetY() + 1, pdfContent, 7}, p.Subtitle, textStyle{Size: 12, Color: colorMuted, Align: "L"})
	y := w.pdf.GetY() + 14
	y += drawCards(c, pdfMargin, y, pdfContent, p.Cards) + 10
	w.y = y
	w.paragraphs(p.Lines, 8.5, colorMuted, 4.6)
}

// ensureSpace начинает новую страницу с тем же заголовком, если места нет.
func (w *pdfWriter) ensureSpace(h float64, title, subtitle string) {
	if w.y+h > pdfBottom {
		w.header(title, subtitle)
	}
}

// table — таблица без вертикальных линий: капитель в шапке, зебра, тонкие
// горизонтали, итог жирным с линией сверху, полоса выполнения в ячейке.
func (w *pdfWriter) table(t *table, title, subtitle string) {
	const rowH, headH = 5.5, 6.0
	c := w.canvas
	total := 0.0
	for _, col := range t.Columns {
		total += col.Weight
	}
	widths := make([]float64, len(t.Columns))
	for i, col := range t.Columns {
		widths[i] = pdfContent * col.Weight / total
	}
	head := func() {
		x := pdfMargin
		for i, col := range t.Columns {
			align := "L"
			if col.Right {
				align = "R"
			}
			c.Text(box{x + 1, w.y, widths[i] - 2, headH}, strings.ToUpper(col.Title), textStyle{Size: 6, Bold: true, Color: colorMuted, Align: align})
			x += widths[i]
		}
		c.Line(pdfMargin, w.y+headH, pdfMargin+pdfContent, w.y+headH, colorMuted, 0.3)
		w.y += headH
	}
	row := func(cells []cell, zebra, bold bool, topLine bool) {
		if zebra {
			c.Rect(box{pdfMargin, w.y, pdfContent, rowH}, ptr(colorZebra), nil, 0)
		}
		if topLine {
			c.Line(pdfMargin, w.y, pdfMargin+pdfContent, w.y, colorInk, 0.35)
		}
		x := pdfMargin
		for i, col := range t.Columns {
			if i >= len(cells) {
				break
			}
			cl := cells[i]
			if cl.Bar >= 0 {
				bw := (widths[i] - 2) * math.Min(cl.Bar, 1.25) / 1.25
				c.Rect(box{x + 1, w.y + 1, bw, rowH - 2}, ptr(colorBar), nil, 0)
			}
			align := "L"
			if col.Right {
				align = "R"
			}
			c.Text(box{x + 1, w.y, widths[i] - 2, rowH}, cl.Text, textStyle{Size: 7.5, Bold: bold || cl.Bold, Color: cl.Tone.color(), Align: align})
			x += widths[i]
		}
		c.Line(pdfMargin, w.y+rowH, pdfMargin+pdfContent, w.y+rowH, colorLine, 0.15)
		w.y += rowH
	}
	// Таблица не начинается впритык к нижнему полю.
	rowsToFit := math.Min(float64(len(t.Rows)), 4)
	w.ensureSpace(headH+rowsToFit*rowH, title, subtitle)
	head()
	for i, r := range t.Rows {
		if w.y+rowH > pdfBottom {
			w.header(title, subtitle)
			head()
		}
		row(r, i%2 == 1, false, false)
	}
	if t.Total != nil {
		if w.y+rowH > pdfBottom {
			w.header(title, subtitle)
			head()
		}
		row(t.Total, false, true, true)
	}
}
