package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Разбор параметров реестра сетей. Эти функции стоят перед каждым обработчиком
// и сами отвечают клиенту, поэтому ошибка в них тихо меняет то, какие данные
// увидит пользователь: не тот год, не тот квартал, не та сеть.

func contextWithParams(params gin.Params, query string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = params
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	return c, recorder
}

func TestNetworkIDParam(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   int
		wantOK bool
	}{
		{name: "обычный id", raw: "42", want: 42, wantOK: true},
		{name: "ноль отклоняется", raw: "0"},
		{name: "отрицательный отклоняется", raw: "-1"},
		{name: "не число", raw: "abc"},
		{name: "пусто", raw: ""},
		// Так выглядит попытка подставить в путь что-то своё.
		{name: "число с хвостом", raw: "42;DROP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := contextWithParams(gin.Params{{Key: "id", Value: tt.raw}}, "")
			got, ok := networkIDParam(c)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("networkIDParam(%q) = (%d, %v), ожидалось (%d, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
			}
			if !tt.wantOK && recorder.Code != http.StatusBadRequest {
				t.Fatalf("код ответа = %d, ожидался 400", recorder.Code)
			}
		})
	}
}

func TestPlanYear(t *testing.T) {
	currentYear := time.Now().Year()
	tests := []struct {
		name   string
		query  string
		want   int
		wantOK bool
	}{
		{name: "год не указан — текущий", query: "", want: currentYear, wantOK: true},
		{name: "явный год", query: "year=2027", want: 2027, wantOK: true},
		{name: "пробелы вокруг года", query: "year=%202026%20", want: 2026, wantOK: true},
		{name: "не число", query: "year=abc"},
		// Границы диапазона: планы за 1999 и 2101 — заведомо ошибка ввода.
		{name: "слишком рано", query: "year=1999"},
		{name: "слишком поздно", query: "year=2101"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := contextWithParams(nil, tt.query)
			got, ok := planYear(c)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("planYear(%q) = (%d, %v), ожидалось (%d, %v)", tt.query, got, ok, tt.want, tt.wantOK)
			}
			if !tt.wantOK && recorder.Code != http.StatusBadRequest {
				t.Fatalf("код ответа = %d, ожидался 400", recorder.Code)
			}
		})
	}
}

func TestPlanQuarter(t *testing.T) {
	currentQuarter := (int(time.Now().Month())-1)/3 + 1
	tests := []struct {
		name   string
		query  string
		want   int
		wantOK bool
	}{
		{name: "квартал не указан — текущий", query: "", want: currentQuarter, wantOK: true},
		{name: "первый", query: "quarter=1", want: 1, wantOK: true},
		{name: "четвёртый", query: "quarter=4", want: 4, wantOK: true},
		{name: "нулевой отклоняется", query: "quarter=0"},
		{name: "пятый отклоняется", query: "quarter=5"},
		{name: "не число", query: "quarter=Q1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := contextWithParams(nil, tt.query)
			got, ok := planQuarter(c)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("planQuarter(%q) = (%d, %v), ожидалось (%d, %v)", tt.query, got, ok, tt.want, tt.wantOK)
			}
			if !tt.wantOK && recorder.Code != http.StatusBadRequest {
				t.Fatalf("код ответа = %d, ожидался 400", recorder.Code)
			}
		})
	}
}

func TestValidNetworkType(t *testing.T) {
	for _, valid := range []string{"regular", "warehouse"} {
		if !validNetworkType(valid) {
			t.Fatalf("%q должен приниматься", valid)
		}
	}
	// Тип сети определяет процесс прогнозирования, поэтому произвольная строка
	// в него попасть не должна.
	for _, invalid := range []string{"", "Regular", "склад", "regular ", "warehouse; --"} {
		if validNetworkType(invalid) {
			t.Fatalf("%q приниматься не должен", invalid)
		}
	}
}

// Отказ по блокировке не должен раскрывать, существует ли учётная запись:
// текст одинаков, а Retry-After говорит, когда повторить.
func TestRespondLoginLockedTellsWhenToRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	now := time.Now()

	respondLoginLocked(c, now.Add(90*time.Second), now)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("код ответа = %d, ожидался 429", recorder.Code)
	}
	if got := recorder.Header().Get("Retry-After"); got != "90" {
		t.Fatalf("Retry-After = %q, ожидалось 90", got)
	}
	// Округление вверх: 90 секунд — это «через 2 мин.», а не «через 1».
	if body := recorder.Body.String(); !strings.Contains(body, "через 2 мин.") {
		t.Fatalf("тело ответа = %s", body)
	}
}

func TestRespondLoginLockedNeverPromisesLessThanMinute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	now := time.Now()

	respondLoginLocked(c, now.Add(2*time.Second), now)

	if body := recorder.Body.String(); !strings.Contains(body, "через 1 мин.") {
		t.Fatalf("тело ответа = %s, остаток меньше минуты округляется до одной", body)
	}
}
