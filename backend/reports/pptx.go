package reports

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"backend/models"
)

// ─── PPTX ───────────────────────────────────────────────────────────────────
//
// Зрелой Go-библиотеки для PPTX нет, а формат — это zip с XML. Писатель
// копирует шаблон и добавляет к нему слайды. Карточки и графики — те же
// примитивы холста, что и в PDF, только фигурами слайда: прямоугольники,
// соединительные линии, путь custGeom для ломаных, текстовые поля.
// Таблицы — нативные, чтобы их можно было править в PowerPoint.
//
// Правятся три служебных файла — [Content_Types].xml, presentation.xml и его
// rels; остальное из шаблона идёт как есть, поэтому корпоративный шаблон
// подменяет пустой без изменения кода. Макет слайда берётся из шаблона:
// пустой ищется по имени, иначе — с наименьшим числом placeholder'ов.

const emuPerMM = 36000

type pptxTemplateInfo struct {
	width, height float64 // мм
	layoutTarget  string  // ../slideLayouts/slideLayoutN.xml
	maxSlideID    int
	maxRelID      int
}

type pptxSlide struct {
	shapes []string
	nextID int
}

// pptxCanvas — холст, пишущий фигуры в текущий слайд.
type pptxCanvas struct{ sl *pptxSlide }

func (s *pptxSlide) id() int {
	s.nextID++
	return s.nextID
}

func emu(mm float64) int { return int(math.Round(mm * emuPerMM)) }

func fillXML(c *rgb) string {
	if c == nil {
		return `<a:noFill/>`
	}
	return `<a:solidFill><a:srgbClr val="` + c.hex() + `"/></a:solidFill>`
}

func lineXML(c *rgb, w float64) string {
	if c == nil {
		return `<a:ln><a:noFill/></a:ln>`
	}
	return fmt.Sprintf(`<a:ln w="%d" cap="rnd"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:round/></a:ln>`, emu(math.Max(w, 0.1)), c.hex())
}

func (c pptxCanvas) Rect(b box, fill *rgb, stroke *rgb, strokeW float64) {
	id := c.sl.id()
	c.sl.shapes = append(c.sl.shapes, fmt.Sprintf(
		`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Rect %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>`+
			`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>%s%s</p:spPr>`+
			`<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody></p:sp>`,
		id, id, emu(b.X), emu(b.Y), emu(math.Max(b.W, 0.05)), emu(math.Max(b.H, 0.05)), fillXML(fill), lineXML(stroke, strokeW)))
}

func (c pptxCanvas) Line(x1, y1, x2, y2 float64, col rgb, w float64) {
	id := c.sl.id()
	flip := ""
	if x2 < x1 {
		flip += ` flipH="1"`
	}
	if y2 < y1 {
		flip += ` flipV="1"`
	}
	c.sl.shapes = append(c.sl.shapes, fmt.Sprintf(
		`<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="%d" name="Line %d"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>`+
			`<p:spPr><a:xfrm%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="line"><a:avLst/></a:prstGeom>%s</p:spPr></p:cxnSp>`,
		id, id, flip, emu(math.Min(x1, x2)), emu(math.Min(y1, y2)), emu(math.Abs(x2-x1)), emu(math.Abs(y2-y1)), lineXML(&col, w)))
}

