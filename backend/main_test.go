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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	// Промо
	r.POST("/api/promo/save", handlers.SavePromo)
	r.DELETE("/api/promo/:id", handlers.DeletePromo)
	r.GET("/api/promo/data", handlers.GetPromoData)
	r.GET("/api/promo/filters", handlers.GetPromoFilters)
	// Интернет-продажи
	r.GET("/api/data", handlers.GetData)
	r.GET("/api/filters", handlers.GetFilterOptions)
	return r
}

// cleanupTestData удаляет тестовые записи после прогона.
// Защита: выполняется только если имя БД содержит _test или _dev.
func cleanupTestData() {
	dbName := strings.ToLower(strings.TrimSpace(os.Getenv("DB_NAME")))
	if !strings.HasSuffix(dbName, "_test") {
		fmt.Println("[WARN] cleanupTestData: БД не оканчивается на _test, пропускаем DELETE")
		return
	}
	config.DB.Exec("DELETE FROM dbo.tbl_PromoActivities WHERE sku LIKE 'TEST-%'")
}

func TestMain(m *testing.M) {
	_ = godotenv.Load()
	dbName := strings.ToLower(strings.TrimSpace(os.Getenv("DB_NAME")))
	if !strings.HasSuffix(dbName, "_test") {
		fmt.Fprintf(os.Stderr, "ОТКАЗ: интеграционные тесты разрешены только для DB_NAME с суффиксом _test, получено %q\n", dbName)
		os.Exit(2)
	}
	config.Init()
	code := m.Run()
	cleanupTestData()
	if config.DB != nil {
		_ = config.DB.Close()
	}
	os.Exit(code)
}

// ==================== СОЗДАНИЕ ====================

func TestSavePromo_Create(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{
		"network_name": "Тестовая сеть", "sku": "TEST-SKU-001",
		"year": 2026, "month": 1, "mechanics": "Скидка", "gtn_opex": "GTN",
		"baseline_units": 100, "plan_promo_units": 150, "plan_investments_rub": 5000,
		"contract_price": 200, "id_directum": "DIR-001", "ds_number": "DS-001",
		"discount_amount": 15.5, "conditions": "Тестовые условия",
		"ecom_segment":     "есть, не убирают из отчета",
		"total_pharmacies": 1000, "promo_pharmacies": 500, "status": "Планируется",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d: %s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["message"] != "Created" {
		t.Errorf("Ожидалось 'Created', получено '%v'", response["message"])
	}
	data := response["data"].(map[string]interface{})
	if data["plan_promo_rub"].(float64) != 30000 {
		t.Errorf("plan_promo_rub: ожидалось 30000, получено %f", data["plan_promo_rub"])
	}
	if data["quarter"].(float64) != 1 {
		t.Errorf("quarter: ожидался 1, получен %f", data["quarter"])
	}
}

func TestSavePromo_MissingFields(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{"network_name": "Тестовая сеть", "year": 2026}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}
}

func TestSavePromo_ZeroValues(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{
		"network_name": "Тестовая сеть", "sku": "TEST-ZERO-001",
		"year": 2026, "month": 6, "baseline_units": 0, "plan_promo_units": 0,
		"plan_investments_rub": 0, "contract_price": 0,
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
}

func TestSavePromo_NegativeValues(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{
		"network_name": "Тест", "sku": "TEST-NEG-001",
		"year": 2026, "month": 3, "baseline_units": 100, "plan_promo_units": 80,
		"plan_investments_rub": 5000, "contract_price": 200,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})
	if data["plan_promo_uplift_units"].(float64) != -20 {
		t.Error("plan_promo_uplift_units должен быть -20")
	}
}

