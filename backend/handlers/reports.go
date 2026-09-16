package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"backend/config"
	"backend/models"
	"backend/reports"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ─── Отчёты PDF/PPTX по витрине реестра сетей ───────────────────────────────
//
// Модель фоновых заданий повторяет Excel-выгрузку продаж
// (sales_export_jobs.go): реестр в БД, файл во временном каталоге, TTL, чистка
// зависших и сирот, recover от паники. Отличие — снимок: витрина читается
// один раз в обработчике, и оба формата собираются из него в горутинах.

const (
	reportJobTTL        = time.Hour
	reportJobStuckAfter = 30 * time.Minute
	reportExportDirName = "analytics-portal-reports"
	// reportWorkers — сколько файлов процесс готовит одновременно: рендер
	// держит в памяти растры графиков и шрифты, второго потока хватает.
	reportWorkers = 2
	// reportActiveJobsPerUser — потолок незавершённых заданий одного
	// пользователя: одна операция «PDF и PPTX» и ничего сверх, пока она идёт.
	reportActiveJobsPerUser = 2
)

const (
	reportFailedMessage  = "Не удалось подготовить отчёт"
	reportTimeoutMessage = "Подготовка отчёта не завершилась вовремя"
	reportLostMessage    = "Файл отчёта недоступен. Сформируйте отчёт заново."
)

// reportJobStore — реестр заданий; тесты подставляют реализацию в памяти.
type reportJobStore interface {
	Create(job models.ReportJob) error
	ForUser(id, owner string) (models.ReportJob, bool)
	ActiveCount(owner string) (int, bool)
	SetRunning(id string)
	SetReady(id, filePath string, completedAt time.Time)
	SetFailed(id, message string, completedAt time.Time)
	CleanExpired(now time.Time)
	LiveFilePaths() (map[string]struct{}, bool)
}

var reportJobs reportJobStore = dbReportJobs{}

var reportWorkerSlots = make(chan struct{}, reportWorkers)

type dbReportJobs struct{}

func (dbReportJobs) Create(job models.ReportJob) error { return repository.InsertReportJob(job) }

func (dbReportJobs) ForUser(id, owner string) (models.ReportJob, bool) {
	job, found, err := repository.GetReportJob(id, owner)
	if err != nil {
		config.Logger.Error("report_job_read_failed", "job_id", id, "error", err.Error())
		return models.ReportJob{}, false
	}
	return job, found
}

func (dbReportJobs) ActiveCount(owner string) (int, bool) {
	n, err := repository.CountActiveReportJobs(owner)
	if err != nil {
		config.Logger.Error("report_jobs_count_failed", "error", err.Error())
		return 0, false
	}
	return n, true
}

func (dbReportJobs) SetRunning(id string) {
	if err := repository.SetReportJobRunning(id); err != nil {
		config.Logger.Error("report_job_update_failed", "job_id", id, "status", "running", "error", err.Error())
	}
}

func (dbReportJobs) SetReady(id, filePath string, completedAt time.Time) {
	if err := repository.SetReportJobReady(id, filePath, completedAt); err != nil {
		config.Logger.Error("report_job_update_failed", "job_id", id, "status", "ready", "error", err.Error())
	}
}

func (dbReportJobs) SetFailed(id, message string, completedAt time.Time) {
	if err := repository.SetReportJobFailed(id, message, completedAt); err != nil {
		config.Logger.Error("report_job_update_failed", "job_id", id, "status", "failed", "error", err.Error())
	}
}

func (dbReportJobs) CleanExpired(now time.Time) {
	jobs, err := repository.ListReportJobs()
	if err != nil {
		config.Logger.Error("report_jobs_list_failed", "error", err.Error())
		return
	}
	policy := repository.SalesExportJobPolicy{TTL: reportJobTTL, StuckAfter: reportJobStuckAfter}
	for _, job := range jobs {
		switch repository.ReportJobCleanup(job, now, policy) {
		case repository.SalesExportJobFail:
			config.Logger.Warn("report_job_stuck", "job_id", job.ID, "age_minutes", int(now.Sub(job.CreatedAt).Minutes()))
			if err := repository.SetReportJobFailed(job.ID, reportTimeoutMessage, now); err != nil {
				config.Logger.Error("report_job_update_failed", "job_id", job.ID, "status", "failed", "error", err.Error())
			}
		case repository.SalesExportJobDrop:
			if job.FilePath != "" {
				_ = os.Remove(job.FilePath)
			}
			if err := repository.DeleteReportJob(job.ID); err != nil {
				config.Logger.Error("report_job_delete_failed", "job_id", job.ID, "error", err.Error())
			}
		case repository.SalesExportJobKeep:
		}
	}
}

