package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"
	"backend/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultSalesBackgroundExportMaxRows = 1000000
	salesExportJobTTL                   = time.Hour
	// salesExportJobStuckAfter — после этого срока задание, не отчитавшееся ни
	// успехом, ни ошибкой, считается зависшим. Без такого потолка «running»
	// пропускалось чисткой без ограничения по времени и оставалось в реестре
	// навсегда: горутина уже мертва, а клиент опрашивает статус.
	salesExportJobStuckAfter = 30 * time.Minute
	// salesExportDirName — свой подкаталог во временной директории. Файлы
	// выгрузок лежат отдельно от чужих temp-файлов, поэтому уборку при старте
	// можно делать по каталогу, сверяясь с реестром заданий.
	salesExportDirName = "analytics-portal-exports"
	// salesExportWorkers — сколько выгрузок процесс готовит одновременно.
	// Ограничение своё у каждой реплики: это защита её собственной памяти и
	// соединений с БД, а не общий лимит на портал.
	salesExportWorkers = 2
)

// salesExportJobStore — реестр фоновых выгрузок.
//
// За интерфейсом стоит БД: состояние переживает перезапуск и видно всем
// репликам. Тесты подставляют свою реализацию — им нужны только переходы
// состояний, а не SQL.
type salesExportJobStore interface {
	Create(job models.SalesExportJob) error
	ForUser(id, owner string) (models.SalesExportJob, bool)
	SetRunning(id string)
	SetReady(id, filePath string, completedAt time.Time)
	SetFailed(id, message string, completedAt time.Time)
	// CleanExpired закрывает зависшие задания и убирает просроченные вместе с
	// их файлами.
	CleanExpired(now time.Time)
	// LiveFilePaths — файлы, за которыми ещё числится задание. Второе значение
	// false означает, что реестр прочитать не удалось: удалять в этом случае
	// нельзя, иначе под чистку попадёт файл живой выгрузки.
	LiveFilePaths() (map[string]struct{}, bool)
}

var exportJobs salesExportJobStore = dbSalesExportJobs{}

// salesExportWorkerSlots ограничивает число одновременно готовящихся файлов.
// Остаётся в памяти процесса: это его собственный ресурс.
var salesExportWorkerSlots = make(chan struct{}, salesExportWorkers)

// dbSalesExportJobs — реестр в dbo.tbl_SalesExportJobs.
//
// Ошибки записи не возвращаются наружу: задание уже идёт, и остановить его
// поздно — остаётся зафиксировать сбой в логе. Опрос статуса при этом покажет
// прежнее состояние, а зависшее задание закроет чистка по времени.
type dbSalesExportJobs struct{}

func (dbSalesExportJobs) Create(job models.SalesExportJob) error {
	return repository.InsertSalesExportJob(job)
}

func (dbSalesExportJobs) ForUser(id, owner string) (models.SalesExportJob, bool) {
	job, found, err := repository.GetSalesExportJob(id, owner)
	if err != nil {
		config.Logger.Error("sales_export_job_read_failed", "job_id", id, "error", err.Error())
		return models.SalesExportJob{}, false
	}
	return job, found
}

func (dbSalesExportJobs) SetRunning(id string) {
	if err := repository.SetSalesExportJobRunning(id); err != nil {
		config.Logger.Error("sales_export_job_update_failed", "job_id", id, "status", "running", "error", err.Error())
	}
}

func (dbSalesExportJobs) SetReady(id, filePath string, completedAt time.Time) {
	if err := repository.SetSalesExportJobReady(id, filePath, completedAt); err != nil {
		config.Logger.Error("sales_export_job_update_failed", "job_id", id, "status", "ready", "error", err.Error())
	}
}

func (dbSalesExportJobs) SetFailed(id, message string, completedAt time.Time) {
	if err := repository.SetSalesExportJobFailed(id, message, completedAt); err != nil {
		config.Logger.Error("sales_export_job_update_failed", "job_id", id, "status", "failed", "error", err.Error())
	}
}

