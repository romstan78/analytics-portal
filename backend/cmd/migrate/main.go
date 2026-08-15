package main

import (
	"backend/config"
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	if err := config.Init(); err != nil {
		log.Fatalf("Ошибка миграции БД: %v", err)
	}
	defer config.DB.Close()
	fmt.Printf("Миграции применены: %s\n", config.GetDBInfo())
}
