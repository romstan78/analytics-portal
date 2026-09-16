package reports

import (
	"math"
)

// ─── Холст ──────────────────────────────────────────────────────────────────
//
// Графики и карточки рисуются примитивами — прямоугольник, линия, ломаная,
// текст — в миллиметрах страницы. PDF выполняет их через fpdf, PPTX —
// фигурами слайда. Рисунок при этом один: bullet-график в презентации и в
// PDF собран одним и тем же кодом, а не двумя библиотеками с разными
// умолчаниями.

type textStyle struct {
	Size  float64 // pt
	Bold  bool
	Color rgb
	Align string // L, C, R
}

type box struct{ X, Y, W, H float64 }

func (b box) inset(d float64) box { return box{b.X + d, b.Y + d, b.W - 2*d, b.H - 2*d} }

type canvas interface {
	// Rect — прямоугольник; fill или stroke могут быть nil.
	Rect(b box, fill *rgb, stroke *rgb, strokeW float64)
	Line(x1, y1, x2, y2 float64, c rgb, w float64)
	Polyline(pts [][2]float64, c rgb, w float64)
	// Text — строка внутри рамки, по вертикали по центру; переносов нет.
	Text(b box, s string, st textStyle)
}

func ptr(c rgb) *rgb { return &c }

var (
	styleAxis   = textStyle{Size: 7, Color: colorMuted, Align: "R"}
	styleLabel  = textStyle{Size: 7.5, Color: colorInk, Align: "L"}
	styleValue  = textStyle{Size: 7, Color: colorMuted, Align: "L"}
	styleLegend = textStyle{Size: 7, Color: colorMuted, Align: "L"}
	styleChartT = textStyle{Size: 9.5, Bold: true, Color: colorInk, Align: "L"}
	styleChartS = textStyle{Size: 7.5, Color: colorMuted, Align: "L"}
)

// ─── Легенда ────────────────────────────────────────────────────────────────

type legendItem struct {
	Label string
	Color rgb
	Kind  string // square | line | marker
}

// drawLegend рисует подписи в строку слева направо, возвращает занятую ширину.
func drawLegend(c canvas, x, y float64, items []legendItem) float64 {
	const h = 4.0
	cur := x
	for _, it := range items {
		switch it.Kind {
		case "line":
			c.Line(cur, y+h/2, cur+4, y+h/2, it.Color, 0.5)
		case "marker":
			c.Rect(box{cur + 1.5, y + 0.3, 1, h - 0.6}, ptr(it.Color), nil, 0)
		default:
			c.Rect(box{cur, y + 0.5, 4, h - 1}, ptr(it.Color), nil, 0)
		}
		cur += 5.5
		w := float64(len([]rune(it.Label)))*1.5 + 3
		c.Text(box{cur, y, w, h}, it.Label, styleLegend)
		cur += w + 3
	}
	return cur - x
}

// chartFrame рисует заголовок и подзаголовок графика и возвращает область
// под сам рисунок.
func chartFrame(c canvas, b box, title, subtitle string) box {
	c.Text(box{b.X, b.Y, b.W, 5}, title, styleChartT)
	top := b.Y + 5
	if subtitle != "" {
		c.Text(box{b.X, top, b.W, 4}, subtitle, styleChartS)
		top += 4
	}
	return box{b.X, top + 1, b.W, b.H - (top + 1 - b.Y)}
}

// ─── Bullet-график ──────────────────────────────────────────────────────────
//
// Строка — одна сущность (квартал, сеть, бренд): серая полоса — план,
// синяя внутри — факт, янтарная риска — ожидаемый итог. Все три на одной
// шкале, ширина полосы плана — доля от максимума среди строк, поэтому
// разрыв виден без чтения чисел.

type bulletRow struct {
	Label           string
	Plan, Fact, EAC float64
	Pct             *float64 // EAC / план
	Sub             string   // подпись под названием: КАМ, тип
}

