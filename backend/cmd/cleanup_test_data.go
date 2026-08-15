// Утилита для удаления тестовых записей (SKU LIKE 'TEST-%') и связанных данных.
// Использование: go run ./cmd/cleanup_test_data.go
package main

import (
	"backend/config"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	dbName := strings.ToLower(strings.TrimSpace(os.Getenv("DB_NAME")))
	if !strings.HasSuffix(dbName, "_test") {
		log.Fatalf("ОТКАЗ: очистка разрешена только для DB_NAME с суффиксом _test, получено %q", dbName)
	}
	if err := config.Init(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer config.DB.Close()

	fmt.Printf("БД: %s\n", config.GetDBInfo())

	// Удаляем связанные комментарии
	r1, err := config.DB.Exec(`
		DELETE FROM dbo.tbl_PromoComments
		WHERE promo_id IN (
			SELECT id FROM dbo.tbl_PromoActivities WHERE sku LIKE 'TEST-%'
		)`)
	if err != nil {
		log.Fatalf("Ошибка удаления комментариев: %v", err)
	}
	c1, _ := r1.RowsAffected()
	fmt.Printf("Удалено комментариев: %d\n", c1)

	// Удаляем связанные аудит-логи
	r2, err := config.DB.Exec(`
		DELETE FROM dbo.tbl_AuditLog
		WHERE entity_type = 'promo'
		  AND entity_id IN (
			SELECT id FROM dbo.tbl_PromoActivities WHERE sku LIKE 'TEST-%'
		)`)
	if err != nil {
		log.Fatalf("Ошибка удаления аудит-логов: %v", err)
	}
	c2, _ := r2.RowsAffected()
	fmt.Printf("Удалено аудит-записей: %d\n", c2)

	// Удаляем сами промо
	r3, err := config.DB.Exec("DELETE FROM dbo.tbl_PromoActivities WHERE sku LIKE 'TEST-%'")
	if err != nil {
		log.Fatalf("Ошибка удаления промо: %v", err)
	}
	c3, _ := r3.RowsAffected()
	fmt.Printf("Удалено промо-записей: %d\n", c3)
	fmt.Println("Готово.")
}
