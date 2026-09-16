package reports

import (
	"fmt"
	"math"
	"strings"
)

// ─── Дизайн-токены ──────────────────────────────────────────────────────────
//
// Одна палитра и одна шкала на оба формата: смысл цвета закреплён за
// величиной, а не за графиком. План — нейтральный серый (обязательство, фон),
// факт — синий (то, что есть), EAC — янтарный (ожидание), прошлый год —
// бледный. Оценки — зелёный/жёлтый/красный.

type rgb struct{ R, G, B uint8 }

func (c rgb) hex() string { return fmt.Sprintf("%02X%02X%02X", c.R, c.G, c.B) }

var (
	colorInk      = rgb{0x1F, 0x29, 0x37} // основной текст
	colorMuted    = rgb{0x6B, 0x72, 0x80} // подписи, сноски
	colorLine     = rgb{0xE5, 0xE7, 0xEB} // линейки таблиц, сетка
	colorZebra    = rgb{0xF6, 0xF7, 0xF9} // чётные строки, фон карточек
	colorCardLine = rgb{0xE2, 0xE6, 0xEB}
	colorWhite    = rgb{0xFF, 0xFF, 0xFF}

	colorPlan  = rgb{0xCB, 0xD2, 0xDB} // серая полоса обязательства
	colorFact  = rgb{0x25, 0x63, 0xEB} // синий
	colorEAC   = rgb{0xF5, 0x9E, 0x0B} // янтарный
	colorEACBg = rgb{0xFD, 0xE6, 0x8A} // прогнозная часть столбца
	colorPrev  = rgb{0x9C, 0xA3, 0xAF} // линия прошлого года
	colorBar   = rgb{0xDB, 0xEA, 0xFE} // data bar в таблице

	colorGood = rgb{0x15, 0x80, 0x3D}
	colorWarn = rgb{0xB4, 0x53, 0x09}
	colorBad  = rgb{0xB9, 0x1C, 0x1C}
)

// tone — оценка величины для цвета текста и бейджей.
type tone int

const (
	toneNeutral tone = iota
	toneGood
	toneWarn
	toneBad
)

func (t tone) color() rgb {
	switch t {
	case toneGood:
		return colorGood
	case toneWarn:
		return colorWarn
	case toneBad:
		return colorBad
	}
	return colorInk
}

// completionTone оценивает выполнение в процентах: ≥100 хорошо, ≥90 —
// внимание, ниже — плохо. nil — нет данных, без оценки.
func completionTone(pct *float64) tone {
	switch {
	case pct == nil:
		return toneNeutral
	case *pct >= 100:
		return toneGood
	case *pct >= 90:
		return toneWarn
	default:
		return toneBad
	}
}

// deltaTone — знак отклонения: отрицательное плохо, положительное хорошо.
func deltaTone(v float64) tone {
	switch {
	case v < 0:
		return toneBad
	case v > 0:
		return toneGood
	}
	return toneNeutral
}

// ─── Шкала чисел ────────────────────────────────────────────────────────────
//
// Копейки в отчёте не нужны: «29 334 880 130,29» не читается, а порядок
// величины теряется. Страница выбирает одну шкалу по своему максимуму —
// млрд, млн, тыс. — и подписывает её в заголовке колонки или карточки.
// Точные суммы остаются в приложении «сеть × квартал».

type scale struct {
	div   float64
	label string // «млрд ₽», «млн уп.»
	// digits — знаков после запятой, общее для всех чисел элемента: по
	// максимуму так, чтобы у него было не меньше трёх значащих цифр.
	digits int
}

// scaleFor подбирает шкалу под максимум значений элемента — карточек,
// графика или таблицы, у каждого своя: таблица сетей на сотни миллионов не
// должна показывать «0,1», потому что итог страницы в миллиардах.
func scaleFor(unit string, values ...float64) scale {
	max := 0.0
	for _, v := range values {
		if a := math.Abs(v); a > max {
			max = a
		}
	}
	base := "₽"
	if unit == "units" {
		base = "уп."
	}
	sc := scale{1, base, 0}
	switch {
	case max >= 1e9:
		sc = scale{1e9, "млрд " + base, 0}
	case max >= 1e6:
		sc = scale{1e6, "млн " + base, 0}
	case max >= 1e3:
		sc = scale{1e3, "тыс. " + base, 0}
	}
	if sc.div == 1 {
		if unit != "units" {
			sc.digits = 2
		}
		return sc
	}
	switch x := max / sc.div; {
	case x < 10:
		sc.digits = 2
	case x < 100:
		sc.digits = 1
	}
	return sc
}

// format — число в шкале элемента.
func (s scale) format(v float64) string {
	return decimal(v/s.div, s.digits)
}

// signed — то же со знаком «+» у положительных: для отклонений.
func (s scale) signed(v float64) string {
	out := s.format(v)
	if v > 0 {
		return "+" + out
	}
	return out
}

func (s scale) optional(v *float64) string {
	if v == nil {
		return "—"
	}
	return s.format(*v)
}

// ─── Форматирование ─────────────────────────────────────────────────────────

func groupThousands(whole int64) string {
	s := fmt.Sprintf("%d", whole)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// decimal — 1 234,5 с заданным числом знаков; знак минуса сохраняется.
func decimal(v float64, digits int) string {
	neg := v < 0
	v = math.Abs(v)
	pow := math.Pow(10, float64(digits))
	rounded := math.Round(v*pow) / pow
	whole := int64(math.Floor(rounded))
	out := groupThousands(whole)
	if digits > 0 {
		frac := int64(math.Round((rounded - float64(whole)) * pow))
		out += "," + fmt.Sprintf("%0*d", digits, frac)
	}
	if neg && rounded != 0 {
		return "−" + out
	}
	return out
}

// money — 1 234 567,89: полная сумма с копейками для приложений.
func money(v float64) string { return decimal(v, 2) }

// integer — 1 234 567, для упаковок и счётчиков.
func integer(v float64) string { return decimal(v, 0) }

// percent — 47,6 %; nil — прочерк: «нет данных» и ноль — разные вещи.
func percent(v *float64) string {
	if v == nil {
		return "—"
	}
	return decimal(*v, 1) + " %"
}

// signedPercent — +11,4 % / −36,2 %.
func signedPercent(v *float64) string {
	if v == nil {
		return "—"
	}
	if *v > 0 {
		return "+" + decimal(*v, 1) + " %"
	}
	return decimal(*v, 1) + " %"
}

// unitLabel — короткая подпись единицы объёма.
func unitLabel(unit string) string {
	if unit == "units" {
		return "уп."
	}
	return "₽"
}

// pick выбирает рубли или упаковки по единице.
func pick(unit string, rub, units float64) float64 {
	if unit == "units" {
		return units
	}
	return rub
}

func pickPtr(unit string, rub, units *float64) *float64 {
	if unit == "units" {
		return units
	}
	return rub
}

var monthNames = [...]string{"", "Янв", "Фев", "Мар", "Апр", "Май", "Июн", "Июл", "Авг", "Сен", "Окт", "Ноя", "Дек"}

func monthName(m int) string {
	if m >= 1 && m <= 12 {
		return monthNames[m]
	}
	return fmt.Sprint(m)
}

// truncate обрезает длинное название с многоточием: в таблице оно занимает
// одну строку, полное имя есть на витрине.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max-1])) + "…"
}
