package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	"backend/reports/chrome"
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

// ─── Рендер через Chromium ───────────────────────────────────────────────────
//
// PDF и снимки слайдов делает headless Chromium (reports/chrome) со страницы
// фронтенда /print/report/:id: адрес DevTools — CHROME_WS_URL, адрес страницы
// — REPORT_PRINT_BASE_URL. Страница получает снимок по одноразовому токену
// задания: у Chromium нет сессии пользователя, и давать её ему нельзя.

const (
	// reportPrintTokenTTL — срок токена печати: с запасом больше времени
	// задания целиком, чтобы очередь на единственный Chromium не гасила токен.
	reportPrintTokenTTL = 15 * time.Minute
	// reportPageTimeout — сколько ждать готовности страницы печати.
	reportPageTimeout = 60 * time.Second
	// reportRenderTimeout — задание целиком: открытие, печать, снимки.
	reportRenderTimeout = 3 * time.Minute
)

var errChromeNotConfigured = errors.New("сервис печати не настроен: не задан CHROME_WS_URL")

func reportPrintBaseURL() string {
	if raw := strings.TrimSpace(os.Getenv("REPORT_PRINT_BASE_URL")); raw != "" {
		return raw
	}
	return "http://frontend/print/report"
}

// chromeRenderer — что умеет Chromium; тесты подставляют фейк.
type chromeRenderer interface {
	Render(ctx context.Context, req chrome.Request) (chrome.Result, error)
}

var newChromeRenderer = func() (chromeRenderer, error) {
	wsURL := strings.TrimSpace(os.Getenv("CHROME_WS_URL"))
	if wsURL == "" {
		return nil, errChromeNotConfigured
	}
	return chrome.New(wsURL, reportPageTimeout), nil
}

// newPrintToken — случайный токен и его хэш для хранения.
func newPrintToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(raw)
	return token, hashPrintToken(token), nil
}

func hashPrintToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// reportJobStore — реестр заданий; тесты подставляют реализацию в памяти.
type reportJobStore interface {
	Create(job models.ReportJob) error
	ForUser(id, owner string) (models.ReportJob, bool)
	// ConsumePrintToken отдаёт снимок по хэшу токена печати и гасит токен.
	ConsumePrintToken(id, tokenHash string, now time.Time) (snapshotJSON string, ok bool)
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

func (dbReportJobs) ConsumePrintToken(id, tokenHash string, now time.Time) (string, bool) {
	snapshot, ok, err := repository.ConsumeReportPrintToken(id, tokenHash, now)
	if err != nil {
		config.Logger.Error("report_print_token_failed", "job_id", id, "error", err.Error())
		return "", false
	}
	return snapshot, ok
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

// renderReport — рендер по формату; подменяется в тестах. printURL — адрес
// страницы печати с токеном. PDF печатается в Chromium целиком; для PPTX
// снимаются графические части секций, а слайды собираются с нативными
// заголовками, примечаниями и таблицами.
var renderReport = func(format string, snapshot *models.ReportSnapshot, printURL string) ([]byte, error) {
	renderer, err := newChromeRenderer()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), reportRenderTimeout)
	defer cancel()
	switch format {
	case models.ReportFormatPDF:
		res, err := renderer.Render(ctx, chrome.Request{URL: printURL, PDF: true})
		if err != nil {
			return nil, err
		}
		if len(res.PDF) == 0 {
			return nil, errors.New("chromium вернул пустой PDF")
		}
		return res.PDF, nil
	case models.ReportFormatPPTX:
		res, err := renderer.Render(ctx, chrome.Request{URL: printURL, Slides: true})
		if err != nil {
			return nil, err
		}
		images := make([]reports.SlideImage, 0, len(res.Slides))
		for _, sl := range res.Slides {
			images = append(images, reports.SlideImage{Index: sl.Index, PNG: sl.PNG, Width: sl.Width, Height: sl.Height})
		}
		return reports.RenderPPTX(snapshot, images)
	}
	return nil, fmt.Errorf("неизвестный формат %q", format)
}

func runReportJob(id, format string, snapshot *models.ReportSnapshot, printURL string) {
	var path string
	defer recoverReportJob(id, &path)

	reportWorkerSlots <- struct{}{}
	defer func() { <-reportWorkerSlots }()

	reportJobs.SetRunning(id)
	data, err := renderReport(format, snapshot, printURL)
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
		token, tokenHash, err := newPrintToken()
		if err != nil {
			config.Logger.Error("report_print_token_failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось запустить подготовку отчёта"})
			return
		}
		job := models.ReportJob{
			ID:                  uuid.NewString(),
			Owner:               username,
			Status:              models.ReportJobQueued,
			Format:              format,
			Title:               snapshot.Title,
			FileName:            reportFileName(snapshot, format, now),
			CreatedAt:           now,
			SnapshotJSON:        string(snapshotJSON),
			PrintTokenHash:      tokenHash,
			PrintTokenExpiresAt: now.Add(reportPrintTokenTTL),
		}
		// Задание заводится до горутины: первый опрос статуса не должен
		// получить 404 на живую подготовку.
		if err := reportJobs.Create(job); err != nil {
			config.Logger.Error("report_job_create_failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось запустить подготовку отчёта"})
			return
		}
		go runReportJob(job.ID, format, snapshot, chrome.PrintURL(reportPrintBaseURL(), job.ID, token))
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

// GetReportPrint — печатная модель для страницы /print/report/:id. Роут
// публичный: вместо JWT — одноразовый токен задания из query; после выдачи
// токен погашен, и повторный запрос, как и чужой или истёкший, получает 403.
// Причины не различаются намеренно: страницу открывает Chromium, а не
// человек, а подбор токена по коду ответа лишнего не должен узнавать.
func GetReportPrint(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет токена печати"})
		return
	}
	snapshotJSON, ok := reportJobs.ConsumePrintToken(c.Param("id"), hashPrintToken(token), time.Now())
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Токен печати недействителен"})
		return
	}
	var snapshot models.ReportSnapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		config.Logger.Error("report_snapshot_unmarshal_failed", "job_id", c.Param("id"), "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Снимок отчёта повреждён"})
		return
	}
	print, err := reports.BuildPrint(&snapshot)
	if err != nil {
		config.Logger.Error("report_print_build_failed", "job_id", c.Param("id"), "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось собрать печатную модель"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, print)
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
