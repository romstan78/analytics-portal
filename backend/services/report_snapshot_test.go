package services

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"backend/models"
)

func validReportRequest() models.ReportRequest {
	return models.ReportRequest{
		Year:     2026,
		Quarters: []int{2, 1, 2},
		Blocks:   []string{models.ReportBlockMethodology, models.ReportBlockNetworkTop, models.ReportBlockSummaryKPI},
		Formats:  []string{models.ReportFormatPDF, models.ReportFormatPDF, models.ReportFormatPPTX},
	}
}

func TestNormalizeReportRequestFillsDefaultsAndOrder(t *testing.T) {
	got, err := NormalizeReportRequest(validReportRequest(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != ReportDefaultTitle {
		t.Errorf("title = %q, want default", got.Title)
	}
	if !reflect.DeepEqual(got.Quarters, []int{1, 2}) {
		t.Errorf("quarters = %v, want [1 2]", got.Quarters)
	}
	if got.Unit != models.ReportUnitRub || got.TableLimit != 20 {
		t.Errorf("unit/limit = %q/%d, want rub/20", got.Unit, got.TableLimit)
	}
	if !reflect.DeepEqual(got.Formats, []string{"pdf", "pptx"}) {
		t.Errorf("formats = %v", got.Formats)
	}
	// cover всегда первый, methodology всегда последний, остальное — в порядке запроса.
	want := []string{models.ReportBlockCover, models.ReportBlockNetworkTop, models.ReportBlockSummaryKPI, models.ReportBlockMethodology}
	if !reflect.DeepEqual(got.Blocks, want) {
		t.Errorf("blocks = %v, want %v", got.Blocks, want)
	}
}

func TestNormalizeReportRequestRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*models.ReportRequest)
	}{
		{"year", func(r *models.ReportRequest) { r.Year = 1999 }},
		{"quarter", func(r *models.ReportRequest) { r.Quarters = []int{5} }},
		{"unit", func(r *models.ReportRequest) { r.Unit = "kg" }},
		{"format", func(r *models.ReportRequest) { r.Formats = []string{"docx"} }},
		{"no formats", func(r *models.ReportRequest) { r.Formats = nil }},
		{"unknown block", func(r *models.ReportRequest) { r.Blocks = []string{"weather"} }},
		{"only cover", func(r *models.ReportRequest) { r.Blocks = []string{models.ReportBlockCover} }},
		{"limit", func(r *models.ReportRequest) { r.TableLimit = 15 }},
		{"network id", func(r *models.ReportRequest) { r.NetworkIDs = []int{0} }},
	}
	for _, tc := range cases {
		req := validReportRequest()
		tc.mut(&req)
		_, err := NormalizeReportRequest(req, "")
		var verr ReportValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: want ReportValidationError, got %v", tc.name, err)
		}
	}
}

// Закреплённый КАМ не выбирает КАМов и не видит разреза по КАМам: его область
// задаёт сервер, а разрез выродился бы в одну строку.
func TestNormalizeReportRequestAppliesKAMScope(t *testing.T) {
	req := validReportRequest()
	req.KAMs = []string{"Иванова А."}
	got, err := NormalizeReportRequest(req, "Петров С.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.KAMs != nil {
		t.Errorf("kams = %v, want nil for scoped KAM", got.KAMs)
	}
	filter := ReportDashboardFilter(got, "Петров С.")
	if filter.OwnKAM != "Петров С." || filter.KAMs != nil {
		t.Errorf("filter = %+v, want own KAM only", filter)
	}

	req.Blocks = append(req.Blocks, models.ReportBlockKAMTop)
	if _, err := NormalizeReportRequest(req, "Петров С."); err == nil {
		t.Error("kam-top must be rejected for scoped KAM")
	}
	if _, err := NormalizeReportRequest(req, ""); err != nil {
		t.Errorf("kam-top must be allowed without scope: %v", err)
	}
}

func TestSnapshotFromDashboardLabelsScope(t *testing.T) {
	req, _ := NormalizeReportRequest(validReportRequest(), "")
	req.NetworkIDs = []int{7, 9}
	dashboard := &models.NetworkDashboardResponse{
		Year: 2026, SelectedQuarters: []int{1, 2},
		Networks: []models.NetworkDashboardBreakdown{
			{Name: "Магнит", NetworkID: models.PtrInt(7)},
			{Name: "Лента", NetworkID: models.PtrInt(9)},
		},
	}
	now := time.Date(2026, 9, 16, 15, 4, 0, 0, time.UTC)
	snap := SnapshotFromDashboard(req, "roman", "", now, dashboard)
	want := []string{"Год 2026 · Q1, Q2", "Сети: Магнит, Лента", "КАМ: все"}
	if !reflect.DeepEqual(snap.FilterLabels, want) {
		t.Errorf("labels = %v, want %v", snap.FilterLabels, want)
	}
	if snap.CreatedAt != "2026-09-16 15:04 UTC" || snap.Owner != "roman" || snap.TemplateVersion != ReportTemplateVersion {
		t.Errorf("metadata = %+v", snap)
	}
	if snap.Dashboard != dashboard {
		t.Error("snapshot must carry the dashboard response as is")
	}

	scoped := SnapshotFromDashboard(req, "kam", "Петров С.", now, dashboard)
	if scoped.FilterLabels[2] != "КАМ: Петров С. (область закрепления)" {
		t.Errorf("scoped label = %q", scoped.FilterLabels[2])
	}
}