func drawBullet(c canvas, b box, rows []bulletRow, sc scale) {
	if len(rows) == 0 {
		return
	}
	legendH := 5.0
	drawLegend(c, b.X, b.Y, []legendItem{
		{"План", colorPlan, "square"}, {"Факт", colorFact, "square"}, {"EAC", colorEAC, "marker"},
	})
	area := box{b.X, b.Y + legendH, b.W, b.H - legendH}

	labelW := math.Max(30, area.W*0.24)
	valueW := math.Max(34, area.W*0.22)
	barX := area.X + labelW + 2
	barW := area.W - labelW - valueW - 4
	rowH := area.H / float64(len(rows))
	if rowH > 16 {
		rowH = 16
	}
	max := 0.0
	for _, r := range rows {
		max = math.Max(max, math.Max(r.Plan, math.Max(r.Fact, r.EAC)))
	}
	if max <= 0 {
		max = 1
	}
	scaleX := func(v float64) float64 { return barW * math.Max(0, v) / max }

	for i, r := range rows {
		y := area.Y + float64(i)*rowH
		// Разделитель между строками.
		if i > 0 {
			c.Line(area.X, y, area.X+area.W, y, colorLine, 0.2)
		}
		// Название и подпись.
		if r.Sub != "" {
			c.Text(box{area.X, y + rowH*0.12, labelW, rowH * 0.42}, truncate(r.Label, 28), styleLabel)
			c.Text(box{area.X, y + rowH*0.52, labelW, rowH * 0.36}, truncate(r.Sub, 30), styleValue)
		} else {
			c.Text(box{area.X, y, labelW, rowH}, truncate(r.Label, 28), styleLabel)
		}
		// Полосы.
		planH := rowH * 0.56
		factH := rowH * 0.26
		c.Rect(box{barX, y + (rowH-planH)/2, scaleX(r.Plan), planH}, ptr(colorPlan), nil, 0)
		c.Rect(box{barX, y + (rowH-factH)/2, scaleX(r.Fact), factH}, ptr(colorFact), nil, 0)
		markH := rowH * 0.7
		c.Rect(box{barX + scaleX(r.EAC) - 0.5, y + (rowH-markH)/2, 1, markH}, ptr(colorEAC), nil, 0)
		// Значения: процент крупно и цветом, суммы мелко.
		vx := barX + barW + 3
		c.Text(box{vx, y + rowH*0.08, valueW, rowH * 0.5},
			percent(r.Pct), textStyle{Size: 8.5, Bold: true, Color: completionTone(r.Pct).color(), Align: "R"})
		c.Text(box{vx, y + rowH*0.52, valueW, rowH * 0.4},
			sc.format(r.Fact)+" / "+sc.format(r.EAC)+" из "+sc.format(r.Plan), textStyle{Size: 6.5, Color: colorMuted, Align: "R"})
	}
}

// ─── Месячный график ────────────────────────────────────────────────────────
//
// Столбец месяца: факт сплошной, прогнозная часть EAC (сверх факта) —
// светлым; тёмная риска — план месяца; прошлый год — линия. Так один столбец
// отвечает и «сколько уже есть», и «сколько ждём», и «сколько обещали».

type monthPoint struct {
	Label           string
	Plan, Fact, EAC float64
	Prev            *float64
	Closed          bool
}

