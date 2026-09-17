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
// копирует шаблон и добавляет к нему слайды. Графическая часть блока
// (плитки и график) — снимок секции печатной страницы, снятый Chromium;
// заголовок, примечания и таблицы — нативные, чтобы их можно было править в
// PowerPoint. Блок без снимка получает только текст и таблицу.
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
	// images — растровые части слайда (снимки секций страницы печати);
	// становятся media-частями пакета и связями слайда.
	images []pptxImage
}

type pptxImage struct {
	relID string
	data  []byte
}

// SlideImage — снимок графической части секции печатной страницы для
// слайда: Index — номер блока в запросе, размер в пикселях с учётом 2×.
type SlideImage struct {
	Index         int
	PNG           []byte
	Width, Height int
}

// pptxCanvas — фигуры текущего слайда: прямоугольники, линии, текст,
// картинки и нативные таблицы в миллиметрах слайда.
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

// Picture вставляет снимок в рамку b: масштаб по ширине, при нехватке высоты —
// по высоте с центровкой; возвращает занятую высоту.
func (c pptxCanvas) Picture(b box, img SlideImage) float64 {
	if img.Width <= 0 || img.Height <= 0 || len(img.PNG) == 0 {
		return 0
	}
	w := b.W
	h := w * float64(img.Height) / float64(img.Width)
	x := b.X
	if b.H > 0 && h > b.H {
		h = b.H
		w = h * float64(img.Width) / float64(img.Height)
		x = b.X + (b.W-w)/2
	}
	relID := fmt.Sprintf("rId%d", len(c.sl.images)+2) // rId1 — макет слайда
	c.sl.images = append(c.sl.images, pptxImage{relID: relID, data: img.PNG})
	id := c.sl.id()
	c.sl.shapes = append(c.sl.shapes, fmt.Sprintf(
		`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="Picture %d"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>`+
			`<p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>`+
			`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		id, id, relID, emu(x), emu(b.Y), emu(w), emu(h)))
	return h
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

// RenderPPTX собирает PPTX: графическая часть блока — снимок секции
// печатной страницы (Chromium), заголовок, примечания и таблицы — нативные
// и правятся в PowerPoint. Блок без снимка получает только текст и таблицу.
func RenderPPTX(s *models.ReportSnapshot, images []SlideImage) ([]byte, error) {
	pages, err := BuildPages(s)
	if err != nil {
		return nil, err
	}
	info, err := inspectTemplate(pptxTemplate)
	if err != nil {
		return nil, err
	}
	byIndex := make(map[int]SlideImage, len(images))
	for _, img := range images {
		byIndex[img.Index] = img
	}
	w := &pptxWriter{info: info}
	for i, p := range pages {
		if img, ok := byIndex[i]; ok {
			w.page(p, &img)
		} else {
			w.page(p, nil)
		}
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
	c.Rect(box{slideMargin, 8, 9, 1.3}, ptr(colorAccent), nil, 0)
	c.Text(box{slideMargin, 10, contentW * 0.7, 10}, title, textStyle{Size: 18, Bold: true, Color: colorInk, Align: "L"})
	if subtitle != "" {
		c.Text(box{slideMargin + contentW*0.7, 10, contentW * 0.3, 10}, subtitle, textStyle{Size: 9, Color: colorMuted, Align: "R"})
	}
	c.Line(slideMargin, 21.5, W-slideMargin, 21.5, colorLine, 0.2)
	return c, 25
}

// page раскладывает блок по слайдам. img — снимок графической части блока
// (плитки и график) с печатной страницы; без него остаются текст и таблица.
func (w *pptxWriter) page(p page, img *SlideImage) {
	W, H := w.info.width, w.info.height
	contentW := W - 2*slideMargin
	if p.Block == models.ReportBlockCover {
		sl := w.newSlide()
		c := pptxCanvas{sl}
		c.Rect(box{0, 0, W, 5}, ptr(colorAccent), nil, 0)
		c.Text(box{slideMargin, H * 0.16, contentW, 6}, "РЕЕСТР СЕТЕЙ · ОТЧЁТ", textStyle{Size: 9, Bold: true, Color: colorMuted, Align: "L"})
		sl.shapes = append(sl.shapes, c.textBox(box{slideMargin, H*0.16 + 7, contentW, 22}, []string{p.Title}, textStyle{Size: 30, Bold: true, Color: colorInk, Align: "L"}, "t", true))
		c.Text(box{slideMargin, H*0.16 + 30, contentW, 8}, p.Subtitle, textStyle{Size: 13, Color: colorMuted, Align: "L"})
		y := H*0.16 + 44
		if img != nil {
			y += c.Picture(box{slideMargin, y, contentW, H - y - 30}, *img) + 8
		}
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
	if img != nil {
		// Снимок во всю ширину; короткой таблице оставляется место под ним,
		// длинная уходит на следующие слайды.
		reserve := 0.0
		if tableRows > 0 && tableRows <= 8 {
			reserve = tableH(tableRows) + 4
		}
		y += c.Picture(box{slideMargin, y, contentW, bottom - y - reserve}, *img) + 4
	}
	if tableRows == 0 {
		notes(c)
		return
	}
	if tableH(tableRows) <= bottom-y {
		c.nativeTable(box{slideMargin, y, contentW, 0}, p.Table, p.Table.Rows, p.Table.Total)
		notes(c)
		return
	}
	// Таблица порциями по слайдам. Слайд со снимком остаётся ему, таблица
	// начинается со следующего; без снимка первая порция идёт на текущий.
	chunks := splitRows(p.Table.Rows, maxTableRowsPerSlide)
	if img != nil {
		notes(c)
	}
	for i, rows := range chunks {
		if i > 0 || img != nil {
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
			if w.hasImages() && !strings.Contains(text, `Extension="png"`) {
				text = strings.Replace(text, `<Default `, `<Default Extension="png" ContentType="image/png"/><Default `, 1)
			}
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

	mediaN := 0
	for i, sl := range w.slides {
		n := i + 1
		var imageRels strings.Builder
		for _, img := range sl.images {
			mediaN++
			name := fmt.Sprintf("image%d.png", mediaN)
			if err := add("ppt/media/"+name, img.data); err != nil {
				return nil, err
			}
			fmt.Fprintf(&imageRels, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/%s"/>`, img.relID, name)
		}
		relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			fmt.Sprintf(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="%s"/>`, w.info.layoutTarget) +
			imageRels.String() +
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

func (w *pptxWriter) hasImages() bool {
	for _, sl := range w.slides {
		if len(sl.images) > 0 {
			return true
		}
	}
	return false
}

func slideXML(shapes []string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		strings.Join(shapes, "") +
		`</p:spTree></p:cSld><a:clrMapOvr><a:masterClrMapping/></a:clrMapOvr></p:sld>`
}
