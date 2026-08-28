package handlers

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"backend/models"
	"backend/repository"
)

// memorySalesExportJobs — реестр заданий для тестов.
//
// Настоящий живёт в БД; проверять здесь нужно поведение обработчиков —
// переходы состояний и уборку файлов, — а не SQL. Правила «зависшее/
// просроченное» проверяются отдельно и без базы: repository.SalesExportJobCleanup.
type memorySalesExportJobs struct {
	mu sync.Mutex
	// listFails — реестр недоступен: уборка каталога в этом случае обязана
	// ничего не удалять.
	listFails bool
	jobs      map[string]models.SalesExportJob
}

func newMemorySalesExportJobs() *memorySalesExportJobs {
	return &memorySalesExportJobs{jobs: map[string]models.SalesExportJob{}}
}

func (s *memorySalesExportJobs) Create(job models.SalesExportJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *memorySalesExportJobs) ForUser(id, owner string) (models.SalesExportJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok || job.Owner != owner {
		return models.SalesExportJob{}, false
	}
	return job, true
}

func (s *memorySalesExportJobs) update(id string, mutate func(*models.SalesExportJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	mutate(&job)
	s.jobs[id] = job
}

func (s *memorySalesExportJobs) SetRunning(id string) {
	s.update(id, func(job *models.SalesExportJob) { job.Status = "running" })
}

func (s *memorySalesExportJobs) SetReady(id, filePath string, completedAt time.Time) {
	s.update(id, func(job *models.SalesExportJob) {
		job.Status = "ready"
		job.FilePath = filePath
		job.CompletedAt = completedAt
	})
}

func (s *memorySalesExportJobs) SetFailed(id, message string, completedAt time.Time) {
	s.update(id, func(job *models.SalesExportJob) {
		job.Status = "failed"
		job.Error = message
		job.CompletedAt = completedAt
	})
}

func (s *memorySalesExportJobs) CleanExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy := repository.SalesExportJobPolicy{TTL: salesExportJobTTL, StuckAfter: salesExportJobStuckAfter}
	for id, job := range s.jobs {
		switch repository.SalesExportJobCleanup(job, now, policy) {
		case repository.SalesExportJobFail:
			job.Status = "failed"
			job.Error = salesExportTimeoutMessage
			job.CompletedAt = now
			s.jobs[id] = job
		case repository.SalesExportJobDrop:
			if job.FilePath != "" {
				_ = os.Remove(job.FilePath)
			}
			delete(s.jobs, id)
		case repository.SalesExportJobKeep:
		}
	}
}

func (s *memorySalesExportJobs) LiveFilePaths() (map[string]struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listFails {
		return nil, false
	}
	paths := map[string]struct{}{}
	for _, job := range s.jobs {
		if job.FilePath != "" {
			paths[job.FilePath] = struct{}{}
		}
	}
	return paths, true
}

// useTestExportJobs подменяет реестр на время теста.
func useTestExportJobs(t *testing.T) *memorySalesExportJobs {
	t.Helper()
	store := newMemorySalesExportJobs()
	previous := exportJobs
	exportJobs = store
	t.Cleanup(func() { exportJobs = previous })
	return store
}

func putTestExportJob(t *testing.T, store *memorySalesExportJobs, job models.SalesExportJob) {
	t.Helper()
	if err := store.Create(job); err != nil {
		t.Fatalf("подготовка задания: %v", err)
	}
}

// Зависшее задание раньше пропускалось чисткой без ограничения по времени и
// оставалось в реестре навсегда: горутина мертва, а клиент опрашивает «running».
func TestCleanExpiredSalesExportJobsClosesStuckJob(t *testing.T) {
	withTestLogger(t)
	store := useTestExportJobs(t)
	now := time.Now()
	putTestExportJob(t, store, models.SalesExportJob{
		ID: "stuck", Owner: "tester", Status: "running",
		CreatedAt: now.Add(-salesExportJobStuckAfter - time.Minute),
	})

	exportJobs.CleanExpired(now)

	job, ok := exportJobs.ForUser("stuck", "tester")
	if !ok {
		t.Fatal("задание пропало вместо перевода в failed")
	}
	if job.Status != "failed" || job.Error == "" {
		t.Fatalf("задание = %+v, ожидался failed с причиной", job)
	}
	if job.CompletedAt.IsZero() {
		t.Fatal("время завершения не проставлено")
	}
}