func (dbSalesExportJobs) CleanExpired(now time.Time) {
	jobs, err := repository.ListSalesExportJobs()
	if err != nil {
		config.Logger.Error("sales_export_jobs_list_failed", "error", err.Error())
		return
	}
	policy := repository.SalesExportJobPolicy{TTL: salesExportJobTTL, StuckAfter: salesExportJobStuckAfter}
	for _, job := range jobs {
		switch repository.SalesExportJobCleanup(job, now, policy) {
		case repository.SalesExportJobFail:
			config.Logger.Warn("sales_background_export_stuck", "job_id", job.ID,
				"age_minutes", int(now.Sub(job.CreatedAt).Minutes()))
			if err := repository.SetSalesExportJobFailed(job.ID, salesExportTimeoutMessage, now); err != nil {
				config.Logger.Error("sales_export_job_update_failed", "job_id", job.ID, "status", "failed", "error", err.Error())
			}
		case repository.SalesExportJobDrop:
			if job.FilePath != "" {
				_ = os.Remove(job.FilePath)
			}
			if err := repository.DeleteSalesExportJob(job.ID); err != nil {
				config.Logger.Error("sales_export_job_delete_failed", "job_id", job.ID, "error", err.Error())
			}
		case repository.SalesExportJobKeep:
		}
	}
}