func (c pptxCanvas) Polyline(pts [][2]float64, col rgb, w float64) {
	if len(pts) < 2 {
		return
	}
	minX, minY, maxX, maxY := pts[0][0], pts[0][1], pts[0][0], pts[0][1]
	for _, p := range pts {
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	bw, bh := math.Max(maxX-minX, 0.1), math.Max(maxY-minY, 0.1)
	var path strings.Builder
	for i, p := range pts {
		tag := "lnTo"
		if i == 0 {
			tag = "moveTo"
		}
		fmt.Fprintf(&path, `<a:%s><a:pt x="%d" y="%d"/></a:%s>`, tag, emu(p[0]-minX), emu(p[1]-minY), tag)
	}
	id := c.sl.id()
	c.sl.shapes = append(c.sl.shapes, fmt.Sprintf(
		`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Path %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>`+
			`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`+
			`<a:custGeom><a:avLst/><a:gdLst/><a:ahLst/><a:cxnLst/><a:rect l="0" t="0" r="r" b="b"/>`+
			`<a:pathLst><a:path w="%d" h="%d" fill="none">%s</a:path></a:pathLst></a:custGeom><a:noFill/>%s</p:spPr>`+
			`<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody></p:sp>`,
		id, id, emu(minX), emu(minY), emu(bw), emu(bh), emu(bw), emu(bh), path.String(), lineXML(&col, w)))
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

func runXML(text string, size float64, bold bool, col rgb) string {
	b := ""
	if bold {
		b = ` b="1"`
	}
	return fmt.Sprintf(`<a:r><a:rPr lang="ru-RU" sz="%d"%s dirty="0"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="%s"/></a:rPr><a:t>%s</a:t></a:r>`,
		int(math.Round(size*100)), b, col.hex(), fontFamily, xmlEscape(text))
}

func alignXML(align string) string {
	switch align {
	case "R":
		return `<a:pPr algn="r"/>`
	case "C":
		return `<a:pPr algn="ctr"/>`
	}
	return `<a:pPr algn="l"/>`
}

// Text — текстовое поле без полей и без автоподбора: рамка задана холстом.
func (c pptxCanvas) Text(b box, s string, st textStyle) {
	c.sl.shapes = append(c.sl.shapes, c.textBox(b, []string{s}, st, "ctr", false))
}

func (c pptxCanvas) textBox(b box, lines []string, st textStyle, anchor string, autofit bool) string {
	id := c.sl.id()
	var paras strings.Builder
	for _, line := range lines {
		paras.WriteString(`<a:p>` + alignXML(st.Align) + runXML(line, st.Size, st.Bold, st.Color) + `</a:p>`)
	}
	fit := `<a:noAutofit/>`
	if autofit {
		fit = `<a:normAutofit/>`
	}
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Text %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`+
		`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr>`+
		`<p:txBody><a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" rtlCol="0" anchor="%s">%s</a:bodyPr><a:lstStyle/>%s</p:txBody></p:sp>`,
		id, id, emu(b.X), emu(b.Y), emu(math.Max(b.W, 0.5)), emu(math.Max(b.H, 0.5)), anchor, fit, paras.String())
}

// ─── Нативная таблица ───────────────────────────────────────────────────────

func borderXML(tag string, col *rgb, w float64) string {
	if col == nil {
		return `<a:` + tag + `><a:noFill/></a:` + tag + `>`
	}
	return fmt.Sprintf(`<a:%s w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:%s>`, tag, emu(w), col.hex(), tag)
}

// nativeTable — таблица PowerPoint без сетки: капитель в шапке, зебра,
// тонкие горизонтали, итог жирным с линией сверху, цвет оценки в тексте.
func (c pptxCanvas) nativeTable(b box, t *table, rows [][]cell, total []cell) {
	const rowH, headH = 6.0, 6.0
	totalW := 0.0
	for _, col := range t.Columns {
		totalW += col.Weight
	}
	var sb strings.Builder
	id := c.sl.id()
	nRows := len(rows) + 1
	if total != nil {
		nRows++
	}
	fmt.Fprintf(&sb, `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Table %d"/><p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`, id, id)
	fmt.Fprintf(&sb, `<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`, emu(b.X), emu(b.Y), emu(b.W), emu(headH+rowH*float64(nRows-1)))
	sb.WriteString(`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/><a:tblGrid>`)
	for _, col := range t.Columns {
		fmt.Fprintf(&sb, `<a:gridCol w="%d"/>`, emu(b.W*col.Weight/totalW))
	}
	sb.WriteString(`</a:tblGrid>`)

	writeRow := func(cells []cell, h float64, style func(i int) (text string, size float64, bold bool, col rgb, fill *rgb, top *rgb, topW float64, bottom *rgb, bottomW float64)) {
		fmt.Fprintf(&sb, `<a:tr h="%d">`, emu(h))
		for i, colDef := range t.Columns {
			text, size, bold, col, fill, top, topW, bottom, bottomW := style(i)
			_ = cells
			fmt.Fprintf(&sb, `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p>%s%s</a:p></a:txBody>`, alignXML(map[bool]string{true: "R", false: "L"}[colDef.Right]), runXML(text, size, bold, col))
			sb.WriteString(`<a:tcPr marL="36000" marR="36000" marT="0" marB="0" anchor="ctr">`)
			sb.WriteString(borderXML("lnL", nil, 0) + borderXML("lnR", nil, 0) + borderXML("lnT", top, topW) + borderXML("lnB", bottom, bottomW))
			if fill != nil {
				sb.WriteString(fillXML(fill))
			}
			sb.WriteString(`</a:tcPr></a:tc>`)
		}
		sb.WriteString(`</a:tr>`)
	}
	writeRow(nil, headH, func(i int) (string, float64, bool, rgb, *rgb, *rgb, float64, *rgb, float64) {
		return strings.ToUpper(t.Columns[i].Title), 6.5, true, colorMuted, nil, nil, 0, ptr(colorMuted), 0.3
	})
	for r, cells := range rows {
		var fill *rgb
		if r%2 == 1 {
			fill = ptr(colorZebra)
		}
		writeRow(cells, rowH, func(i int) (string, float64, bool, rgb, *rgb, *rgb, float64, *rgb, float64) {
			cl := cell{Text: "", Bar: -1}
			if i < len(cells) {
				cl = cells[i]
			}
			return cl.Text, 8, cl.Bold, cl.Tone.color(), fill, nil, 0, ptr(colorLine), 0.15
		})
	}
	if total != nil {
		writeRow(total, rowH, func(i int) (string, float64, bool, rgb, *rgb, *rgb, float64, *rgb, float64) {
			cl := cell{Text: "", Bar: -1}
			if i < len(total) {
				cl = total[i]
			}
			return cl.Text, 8, true, cl.Tone.color(), nil, ptr(colorInk), 0.35, nil, 0
		})
	}
	sb.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	c.sl.shapes = append(c.sl.shapes, sb.String())
}

// ─── Раскладка ──────────────────────────────────────────────────────────────

type pptxWriter struct {
	info   pptxTemplateInfo
	slides []*pptxSlide
}

// RenderPPTX собирает PPTX по снимку.
func RenderPPTX(s *models.ReportSnapshot) ([]byte, error) {
	pages, err := BuildPages(s)
	if err != nil {
		return nil, err
	}
	info, err := inspectTemplate(pptxTemplate)
	if err != nil {
		return nil, err
	}
	w := &pptxWriter{info: info}
	for _, p := range pages {
		w.page(p)
	}
	return w.write(pptxTemplate)
}

func (w *pptxWriter) newSlide() *pptxSlide {
	sl := &pptxSlide{nextID: 1}
	w.slides = append(w.slides, sl)
	return sl
}

const slideMargin = 12.0

// slideHeader рисует заголовок и возвращает холст и y начала содержимого.
func (w *pptxWriter) slideHeader(title, subtitle string) (pptxCanvas, float64) {
	sl := w.newSlide()
	c := pptxCanvas{sl}
	W := w.info.width
	contentW := W - 2*slideMargin
	c.Rect(box{slideMargin, 8, 9, 1.3}, ptr(colorFact), nil, 0)
	c.Text(box{slideMargin, 10, contentW * 0.7, 10}, title, textStyle{Size: 18, Bold: true, Color: colorInk, Align: "L"})
	if subtitle != "" {
		c.Text(box{slideMargin + contentW*0.7, 10, contentW * 0.3, 10}, subtitle, textStyle{Size: 9, Color: colorMuted, Align: "R"})
	}
	c.Line(slideMargin, 21.5, W-slideMargin, 21.5, colorLine, 0.2)
	return c, 25
}

func (w *pptxWriter) page(p page) {
	W, H := w.info.width, w.info.height
	contentW := W - 2*slideMargin
	if p.Block == models.ReportBlockCover {
		sl := w.newSlide()
		c := pptxCanvas{sl}
		c.Rect(box{0, 0, W, 5}, ptr(colorFact), nil, 0)
		c.Text(box{slideMargin, H * 0.16, contentW, 6}, "РЕЕСТР СЕТЕЙ · ОТЧЁТ", textStyle{Size: 9, Bold: true, Color: colorMuted, Align: "L"})
		sl.shapes = append(sl.shapes, c.textBox(box{slideMargin, H*0.16 + 7, contentW, 22}, []string{p.Title}, textStyle{Size: 30, Bold: true, Color: colorInk, Align: "L"}, "t", true))
		c.Text(box{slideMargin, H*0.16 + 30, contentW, 8}, p.Subtitle, textStyle{Size: 13, Color: colorMuted, Align: "L"})
		y := H*0.16 + 44
		y += drawCards(c, slideMargin, y, contentW, p.Cards) + 8
		sl.shapes = append(sl.shapes, c.textBox(box{slideMargin, y, contentW, H - y - 10}, p.Lines, textStyle{Size: 9.5, Color: colorMuted, Align: "L"}, "t", true))
		return
	}

	c, y := w.slideHeader(p.Title, p.Subtitle)
	notesH := 0.0
	if len(p.Notes) > 0 {
		notesH = math.Min(4.5*float64(len(p.Notes))+1, 14)
	}
	bottom := H - 8 - notesH
	notes := func(c pptxCanvas) {
		if notesH > 0 {
			c.sl.shapes = append(c.sl.shapes, c.textBox(box{slideMargin, bottom + 1, contentW, notesH}, p.Notes,
				textStyle{Size: 7.5, Color: colorMuted, Align: "L"}, "t", true))
		}
	}
	if len(p.Lines) > 0 {
		c.sl.shapes = append(c.sl.shapes, c.textBox(box{slideMargin, y, contentW, bottom - y}, p.Lines,
			textStyle{Size: 11, Color: colorInk, Align: "L"}, "t", true))
		notes(c)
		return
	}
	if len(p.Cards) > 0 {
		y += drawCards(c, slideMargin, y, contentW, p.Cards) + 5
	}
	tableRows := 0
	if p.Table != nil {
		tableRows = len(p.Table.Rows)
	}
	tableH := func(rows int) float64 {
		h := 6 + 6*float64(rows)
		if p.Table != nil && p.Table.Total != nil {
			h += 6
		}
		return h
	}
	remaining := bottom - y
	switch {
	case p.Chart != nil && tableRows > 0 && tableRows <= 8 && tableH(tableRows) <= remaining:
		// График слева, короткая таблица справа.
		chartW := contentW * 0.56
		drawChart(c, box{slideMargin, y, chartW, math.Min(p.Chart.preferredHeight()+10, remaining)}, p.Chart)
		c.nativeTable(box{slideMargin + chartW + 6, y + 2, contentW - chartW - 6, 0}, p.Table, p.Table.Rows, p.Table.Total)
		notes(c)
		return
	case p.Chart != nil:
		drawChart(c, box{slideMargin, y, contentW, math.Min(p.Chart.preferredHeight()+12, remaining)}, p.Chart)
		notes(c)
		if tableRows == 0 {
			return
		}
		c, y = w.slideHeader(p.Title, p.Subtitle)
	case tableRows == 0:
		notes(c)
		return
	}
	// Таблица во всю ширину, порциями по слайдам.
	perSlide := maxTableRowsPerSlide
	if len(p.Cards) > 0 {
		perSlide = int((bottom - y - 6) / 6)
	}
	chunks := splitRows(p.Table.Rows, perSlide)
	for i, rows := range chunks {
		if i > 0 {
			c, y = w.slideHeader(continuationTitle(p.Title, i+1, len(chunks)), p.Subtitle)
		} else if len(chunks) > 1 {
			// Первый слайд уже создан: заменяем заголовок нумерацией.
			c.sl.shapes[1] = c.textBox(box{slideMargin, 10, contentW * 0.7, 10}, []string{continuationTitle(p.Title, 1, len(chunks))},
				textStyle{Size: 18, Bold: true, Color: colorInk, Align: "L"}, "ctr", false)
		}
		var total []cell
		if i == len(chunks)-1 {
			total = p.Table.Total
		}
		c.nativeTable(box{slideMargin, y, contentW, 0}, p.Table, rows, total)
		notes(c)
	}
}

// ─── Шаблон ─────────────────────────────────────────────────────────────────

var (
	reSlideSize = regexp.MustCompile(`<p:sldSz[^>]*\bcx="(\d+)"[^>]*\bcy="(\d+)"`)
	reSlideID   = regexp.MustCompile(`<p:sldId\b[^>]*\bid="(\d+)"`)
	reRelID     = regexp.MustCompile(`\bId="rId(\d+)"`)
	reLayoutNum = regexp.MustCompile(`^ppt/slideLayouts/slideLayout(\d+)\.xml$`)
	reCSldName  = regexp.MustCompile(`<p:cSld\b[^>]*\bname="([^"]*)"`)
	rePh        = regexp.MustCompile(`<p:ph\b`)
)

func inspectTemplate(tpl []byte) (pptxTemplateInfo, error) {
	zr, err := zip.NewReader(bytes.NewReader(tpl), int64(len(tpl)))
	if err != nil {
		return pptxTemplateInfo{}, fmt.Errorf("шаблон PPTX: %w", err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return pptxTemplateInfo{}, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return pptxTemplateInfo{}, err
		}
		files[f.Name] = data
	}
	pres, ok := files["ppt/presentation.xml"]
	if !ok {
		return pptxTemplateInfo{}, fmt.Errorf("шаблон PPTX: нет presentation.xml")
	}
	info := pptxTemplateInfo{width: 13.333 * 25.4, height: 7.5 * 25.4}
	if m := reSlideSize.FindSubmatch(pres); m != nil {
		cx, _ := strconv.Atoi(string(m[1]))
		cy, _ := strconv.Atoi(string(m[2]))
		if cx > 0 && cy > 0 {
			info.width, info.height = float64(cx)/emuPerMM, float64(cy)/emuPerMM
		}
	}
	for _, m := range reSlideID.FindAllSubmatch(pres, -1) {
		if id, _ := strconv.Atoi(string(m[1])); id > info.maxSlideID {
			info.maxSlideID = id
		}
	}
	if info.maxSlideID < 255 {
		info.maxSlideID = 255
	}
	for _, m := range reRelID.FindAllSubmatch(files["ppt/_rels/presentation.xml.rels"], -1) {
		if id, _ := strconv.Atoi(string(m[1])); id > info.maxRelID {
			info.maxRelID = id
		}
	}
	type layout struct {
		num, placeholders int
		name              string
	}
	var layouts []layout
	for name, data := range files {
		m := reLayoutNum.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		l := layout{num: num, placeholders: len(rePh.FindAll(data, -1))}
		if nm := reCSldName.FindSubmatch(data); nm != nil {
			l.name = string(nm[1])
		}
		layouts = append(layouts, l)
	}
	if len(layouts) == 0 {
		return pptxTemplateInfo{}, fmt.Errorf("шаблон PPTX: нет макетов слайдов")
	}
	sort.Slice(layouts, func(i, j int) bool {
		bi, bj := isBlankName(layouts[i].name), isBlankName(layouts[j].name)
		if bi != bj {
			return bi
		}
		if layouts[i].placeholders != layouts[j].placeholders {
			return layouts[i].placeholders < layouts[j].placeholders
		}
		return layouts[i].num < layouts[j].num
	})
	info.layoutTarget = fmt.Sprintf("../slideLayouts/slideLayout%d.xml", layouts[0].num)
	return info, nil
}

func isBlankName(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "blank") || strings.Contains(n, "пуст")
}

