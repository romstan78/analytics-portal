package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// putTestExportJob кладёт задание в реестр и убирает его за собой.
func putTestExportJob(t *testing.T, job *salesExportJob) {
	t.Helper()
	salesExportJobStore.Lock()
	salesExportJobStore.jobs[job.ID] = job
	salesExportJobStore.Unlock()
	t.Cleanup(func() {
		salesExportJobStore.Lock()
		delete(salesExportJobStore.jobs, job.ID)
		salesExportJobStore.Unlock()
	})
}

func testExportJobStatus(t *testing.T, id string) (salesExportJob, bool) {
	t.Helper()
	salesExportJobStore.RLock()
	defer salesExportJobStore.RUnlock()
	job, ok := salesExportJobStore.jobs[id]
	if !ok {
		return salesExportJob{}, false
	}
	return *job, true
}

// Зависшее задание раньше пропускалось чисткой без ограничения по времени и
// оставалось в памяти навсегда: горутина мертва, а клиент опрашивает «running».
func TestCleanExpiredSalesExportJobsClosesStuckJob(t *testing.T) {
	withTestLogger(t)
	now := time.Now()
	putTestExportJob(t, &salesExportJob{
		ID: "stuck", Owner: "tester", Status: "running",
		CreatedAt: now.Add(-salesExportJobStuckAfter - time.Minute),
	})

	cleanExpiredSalesExportJobs(now)

	job, ok := testExportJobStatus(t, "stuck")
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

// Идущая выгрузка большого файла не должна закрываться раньше срока.
func TestCleanExpiredSalesExportJobsKeepsFreshRunningJob(t *testing.T) {
	withTestLogger(t)
	now := time.Now()
	putTestExportJob(t, &salesExportJob{
		ID: "fresh", Owner: "tester", Status: "running",
		CreatedAt: now.Add(-time.Minute),
	})

	cleanExpiredSalesExportJobs(now)

	job, ok := testExportJobStatus(t, "fresh")
	if !ok || job.Status != "running" {
		t.Fatalf("задание = %+v, свежую выгрузку трогать нельзя", job)
	}
}

// Готовое задание по истечении TTL уходит вместе со своим файлом.
func TestCleanExpiredSalesExportJobsRemovesExpiredFile(t *testing.T) {
	withTestLogger(t)
	now := time.Now()
	path := filepath.Join(t.TempDir(), "expired.xlsx")
	if err := os.WriteFile(path, []byte("файл выгрузки"), 0o600); err != nil {
		t.Fatalf("подготовка файла: %v", err)
	}
	putTestExportJob(t, &salesExportJob{
		ID: "expired", Owner: "tester", Status: "ready", FilePath: path,
		CreatedAt: now.Add(-salesExportJobTTL - time.Minute),
	})

	cleanExpiredSalesExportJobs(now)

	if _, ok := testExportJobStatus(t, "expired"); ok {
		t.Fatal("просроченное задание осталось в реестре")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("файл просроченного задания остался: %v", err)
	}
}

// После перезапуска карта заданий пуста, и файлы предыдущего процесса не
// удалил бы никто: чистка по расписанию обходит именно карту.
func TestCleanupSalesExportDirRemovesOrphans(t *testing.T) {
	withTestLogger(t)
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

// Отсутствующий каталог — обычное состояние первого запуска, а не ошибка.
func TestCleanupSalesExportDirIgnoresMissingDir(t *testing.T) {
	withTestLogger(t)
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