func drawMonths(c canvas, b box, months []monthPoint, sc scale) {
	if len(months) == 0 {
		return
	}
	drawLegend(c, b.X, b.Y, []legendItem{
		{"Факт", colorFact, "square"}, {"Прогноз до EAC", colorEACBg, "square"},
		{"План месяца", colorInk, "line"}, {"Прошлый год", colorPrev, "line"},
	})
	axisW := 14.0
	plot := box{b.X + axisW, b.Y + 8, b.W - axisW, b.H - 8 - 6}
	max := 0.0
	for _, m := range months {
		max = math.Max(max, math.Max(m.Plan, m.EAC))
		if m.Prev != nil {
			max = math.Max(max, *m.Prev)
		}
	}
	if max <= 0 {
		max = 1
	}
	max *= 1.12
	yOf := func(v float64) float64 { return plot.Y + plot.H - plot.H*math.Max(0, v)/max }

	// Сетка: четыре горизонтали с подписями в шкале.
	for i := 0; i <= 4; i++ {
		v := max / 1.12 * float64(i) / 4
		y := yOf(v)
		c.Line(plot.X, y, plot.X+plot.W, y, colorLine, 0.2)
		c.Text(box{b.X, y - 2.5, axisW - 2, 5}, sc.format(v), styleAxis)
	}
	slot := plot.W / float64(len(months))
	barW := slot * 0.56
	var prevPts [][2]float64
	for i, m := range months {
		x := plot.X + slot*float64(i) + (slot-barW)/2
		if m.Fact > 0 {
			c.Rect(box{x, yOf(m.Fact), barW, yOf(0) - yOf(m.Fact)}, ptr(colorFact), nil, 0)
		}
		if m.EAC > m.Fact {
			c.Rect(box{x, yOf(m.EAC), barW, yOf(m.Fact) - yOf(m.EAC)}, ptr(colorEACBg), nil, 0)
		}
		if m.Plan > 0 {
			py := yOf(m.Plan)
			c.Line(x-slot*0.08, py, x+barW+slot*0.08, py, colorInk, 0.5)
		}
		if m.Prev != nil {
			prevPts = append(prevPts, [2]float64{x + barW/2, yOf(*m.Prev)})
		}
		// Подпись значения над столбцом (и над риской плана, если она выше) и
		// месяц под ним.
		top := yOf(math.Max(m.Plan, math.Max(m.EAC, m.Fact)))
		c.Text(box{x - slot*0.2, top - 4.2, barW + slot*0.4, 4}, sc.format(math.Max(m.EAC, m.Fact)),
			textStyle{Size: 6, Color: colorMuted, Align: "C"})
		c.Text(box{x - slot*0.2, plot.Y + plot.H + 0.5, barW + slot*0.4, 5}, m.Label,
			textStyle{Size: 7, Color: colorMuted, Align: "C"})
	}
	if len(prevPts) > 1 {
		c.Polyline(prevPts, colorPrev, 0.6)
		for _, p := range prevPts {
			c.Rect(box{p[0] - 0.7, p[1] - 0.7, 1.4, 1.4}, ptr(colorPrev), nil, 0)
		}
	}
	c.Line(plot.X, yOf(0), plot.X+plot.W, yOf(0), colorMuted, 0.3)
}

// ─── Водопад ────────────────────────────────────────────────────────────────
//
// От плана к ожидаемому итогу через отклонение: «сколько обещали — сколько
// не добрали — сколько ждём». Для инвестиций это честнее двух столбцов
// рядом: отклонение здесь и есть смысл блока.

type waterfallStep struct {
	Label string
	Value float64
	Total bool // опорный столбец от нуля
}

func drawWaterfall(c canvas, b box, steps []waterfallStep, sc scale) {
	if len(steps) == 0 {
		return
	}
	axisW := 14.0
	plot := box{b.X + axisW, b.Y + 4, b.W - axisW, b.H - 4 - 6}
	// Диапазон по накопленной сумме.
	max, run := 0.0, 0.0
	for _, s := range steps {
		if s.Total {
			run = s.Value
		} else {
			run += s.Value
		}
		max = math.Max(max, run)
	}
	if max <= 0 {
		max = 1
	}
	max *= 1.15
	yOf := func(v float64) float64 { return plot.Y + plot.H - plot.H*math.Max(0, v)/max }
	for i := 0; i <= 4; i++ {
		v := max / 1.15 * float64(i) / 4
		c.Line(plot.X, yOf(v), plot.X+plot.W, yOf(v), colorLine, 0.2)
		c.Text(box{b.X, yOf(v) - 2.5, axisW - 2, 5}, sc.format(v), styleAxis)
	}
	slot := plot.W / float64(len(steps))
	barW := math.Min(slot*0.5, 28)
	run = 0
	for i, s := range steps {
		x := plot.X + slot*float64(i) + (slot-barW)/2
		var from, to float64
		fill := colorPlan
		switch {
		case s.Total && i == 0:
			from, to, run = 0, s.Value, s.Value
		case s.Total:
			from, to, run = 0, s.Value, s.Value
			fill = colorEAC
		default:
			from, to = run, run+s.Value
			run += s.Value
			if s.Value < 0 {
				fill = colorBad
			} else {
				fill = colorGood
			}
		}
		top, bottom := yOf(math.Max(from, to)), yOf(math.Min(from, to))
		c.Rect(box{x, top, barW, math.Max(bottom-top, 0.3)}, ptr(fill), nil, 0)
		label := sc.format(s.Value)
		if !s.Total {
			label = sc.signed(s.Value)
		}
		c.Text(box{x - slot*0.2, top - 4.2, barW + slot*0.4, 4}, label, textStyle{Size: 7, Bold: true, Color: colorInk, Align: "C"})
		c.Text(box{x - slot*0.25, plot.Y + plot.H + 0.5, barW + slot*0.5, 5}, s.Label, textStyle{Size: 7, Color: colorMuted, Align: "C"})
		// Соединительная линия к следующему шагу.
		if i < len(steps)-1 {
			c.Line(x+barW, yOf(run), x+slot, yOf(run), colorMuted, 0.2)
		}
	}
	c.Line(plot.X, yOf(0), plot.X+plot.W, yOf(0), colorMuted, 0.3)
}

