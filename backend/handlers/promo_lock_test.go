package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/config"
	"backend/repository"

	"github.com/gin-gonic/gin"
)

// Логгер настраивается при инициализации БД, которой в юнит-тестах нет.
func init() {
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

// Пока идёт дедупликация, запись промо должна отвечать 503 с Retry-After,
// а не 500: клиенту нужно понять, что запрос имеет смысл повторить.
func TestRespondIfDedupInProgress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	handled := respondIfDedupInProgress(c, fmt.Errorf("обёртка: %w", repository.ErrPromoDedupInProgress))

	if !handled {
		t.Fatal("ошибка дедупликации не распознана")
	}
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("код ответа = %d, ожидался %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Error("нет заголовка Retry-After")
	}
	if !strings.Contains(recorder.Body.String(), "дедупликац") {
		t.Errorf("тело ответа не объясняет причину: %s", recorder.Body.String())
	}
}

func TestRespondIfDedupInProgressPassesOtherErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	if respondIfDedupInProgress(c, errors.New("обычная ошибка")) {
		t.Fatal("посторонняя ошибка обработана как дедупликация")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("ответ отправлен раньше времени: код %d", recorder.Code)
	}
}