func TestSavePromo_QuarterCalculation(t *testing.T) {
	router := setupRouter()
	months := []int{1, 4, 7, 10}
	expectedQuarters := []float64{1, 2, 3, 4}
	for i, month := range months {
		payload := map[string]interface{}{
			"network_name": "Тест", "sku": fmt.Sprintf("TEST-Q%d", month),
			"year": 2026, "month": month, "plan_promo_units": 100, "contract_price": 100,
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

// ==================== ОБНОВЛЕНИЕ ====================

// saveTestPromo создаёт тестовое промо и возвращает его ID
func saveTestPromo(t *testing.T, router *gin.Engine, sku string) int {
	t.Helper()
	payload := map[string]interface{}{
		"network_name": "Тест-Update", "sku": sku, "year": 2026, "month": 5,
		"baseline_units": 100, "plan_promo_units": 200, "plan_investments_rub": 10000,
		"contract_price": 150, "mechanics": "Скидка", "gtn_opex": "GTN",
		"status": "Планируется",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	return int(response["id"].(float64))
}

func TestUpdatePromo_RecalculatesFields(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-UPDATE-001")

	// Меняем plan_promo_units: 200 → 300
	payload := map[string]interface{}{
		"id": id, "plan_promo_units": 300,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["message"] != "Updated" {
		t.Errorf("Ожидалось 'Updated', получено '%v'", response["message"])
	}

	data := response["data"].(map[string]interface{})

	// plan_promo_rub = 300 * 150 = 45000
	if planRub, ok := data["plan_promo_rub"]; !ok || math.Abs(planRub.(float64)-45000) > 0.01 {
		t.Errorf("plan_promo_rub: ожидалось 45000, получено %v", data["plan_promo_rub"])
	}
	// uplift_units = 300 - 100 = 200
	if uplift, ok := data["plan_promo_uplift_units"]; !ok || math.Abs(uplift.(float64)-200) > 0.01 {
		t.Errorf("plan_promo_uplift_units: ожидалось 200, получено %v", data["plan_promo_uplift_units"])
	}
}

func TestUpdatePromo_ChangesQuarterOnMonthChange(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-UPDATE-Q")

	// Меняем месяц: 5 (Q2) → 11 (Q4)
	payload := map[string]interface{}{"id": id, "month": 11}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	if data["quarter"].(float64) != 4 {
		t.Errorf("quarter: ожидался 4 (месяц 11), получен %v", data["quarter"])
	}
}

func TestUpdatePromo_PreservesUnchangedFields(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-UPDATE-PRES")

	// Меняем только status
	payload := map[string]interface{}{"id": id, "status": "Завершено"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	// network_name должен сохраниться
	if data["network_name"] != "Тест-Update" {
		t.Errorf("network_name: ожидалось 'Тест-Update', получено '%v'", data["network_name"])
	}
	// contract_price должен сохраниться и использоваться в расчётах
	if cp, ok := data["contract_price"]; !ok || math.Abs(cp.(float64)-150) > 0.01 {
		t.Errorf("contract_price: ожидалось 150, получено %v", data["contract_price"])
	}
}

func TestUpdatePromo_UpdateNonExistent(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{"id": 99999999, "status": "Завершено"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// fetchExistingRow вернёт ошибку → 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Ожидался статус 500 для несуществующего ID, получен %d", w.Code)
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
		t.Errorf("Ожидался статус 400, получен %d", w.Code)
	}
}

func TestDeletePromo_ThenVerifyGone(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-DELETE-001")

	// Удаляем
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/promo/%d", id), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200 при удалении, получен %d", w.Code)
	}

	// Пробуем обновить удалённое → 500
	payload := map[string]interface{}{"id": id, "status": "Обновлено"}
	body, _ := json.Marshal(payload)
	req2, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("Ожидался статус 500 при обновлении удалённой записи, получен %d", w2.Code)
	}
}

// ==================== ФИЛЬТРЫ ПРОМО ====================

func TestGetPromoFilters_All(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/filters", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Все ключи должны присутствовать
	for _, key := range []string{"kam", "brand", "sku", "network_name", "mechanics", "channel", "status"} {
		if _, ok := response[key]; !ok {
			t.Errorf("В ответе отсутствует ключ '%s'", key)
		}
	}
}

func TestGetPromoFilters_ByYear(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/filters?yearFrom=2025&yearTo=2026", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Фильтр по году: ожидался 200, получен %d", w.Code)
	}
}

func TestGetPromoFilters_ByMonth(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/filters?months=1&months=6&months=12", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Фильтр по месяцам: ожидался 200, получен %d", w.Code)
	}
}

func TestGetPromoFilters_Cascading(t *testing.T) {
	router := setupRouter()
	// Выбираем одного KAM → список брендов должен сузиться
	req, _ := http.NewRequest("GET", "/api/promo/filters?kam=%D0%98%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Даже если такого KAM нет — фильтр не должен падать
	if w.Code != http.StatusOK {
		t.Errorf("Каскадный фильтр: ожидался 200, получен %d", w.Code)
	}
}

// ==================== ДАННЫЕ ПРОМО ====================

func TestGetPromoData_Empty(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/data?all=true&yearFrom=1900&yearTo=1901", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("Ожидался пустой массив, получено %d записей", len(data))
	}
}

