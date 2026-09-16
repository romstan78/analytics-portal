package reports

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/models"
)

// sampleSnapshot — снимок со всеми блоками: 25 сетей (больше лимита 20 и
// больше строк на слайд), длинные названия, отрицательный разрыв с копейками,
// «нет данных» за прошлый год у одной сети.
func sampleSnapshot(unit string) *models.ReportSnapshot {
	metrics := func(plan, fact, eac float64) models.NetworkDashboardMetrics {
		completion := fact / plan * 100
		eacCompletion := eac / plan * 100
		return models.NetworkDashboardMetrics{
			NetworkCount: 1, BrandCount: 3,
			PlanRub: plan, FactRub: fact, EACRub: eac,
			PlanUnits: plan / 100, FactUnits: fact / 100, EACUnits: eac / 100,
			CompletionPct: &completion, EACCompletionPct: &eacCompletion,
			GapRub: eac - plan, GapUnits: (eac - plan) / 100,
			PlanInvestmentsRub: plan * 0.05, PlanInvestmentsRubNet: plan * 0.05 / 1.2,
			FactInvestmentsRub: fact * 0.05, FactInvestmentsRubNet: fact * 0.05 / 1.2,
			EACInvestmentsRub: eac * 0.05, EACInvestmentsRubNet: eac * 0.05 / 1.2,
			InvestmentVarianceRub: (eac - plan) * 0.05 / 1.2,
			RegistryOpexBudgetRub: 1250000, RegistryOpexBudgetRubNet: 1041666.67,
			ClosedCells: 12, ClosedCellsWithFact: 11, FactCoveragePct: models.PtrFloat(91.7), OpenCellsWithoutForecast: 2,
			PrevFactRub: models.PtrFloat(fact * 0.9), FactYoYPct: models.PtrFloat(11.1),
			PromoCount: 4, PromoOnlineCount: 1, PromoOfflineCount: 3, PromoInvestmentsRub: 300000,
			PromoInvestmentsGTN:  models.NetworkDashboardInvestmentSplit{PlanRub: 200000, PlanRubNet: 166666.67, EACRub: 180000, EACRubNet: 150000},
			PromoInvestmentsOPEX: models.NetworkDashboardInvestmentSplit{PlanRub: 100000, PlanRubNet: 83333.33, FactRub: -1500.55, FactRubNet: -1250.46},
		}
	}
	d := &models.NetworkDashboardResponse{Year: 2026, SelectedQuarters: []int{1, 2}, AvailableYears: []int{2025, 2026}}
	d.Summary = metrics(1284560000.5, 611230450.25, 1192870300.75)
	d.Summary.NetworkCount, d.Summary.BrandCount = 25, 14
	d.Summary.UndistributedRub = models.PtrFloat(1500000)
	// Сопоставимого прошлого года у среза нет: карточка обязана показать
	// прочерк, а не ноль.
	d.Summary.PrevFactRub, d.Summary.PrevFactUnits, d.Summary.FactYoYPct = nil, nil, nil
	for q := 1; q <= 2; q++ {
		d.Quarters = append(d.Quarters, models.NetworkDashboardPeriodPoint{Year: 2026, Quarter: q, Metrics: metrics(640000000, 300000000*float64(3-q), 590000000)})
	}
	for m := 1; m <= 6; m++ {
		d.Months = append(d.Months, models.NetworkDashboardMonthPoint{
			Year: 2026, Month: m, Quarter: (m-1)/3 + 1,
			PlanRub: 213000000, FactRub: 100000000 * float64(7-m) / 6, EACRub: 200000000,
			PlanUnits: 2130000, FactUnits: 1000000, EACUnits: 2000000,
			PrevFactRub: models.PtrFloat(90000000), PrevFactUnits: models.PtrFloat(900000),
			Closed: m <= 4, CellsWithoutForecast: m,
		})
	}
	kams := []string{"Иванова А.", "Петров С.", "Кузнецов Д."}
	for i := 0; i < 25; i++ {
		name := fmt.Sprintf("Сеть %02d", i+1)
		if i == 1 {
			name = "Аллея — сеть супермаркетов Красноярского края и Хакасии с очень длинным названием"
		}
		plan := 100000000 - float64(i)*3000000
		br := models.NetworkDashboardBreakdown{Name: name, NetworkID: models.PtrInt(i + 1), KAM: models.PtrString(kams[i%3]), Metrics: metrics(plan, plan*0.48, plan*0.93)}
		if i == 2 {
			br.Metrics.PrevFactRub, br.Metrics.FactYoYPct = nil, nil
		}
		d.Networks = append(d.Networks, br)
		for q := 1; q <= 2; q++ {
			d.NetworkQuarters = append(d.NetworkQuarters, models.NetworkDashboardCell{NetworkID: i + 1, Name: name, Quarter: q, Metrics: metrics(plan/2, plan*0.24, plan*0.46)})
		}
	}
	for i := 0; i < 14; i++ {
		br := models.NetworkDashboardBreakdown{Name: fmt.Sprintf("Бренд %d", i+1), Metrics: metrics(90000000-float64(i)*5000000, 40000000, 85000000)}
		switch i % 3 {
		case 0:
			br.InGross = ptrBool(true)
		case 1:
			br.InGross = ptrBool(false)
		}
		d.Brands = append(d.Brands, br)
	}
	for _, k := range kams {
		d.KAMs = append(d.KAMs, models.NetworkDashboardBreakdown{Name: k, Metrics: metrics(400000000, 200000000, 390000000)})
	}
	blocks := []string{
		models.ReportBlockCover, models.ReportBlockSummaryKPI, models.ReportBlockPeriodTrend, models.ReportBlockPlanFactEAC,
		models.ReportBlockNetworkTop, models.ReportBlockBrandTop, models.ReportBlockKAMTop, models.ReportBlockGTN,
		models.ReportBlockRegistryOpex, models.ReportBlockPromoInvestments, models.ReportBlockNetworkQuarters, models.ReportBlockMethodology,
	}
	return &models.ReportSnapshot{
		Request: models.ReportRequest{
			Title: "Итоги периода по сетям", Year: 2026, Quarters: []int{1, 2}, Unit: unit,
			Blocks: blocks, Formats: []string{"pdf", "pptx"}, TableLimit: 20,
		},
		Title: "Итоги периода по сетям", Owner: "Роман Станкевич", CreatedAt: "2026-09-16 15:04",
		TemplateVersion: "test", FilterLabels: []string{"Год 2026 · Q1, Q2", "Сети: весь доступный портфель", "КАМ: все"},
		Dashboard: d,
	}
}