// Готовое задание по истечении TTL уходит вместе со своим файлом.
func TestCleanExpiredSalesExportJobsRemovesExpiredFile(t *testing.T) {
	withTestLogger(t)
	store := useTestExportJobs(t)
	now := time.Now()
	path := filepath.Join(t.TempDir(), "expired.xlsx")
	if err := os.WriteFile(path, []byte("файл выгрузки"), 0o600); err != nil {
		t.Fatalf("подготовка файла: %v", err)
	}
	putTestExportJob(t, store, models.SalesExportJob{
		ID: "expired", Owner: "tester", Status: "ready", FilePath: path,
		CreatedAt: now.Add(-salesExportJobTTL - time.Minute),
	})

	exportJobs.CleanExpired(now)

	if _, ok := exportJobs.ForUser("expired", "tester"); ok {
		t.Fatal("просроченное задание осталось в реестре")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("файл просроченного задания остался: %v", err)
	}
}

// Файлы прошлого запуска не удалил бы никто: чистка по расписанию обходит
// реестр заданий, а этих файлов там уже нет.
func TestCleanupSalesExportDirRemovesOrphans(t *testing.T) {
	withTestLogger(t)
	useTestExportJobs(t)
	dir := t.TempDir()
	t.Setenv("SALES_EXPORT_DIR", dir)

	orphan := filepath.Join(dir, "internet-sales-123.xlsx")
	if err := os.WriteFile(orphan, []byte("сирота"), 0o600); err != nil {
		t.Fatalf("подготовка файла: %v", err)
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o750); err != nil {
		t.Fatalf("подготовка каталога: %v", err)
	}

	CleanupSalesExportDir()

	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("файл-сирота остался: %v", err)
	}
	// Вложенные каталоги не наши: их не трогаем.
	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("каталог удалён напрасно: %v", err)
	}
}

// Реестр пережил перезапуск: файл готового задания при старте удалять нельзя,
// иначе выгрузка, которую клиент ещё ждёт, исчезнет вместе с ним.
func TestCleanupSalesExportDirKeepsFileOfLiveJob(t *testing.T) {
	withTestLogger(t)
	store := useTestExportJobs(t)
	dir := t.TempDir()
	t.Setenv("SALES_EXPORT_DIR", dir)

	path := filepath.Join(dir, "internet-sales-live.xlsx")
	if err := os.WriteFile(path, []byte("готовый файл"), 0o600); err != nil {
		t.Fatalf("подготовка файла: %v", err)
	}
	putTestExportJob(t, store, models.SalesExportJob{
		ID: "live", Owner: "tester", Status: "ready", FilePath: path, CreatedAt: time.Now(),
	})

	CleanupSalesExportDir()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("файл живого задания удалён: %v", err)
	}
}

// Реестр недоступен — значит, неизвестно, за каким файлом ещё числится
// задание. Удалять в этом случае нельзя.
func TestCleanupSalesExportDirKeepsFilesWhenRegistryUnavailable(t *testing.T) {
	withTestLogger(t)
	store := useTestExportJobs(t)
	store.listFails = true
	dir := t.TempDir()
	t.Setenv("SALES_EXPORT_DIR", dir)

	path := filepath.Join(dir, "internet-sales-unknown.xlsx")
	if err := os.WriteFile(path, []byte("файл"), 0o600); err != nil {
		t.Fatalf("подготовка файла: %v", err)
	}

	CleanupSalesExportDir()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("файл удалён при недоступном реестре: %v", err)
	}
}

// Отсутствующий каталог — обычное состояние первого запуска, а не ошибка.
func TestCleanupSalesExportDirIgnoresMissingDir(t *testing.T) {
	withTestLogger(t)
	useTestExportJobs(t)
	t.Setenv("SALES_EXPORT_DIR", filepath.Join(t.TempDir(), "нет-такого"))
	CleanupSalesExportDir()
}

func TestSalesExportDirUsesEnvOverride(t *testing.T) {
	t.Setenv("SALES_EXPORT_DIR", "/data/exports")
	if got := salesExportDir(); got != "/data/exports" {
		t.Fatalf("salesExportDir() = %q", got)
	}
	t.Setenv("SALES_EXPORT_DIR", "  ")
	if got := salesExportDir(); got != filepath.Join(os.TempDir(), salesExportDirName) {
		t.Fatalf("salesExportDir() = %q, ожидался подкаталог во временной директории", got)
	}
}
