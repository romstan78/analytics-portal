package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"backend/config"
	"backend/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultSalesBackgroundExportMaxRows = 1000000
	salesExportJobTTL                   = time.Hour
	// salesExportJobStuckAfter — после этого срока задание, не отчитавшееся ни
	// успехом, ни ошибкой, считается зависшим. Без такого потолка «running»
	// пропускалось чисткой без ограничения по времени и оставалось в памяти
	// навсегда: горутина уже мертва, а клиент опрашивает статус до перезапуска.
	salesExportJobStuckAfter = 30 * time.Minute
	// salesExportDirName — свой подкаталог во временной директории. Файлы
	// выгрузок лежат отдельно от чужих temp-файлов, поэтому уборку при старте
	// можно делать по каталогу, а не по карте заданий, которая после
	// перезапуска пуста.
	salesExportDirName = "analytics-portal-exports"
)

type salesExportJob struct {
	ID          string    `json:"id"`
	Owner       string    `json:"-"`
	Status      string    `json:"status"`
	TotalRows   int       `json:"totalRows"`
	FileName    string    `json:"fileName"`
	FilePath    string    `json:"-"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

var salesExportJobStore = struct {
	sync.RWMutex
	jobs    map[string]*salesExportJob
	workers chan struct{}
}{
	jobs:    make(map[string]*salesExportJob),
	workers: make(chan struct{}, 2),
}

func salesBackgroundExportMaxRows() int {
	if raw := strings.TrimSpace(os.Getenv("SALES_BACKGROUND_EXPORT_MAX_ROWS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultSalesBackgroundExportMaxRows
}

// salesExportDir — каталог готовых файлов выгрузки.
// SALES_EXPORT_DIR позволяет вынести их на отдельный том: временная директория
// контейнера живёт ровно до его пересоздания.
func salesExportDir() string {
	if raw := strings.TrimSpace(os.Getenv("SALES_EXPORT_DIR")); raw != "" {
		return raw
	}
	return filepath.Join(os.TempDir(), salesExportDirName)
}

// CleanupSalesExportDir удаляет файлы выгрузок, оставшиеся от прошлого запуска.
//
// Состояние заданий живёт в памяти процесса, поэтому после перезапуска карта
// пуста — а чистка по расписанию обходит именно её. Файлы предыдущего процесса
// не удалил бы никто: за ними больше не числится ни одного задания. Вызывается
// при старте, до приёма запросов.
func CleanupSalesExportDir() {
	dir := salesExportDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			config.Logger.Warn("sales_export_dir_scan_failed", "dir", dir, "error", err.Error())
		}
		return
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err == nil {
			removed++
		}
	}
	if removed > 0 {
		config.Logger.Info("sales_export_orphans_removed", "dir", dir, "files", removed)
	}
}

func cleanExpiredSalesExportJobs(now time.Time) {
	salesExportJobStore.Lock()
	defer salesExportJobStore.Unlock()
	for id, job := range salesExportJobStore.jobs {
		if job.Status == "queued" || job.Status == "running" {
			// Зависшее задание закрываем сами: горутина, которая перевела бы
			// его в failed, уже не отчитается — процесс мог быть перезапущен
			// или задание оборвалось иначе.
			if now.Sub(job.CreatedAt) >= salesExportJobStuckAfter {
				job.Status = "failed"
				job.Error = "Выгрузка не завершилась вовремя"
				job.CompletedAt = now
				config.Logger.Warn("sales_background_export_stuck", "job_id", id,
					"age_minutes", int(now.Sub(job.CreatedAt).Minutes()))
			}
			continue
		}
		if now.Sub(job.CreatedAt) < salesExportJobTTL {
			continue
		}
		if job.FilePath != "" {
			_ = os.Remove(job.FilePath)
		}
		delete(salesExportJobStore.jobs, id)
	}
}

func salesExportJobForUser(id, owner string) (salesExportJob, bool) {
	salesExportJobStore.RLock()
	defer salesExportJobStore.RUnlock()
	job, ok := salesExportJobStore.jobs[id]
	if !ok || job.Owner != owner {
		return salesExportJob{}, false
	}
	return *job, true
}

func updateSalesExportJob(id string, update func(*salesExportJob)) {
	salesExportJobStore.Lock()
	defer salesExportJobStore.Unlock()
	if job, ok := salesExportJobStore.jobs[id]; ok {
		update(job)
	}
}

func failSalesExportJob(id string, err error) {
	updateSalesExportJob(id, func(job *salesExportJob) {
		job.Status = "failed"
		job.Error = "Не удалось подготовить файл"
		job.CompletedAt = time.Now()
	})
	config.Logger.Error("sales_background_export_failed", "job_id", id, "error", err.Error())
	time.AfterFunc(salesExportJobTTL, func() { cleanExpiredSalesExportJobs(time.Now()) })
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

	salesExportJobStore.workers <- struct{}{}
	defer func() { <-salesExportJobStore.workers }()

	updateSalesExportJob(id, func(job *salesExportJob) { job.Status = "running" })
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

	updateSalesExportJob(id, func(job *salesExportJob) {
		job.Status = "ready"
		job.FilePath = path
		job.CompletedAt = time.Now()
	})
	// Файл дошёл до задания: удалять его в обработчике паники больше нельзя.
	path = ""
	time.AfterFunc(salesExportJobTTL, func() { cleanExpiredSalesExportJobs(time.Now()) })
}

// StartSalesExcelExport создаёт фоновую выгрузку. В памяти хранится только
// состояние задания; готовый файл лежит во временной директории не более часа.
func StartSalesExcelExport(c *gin.Context) {
	cleanExpiredSalesExportJobs(time.Now())
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
	job := &salesExportJob{
		ID:        uuid.NewString(),
		Owner:     username,
		Status:    "queued",
		TotalRows: totalRows,
		FileName:  fmt.Sprintf("internet-sales_%s.xlsx", time.Now().Format("2006-01-02")),
		CreatedAt: time.Now(),
	}
	salesExportJobStore.Lock()
	salesExportJobStore.jobs[job.ID] = job
	salesExportJobStore.Unlock()

	go runSalesExportJob(job.ID, filter, selectedSalesExcelColumns(c.QueryArray("columns")))
	c.JSON(http.StatusAccepted, job)
}

func GetSalesExcelExportJob(c *gin.Context) {
	cleanExpiredSalesExportJobs(time.Now())
	username, _ := currentUser(c)
	job, ok := salesExportJobForUser(c.Param("id"), username)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Выгрузка не найдена"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func DownloadSalesExcelExport(c *gin.Context) {
	cleanExpiredSalesExportJobs(time.Now())
	username, _ := currentUser(c)
	job, ok := salesExportJobForUser(c.Param("id"), username)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Выгрузка не найдена"})
		return
	}
	if job.Status != "ready" || job.FilePath == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Файл ещё не готов"})
		return
	}
	c.FileAttachment(job.FilePath, job.FileName)
}