func (dbSalesExportJobs) LiveFilePaths() (map[string]struct{}, bool) {
	jobs, err := repository.ListSalesExportJobs()
	if err != nil {
		config.Logger.Error("sales_export_jobs_list_failed", "error", err.Error())
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

const (
	salesExportFailedMessage  = "Не удалось подготовить файл"
	salesExportTimeoutMessage = "Выгрузка не завершилась вовремя"
	salesExportLostMessage    = "Файл выгрузки недоступен. Запустите выгрузку заново."
)

func salesBackgroundExportMaxRows() int {
	if raw := strings.TrimSpace(os.Getenv("SALES_BACKGROUND_EXPORT_MAX_ROWS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultSalesBackgroundExportMaxRows
}

// salesExportDir — каталог готовых файлов выгрузки.
// SALES_EXPORT_DIR позволяет вынести их на общий том: временная директория
// контейнера живёт ровно до его пересоздания, а задание теперь переживает и
// перезапуск, и переезд на другую реплику.
func salesExportDir() string {
	if raw := strings.TrimSpace(os.Getenv("SALES_EXPORT_DIR")); raw != "" {
		return raw
	}
	return filepath.Join(os.TempDir(), salesExportDirName)
}

// CleanupSalesExportDir удаляет файлы, за которыми больше не числится задание.
//
// Файлы прошлого запуска не удалил бы никто: чистка по расписанию обходит
// реестр заданий, а этих файлов там уже нет. Живые задания при этом трогать
// нельзя — их файлы могли остаться от прошлого процесса или готовиться
// соседней репликой на общем томе. Вызывается при старте, до приёма запросов.
func CleanupSalesExportDir() {
	dir := salesExportDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			config.Logger.Warn("sales_export_dir_scan_failed", "dir", dir, "error", err.Error())
		}
		return
	}
	live, ok := exportJobs.LiveFilePaths()
	if !ok {
		// Реестр недоступен: без него любой файл может оказаться нужным.
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
		config.Logger.Info("sales_export_orphans_removed", "dir", dir, "files", removed)
	}
}

func failSalesExportJob(id string, err error) {
	exportJobs.SetFailed(id, salesExportFailedMessage, time.Now())
	config.Logger.Error("sales_background_export_failed", "job_id", id, "error", err.Error())
}

// recoverSalesExportJob перехватывает панику фоновой выгрузки.
//
// Задание выполняется в собственной горутине, а значит вне Recovery-middleware
// Gin: без своего recover любая паника внутри buildSalesExcel завершала бы
// процесс целиком — вместе со всеми чужими запросами. Само задание при этом
// нужно перевести в failed: иначе оно навсегда осталось бы «running», а клиент
// продолжал бы опрашивать его статус.
//
// tmpPath указывает на недописанный файл, если паника случилась после его
// создания: он никому не достанется, а сирота остался бы на диске.
func recoverSalesExportJob(id string, tmpPath *string) {
	recovered := recover()
	if recovered == nil {
		return
	}
	if tmpPath != nil && *tmpPath != "" {
		_ = os.Remove(*tmpPath)
	}
	config.Logger.Error("sales_background_export_panic",
		"job_id", id,
		"panic", fmt.Sprint(recovered),
		"stack", string(debug.Stack()),
	)
	failSalesExportJob(id, fmt.Errorf("паника фоновой выгрузки: %v", recovered))
}

func runSalesExportJob(id string, filter repository.SalesFilter, columns []salesExcelColumn) {
	// path объявлен до recover: файл может появиться в любой момент работы, и
	// обработчик паники должен видеть актуальное значение.
	var path string
	defer recoverSalesExportJob(id, &path)

	salesExportWorkerSlots <- struct{}{}
	defer func() { <-salesExportWorkerSlots }()

	exportJobs.SetRunning(id)
	f, err := buildSalesExcel(filter, columns)
	if err != nil {
		failSalesExportJob(id, err)
		return
	}
	defer f.Close()

	dir := salesExportDir()
	if err = os.MkdirAll(dir, 0o750); err != nil {
		failSalesExportJob(id, err)
		return
	}
	tmp, err := os.CreateTemp(dir, "internet-sales-*.xlsx")
	if err != nil {
		failSalesExportJob(id, err)
		return
	}
	path = tmp.Name()
	if err = f.Write(tmp); err != nil {
		tmp.Close()
		_ = os.Remove(path)
		failSalesExportJob(id, err)
		return
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(path)
		failSalesExportJob(id, err)
		return
	}

	exportJobs.SetReady(id, path, time.Now())
	// Файл дошёл до задания: удалять его в обработчике паники больше нельзя.
	path = ""
	time.AfterFunc(salesExportJobTTL, func() { exportJobs.CleanExpired(time.Now()) })
}

// StartSalesExcelExport создаёт фоновую выгрузку. Состояние задания хранится в
// БД; готовый файл лежит в каталоге выгрузок не более часа.
func StartSalesExcelExport(c *gin.Context) {
	exportJobs.CleanExpired(time.Now())
	filter := salesFilterFromQuery(c)
	totalRows, err := repository.SalesRowsCount(filter)
	if err != nil {
		respondSalesError(c, err)
		return
	}
	if totalRows == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Нет данных для выгрузки"})
		return
	}
	limit := salesBackgroundExportMaxRows()
	if totalRows > limit {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("Выгрузка слишком большая: %d строк при лимите %d. Уточните фильтры.", totalRows, limit),
			"total": totalRows,
			"limit": limit,
		})
		return
	}

	username, _ := currentUser(c)
	job := models.SalesExportJob{
		ID:        uuid.NewString(),
		Owner:     username,
		Status:    "queued",
		TotalRows: totalRows,
		FileName:  fmt.Sprintf("internet-sales_%s.xlsx", time.Now().Format("2006-01-02")),
		CreatedAt: time.Now(),
	}
	// Задание заводится до запуска горутины: иначе первый же опрос статуса
	// пришёл бы к пустому реестру и получил 404 на живую выгрузку.
	if err := exportJobs.Create(job); err != nil {
		config.Logger.Error("sales_export_job_create_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось запустить выгрузку"})
		return
	}

	go runSalesExportJob(job.ID, filter, selectedSalesExcelColumns(c.QueryArray("columns")))
	c.JSON(http.StatusAccepted, job)
}

func GetSalesExcelExportJob(c *gin.Context) {
	exportJobs.CleanExpired(time.Now())
	username, _ := currentUser(c)
	job, ok := exportJobs.ForUser(c.Param("id"), username)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Выгрузка не найдена"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func DownloadSalesExcelExport(c *gin.Context) {
	exportJobs.CleanExpired(time.Now())
	username, _ := currentUser(c)
	job, ok := exportJobs.ForUser(c.Param("id"), username)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Выгрузка не найдена"})
		return
	}
	if job.Status != "ready" || job.FilePath == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Файл ещё не готов"})
		return
	}
	// Задание пережило процесс, а файл — не обязательно: временный каталог
	// исчезает вместе с контейнером, и на общем томе файл готовила соседняя
	// реплика. Задание закрываем, чтобы клиент перестал считать его готовым.
	if _, statErr := os.Stat(job.FilePath); statErr != nil {
		config.Logger.Warn("sales_export_file_missing", "job_id", job.ID, "path", job.FilePath)
		exportJobs.SetFailed(job.ID, salesExportLostMessage, time.Now())
		c.JSON(http.StatusConflict, gin.H{"error": salesExportLostMessage})
		return
	}
	c.FileAttachment(job.FilePath, job.FileName)
}
