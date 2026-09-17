package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend/models"
	"backend/reports/chrome"

	"github.com/gin-gonic/gin"
)

func printTestSnapshot() *models.ReportSnapshot {
	d := &models.NetworkDashboardResponse{Year: 2026, SelectedQuarters: []int{1}}
	d.Summary = models.NetworkDashboardMetrics{PlanRub: 1000, FactRub: 500, EACRub: 900}
	d.Quarters = []models.NetworkDashboardPeriodPoint{{Year: 2026, Quarter: 1, Metrics: d.Summary}}
	d.Months = []models.NetworkDashboardMonthPoint{{Year: 2026, Month: 1, Quarter: 1, PlanRub: 1000, FactRub: 500, EACRub: 900, Closed: true}}
	return &models.ReportSnapshot{
		Request:      models.ReportRequest{Title: "Тест", Year: 2026, Quarters: []int{1}, Unit: "rub", Blocks: []string{models.ReportBlockCover, models.ReportBlockPlanFactEAC}, TableLimit: 10},
		Title:        "Тест",
		Owner:        "roman",
		FilterLabels: []string{"Год 2026 · Q1"},
		Dashboard:    d,
	}
}

func printTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/reports/:id/print", GetReportPrint)
	return r
}

func TestGetReportPrintConsumesTokenOnce(t *testing.T) {
	store := useTestReportJobs(t, nil)
	token, hash, err := newPrintToken()
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, _ := json.Marshal(printTestSnapshot())
	_ = store.Create(models.ReportJob{
		ID: "job-print", Owner: "roman", Status: models.ReportJobRunning, Format: "pdf", CreatedAt: time.Now(),
		SnapshotJSON: string(snapshotJSON), PrintTokenHash: hash, PrintTokenExpiresAt: time.Now().Add(time.Minute),
	})
	router := printTestRouter()

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/reports/job-print/print?token="+token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("first request: code %d body %s", w.Code, w.Body.String())
	}
	var print models.ReportPrint
	if err := json.Unmarshal(w.Body.Bytes(), &print); err != nil {
		t.Fatal(err)
	}
	if print.Title != "Тест" || len(print.Pages) != 2 || print.Pages[1].Chart == nil {
		t.Fatalf("print = %+v", print)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}

	// Повтор с тем же токеном — токен погашен.
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/reports/job-print/print?token="+token, nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("second request: code %d, want 403", w.Code)
	}
}

func TestGetReportPrintRejectsBadTokens(t *testing.T) {
	store := useTestReportJobs(t, nil)
	token, hash, _ := newPrintToken()
	snapshotJSON, _ := json.Marshal(printTestSnapshot())
	_ = store.Create(models.ReportJob{
		ID: "job-expired", Owner: "roman", Status: models.ReportJobRunning, Format: "pdf", CreatedAt: time.Now(),
		SnapshotJSON: string(snapshotJSON), PrintTokenHash: hash, PrintTokenExpiresAt: time.Now().Add(-time.Second),
	})
	_ = store.Create(models.ReportJob{
		ID: "job-ready", Owner: "roman", Status: models.ReportJobReady, Format: "pdf", CreatedAt: time.Now(),
		SnapshotJSON: string(snapshotJSON), PrintTokenHash: hash, PrintTokenExpiresAt: time.Now().Add(time.Minute),
	})
	router := printTestRouter()
	cases := map[string]string{
		"без токена":         "/api/reports/job-expired/print",
		"чужой токен":        "/api/reports/job-expired/print?token=deadbeef",
		"истёкший токен":     "/api/reports/job-expired/print?token=" + token,
		"задание завершено":  "/api/reports/job-ready/print?token=" + token,
		"нет такого задания": "/api/reports/nope/print?token=" + token,
	}
	for name, url := range cases {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: code %d, want 403", name, w.Code)
		}
	}
}

// fakeChrome — Chromium для тестов: отдаёт заранее заданный результат и
// запоминает запрос.
type fakeChrome struct {
	req chrome.Request
	res chrome.Result
	err error
}

func (f *fakeChrome) Render(_ context.Context, req chrome.Request) (chrome.Result, error) {
	f.req = req
	return f.res, f.err
}

func useFakeChrome(t *testing.T, f *fakeChrome) {
	t.Helper()
	prev := newChromeRenderer
	newChromeRenderer = func() (chromeRenderer, error) { return f, nil }
	t.Cleanup(func() { newChromeRenderer = prev })
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRenderReportChromePDF(t *testing.T) {
	f := &fakeChrome{res: chrome.Result{PDF: []byte("%PDF-1.7 chrome")}}
	useFakeChrome(t, f)

	data, err := renderReport(models.ReportFormatPDF, printTestSnapshot(), "http://frontend/print/report/j?token=t")
	if err != nil || string(data) != "%PDF-1.7 chrome" {
		t.Fatalf("data = %q, err = %v", data, err)
	}
	if !f.req.PDF || f.req.Slides || f.req.URL != "http://frontend/print/report/j?token=t" {
		t.Fatalf("request = %+v", f.req)
	}
}

func TestRenderReportChromePPTXUsesSlides(t *testing.T) {
	f := &fakeChrome{res: chrome.Result{Slides: []chrome.Slide{
		{Index: 1, Block: models.ReportBlockPlanFactEAC, PNG: testPNG(t, 2064, 500), Width: 2064, Height: 500},
	}}}
	useFakeChrome(t, f)

	data, err := renderReport(models.ReportFormatPPTX, printTestSnapshot(), "http://frontend/print/report/j?token=t")
	if err != nil {
		t.Fatal(err)
	}
	if f.req.PDF || !f.req.Slides {
		t.Fatalf("request = %+v", f.req)
	}
	if !bytes.Contains(data, []byte("ppt/media/image1.png")) {
		t.Fatal("в PPTX нет снимка секции")
	}
}

func TestRenderReportChromeErrorsFailJob(t *testing.T) {
	f := &fakeChrome{err: errors.New("страница печати не подготовилась вовремя")}
	useFakeChrome(t, f)
	store := useTestReportJobs(t, renderReport)
	queuedReportJob(store, "job-chrome")

	runReportJob("job-chrome", "pdf", printTestSnapshot(), "http://frontend/print/report/job-chrome?token=t")

	job, _ := store.ForUser("job-chrome", "roman")
	if job.Status != models.ReportJobFailed || job.Error != reportFailedMessage {
		t.Fatalf("job = %+v, want failed", job)
	}
}

func TestRenderReportWithoutChromeFails(t *testing.T) {
	t.Setenv("CHROME_WS_URL", "")
	if _, err := renderReport(models.ReportFormatPDF, printTestSnapshot(), "http://frontend/print/report/j?token=t"); !errors.Is(err, errChromeNotConfigured) {
		t.Fatalf("err = %v, want errChromeNotConfigured", err)
	}
	if url := chrome.PrintURL("http://frontend/print/report/", "id-1", "tok"); !strings.HasSuffix(url, "/print/report/id-1?token=tok") {
		t.Errorf("print url = %s", url)
	}
}
