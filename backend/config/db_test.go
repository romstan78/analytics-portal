package config

import (
	"strings"
	"testing"
	"time"
)

func TestDatabaseNameValidation(t *testing.T) {
	t.Setenv("DB_NAME", "analytics_portal_test")
	name, err := databaseName()
	if err != nil || name != "analytics_portal_test" {
		t.Fatalf("databaseName() = %q, %v", name, err)
	}

	t.Setenv("DB_NAME", "invalid]; DROP DATABASE master--")
	if _, err := databaseName(); err == nil {
		t.Fatal("databaseName() должен отклонять небезопасное имя")
	}
}

func TestBuildConnStringUsesDefaultPort(t *testing.T) {
	t.Setenv("DB_SERVER", "db")
	t.Setenv("DB_USER", "app")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_PORT", "")
	conn := buildConnString("analytics")
	if !strings.Contains(conn, "database=analytics") || !strings.Contains(conn, "port=1433") {
		t.Fatalf("неожиданная строка подключения: %s", conn)
	}
}

func TestStartupTimeout(t *testing.T) {
	t.Setenv("DB_STARTUP_TIMEOUT_SECONDS", "12")
	if got := startupTimeout(); got != 12*time.Second {
		t.Fatalf("startupTimeout() = %s", got)
	}
}

func TestEnvEnabled(t *testing.T) {
	t.Setenv("FEATURE_FLAG", "true")
	if !envEnabled("FEATURE_FLAG") {
		t.Fatal("true должен включать флаг")
	}
	t.Setenv("FEATURE_FLAG", "false")
	if envEnabled("FEATURE_FLAG") {
		t.Fatal("false не должен включать флаг")
	}
}
