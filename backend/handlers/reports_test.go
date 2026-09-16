package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"backend/models"
)

// memoryReportJobs — реестр заданий отчётов для тестов: проверяются переходы
// состояний и файлы, а не SQL.
type memoryReportJobs struct {
	mu   sync.Mutex
	jobs map[string]models.ReportJob
}

func newMemoryReportJobs() *memoryReportJobs {
	return &memoryReportJobs{jobs: map[string]models.ReportJob{}}
}

func (s *memoryReportJobs) Create(job models.ReportJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *memoryReportJobs) ForUser(id, owner string) (models.ReportJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok || job.Owner != owner {
		return models.ReportJob{}, false
	}
	return job, true
}

func (s *memoryReportJobs) ActiveCount(owner string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, job := range s.jobs {
		if job.Owner == owner && (job.Status == models.ReportJobQueued || job.Status == models.ReportJobRunning) {
			n++
		}
	}
	return n, true
}

func (s *memoryReportJobs) update(id string, mutate func(*models.ReportJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	mutate(&job)
	s.jobs[id] = job
}

func (s *memoryReportJobs) SetRunning(id string) {
	s.update(id, func(job *models.ReportJob) { job.Status = models.ReportJobRunning })
}

func (s *memoryReportJobs) SetReady(id, filePath string, completedAt time.Time) {
	s.update(id, func(job *models.ReportJob) {
		job.Status, job.FilePath, job.CompletedAt = models.ReportJobReady, filePath, completedAt
	})
}

func (s *memoryReportJobs) SetFailed(id, message string, completedAt time.Time) {
	s.update(id, func(job *models.ReportJob) {
		job.Status, job.Error, job.CompletedAt = models.ReportJobFailed, message, completedAt
	})
}

func (s *memoryReportJobs) CleanExpired(time.Time) {}

func (s *memoryReportJobs) LiveFilePaths() (map[string]struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths := map[string]struct{}{}
	for _, job := range s.jobs {
		if job.FilePath != "" {
			paths[job.FilePath] = struct{}{}
		}
	}
	return paths, true
}

// useTestReportJobs подменяет реестр, каталог выгрузок и рендер на время теста.
func useTestReportJobs(t *testing.T, render func(string, *models.ReportSnapshot) ([]byte, error)) *memoryReportJobs {
	t.Helper()
	withTestLogger(t)
	store := newMemoryReportJobs()
	prevStore, prevRender := reportJobs, renderReport
	reportJobs, renderReport = store, render
	t.Setenv("REPORT_EXPORT_DIR", t.TempDir())
	t.Cleanup(func() { reportJobs, renderReport = prevStore, prevRender })
	return store
}

func queuedReportJob(store *memoryReportJobs, id string) {
	_ = store.Create(models.ReportJob{ID: id, Owner: "roman", Status: models.ReportJobQueued, Format: "pdf", CreatedAt: time.Now()})
}

func TestRunReportJobWritesFileAndMarksReady(t *testing.T) {
	store := useTestReportJobs(t, func(format string, _ *models.ReportSnapshot) ([]byte, error) {
		return []byte("%PDF-" + format), nil
	})
	queuedReportJob(store, "job-ok")

	runReportJob("job-ok", "pdf", &models.ReportSnapshot{})

	job, _ := store.ForUser("job-ok", "roman")
	if job.Status != models.ReportJobReady || job.FilePath == "" {
		t.Fatalf("job = %+v, want ready with file", job)
	}
	data, err := os.ReadFile(job.FilePath)
	if err != nil || string(data) != "%PDF-pdf" {
		t.Fatalf("file content = %q, err = %v", data, err)
	}
	if filepath.Ext(job.FilePath) != ".pdf" {
		t.Errorf("file ext = %q, want .pdf", filepath.Ext(job.FilePath))
	}
}

func TestRunReportJobMarksFailedOnRenderError(t *testing.T) {
	store := useTestReportJobs(t, func(string, *models.ReportSnapshot) ([]byte, error) {
		return nil, errors.New("boom")
	})
	queuedReportJob(store, "job-err")

	runReportJob("job-err", "pptx", &models.ReportSnapshot{})

	job, _ := store.ForUser("job-err", "roman")
	if job.Status != models.ReportJobFailed || job.Error != reportFailedMessage {
		t.Fatalf("job = %+v, want failed with user-facing message", job)
	}
	entries, _ := os.ReadDir(reportExportDir())
	if len(entries) != 0 {
		t.Errorf("no file must remain after failure, got %d", len(entries))
	}
}

// Паника рендера не должна ронять процесс и оставлять задание «running».
func TestRunReportJobRecoversFromPanic(t *testing.T) {
	store := useTestReportJobs(t, func(string, *models.ReportSnapshot) ([]byte, error) {
		panic("renderer exploded")
	})
	queuedReportJob(store, "job-panic")

	runReportJob("job-panic", "pdf", &models.ReportSnapshot{})

	job, _ := store.ForUser("job-panic", "roman")
	if job.Status != models.ReportJobFailed {
		t.Fatalf("job = %+v, want failed after panic", job)
	}
}

func TestReportFileNameEncodesPeriod(t *testing.T) {
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	s := &models.ReportSnapshot{Request: models.ReportRequest{Year: 2026, Quarters: []int{1, 2}}}
	if got := reportFileName(s, "pptx", now); got != "networks-report_2026-Q1Q2_2026-09-16.pptx" {
		t.Errorf("name = %q", got)
	}
	s.Request.Quarters = nil
	if got := reportFileName(s, "pdf", now); got != "networks-report_2026_2026-09-16.pdf" {
		t.Errorf("name = %q", got)
	}
}
