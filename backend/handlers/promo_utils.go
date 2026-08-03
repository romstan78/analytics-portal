package handlers

import (
	"fmt"
	"strconv"
)

// safeFloat, safeInt, safeString — хелперы для безопасного извлечения
// типизированных значений из map[string]interface{}. Используются
// в promo.go для логирования и парсинга входных данных.

func safeFloat(input map[string]interface{}, key string) float64 {
	val, _ := strconv.ParseFloat(fmt.Sprint(input[key]), 64)
	return val
}

func safeInt(input map[string]interface{}, key string) int {
	val, _ := strconv.Atoi(fmt.Sprint(input[key]))
	return val
}

func safeString(input map[string]interface{}, key string) string {
	return fmt.Sprint(input[key])
}
