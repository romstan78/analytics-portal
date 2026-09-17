package models

import "time"

// ─── Выгрузка отчётов PDF/PPTX ──────────────────────────────────────────────
//
// Отчёт строится по витрине реестра сетей: снимок — это готовый
// NetworkDashboardResponse плюс метаданные. Своих чисел у отчёта нет, поэтому
// он по построению сходится с витриной для того же фильтра.

// Форматы файла. Каждому формату — своё задание: у них разное время подготовки
// и разные ошибки, а клиент показывает ссылки по мере готовности.
const (
	ReportFormatPDF  = "pdf"
	ReportFormatPPTX = "pptx"
)

// Единица измерения объёма. Витрина переключается между рублями и упаковками
// целиком, отчёт наследует выбранную. Инвестиции всегда в рублях.
const (
	ReportUnitRub   = "rub"
	ReportUnitUnits = "units"
)

// Коды блоков — фиксированный каталог, из которого пользователь составляет
// отчёт. Порядок здесь — порядок по умолчанию; cover всегда первый,
// methodology всегда последний.
const (
	ReportBlockCover            = "cover"
	ReportBlockSummaryKPI       = "summary-kpi"
	ReportBlockPeriodTrend      = "period-trend"
	ReportBlockPlanFactEAC      = "plan-fact-eac"
	ReportBlockNetworkTop       = "network-top"
	ReportBlockBrandTop         = "brand-top"
	ReportBlockKAMTop           = "kam-top"
	ReportBlockGTN              = "gtn"
	ReportBlockRegistryOpex     = "registry-opex"
	ReportBlockPromoInvestments = "promo-investments"
	ReportBlockNetworkQuarters  = "network-quarters"
	ReportBlockMethodology      = "methodology"
)

// Статусы задания — те же, что у Excel-выгрузки продаж.
const (
	ReportJobQueued  = "queued"
	ReportJobRunning = "running"
	ReportJobReady   = "ready"
	ReportJobFailed  = "failed"
)

// ReportBlock — элемент каталога блоков для конструктора.
type ReportBlock struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// Fixed — блок нельзя переставить: cover стоит первым, methodology последним.
	Fixed bool `json:"fixed"`
	// HasTable — у блока есть таблица, к которой применяется лимит строк.
	HasTable bool `json:"hasTable"`
	// KAMScopeOnly — блок имеет смысл только для ролей без закрепления: КАМ
	// видит одного себя, и разрез по КАМам выродился бы в одну строку.
	KAMScopeOnly bool `json:"kamScopeOnly"`
}

// ReportRequest — тело POST /api/reports. Фильтры — ровно те, что у витрины
// (NetworkDashboardFilter); фильтра по брендам у витрины нет, и здесь его нет.
type ReportRequest struct {
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Quarters []int  `json:"quarters"`
	// KAMs учитываются только у ролей без закрепления: закреплённый КАМ
	// получает свою область независимо от запроса.
	KAMs       []string `json:"kams"`
	NetworkIDs []int    `json:"networkIds"`
	Unit       string   `json:"unit"`
	// Blocks — выбранные блоки в порядке показа.
	Blocks  []string `json:"blocks"`
	Formats []string `json:"formats"`
	// TableLimit — потолок строк таблиц: 10 или 20.
	TableLimit int `json:"tableLimit"`
}

// ReportSnapshot — серверный снимок, из которого строятся оба формата.
// Хранится в задании как JSON: файл воспроизводим и объясним.
type ReportSnapshot struct {
	Request ReportRequest `json:"request"`
	// Title — заголовок отчёта после подстановки умолчания.
	Title           string `json:"title"`
	Owner           string `json:"owner"`
	CreatedAt       string `json:"createdAt"`
	TemplateVersion string `json:"templateVersion"`
	// FilterLabels — человекочитаемое описание области: год и кварталы,
	// названия выбранных сетей и КАМов. Идёт на титул и в колонтитул.
	FilterLabels []string `json:"filterLabels"`
	// OwnKAM — закрепление автора; непустое означает, что область сужена
	// сервером, и об этом сказано в отчёте.
	OwnKAM    string                    `json:"ownKam"`
	Dashboard *NetworkDashboardResponse `json:"dashboard"`
}

