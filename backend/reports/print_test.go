package reports

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"backend/models"
)

func TestBuildPrintMirrorsPages(t *testing.T) {
	s := sampleSnapshot(models.ReportUnitRub)
	print, err := BuildPrint(s)
	if err != nil {
		t.Fatalf("BuildPrint: %v", err)
	}
	if len(print.Pages) != len(s.Request.Blocks) {
		t.Fatalf("страниц %d, блоков %d", len(print.Pages), len(s.Request.Blocks))
	}
	if print.Title != s.Title || print.Owner != s.Owner || print.Unit != models.ReportUnitRub {
		t.Fatalf("метаданные не перенесены: %+v", print)
	}
	byBlock := map[string]models.ReportPrintPage{}
	for i, p := range print.Pages {
		if p.Block != s.Request.Blocks[i] {
			t.Fatalf("страница %d: блок %s, ожидался %s", i, p.Block, s.Request.Blocks[i])
		}
		byBlock[p.Block] = p
	}

	// Титул: три плитки со спарклайнами у факта и EAC, без таблицы.
	cover := byBlock[models.ReportBlockCover]
	if len(cover.Cards) != 3 || cover.Table != nil || len(cover.Cards[1].Spark) == 0 {
		t.Fatalf("титул: %+v", cover)
	}
	// Кварталы: bullet-график с готовой подписью и оценкой выполнения, таблица с итогом.
	quarters := byBlock[models.ReportBlockPlanFactEAC]
	if quarters.Chart == nil || len(quarters.Chart.Bullet) != 2 || len(quarters.Table.Total) == 0 {
		t.Fatalf("кварталы: %+v", quarters)
	}
	b := quarters.Chart.Bullet[0]
	if b.PctLabel == "" || b.Tone != models.ReportToneWarn || quarters.Chart.Scale.Label != "млрд ₽" {
		t.Fatalf("bullet: %+v, шкала %+v", b, quarters.Chart.Scale)
	}
	// Ячейка выполнения несёт полосу и оценку; обычная — без полосы.
	row := quarters.Table.Rows[0]
	if row[5].Bar == nil || row[5].Tone == models.ReportToneNeutral || row[1].Bar != nil {
		t.Fatalf("ячейки: %+v", row)
	}
	// Месяцы: точки динамики с прошлым годом, шаги — только у GTN.
	if trend := byBlock[models.ReportBlockPeriodTrend]; trend.Chart == nil || len(trend.Chart.Months) != 6 || trend.Chart.Months[0].Prev == nil {
		t.Fatalf("динамика: %+v", trend.Chart)
	}
	if gtn := byBlock[models.ReportBlockGTN]; gtn.Chart == nil || len(gtn.Chart.Steps) != 3 || !gtn.Chart.Steps[2].Total {
		t.Fatalf("GTN: %+v", gtn.Chart)
	}

	data, err := json.MarshalIndent(print, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeSample(t, "sample-print.json", data)
}

func samplePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{0x63, 0x66, 0xf1, 0xff})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRenderPPTXWithImagesEmbedsMedia(t *testing.T) {
	s := sampleSnapshot(models.ReportUnitRub)
	// Снимки есть у титула (0), кварталов (3, короткая таблица под снимком) и
	// топа сетей (4, длинная таблица уходит на следующие слайды).
	images := []SlideImage{
		{Index: 0, PNG: samplePNG(t, 2064, 400), Width: 2064, Height: 400},
		{Index: 3, PNG: samplePNG(t, 2064, 600), Width: 2064, Height: 600},
		{Index: 4, PNG: samplePNG(t, 2064, 900), Width: 2064, Height: 900},
	}
	data, err := RenderPPTX(s, images)
	if err != nil {
		t.Fatalf("RenderPPTX: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			if err := checkWellFormed(b); err != nil {
				t.Fatalf("%s: %v", f.Name, err)
			}
		}
	}
	for _, name := range []string{"ppt/media/image1.png", "ppt/media/image2.png", "ppt/media/image3.png"} {
		if _, ok := files[name]; !ok {
			t.Fatalf("нет media-части %s", name)
		}
	}
	if _, ok := files["ppt/media/image4.png"]; ok {
		t.Fatal("лишняя media-часть: снимков три")
	}
	if !strings.Contains(files["[Content_Types].xml"], `Extension="png"`) {
		t.Fatal("нет типа содержимого для png")
	}
	if !strings.Contains(files["ppt/slides/slide1.xml"], `<p:pic>`) || !strings.Contains(files["ppt/slides/_rels/slide1.xml.rels"], "image1.png") {
		t.Fatal("титул без картинки или без связи на неё")
	}
	// Кварталы: картинка и нативная таблица на одном слайде.
	if sl := files["ppt/slides/slide4.xml"]; !strings.Contains(sl, `<p:pic>`) || !strings.Contains(sl, "<a:tbl>") {
		t.Fatal("слайд кварталов: ожидались картинка и таблица вместе")
	}
	// Топ сетей: слайд с картинкой, таблица (20 строк) — на следующих.
	if sl := files["ppt/slides/slide5.xml"]; !strings.Contains(sl, `<p:pic>`) || strings.Contains(sl, "<a:tbl>") {
		t.Fatal("слайд топа сетей: ожидалась только картинка")
	}
	if sl := files["ppt/slides/slide6.xml"]; !strings.Contains(sl, "<a:tbl>") || !strings.Contains(sl, "(1/2)") {
		t.Fatal("после снимка топа ожидалась таблица с нумерацией продолжения")
	}
}
