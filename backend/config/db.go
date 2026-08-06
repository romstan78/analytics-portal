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

	// Авто-создание таблиц, если их нет (миграции без отдельного тула)
	ensureTables()

	Logger.Info("db_connected", "host", os.Getenv("DB_SERVER"), "database", os.Getenv("DB_NAME"))
}

// ensureTables создаёт таблицы, если они ещё не существуют в БД.
func ensureTables() {
	execDDL := func(query string) {
		if _, err := DB.Exec(query); err != nil {
			Logger.Warn("ddl_exec_warning", "error", err.Error())
		}
	}

	// Таблица комментариев
	execDDL(`IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'tbl_PromoComments' AND TABLE_SCHEMA = 'dbo')
		CREATE TABLE dbo.tbl_PromoComments (
			id BIGINT IDENTITY PRIMARY KEY,
			promo_id INT NOT NULL,
			user_name NVARCHAR(100) NOT NULL,
			role NVARCHAR(50) NOT NULL,
			comment_text NVARCHAR(MAX) NOT NULL,
			created_at DATETIME DEFAULT GETDATE(),
			CONSTRAINT FK_PromoComments_Promo FOREIGN KEY (promo_id) REFERENCES dbo.tbl_PromoActivities(id)
		)`)
	execDDL(`IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_PromoComments_promo_id')
		CREATE INDEX IX_PromoComments_promo_id ON dbo.tbl_PromoComments(promo_id)`)

	// Таблица аудита
	execDDL(`IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'tbl_AuditLog' AND TABLE_SCHEMA = 'dbo')
		CREATE TABLE dbo.tbl_AuditLog (
			id BIGINT IDENTITY PRIMARY KEY,
			entity_type NVARCHAR(50) NOT NULL DEFAULT 'promo',
			entity_id INT NOT NULL,
			user_name NVARCHAR(100) NOT NULL,
			action_type NVARCHAR(20) NOT NULL,
			changed_fields NVARCHAR(MAX),
			created_at DATETIME DEFAULT GETDATE()
		)`)
	execDDL(`IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_AuditLog_entity')
		CREATE INDEX IX_AuditLog_entity ON dbo.tbl_AuditLog(entity_type, entity_id)`)
	execDDL(`IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_AuditLog_created')
		CREATE INDEX IX_AuditLog_created ON dbo.tbl_AuditLog(created_at DESC)`)
}