// ReportJob — фоновая подготовка одного файла.
type ReportJob struct {
	ID          string    `json:"id"`
	Owner       string    `json:"-"`
	Status      string    `json:"status"`
	Format      string    `json:"format"`
	Title       string    `json:"title"`
	FileName    string    `json:"fileName"`
	FilePath    string    `json:"-"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
	// SnapshotJSON — снимок запроса и данных; наружу не отдаётся.
	SnapshotJSON string `json:"-"`
	// PrintTokenHash — SHA-256 одноразового токена, по которому страница
	// печати получает снимок без сессии; сам токен живёт только в URL,
	// который открывает Chromium. Обнуляется при выдаче.
	PrintTokenHash      string    `json:"-"`
	PrintTokenExpiresAt time.Time `json:"-"`
}

// ReportJobStatus — ответ статуса задания. Время строками: контракт
// генерируется из Go, а time.Time генератор не переводит.
type ReportJobStatus struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Format      string `json:"format"`
	Title       string `json:"title"`
	FileName    string `json:"fileName"`
	Error       string `json:"error,omitempty"`
	CreatedAt   string `json:"createdAt"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// ReportCreateResponse — ответ POST /api/reports: по заданию на формат.
type ReportCreateResponse struct {
	Jobs []ReportJobStatus `json:"jobs"`
}

// ─── Печатная модель ────────────────────────────────────────────────────────
//
// Страница печати (/print/report/:id) получает не сырой снимок, а готовые
// страницы: подписи, шкалы, оценки ячеек и примечания собраны в reports/pages.go
// один раз для PDF, PPTX и HTML. Фронтенд их только раскладывает и рисует
// графики по числам; вторая копия правил «какая шкала и какая колонка» на
// фронте разошлась бы с первой. Числа в ячейках и карточках уже
// отформатированы; графикам нужны исходные величины, они идут отдельно.

// Оценка величины цветом — те же четыре тона, что у векторного рендера.
const (
	ReportToneNeutral = "neutral"
	ReportToneGood    = "good"
	ReportToneWarn    = "warn"
	ReportToneBad     = "bad"
)

// ReportPrint — ответ GET /api/reports/:id/print.
type ReportPrint struct {
	Title        string   `json:"title"`
	Owner        string   `json:"owner"`
	CreatedAt    string   `json:"createdAt"`
	FilterLabels []string `json:"filterLabels"`
	// Unit — единица объёма запроса (rub | units): подсказка для осей графиков.
	Unit  string            `json:"unit"`
	Pages []ReportPrintPage `json:"pages"`
}

// ReportPrintPage — один блок отчёта: страница PDF, слайд PPTX.
type ReportPrintPage struct {
	Block    string `json:"block"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	// Lines — абзацы текста: титул, методика.
	Lines []string          `json:"lines,omitempty"`
	Cards []ReportPrintCard `json:"cards,omitempty"`
	Chart *ReportPrintChart `json:"chart,omitempty"`
	Table *ReportPrintTable `json:"table,omitempty"`
	Notes []string          `json:"notes,omitempty"`
}

// ReportPrintCard — плитка показателя: подпись, число в шкале, отклонение с
// оценкой, пояснение и ряд для спарклайна.
type ReportPrintCard struct {
	Label     string    `json:"label"`
	Value     string    `json:"value"`
	Delta     string    `json:"delta,omitempty"`
	DeltaTone string    `json:"deltaTone"`
	Sub       string    `json:"sub,omitempty"`
	Spark     []float64 `json:"spark,omitempty"`
}

// ReportPrintScale — шкала осей графика: делитель, подпись, знаки после
// запятой. Повторяет reports.scale, чтобы оси подписывались как таблицы.
type ReportPrintScale struct {
	Div    float64 `json:"div"`
	Label  string  `json:"label"`
	Digits int     `json:"digits"`
}

// ReportPrintChart — спецификация графика; заполнен ровно один из рядов.
type ReportPrintChart struct {
	Title    string              `json:"title"`
	Subtitle string              `json:"subtitle,omitempty"`
	Scale    ReportPrintScale    `json:"scale"`
	Bullet   []ReportPrintBullet `json:"bullet,omitempty"`
	Months   []ReportPrintMonth  `json:"months,omitempty"`
	Steps    []ReportPrintStep   `json:"steps,omitempty"`
}

// ReportPrintBullet — строка bullet-графика: план полосой, факт внутри, EAC
// риской. PctLabel и Tone — подпись выполнения и её оценка, уже готовые.
type ReportPrintBullet struct {
	Label    string  `json:"label"`
	Sub      string  `json:"sub,omitempty"`
	Plan     float64 `json:"plan"`
	Fact     float64 `json:"fact"`
	EAC      float64 `json:"eac"`
	PctLabel string  `json:"pctLabel"`
	Tone     string  `json:"tone"`
}

// ReportPrintMonth — точка месячной динамики.
type ReportPrintMonth struct {
	Label  string   `json:"label"`
	Plan   float64  `json:"plan"`
	Fact   float64  `json:"fact"`
	EAC    float64  `json:"eac"`
	Prev   *float64 `json:"prev"`
	Closed bool     `json:"closed"`
}

// ReportPrintStep — ступень водопада; Total — опорный столбец от нуля.
type ReportPrintStep struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Total bool    `json:"total"`
}

// ReportPrintTable — таблица с готовыми ячейками.
type ReportPrintTable struct {
	Columns []ReportPrintColumn `json:"columns"`
	Rows    [][]ReportPrintCell `json:"rows"`
	// Total — итоговая строка: жирная, с линией сверху.
	Total []ReportPrintCell `json:"total,omitempty"`
}

// ReportPrintColumn — колонка: Weight — доля ширины, Right — числовая.
type ReportPrintColumn struct {
	Title  string  `json:"title"`
	Weight float64 `json:"weight"`
	Right  bool    `json:"right"`
}

// ReportPrintCell — ячейка: текст, оценка цветом, доля полосы (nil — без
// полосы), жирность.
type ReportPrintCell struct {
	Text string   `json:"text"`
	Tone string   `json:"tone"`
	Bar  *float64 `json:"bar"`
	Bold bool     `json:"bold"`
}

// StatusView переводит задание в ответ API.
func (j ReportJob) StatusView() ReportJobStatus {
	view := ReportJobStatus{
		ID: j.ID, Status: j.Status, Format: j.Format, Title: j.Title,
		FileName: j.FileName, Error: j.Error,
		CreatedAt: j.CreatedAt.UTC().Format(time.RFC3339),
	}
	if !j.CompletedAt.IsZero() {
		view.CompletedAt = j.CompletedAt.UTC().Format(time.RFC3339)
	}
	return view
}
