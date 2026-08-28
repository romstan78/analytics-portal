package handlers

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"backend/config"
)

func withTestLogger(t *testing.T) {
	t.Helper()
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

func registerTestExportJob(t *testing.T, id, owner string) {
	t.Helper()
	salesExportJobStore.Lock()
	salesExportJobStore.jobs[id] = &salesExportJob{
		ID: id, Owner: owner, Status: "running", CreatedAt: time.Now(),
	}
	salesExportJobStore.Unlock()
	t.Cleanup(func() {
		salesExportJobStore.Lock()
		delete(salesExportJobStore.jobs, id)
		salesExportJobStore.Unlock()
	})
}

// Задание выполняется вне Recovery-middleware Gin, поэтому паника внутри него
// раньше завершала весь процесс. Тест доходит до конца только если панику
// перехватили: сам факт возврата из горутины и есть проверка.
func TestRecoverSalesExportJobKeepsProcessAlive(t *testing.T) {
	withTestLogger(t)
	const id = "panic-job"
	registerTestExportJob(t, id, "tester")

	done := make(chan struct{})
	go func() {
		var path string
		defer close(done)
		defer recoverSalesExportJob(id, &path)
		panic("buildSalesExcel упал")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("горутина выгрузки не завершилась")
	}

	job, ok := salesExportJobForUser(id, "tester")
	if !ok {
		t.Fatal("задание пропало из реестра")
	}
	// Без перевода в failed клиент опрашивал бы «running» до перезапуска.
	if job.Status != "failed" {
		t.Fatalf("статус задания = %q, ожидался failed", job.Status)
	}
	if job.Error == "" {
		t.Fatal("клиенту не осталось причины отказа")
	}
	if job.CompletedAt.IsZero() {
		t.Fatal("время завершения не проставлено")
	}
}

// Недописанный файл после паники никому не достанется, и оставлять его на диске
// нельзя: чистка идёт обходом реестра заданий, а этого файла там уже нет.
func TestRecoverSalesExportJobRemovesHalfWrittenFile(t *testing.T) {
	withTestLogger(t)
	const id = "panic-job-file"
	registerTestExportJob(t, id, "tester")

	path := filepath.Join(t.TempDir(), "internet-sales-half.xlsx")
	if err := os.WriteFile(path, []byte("недописанный файл"), 0o600); err != nil {
		t.Fatalf("подготовка файла: %v", err)
	}

	func() {
		defer recoverSalesExportJob(id, &path)
		panic("паника после создания файла")
	}()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("файл-полуфабрикат остался на диске: %v", err)
	}
}

// Успешное завершение не должно ничего трогать: recover без паники — no-op.
func TestRecoverSalesExportJobIgnoresSuccess(t *testing.T) {
	withTestLogger(t)
	const id = "ok-job"
	registerTestExportJob(t, id, "tester")

	path := filepath.Join(t.TempDir(), "ready.xlsx")
	if err := os.WriteFile(path, []byte("готовый файл"), 0o600); err != nil {
		t.Fatalf("подготовка файла: %v", err)
	}

	func() {
		defer recoverSalesExportJob(id, &path)
	}()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("готовый файл удалён: %v", err)
	}
	job, _ := salesExportJobForUser(id, "tester")
	if job.Status != "running" {
		t.Fatalf("статус задания = %q, менять его без паники не следует", job.Status)
	}
}
