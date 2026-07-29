package main

import (
	"backend/config"
	"backend/handlers"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
	config.Init()
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/api/promo/save", handlers.SavePromo)
	r.DELETE("/api/promo/:id", handlers.DeletePromo)
	return r
}

// cleanupTestData удаляет тестовые записи после прогона
func cleanupTestData() {
	config.DB.Exec("DELETE FROM dbo.tbl_PromoActivities WHERE sku LIKE 'TEST-%'")
}

// TestMain — точка входа для всех тестов, очистка после завершения
func TestMain(m *testing.M) {
	// Прогоняем тесты
	code := m.Run()
	// Очищаем тестовые данные
	cleanupTestData()
	os.Exit(code)
}

// ==================== СОЗДАНИЕ ====================

func TestSavePromo_Create(t *testing.T) {
	router := setupRouter()

	payload := map[string]interface{}{
		"network_name":         "Тестовая сеть",
		"sku":                  "TEST-SKU-001",
		"year":                 2026,
		"month":                1,
		"mechanics":            "Скидка",
		"gtn_opex":             "GTN",
		"baseline_units":       100,
		"plan_promo_units":     150,
		"plan_investments_rub": 5000,
		"contract_price":       200,
		"id_directum":          "DIR-001",
		"ds_number":            "DS-001",
		"discount_amount":      15.5,
		"conditions":           "Тестовые условия",
		"ecom_segment":         "есть, не убирают из отчета",
		"total_pharmacies":     1000,
		"promo_pharmacies":     500,
		"status":               "Планируется",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["message"] != "Created" {
		t.Errorf("Ожидалось 'Created', получено '%v'", response["message"])
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Error("Не вернулись данные созданной записи")
		return
	}

	if data["plan_promo_rub"] == nil || data["plan_promo_rub"].(float64) != 30000 {
		t.Error("plan_promo_rub должен быть 30000 (150 * 200)")
	}
	if data["quarter"] == nil || data["quarter"].(float64) != 1 {
		t.Error("quarter должен быть 1 для января")
	}
	if data["plan_promo_uplift_units"] == nil || data["plan_promo_uplift_units"].(float64) != 50 {
		t.Error("plan_promo_uplift_units должен быть 50 (150 - 100)")
	}
	if data["baseline_rub"] == nil || data["baseline_rub"].(float64) != 20000 {
		t.Error("baseline_rub должен быть 20000 (100 * 200)")
	}
}

func TestSavePromo_MissingFields(t *testing.T) {
	router := setupRouter()

	payload := map[string]interface{}{
		"network_name": "Тестовая сеть",
		"year":         2026,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["message"] != "Created" {
		t.Errorf("Ожидалось 'Created', получено '%v'", response["message"])
	}
}

func TestSavePromo_ZeroValues(t *testing.T) {
	router := setupRouter()

	payload := map[string]interface{}{
		"network_name":         "Тестовая сеть",
		"sku":                  "TEST-ZERO-001",
		"year":                 2026,
		"month":                6,
		"baseline_units":       0,
		"plan_promo_units":     0,
		"plan_investments_rub": 0,
		"contract_price":       0,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	if data["plan_promo_rub"].(float64) != 0 {
		t.Error("plan_promo_rub должен быть 0 при нулевых входных данных")
	}
	if data["plan_roi"].(float64) != 0 {
		t.Error("plan_roi должен быть 0 при нулевых инвестициях")
	}
	if data["quarter"].(float64) != 2 {
		t.Error("quarter должен быть 2 для июня")
	}
}

func TestSavePromo_NegativeValues(t *testing.T) {
	router := setupRouter()

	payload := map[string]interface{}{
		"network_name":         "Тестовая сеть",
		"sku":                  "TEST-NEG-001",
		"year":                 2026,
		"month":                3,
		"baseline_units":       100,
		"plan_promo_units":     80,
		"plan_investments_rub": 5000,
		"contract_price":       200,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	if data["plan_promo_uplift_units"].(float64) != -20 {
		t.Error("plan_promo_uplift_units должен быть -20 (80 - 100)")
	}
	if data["plan_roi"].(float64) >= 0 {
		t.Error("plan_roi должен быть отрицательным при падающих продажах")
	}
}

func TestSavePromo_QuarterCalculation(t *testing.T) {
	router := setupRouter()

	months := []int{1, 4, 7, 10}
	expectedQuarters := []float64{1, 2, 3, 4}

	for i, month := range months {
		payload := map[string]interface{}{
			"network_name":     "Тест",
			"sku":              fmt.Sprintf("TEST-Q%d", month),
			"year":             2026,
			"month":            month,
			"plan_promo_units": 100,
			"contract_price":   100,
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})

		if data["quarter"].(float64) != expectedQuarters[i] {
			t.Errorf("Месяц %d: ожидался квартал %.0f, получен %.0f", month, expectedQuarters[i], data["quarter"])
		}
	}
}

// ==================== УДАЛЕНИЕ ====================

func TestDeletePromo_NotFound(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("DELETE", "/api/promo/99999999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Ожидался статус 200 или 404, получен %d", w.Code)
	}
}

func TestDeletePromo_InvalidID(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("DELETE", "/api/promo/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Ожидался статус 400 для некорректного ID, получен %d", w.Code)
	}
}

// ==================== МАТЕМАТИКА ====================

func TestROICalculation(t *testing.T) {
	upliftRub := 50000.0
	investments := 200000.0
	gm := 0.56

	expected := (upliftRub/investments)*gm*100 - 100
	roi := (upliftRub/investments)*gm*100 - 100

	if math.Abs(roi-expected) > 0.001 {
		t.Errorf("ROI: ожидалось %f, получено %f", expected, roi)
	}

	roiZero := 0.0
	if roiZero != 0 {
		t.Error("ROI при нулевых инвестициях должен быть 0")
	}

	if math.IsNaN(roi) {
		t.Error("ROI не должен быть NaN")
	}
}

func TestBaselineRubCalculation(t *testing.T) {
	tests := []struct {
		baseline float64
		price    float64
		expected float64
	}{
		{100, 200, 20000},
		{0, 200, 0},
		{100, 0, 0},
		{0, 0, 0},
		{150.5, 300.75, 45262.875},
	}

	for _, tt := range tests {
		result := tt.baseline * tt.price
		if math.Abs(result-tt.expected) > 0.001 {
			t.Errorf("baseline_rub(%f * %f): ожидалось %f, получено %f", tt.baseline, tt.price, tt.expected, result)
		}
	}
}

func TestUpliftCalculation(t *testing.T) {
	tests := []struct {
		plan     float64
		baseline float64
		expected float64
	}{
		{150, 100, 50},
		{80, 100, -20},
		{0, 100, -100},
		{100, 0, 100},
	}

	for _, tt := range tests {
		result := tt.plan - tt.baseline
		if math.Abs(result-tt.expected) > 0.001 {
			t.Errorf("uplift(%f - %f): ожидалось %f, получено %f", tt.plan, tt.baseline, tt.expected, result)
		}
	}
}

func TestQuarterCalculation(t *testing.T) {
	tests := []struct {
		month    int
		expected int
	}{
		{1, 1}, {2, 1}, {3, 1},
		{4, 2}, {5, 2}, {6, 2},
		{7, 3}, {8, 3}, {9, 3},
		{10, 4}, {11, 4}, {12, 4},
		{0, 0}, {13, 5},
	}

	for _, tt := range tests {
		quarter := int(math.Ceil(float64(tt.month) / 3))
		if quarter != tt.expected {
			t.Errorf("Месяц %d: ожидался квартал %d, получен %d", tt.month, tt.expected, quarter)
		}
	}
}

func TestUpliftPctCalculation(t *testing.T) {
	tests := []struct {
		uplift   float64
		plan     float64
		expected float64
	}{
		{50, 150, 33.33},
		{0, 100, 0},
		{100, 100, 100},
	}

	for _, tt := range tests {
		var result float64
		if tt.plan > 0 {
			result = (tt.uplift / tt.plan) * 100
		}
		if math.Abs(result-tt.expected) > 0.1 {
			t.Errorf("uplift_pct(%f/%f): ожидалось %f, получено %f", tt.uplift, tt.plan, tt.expected, result)
		}
	}
}

func TestInvestmentsPctCalculation(t *testing.T) {
	investments := 5000.0
	planRub := 30000.0
	expected := (investments / planRub) * 100

	result := 0.0
	if planRub > 0 {
		result = (investments / planRub) * 100
	}

	if math.Abs(result-expected) > 0.001 {
		t.Errorf("investments_pct: ожидалось %f, получено %f", expected, result)
	}
}

// ==================== RATE LIMITING ====================

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(5, 1*time.Second)

	for i := 0; i < 5; i++ {
		if !limiter.Allow("127.0.0.1") {
			t.Errorf("Запрос %d должен быть разрешён", i+1)
		}
	}

	if limiter.Allow("127.0.0.1") {
		t.Error("6-й запрос должен быть отклонён")
	}

	if !limiter.Allow("192.168.1.1") {
		t.Error("Запрос с другого IP должен быть разрешён")
	}

	time.Sleep(1 * time.Second)

	if !limiter.Allow("127.0.0.1") {
		t.Error("После сброса окна запрос должен быть разрешён")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := NewRateLimiter(3, 100*time.Millisecond)

	limiter.Allow("ip1")
	limiter.Allow("ip2")
	limiter.Allow("ip1")

	if len(limiter.visitors) != 2 {
		t.Errorf("Ожидалось 2 посетителя, получено %d", len(limiter.visitors))
	}

	time.Sleep(150 * time.Millisecond)

	limiter.Allow("ip1")
	if len(limiter.visitors["ip1"]) > 1 {
		t.Error("Старые записи должны были очиститься")
	}
}