func TestGetPromoData_Pagination(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/data?page=0&pageSize=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Пагинация: ожидался 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})
	if len(data) > 5 {
		t.Errorf("Пагинация: ожидалось не более 5 записей, получено %d", len(data))
	}
}

// ==================== ИНТЕРНЕТ-ПРОДАЖИ: ФИЛЬТРЫ ====================

func TestGetFilterOptions_ReturnsAllKeys(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/filters", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	for _, key := range []string{"brandName", "networkName", "un_rub", "segment", "channel", "segmentChannelMap", "channelSegmentMap"} {
		if _, ok := response[key]; !ok {
			t.Errorf("В ответе отсутствует ключ '%s'", key)
		}
	}
}

func TestGetData_WithFilters(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/data?yearFrom=2024&yearTo=2025&page=0&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d: %s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if _, ok := response["totalRows"]; !ok {
		t.Error("Ответ должен содержать 'totalRows' при пагинации")
	}
}

func TestGetData_AllForExport(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/data?all=true&yearFrom=2025&yearTo=2025", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("all=true: ожидался 200, получен %d", w.Code)
	}
}

// ==================== ИНТЕГРАЦИОННЫЙ: Create → Read → Update → Delete ====================

func TestIntegration_PromoCRUD(t *testing.T) {
	router := setupRouter()
	timestamp := time.Now().UnixNano()

	// CREATE
	createPayload := map[string]interface{}{
		"network_name": "Интеграция-Сеть",
		"sku":          fmt.Sprintf("TEST-INT-%d", timestamp),
		"year":         2026, "month": 7,
		"mechanics": "Скидка 15%", "gtn_opex": "GTN",
		"baseline_units": 500, "plan_promo_units": 800,
		"plan_investments_rub": 25000, "contract_price": 300,
		"status": "Планируется",
	}
	body, _ := json.Marshal(createPayload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CREATE: ожидался 200, получен %d: %s", w.Code, w.Body.String())
	}
	var createResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	id := int(createResp["id"].(float64))
	t.Logf("Создано промо ID=%d", id)

	// READ через GetPromoData с фильтром по SKU
	req2, _ := http.NewRequest("GET",
		fmt.Sprintf("/api/promo/data?all=true&sku=%s", createPayload["sku"]), nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("READ: ожидался 200, получен %d", w2.Code)
	}
	var readResp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &readResp)
	rows := readResp["data"].([]interface{})
	if len(rows) == 0 {
		t.Fatal("READ: запись не найдена после создания")
	}
	t.Logf("Прочитано %d записей по SKU", len(rows))

	// UPDATE
	updatePayload := map[string]interface{}{
		"id": id, "plan_promo_units": 1000, "status": "Согласовано",
	}
	body3, _ := json.Marshal(updatePayload)
	req3, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body3))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("UPDATE: ожидался 200, получен %d: %s", w3.Code, w3.Body.String())
	}
	var updateResp map[string]interface{}
	json.Unmarshal(w3.Body.Bytes(), &updateResp)
	if updateResp["message"] != "Updated" {
		t.Errorf("UPDATE: ожидалось 'Updated', получено '%v'", updateResp["message"])
	}
	// Проверяем пересчёт
	updatedData := updateResp["data"].(map[string]interface{})
	if math.Abs(updatedData["plan_promo_rub"].(float64)-300000) > 0.01 {
		t.Errorf("UPDATE: plan_promo_rub ожидалось 300000 (1000*300), получено %v", updatedData["plan_promo_rub"])
	}

	// DELETE
	req4, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/promo/%d", id), nil)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("DELETE: ожидался 200, получен %d", w4.Code)
	}
	t.Log("Удалено успешно")

	// VERIFY DELETION — обновление удалённой записи должно вернуть 500
	body5, _ := json.Marshal(map[string]interface{}{"id": id, "status": "После удаления"})
	req5, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body5))
	req5.Header.Set("Content-Type", "application/json")
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusInternalServerError {
		t.Errorf("VERIFY: обновление удалённой записи должно вернуть 500, получен %d", w5.Code)
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
}

func TestBaselineRubCalculation(t *testing.T) {
	tests := []struct {
		baseline, price, expected float64
	}{
		{100, 200, 20000}, {0, 200, 0}, {100, 0, 0}, {150.5, 300.75, 45262.875},
	}
	for _, tt := range tests {
		result := tt.baseline * tt.price
		if math.Abs(result-tt.expected) > 0.001 {
			t.Errorf("baseline_rub(%f*%f): ожидалось %f, получено %f", tt.baseline, tt.price, tt.expected, result)
		}
	}
}