func (dbReportJobs) LiveFilePaths() (map[string]struct{}, bool) {
	jobs, err := repository.ListReportJobs()
	if err != nil {
		config.Logger.Error("report_jobs_list_failed", "error", err.Error())
		return nil, false
	}
	paths := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		if job.FilePath != "" {
			paths[job.FilePath] = struct{}{}
		}
	}
	return paths, true
}

// reportExportDir — каталог готовых отчётов; REPORT_EXPORT_DIR выносит его на
// общий том, как SALES_EXPORT_DIR у Excel-выгрузок.
func reportExportDir() string {
	if raw := strings.TrimSpace(os.Getenv("REPORT_EXPORT_DIR")); raw != "" {
		return raw
	}
	return filepath.Join(os.TempDir(), reportExportDirName)
}

// CleanupReportExportDir удаляет файлы, за которыми не числится задание.
// Вызывается при старте, до приёма запросов.
func CleanupReportExportDir() {
	dir := reportExportDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			config.Logger.Warn("report_export_dir_scan_failed", "dir", dir, "error", err.Error())
		}
		return
	}
	live, ok := reportJobs.LiveFilePaths()
	if !ok {
		return
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if _, busy := live[path]; busy {
			continue
		}
		if err := os.Remove(path); err == nil {
			removed++
		}
	}
	if removed > 0 {
		config.Logger.Info("report_export_orphans_removed", "dir", dir, "files", removed)
	}
}

func failReportJob(id string, err error) {
	reportJobs.SetFailed(id, reportFailedMessage, time.Now())
	config.Logger.Error("report_job_failed", "job_id", id, "error", err.Error())
}

// recoverReportJob перехватывает панику рендера: горутина живёт вне
// Recovery-middleware Gin, и без своего recover паника уронила бы процесс.
func recoverReportJob(id string, tmpPath *string) {
	recovered := recover()
	if recovered == nil {
		return
	}
	if tmpPath != nil && *tmpPath != "" {
		_ = os.Remove(*tmpPath)
	}
	config.Logger.Error("report_job_panic", "job_id", id, "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
	failReportJob(id, fmt.Errorf("паника подготовки отчёта: %v", recovered))
}

// renderReport — рендер по формату; подменяется в тестах.
var renderReport = func(format string, snapshot *models.ReportSnapshot) ([]byte, error) {
	switch format {
	case models.ReportFormatPDF:
		return reports.RenderPDF(snapshot)
	case models.ReportFormatPPTX:
		return reports.RenderPPTX(snapshot)
	}
	return nil, fmt.Errorf("неизвестный формат %q", format)
}

func runReportJob(id, format string, snapshot *models.ReportSnapshot) {
	var path string
	defer recoverReportJob(id, &path)

	reportWorkerSlots <- struct{}{}
	defer func() { <-reportWorkerSlots }()

	reportJobs.SetRunning(id)
	data, err := renderReport(format, snapshot)
	if err != nil {
		failReportJob(id, err)
		return
	}
	dir := reportExportDir()
	if err = os.MkdirAll(dir, 0o750); err != nil {
		failReportJob(id, err)
		return
	}
	tmp, err := os.CreateTemp(dir, "report-*."+format)
	if err != nil {
		failReportJob(id, err)
		return
	}
	path = tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(path)
		failReportJob(id, err)
		return
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(path)
		failReportJob(id, err)
		return
	}
	reportJobs.SetReady(id, path, time.Now())
	path = ""
	time.AfterFunc(reportJobTTL, func() { reportJobs.CleanExpired(time.Now()) })
}

