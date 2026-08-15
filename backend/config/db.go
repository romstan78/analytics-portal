package config

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"backend/migrations"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/pressly/goose/v3"
	"gopkg.in/natefinch/lumberjack.v2"
)

var DB *sql.DB
var Logger *slog.Logger

var databaseNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func buildConnString(database string) string {
	port := strings.TrimSpace(os.Getenv("DB_PORT"))
	if port == "" {
		port = "1433"
	}
	return fmt.Sprintf(
		"server=%s;user id=%s;password=%s;database=%s;port=%s;TrustServerCertificate=1;",
		os.Getenv("DB_SERVER"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		database,
		port,
	)
}

func databaseName() (string, error) {
	name := strings.TrimSpace(os.Getenv("DB_NAME"))
	if name == "" {
		return "", fmt.Errorf("DB_NAME не задан")
	}
	if !databaseNamePattern.MatchString(name) {
		return "", fmt.Errorf("DB_NAME содержит недопустимые символы: разрешены латинские буквы, цифры, _ и -")
	}
	return name, nil
}

func startupTimeout() time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(os.Getenv("DB_STARTUP_TIMEOUT_SECONDS")))
	if err != nil || seconds <= 0 {
		seconds = 90
	}
	return time.Duration(seconds) * time.Second
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func waitForPing(ctx context.Context, db *sql.DB) error {
	var lastErr error
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("SQL Server недоступен: %w (последняя ошибка: %v)", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func ensureDatabase(ctx context.Context, name string) error {
	masterDB, err := sql.Open("sqlserver", buildConnString("master"))
	if err != nil {
		return fmt.Errorf("открытие подключения к master: %w", err)
	}
	defer masterDB.Close()

	if err := waitForPing(ctx, masterDB); err != nil {
		return err
	}

	var exists int
	if err := masterDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sys.databases WHERE name = @p1", name).Scan(&exists); err != nil {
		return fmt.Errorf("проверка базы %s: %w", name, err)
	}
	if exists > 0 {
		return nil
	}
	if !envEnabled("DB_AUTO_CREATE") {
		return fmt.Errorf("база %s не существует; создайте её или установите DB_AUTO_CREATE=true", name)
	}

	if _, err := masterDB.ExecContext(ctx, "CREATE DATABASE ["+name+"]"); err != nil {
		return fmt.Errorf("создание базы %s: %w", name, err)
	}
	Logger.Info("db_created", "database", name)
	return nil
}

func GetDBInfo() string {
	return fmt.Sprintf("%s@%s/%s", os.Getenv("DB_USER"), os.Getenv("DB_SERVER"), os.Getenv("DB_NAME"))
}

func Init() error {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("создание каталога логов: %w", err)
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

	name, err := databaseName()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout())
	defer cancel()
	if err := ensureDatabase(ctx, name); err != nil {
		Logger.Error("database_prepare_failed", "error", err.Error())
		return err
	}

	// ── Миграции goose (временное соединение "sqlserver") ───────────
	// Драйвер microsoft/go-mssqldb регистрирует два имени:
	//   - "mssql":     авто-конвертация ? и :N → @pN (для squirrel)
	//   - "sqlserver": ожидает готовые @pN (используется goose)
	migrateDB, err := sql.Open("sqlserver", buildConnString(name))
	if err != nil {
		Logger.Error("migrations_db_failed", "error", err.Error())
		return fmt.Errorf("подключение для миграций: %w", err)
	}
	if err := waitForPing(ctx, migrateDB); err != nil {
		migrateDB.Close()
		Logger.Error("migrations_db_unavailable", "error", err.Error())
		return err
	}
	provider, err := goose.NewProvider(goose.DialectMSSQL, migrateDB, migrations.FS)
	if err != nil {
		migrateDB.Close()
		Logger.Error("migrations_provider_failed", "error", err.Error())
		return fmt.Errorf("инициализация миграций: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		migrateDB.Close()
		Logger.Error("migrations_up_failed", "error", err.Error())
		return fmt.Errorf("применение миграций: %w", err)
	}
	migrateDB.Close()

	// ── Основной пул соединений (squirrel, бизнес-логика) ────────────
	DB, err = sql.Open("mssql", buildConnString(name))
	if err != nil {
		Logger.Error("db_connection_failed", "error", err.Error())
		return fmt.Errorf("открытие основного пула БД: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * 60 * 1e9)
	DB.SetConnMaxIdleTime(5 * time.Minute)

	if err = DB.PingContext(ctx); err != nil {
		DB.Close()
		DB = nil
		Logger.Error("db_ping_failed", "error", err.Error())
		return fmt.Errorf("проверка основного пула БД: %w", err)
	}

	Logger.Info("db_connected", "host", os.Getenv("DB_SERVER"), "database", name)
	return nil
}