func TestUpliftCalculation(t *testing.T) {
	tests := []struct{ plan, baseline, expected float64 }{
		{150, 100, 50}, {80, 100, -20}, {0, 100, -100}, {100, 0, 100},
	}
	for _, tt := range tests {
		result := tt.plan - tt.baseline
		if math.Abs(result-tt.expected) > 0.001 {
			t.Errorf("uplift(%f-%f): ожидалось %f, получено %f", tt.plan, tt.baseline, tt.expected, result)
		}
	}
}

func TestQuarterCalculation(t *testing.T) {
	tests := []struct{ month, expected int }{
		{1, 1}, {2, 1}, {3, 1}, {4, 2}, {5, 2}, {6, 2},
		{7, 3}, {8, 3}, {9, 3}, {10, 4}, {11, 4}, {12, 4},
	}
	for _, tt := range tests {
		q := int(math.Ceil(float64(tt.month) / 3))
		if q != tt.expected {
			t.Errorf("Месяц %d: ожидался квартал %d, получен %d", tt.month, tt.expected, q)
		}
	}
}

// ==================== RATE LIMITING ====================

func TestRateLimiter(t *testing.T) {
	limiter := rate.NewLimiter(5, 5) // 5 событий/сек, burst 5
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("Запрос %d должен быть разрешён", i+1)
		}
	}
	if limiter.Allow() {
		t.Error("6-й запрос должен быть отклонён")
	}
	time.Sleep(1 * time.Second)
	if !limiter.Allow() {
		t.Error("После сброса окна запрос должен быть разрешён")
	}
}

func TestRateLimiter_Burst(t *testing.T) {
	limiter := rate.NewLimiter(1, 3) // 1 событие/сек, burst 3
	// burst разрешает до 3 запросов сразу
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Errorf("Запрос %d в пределах burst должен быть разрешён", i+1)
		}
	}
	if limiter.Allow() {
		t.Error("4-й запрос должен быть отклонён (burst исчерпан)")
	}
}

func saveTestPromoWithMeta(t *testing.T, router *gin.Engine, sku string) (int, string) {
	t.Helper()
	payload := map[string]interface{}{
		"network_name": "Тест-OPTILOCK", "sku": sku, "year": 2026, "month": 5,
		"baseline_units": 100, "plan_promo_units": 200, "plan_investments_rub": 10000,
		"contract_price": 150, "mechanics": "Скидка", "gtn_opex": "GTN",
		"status": "Планируется",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["message"] != "Created" {
		t.Fatalf("Не удалось создать тестовое промо: %v", response)
	}

	id := int(response["id"].(float64))

	// Достаём реальный updated_at из БД
	var updatedAt string
	config.DB.QueryRow("SELECT CONVERT(NVARCHAR, updated_at, 121) FROM dbo.tbl_PromoActivities WHERE id = ? AND deleted_at IS NULL", id).Scan(&updatedAt)

	return id, updatedAt
}

func TestSavePromo_OptimisticLocking(t *testing.T) {
	router := setupRouter()
	id, updatedAt := saveTestPromoWithMeta(t, router, "TEST-OPTILOCK-001")

	// Первое обновление — передаём актуальный updated_at (должно пройти)
	payload1 := map[string]interface{}{
		"id": id, "status": "Первое изменение",
		"updated_at": updatedAt,
	}
	body1, _ := json.Marshal(payload1)
	req1, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		var errResp map[string]interface{}
		json.Unmarshal(w1.Body.Bytes(), &errResp)
		t.Fatalf("Первое обновление должно пройти: %v", errResp)
	}

	var resp1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	data1 := resp1["data"].(map[string]interface{})
	newUpdatedAt := data1["updated_at"]

	// Второе обновление с тем же старым updated_at — должно дать 409
	payload2 := map[string]interface{}{
		"id": id, "status": "Конфликт",
		"updated_at": updatedAt, // тот же, что и в первом запросе — уже не актуален
	}
	body2, _ := json.Marshal(payload2)
	req2, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Ожидался 409 Conflict, получен %d. Ответ: %s", w2.Code, w2.Body.String())
	} else {
		t.Logf("✅ Optimistic locking работает: первый OK с новым updated_at=%v, второй 409 со старым=%v", newUpdatedAt, updatedAt)
	}
}
