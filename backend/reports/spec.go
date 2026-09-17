package reports

// ─── Спецификации элементов страничной модели ───────────────────────────────
//
// Страница описана данными, а не командами рисования: карточки, график,
// таблица. Рисует их страница печати фронтенда (Chromium), а PPTX кладёт
// снимок и нативные таблицы; здесь только структуры, которые BuildPages
// заполняет, а print.go переводит в JSON-контракт.

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

// bulletRow — строка bullet-графика: полоса — план, внутри — факт, риска —
// ожидаемый итог; все три в одной шкале.
type bulletRow struct {
	Label           string
	Plan, Fact, EAC float64
	Pct             *float64 // EAC / план
	Sub             string   // подпись под названием: КАМ, тип
}

// monthPoint — точка месячной динамики.
type monthPoint struct {
	Label           string
	Plan, Fact, EAC float64
	Prev            *float64
	Closed          bool
}

// waterfallStep — ступень водопада «план → отклонение → EAC».
type waterfallStep struct {
	Label string
	Value float64
	Total bool // опорный столбец от нуля
}

// chartSpec — график страницы; заполнен ровно один из рядов.
type chartSpec struct {
	Title, Subtitle string
	Scale           scale
	Bullet          []bulletRow
	Months          []monthPoint
	Steps           []waterfallStep
}

// ─── Геометрия слайда ───────────────────────────────────────────────────────

// box — рамка в миллиметрах слайда.
type box struct{ X, Y, W, H float64 }

type textStyle struct {
	Size  float64 // pt
	Bold  bool
	Color rgb
	Align string // L, C, R
}

func ptr(c rgb) *rgb { return &c }
