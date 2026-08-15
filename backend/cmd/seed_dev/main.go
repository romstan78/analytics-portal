package main

import (
	"backend/config"
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

//go:embed dev.sql
var devSeedSQL string

func main() {
	_ = godotenv.Load()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		log.Fatal("ОТКАЗ: dev-seed запрещён при APP_ENV=production")
	}
	if err := config.Init(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer config.DB.Close()

	if _, err := config.DB.Exec(devSeedSQL); err != nil {
		log.Fatalf("Ошибка загрузки dev-seed: %v", err)
	}
	fmt.Println("Dev-seed применён. Повторный запуск безопасен.")
}
