package handlers

import (
	"fmt"
	"net/http"
	"os"
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

func cleanExpiredSalesExportJobs(now time.Time) {
	salesExportJobStore.Lock()
	defer salesExportJobStore.Unlock()
	for id, job := range salesExportJobStore.jobs {
		if job.Status == "queued" || job.Status == "running" || now.Sub(job.CreatedAt) < salesExportJobTTL {
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

func runSalesExportJob(id string, filter repository.SalesFilter, columns []salesExcelColumn) {
	salesExportJobStore.workers <- struct{}{}
	defer func() { <-salesExportJobStore.workers }()

	updateSalesExportJob(id, func(job *salesExportJob) { job.Status = "running" })
	f, err := buildSalesExcel(filter, columns)
	if err != nil {
		failSalesExportJob(id, err)
		return
	}
	defer f.Close()

	tmp, err := os.CreateTemp("", "internet-sales-*.xlsx")
	if err != nil {
		failSalesExportJob(id, err)
		return
	}
	path := tmp.Name()
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
