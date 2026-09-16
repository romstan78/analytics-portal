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
