package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"backend/models"
	"backend/repository"
)

// ─── Отчёты PDF/PPTX: каталог блоков, проверка запроса, снимок ──────────────

// ReportTemplateVersion меняется при правке макетов: по нему видно, каким
// кодом собран сохранённый в задании снимок.
const ReportTemplateVersion = "networks-period-1"

// ReportDefaultTitle подставляется, когда пользователь не назвал отчёт.
const ReportDefaultTitle = "Итоги периода по сетям"

// ReportTableLimits — допустимые потолки строк таблиц.
var ReportTableLimits = []int{10, 20}

// reportBlocks — каталог в порядке по умолчанию.
var reportBlocks = []models.ReportBlock{
	{Code: models.ReportBlockCover, Title: "Титул", Description: "Название, период, фильтры, автор и дата", Fixed: true},
	{Code: models.ReportBlockSummaryKPI, Title: "Ключевые показатели", Description: "План, факт, EAC, выполнение, разрыв, сравнение с прошлым годом, готовность данных"},
	{Code: models.ReportBlockPeriodTrend, Title: "Динамика по месяцам", Description: "Факт и ожидаемый итог по месяцам, факт прошлого года"},
	{Code: models.ReportBlockPlanFactEAC, Title: "План, факт и EAC по кварталам", Description: "Сравнение обязательства, факта и ожидаемого итога", HasTable: true},
	{Code: models.ReportBlockNetworkTop, Title: "Топ сетей", Description: "Сети по плановому объёму с фактом, EAC и выполнением", HasTable: true},
	{Code: models.ReportBlockBrandTop, Title: "Топ брендов", Description: "Бренды по плановому объёму; признак валового пула и нераспределённый остаток", HasTable: true},
	{Code: models.ReportBlockKAMTop, Title: "Разрез по КАМам", Description: "Портфель каждого КАМа: план, факт, EAC", HasTable: true, KAMScopeOnly: true},
	{Code: models.ReportBlockGTN, Title: "Инвестиции реестра (GTN)", Description: "План, факт, EAC и отклонение в базе без НДС; отдельно от OPEX и промо"},
	{Code: models.ReportBlockRegistryOpex, Title: "Бюджет OPEX реестра", Description: "Согласованный бюджет по статьям договора — одно число, без пары план/факт"},
	{Code: models.ReportBlockPromoInvestments, Title: "Инвестиции промо", Description: "GTN и OPEX промо: план, факт, EAC; число активностей по каналам"},
	{Code: models.ReportBlockNetworkQuarters, Title: "Сети по кварталам", Description: "Таблица «сеть × квартал»: план, факт, EAC", HasTable: true},
	{Code: models.ReportBlockMethodology, Title: "Методика", Description: "Источники, единицы, база НДС, оговорки", Fixed: true},
}

// ReportBlocks возвращает каталог блоков в порядке по умолчанию.
func ReportBlocks() []models.ReportBlock {
	out := make([]models.ReportBlock, len(reportBlocks))
	copy(out, reportBlocks)
	return out
}

func reportBlockByCode(code string) (models.ReportBlock, bool) {
	for _, b := range reportBlocks {
		if b.Code == code {
			return b, true
		}
	}
	return models.ReportBlock{}, false
}

// ReportValidationError — ошибка запроса, которую можно показать пользователю.
type ReportValidationError struct{ Message string }

func (e ReportValidationError) Error() string { return e.Message }

func reportInvalid(format string, args ...interface{}) error {
	return ReportValidationError{Message: fmt.Sprintf(format, args...)}
}

// NormalizeReportRequest проверяет запрос и приводит его к каноническому виду:
// кварталы по возрастанию без повторов, cover первым, methodology последним,
// умолчания подставлены. Возвращает ReportValidationError на ошибку ввода.
//
// ownKAM — закрепление автора. У закреплённого КАМа запрошенные КАМы
// отбрасываются (область задаёт сервер), а разрез по КАМам недоступен.
func NormalizeReportRequest(req models.ReportRequest, ownKAM string) (models.ReportRequest, error) {
	out := req
	out.Title = strings.TrimSpace(req.Title)
	if out.Title == "" {
		out.Title = ReportDefaultTitle
	}
	if len([]rune(out.Title)) > 120 {
		return out, reportInvalid("Название отчёта длиннее 120 символов")
	}
	if req.Year < 2000 || req.Year > 2100 {
		return out, reportInvalid("Некорректный год")
	}

	seenQ := map[int]bool{}
	out.Quarters = nil
	for _, q := range req.Quarters {
		if q < 1 || q > 4 {
			return out, reportInvalid("Некорректный квартал: %d", q)
		}
		if !seenQ[q] {
			seenQ[q] = true
			out.Quarters = append(out.Quarters, q)
		}
	}
	sort.Ints(out.Quarters)

	seenN := map[int]bool{}
	out.NetworkIDs = nil
	for _, id := range req.NetworkIDs {
		if id <= 0 {
			return out, reportInvalid("Некорректный идентификатор сети: %d", id)
		}
		if !seenN[id] {
			seenN[id] = true
			out.NetworkIDs = append(out.NetworkIDs, id)
		}
	}
	sort.Ints(out.NetworkIDs)

	if ownKAM != "" {
		out.KAMs = nil
	} else {
		seenK := map[string]bool{}
		out.KAMs = nil
		for _, k := range req.KAMs {
			k = strings.TrimSpace(k)
			if k != "" && !seenK[k] {
				seenK[k] = true
				out.KAMs = append(out.KAMs, k)
			}
		}
		sort.Strings(out.KAMs)
	}

	switch req.Unit {
	case "":
		out.Unit = models.ReportUnitRub
	case models.ReportUnitRub, models.ReportUnitUnits:
	default:
		return out, reportInvalid("Некорректная единица измерения: %s", req.Unit)
	}

	if req.TableLimit == 0 {
		out.TableLimit = ReportTableLimits[len(ReportTableLimits)-1]
	} else {
		allowed := false
		for _, l := range ReportTableLimits {
			allowed = allowed || l == req.TableLimit
		}
		if !allowed {
			return out, reportInvalid("Лимит строк таблицы может быть только 10 или 20")
		}
	}

	seenF := map[string]bool{}
	out.Formats = nil
	for _, f := range req.Formats {
		switch f {
		case models.ReportFormatPDF, models.ReportFormatPPTX:
		default:
			return out, reportInvalid("Неизвестный формат: %s", f)
		}
		if !seenF[f] {
			seenF[f] = true
			out.Formats = append(out.Formats, f)
		}
	}
	if len(out.Formats) == 0 {
		return out, reportInvalid("Выберите хотя бы один формат")
	}

	// Блоки: известные, без повторов; cover и methodology на своих местах.
	seenB := map[string]bool{}
	var middle []string
	hasMethodology := false
	for _, code := range req.Blocks {
		block, ok := reportBlockByCode(code)
		if !ok {
			return out, reportInvalid("Неизвестный блок: %s", code)
		}
		if seenB[code] {
			continue
		}
		seenB[code] = true
		if block.KAMScopeOnly && ownKAM != "" {
			return out, reportInvalid("Блок «%s» недоступен при закреплении за КАМом", block.Title)
		}
		switch code {
		case models.ReportBlockCover:
		case models.ReportBlockMethodology:
			hasMethodology = true
		default:
			middle = append(middle, code)
		}
	}
	if len(middle) == 0 {
		return out, reportInvalid("Выберите хотя бы один содержательный блок")
	}
	out.Blocks = append([]string{models.ReportBlockCover}, middle...)
	if hasMethodology {
		out.Blocks = append(out.Blocks, models.ReportBlockMethodology)
	}
	return out, nil
}

