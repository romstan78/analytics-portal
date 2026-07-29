package config

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"gopkg.in/natefinch/lumberjack.v2"
)

var DB *sql.DB
var Logger *slog.Logger

func buildConnString() string {
	return fmt.Sprintf(
		"server=%s;user id=%s;password=%s;database=%s;port=%s;TrustServerCertificate=1;",
		os.Getenv("DB_SERVER"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
}

func Init() {
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("Не удалось создать папку logs: %v", err)
	}
	logWriter := &lumberjack.Logger{
		Filename:   "logs/app.log",
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
	handler := slog.NewJSONHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelInfo})
	Logger = slog.New(handler)
	slog.SetDefault(Logger)

	var err error
	DB, err = sql.Open("mssql", buildConnString())
	if err != nil {
		Logger.Error("db_connection_failed", "error", err.Error())
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * 60 * 1e9)
	DB.SetConnMaxIdleTime(5 * time.Minute)

	if err = DB.Ping(); err != nil {
		Logger.Error("db_ping_failed", "error", err.Error())
		log.Fatalf("Нет соединения с БД: %v", err)
	}
	Logger.Info("db_connected", "host", os.Getenv("DB_SERVER"), "database", os.Getenv("DB_NAME"))
}