// GetReportBlocks — каталог блоков для конструктора.
func GetReportBlocks(c *gin.Context) {
	c.JSON(http.StatusOK, services.ReportBlocks())
}

// CreateReport принимает запрос конструктора, читает витрину по тем же
// правилам области, что и GET /api/networks/dashboard, и заводит по заданию
// на формат. Оба формата собираются из одного снимка.
func CreateReport(c *gin.Context) {
	reportJobs.CleanExpired(time.Now())
	var req models.ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	ownKAM, ok := networkOwnKAM(c)
	if !ok {
		return
	}
	req, err := services.NormalizeReportRequest(req, ownKAM)
	if err != nil {
		var verr services.ReportValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": verr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось проверить запрос"})
		return
	}

	username, _ := currentUser(c)
	active, ok := reportJobs.ActiveCount(username)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось запустить подготовку отчёта"})
		return
	}
	if active+len(req.Formats) > reportActiveJobsPerUser {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Предыдущий отчёт ещё готовится. Дождитесь его завершения."})
		return
	}

	now := time.Now()
	snapshot, err := services.BuildReportSnapshot(req, username, ownKAM, now)
	if err != nil {
		config.Logger.Error("report_snapshot_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось собрать данные отчёта"})
		return
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		config.Logger.Error("report_snapshot_marshal_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось собрать данные отчёта"})
		return
	}

	response := models.ReportCreateResponse{Jobs: make([]models.ReportJobStatus, 0, len(req.Formats))}
	for _, format := range req.Formats {
		job := models.ReportJob{
			ID:           uuid.NewString(),
			Owner:        username,
			Status:       models.ReportJobQueued,
			Format:       format,
			Title:        snapshot.Title,
			FileName:     reportFileName(snapshot, format, now),
			CreatedAt:    now,
			SnapshotJSON: string(snapshotJSON),
		}
		// Задание заводится до горутины: первый опрос статуса не должен
		// получить 404 на живую подготовку.
		if err := reportJobs.Create(job); err != nil {
			config.Logger.Error("report_job_create_failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось запустить подготовку отчёта"})
			return
		}
		go runReportJob(job.ID, format, snapshot)
		response.Jobs = append(response.Jobs, job.StatusView())
	}
	c.JSON(http.StatusAccepted, response)
}

// reportFileName — имя для скачивания: период и дата формирования, без
// пользовательского заголовка, чтобы не чистить его от недопустимых символов.
func reportFileName(s *models.ReportSnapshot, format string, now time.Time) string {
	period := fmt.Sprintf("%d", s.Request.Year)
	if len(s.Request.Quarters) > 0 && len(s.Request.Quarters) < 4 {
		parts := make([]string, 0, len(s.Request.Quarters))
		for _, q := range s.Request.Quarters {
			parts = append(parts, fmt.Sprintf("Q%d", q))
		}
		period += "-" + strings.Join(parts, "")
	}
	return fmt.Sprintf("networks-report_%s_%s.%s", period, now.Format("2006-01-02"), format)
}

func GetReportJob(c *gin.Context) {
	reportJobs.CleanExpired(time.Now())
	username, _ := currentUser(c)
	job, ok := reportJobs.ForUser(c.Param("id"), username)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Отчёт не найден"})
		return
	}
	c.JSON(http.StatusOK, job.StatusView())
}

func DownloadReport(c *gin.Context) {
	reportJobs.CleanExpired(time.Now())
	username, _ := currentUser(c)
	job, ok := reportJobs.ForUser(c.Param("id"), username)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Отчёт не найден"})
		return
	}
	if job.Status != models.ReportJobReady || job.FilePath == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Файл ещё не готов"})
		return
	}
	if _, statErr := os.Stat(job.FilePath); statErr != nil {
		config.Logger.Warn("report_file_missing", "job_id", job.ID, "path", job.FilePath)
		reportJobs.SetFailed(job.ID, reportLostMessage, time.Now())
		c.JSON(http.StatusConflict, gin.H{"error": reportLostMessage})
		return
	}
	c.FileAttachment(job.FilePath, job.FileName)
}