// ReportDashboardFilter — фильтр витрины для нормализованного запроса. Тот же,
// что собирает GET /api/networks/dashboard: область КАМа подменяет запрос.
func ReportDashboardFilter(req models.ReportRequest, ownKAM string) repository.NetworkDashboardFilter {
	kams := req.KAMs
	if ownKAM != "" {
		kams = nil
	}
	return repository.NetworkDashboardFilter{
		Year:       req.Year,
		Quarters:   req.Quarters,
		OwnKAM:     ownKAM,
		KAMs:       kams,
		NetworkIDs: req.NetworkIDs,
	}
}

// BuildReportSnapshot собирает снимок: витрина по тому же фильтру плюс
// метаданные. Запрос должен быть нормализован.
func BuildReportSnapshot(req models.ReportRequest, owner, ownKAM string, now time.Time) (*models.ReportSnapshot, error) {
	dashboard, err := BuildNetworkDashboard(ReportDashboardFilter(req, ownKAM))
	if err != nil {
		return nil, err
	}
	return SnapshotFromDashboard(req, owner, ownKAM, now, dashboard), nil
}

// SnapshotFromDashboard — чистая часть сборки снимка: метаданные и подписи
// области поверх готового ответа витрины.
func SnapshotFromDashboard(req models.ReportRequest, owner, ownKAM string, now time.Time, dashboard *models.NetworkDashboardResponse) *models.ReportSnapshot {
	return &models.ReportSnapshot{
		Request: req,
		Title:   req.Title,
		Owner:   owner,
		// Зона печатается явно: сервер живёт в UTC, если в контейнере не задан TZ,
		// и «08:44» без пояса читалось бы как местное время.
		CreatedAt:       now.Format("2006-01-02 15:04 MST"),
		TemplateVersion: ReportTemplateVersion,
		FilterLabels:    reportFilterLabels(req, ownKAM, dashboard),
		OwnKAM:          ownKAM,
		Dashboard:       dashboard,
	}
}

// reportFilterLabels описывает область словами: период, сети, КАМы.
// Названия сетей берутся из ответа витрины — там только сети в области.
func reportFilterLabels(req models.ReportRequest, ownKAM string, dashboard *models.NetworkDashboardResponse) []string {
	labels := []string{fmt.Sprintf("Год %d · %s", req.Year, quartersLabel(dashboard.SelectedQuarters))}

	if len(req.NetworkIDs) == 0 {
		labels = append(labels, "Сети: весь доступный портфель")
	} else {
		names := make([]string, 0, len(req.NetworkIDs))
		byID := map[int]string{}
		for _, n := range dashboard.Networks {
			if n.NetworkID != nil {
				byID[*n.NetworkID] = n.Name
			}
		}
		for _, id := range req.NetworkIDs {
			if name, ok := byID[id]; ok {
				names = append(names, name)
			}
		}
		if len(names) == 0 {
			labels = append(labels, "Сети: нет данных по выбранным сетям")
		} else {
			labels = append(labels, "Сети: "+strings.Join(names, ", "))
		}
	}

	switch {
	case ownKAM != "":
		labels = append(labels, "КАМ: "+ownKAM+" (область закрепления)")
	case len(req.KAMs) > 0:
		labels = append(labels, "КАМ: "+strings.Join(req.KAMs, ", "))
	default:
		labels = append(labels, "КАМ: все")
	}
	return labels
}

func quartersLabel(quarters []int) string {
	if len(quarters) == 0 || len(quarters) == 4 {
		return "весь год"
	}
	parts := make([]string, 0, len(quarters))
	for _, q := range quarters {
		parts = append(parts, fmt.Sprintf("Q%d", q))
	}
	return strings.Join(parts, ", ")
}