// writeSample сохраняет файл для визуальной проверки, если задан REPORT_SAMPLE_DIR.
func writeSample(t *testing.T, name string, data []byte) {
	dir := os.Getenv("REPORT_SAMPLE_DIR")
	if dir == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
}

func TestBuildPagesFollowsRequestedBlocks(t *testing.T) {
	s := sampleSnapshot(models.ReportUnitRub)
	pages, err := BuildPages(s)
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if len(pages) != len(s.Request.Blocks) {
		t.Fatalf("pages = %d, want %d", len(pages), len(s.Request.Blocks))
	}
	for i, p := range pages {
		if p.Block != s.Request.Blocks[i] {
			t.Errorf("page %d = %s, want %s", i, p.Block, s.Request.Blocks[i])
		}
	}
	// Лимит строк: 25 сетей → 20 в топе; в «сеть × квартал» — 20 сетей × 2 квартала.
	if rows := len(pages[4].Table.Rows); rows != 20 {
		t.Errorf("network-top rows = %d, want 20", rows)
	}
	if rows := len(pages[10].Table.Rows); rows != 40 {
		t.Errorf("network-quarters rows = %d, want 40", rows)
	}
	// Отрицательная сумма сохраняет знак и в шкале страницы.
	if sub := pages[9].Cards[2].Sub; !strings.Contains(sub, "факт −1") {
		t.Errorf("promo OPEX fact must keep its sign, got %q", sub)
	}
	// Приложение хранит точные суммы с копейками.
	if got := pages[10].Table.Rows[0][2].Text; !strings.HasSuffix(got, ",00") {
		t.Errorf("appendix must keep kopecks, got %q", got)
	}
	// «Нет данных за прошлый год» — прочерк, не ноль.
	if got := pages[1].Cards[3].Value; got != "—" {
		t.Errorf("prev year = %q, want dash", got)
	}
	// Оценка выполнения окрашивает ячейку и даёт полосу.
	if c := pages[4].Table.Rows[0][6]; c.Bar < 0 || c.Tone == toneNeutral {
		t.Errorf("EAC/plan cell must carry bar and tone: %+v", c)
	}
	for _, p := range pages {
		switch p.Block {
		case models.ReportBlockPeriodTrend, models.ReportBlockPlanFactEAC, models.ReportBlockGTN, models.ReportBlockNetworkTop:
			if p.Chart == nil {
				t.Errorf("%s: chart missing", p.Block)
			}
		}
	}
	if len(pages[3].Chart.Bullet) != 2 || len(pages[2].Chart.Months) != 6 || len(pages[7].Chart.Steps) != 3 {
		t.Errorf("chart specs: bullet=%d months=%d steps=%d", len(pages[3].Chart.Bullet), len(pages[2].Chart.Months), len(pages[7].Chart.Steps))
	}
}