// ─── Спарклайн и карточка ───────────────────────────────────────────────────

func drawSparkline(c canvas, b box, values []float64, col rgb) {
	if len(values) < 2 {
		return
	}
	max := 0.0
	for _, v := range values {
		max = math.Max(max, v)
	}
	if max <= 0 {
		return
	}
	pts := make([][2]float64, 0, len(values))
	for i, v := range values {
		x := b.X + b.W*float64(i)/float64(len(values)-1)
		y := b.Y + b.H - b.H*math.Max(0, v)/max
		pts = append(pts, [2]float64{x, y})
	}
	c.Polyline(pts, col, 0.5)
	last := pts[len(pts)-1]
	c.Rect(box{last[0] - 0.8, last[1] - 0.8, 1.6, 1.6}, ptr(col), nil, 0)
}

// card — плитка показателя: подпись, число, дельта с оценкой, пояснение,
// спарклайн по месяцам.
type card struct {
	Label     string
	Value     string
	Delta     string
	DeltaTone tone
	Sub       string
	Spark     []float64
}

func drawCard(c canvas, b box, k card) {
	c.Rect(b, ptr(colorZebra), ptr(colorCardLine), 0.2)
	in := b.inset(3)
	c.Text(box{in.X, in.Y, in.W, 4}, k.Label, textStyle{Size: 7, Color: colorMuted, Align: "L"})
	valueW := in.W
	if len(k.Spark) > 1 {
		valueW = in.W * 0.62
		drawSparkline(c, box{in.X + in.W*0.66, in.Y + 5, in.W * 0.34, in.H * 0.45}, k.Spark, colorFact)
	}
	c.Text(box{in.X, in.Y + 4.5, valueW, 8}, k.Value, textStyle{Size: 15, Bold: true, Color: colorInk, Align: "L"})
	y := in.Y + 13
	if k.Delta != "" {
		c.Text(box{in.X, y, in.W, 4}, k.Delta, textStyle{Size: 7.5, Bold: true, Color: k.DeltaTone.color(), Align: "L"})
		y += 4
	}
	if k.Sub != "" {
		c.Text(box{in.X, y, in.W, 4}, k.Sub, textStyle{Size: 6.5, Color: colorMuted, Align: "L"})
	}
}

// drawCards раскладывает карточки сеткой по три в ряд; возвращает высоту.
func drawCards(c canvas, x, y, w float64, cards []card) float64 {
	if len(cards) == 0 {
		return 0
	}
	const cardH, gap = 23.0, 3.0
	cols := 3
	if len(cards) < 3 {
		cols = len(cards)
	}
	cardW := (w - gap*float64(cols-1)) / float64(cols)
	rows := 0
	for i, k := range cards {
		col, row := i%cols, i/cols
		drawCard(c, box{x + float64(col)*(cardW+gap), y + float64(row)*(cardH+gap), cardW, cardH}, k)
		rows = row + 1
	}
	return float64(rows)*cardH + float64(rows-1)*gap
}

// ─── Спецификация графика в страничной модели ───────────────────────────────

type chartSpec struct {
	Title, Subtitle string
	Scale           scale
	Bullet          []bulletRow
	Months          []monthPoint
	Steps           []waterfallStep
}

// preferredHeight — сколько места график просит по вертикали.
func (ch *chartSpec) preferredHeight() float64 {
	switch {
	case len(ch.Bullet) > 0:
		return math.Min(16, math.Max(9, 90/float64(len(ch.Bullet))))*float64(len(ch.Bullet)) + 16
	case len(ch.Months) > 0:
		return 70
	default:
		return 60
	}
}

func drawChart(c canvas, b box, ch *chartSpec) {
	inner := chartFrame(c, b, ch.Title, ch.Subtitle)
	switch {
	case len(ch.Bullet) > 0:
		drawBullet(c, inner, ch.Bullet, ch.Scale)
	case len(ch.Months) > 0:
		drawMonths(c, inner, ch.Months, ch.Scale)
	case len(ch.Steps) > 0:
		drawWaterfall(c, inner, ch.Steps, ch.Scale)
	}
}