// ─── Сборка файла ───────────────────────────────────────────────────────────

func (w *pptxWriter) write(tpl []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(tpl), int64(len(tpl)))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	add := func(name string, data []byte) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}

	var overrides, rels, ids strings.Builder
	for i := range w.slides {
		n := i + 1
		rID := w.info.maxRelID + n
		fmt.Fprintf(&overrides, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, n)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, rID, n)
		fmt.Fprintf(&ids, `<p:sldId id="%d" r:id="rId%d"/>`, w.info.maxSlideID+n, rID)
	}

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		text := string(data)
		switch f.Name {
		case "[Content_Types].xml":
			text = strings.Replace(text, `</Types>`, overrides.String()+`</Types>`, 1)
		case "ppt/_rels/presentation.xml.rels":
			text = strings.Replace(text, `</Relationships>`, rels.String()+`</Relationships>`, 1)
		case "ppt/presentation.xml":
			if strings.Contains(text, "<p:sldIdLst>") {
				text = strings.Replace(text, "</p:sldIdLst>", ids.String()+"</p:sldIdLst>", 1)
			} else if strings.Contains(text, "<p:sldIdLst/>") {
				text = strings.Replace(text, "<p:sldIdLst/>", "<p:sldIdLst>"+ids.String()+"</p:sldIdLst>", 1)
			} else {
				text = strings.Replace(text, "</p:sldMasterIdLst>", "</p:sldMasterIdLst><p:sldIdLst>"+ids.String()+"</p:sldIdLst>", 1)
			}
		}
		if err := add(f.Name, []byte(text)); err != nil {
			return nil, err
		}
	}

	for i, sl := range w.slides {
		n := i + 1
		relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			fmt.Sprintf(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="%s"/>`, w.info.layoutTarget) +
			`</Relationships>`
		if err := add(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", n), []byte(relsXML)); err != nil {
			return nil, err
		}
		if err := add(fmt.Sprintf("ppt/slides/slide%d.xml", n), []byte(slideXML(sl.shapes))); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func slideXML(shapes []string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		strings.Join(shapes, "") +
		`</p:spTree></p:cSld><a:clrMapOvr><a:masterClrMapping/></a:clrMapOvr></p:sld>`
}