func TestBuildPagesUnitsSwitchVolumes(t *testing.T) {
	pages, err := BuildPages(sampleSnapshot(models.ReportUnitUnits))
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if got := pages[1].Cards[0]; got.Label != "План, млн уп." || got.Value != "12,8" {
		t.Errorf("plan in units = %+v", got)
	}
	// Инвестиции остаются в рублях независимо от единицы объёма.
	if got := pages[7].Cards[0].Label; !strings.Contains(got, "₽") {
		t.Errorf("investments must stay in rubles: %q", got)
	}
}

func TestRenderPDF(t *testing.T) {
	for _, unit := range []string{models.ReportUnitRub, models.ReportUnitUnits} {
		data, err := RenderPDF(sampleSnapshot(unit))
		if err != nil {
			t.Fatalf("RenderPDF(%s): %v", unit, err)
		}
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			t.Fatalf("not a PDF")
		}
		// Страниц не меньше блоков: длинные таблицы добавляют свои.
		if n := bytes.Count(data, []byte("/Type /Page\n")) + bytes.Count(data, []byte("/Type /Page/")); n < 12 {
			t.Errorf("pages = %d, want >= 12", n)
		}
		writeSample(t, "report-"+unit+".pdf", data)
	}
}

func TestRenderPPTX(t *testing.T) {
	s := sampleSnapshot(models.ReportUnitRub)
	data, err := RenderPPTX(s)
	if err != nil {
		t.Fatalf("RenderPPTX: %v", err)
	}
	writeSample(t, "report-rub.pptx", data)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	slides := 0
	var presentation, contentTypes string
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml"):
			slides++
		case f.Name == "ppt/presentation.xml":
			presentation = string(body)
		case f.Name == "[Content_Types].xml":
			contentTypes = string(body)
		}
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			if err := checkWellFormed(body); err != nil {
				t.Errorf("%s: %v", f.Name, err)
			}
		}
	}
	// 12 блоков; длинные таблицы (топ сетей на 20 строк, топ брендов, «сеть ×
	// квартал» на 40) уходят на отдельные слайды, короткие делят слайд с
	// графиком и карточками.
	if slides != 17 {
		t.Errorf("slides = %d, want 17", slides)
	}
	if strings.Count(presentation, "<p:sldId ") != slides {
		t.Errorf("sldIdLst has %d entries for %d slides", strings.Count(presentation, "<p:sldId "), slides)
	}
	if strings.Count(contentTypes, "presentationml.slide+xml") != slides {
		t.Errorf("content types list %d slides of %d", strings.Count(contentTypes, "presentationml.slide+xml"), slides)
	}
}

func TestInspectTemplatePrefersBlankLayout(t *testing.T) {
	info, err := inspectTemplate(pptxTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if info.width < 13 || info.height < 7 {
		t.Errorf("slide size = %.2f×%.2f, want 16:9", info.width, info.height)
	}
	if !strings.HasPrefix(info.layoutTarget, "../slideLayouts/slideLayout") {
		t.Errorf("layout = %q", info.layoutTarget)
	}
}

func checkWellFormed(body []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func ptrBool(v bool) *bool { return &v }
