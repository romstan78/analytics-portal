package config

import (
	"strings"
	"testing"
)

func TestInitAuthRejectsWeakSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "too-short")
	if err := InitAuth(); err == nil {
		t.Fatal("InitAuth должен отклонять короткий JWT_SECRET")
	}
}

func TestAccessAndRefreshTokensAreNotInterchangeable(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("a", minJWTSecretLen))
	if err := InitAuth(); err != nil {
		t.Fatalf("InitAuth: %v", err)
	}

	accessToken, err := GenerateAccessToken("manager1", "agreement1")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	refreshToken, err := GenerateRefreshToken("manager1", "agreement1")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if _, err := ValidateAccessToken(accessToken); err != nil {
		t.Fatalf("валидный access token отклонён: %v", err)
	}
	if _, err := ValidateRefreshToken(refreshToken); err != nil {
		t.Fatalf("валидный refresh token отклонён: %v", err)
	}
	if _, err := ValidateAccessToken(refreshToken); err == nil {
		t.Fatal("refresh token не должен приниматься как access token")
	}
	if _, err := ValidateRefreshToken(accessToken); err == nil {
		t.Fatal("access token не должен приниматься как refresh token")
	}
}

func TestValidateRuntimeAllowsDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_AUTO_CREATE", "true")
	t.Setenv("CORS_ORIGINS", "http://localhost:5173")
	t.Setenv("TRUSTED_PROXIES", "")
	if err := ValidateRuntime(); err != nil {
		t.Fatalf("development-конфигурация отклонена: %v", err)
	}
}

func TestValidateRuntimeRejectsUnsafeProductionDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_AUTO_CREATE", "true")
	t.Setenv("CORS_ORIGINS", "http://localhost:5173")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.10")
	if err := ValidateRuntime(); err == nil {
		t.Fatal("production с DB_AUTO_CREATE=true должен быть отклонён")
	}

	t.Setenv("DB_AUTO_CREATE", "false")
	if err := ValidateRuntime(); err == nil {
		t.Fatal("production с localhost в CORS_ORIGINS должен быть отклонён")
	}

	t.Setenv("CORS_ORIGINS", "http://portal.example.com")
	if err := ValidateRuntime(); err == nil {
		t.Fatal("production с HTTP origin должен быть отклонён")
	}
}

func TestValidateRuntimeRejectsUnsafeDatabaseTLS(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_AUTO_CREATE", "false")
	t.Setenv("CORS_ORIGINS", "https://portal.example.com")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.10")

	// Незаданное значение означает молчаливое доверие любому сертификату.
	t.Setenv("DB_TRUST_SERVER_CERT", "")
	if err := ValidateRuntime(); err == nil {
		t.Fatal("production без явного DB_TRUST_SERVER_CERT должен быть отклонён")
	}

	// Осознанный выбор допустим в обе стороны.
	t.Setenv("DB_TRUST_SERVER_CERT", "true")
	if err := ValidateRuntime(); err != nil {
		t.Fatalf("явный DB_TRUST_SERVER_CERT=true должен приниматься: %v", err)
	}

	t.Setenv("DB_TRUST_SERVER_CERT", "false")
	t.Setenv("DB_ENCRYPT", "disable")
	if err := ValidateRuntime(); err == nil {
		t.Fatal("production с выключенным шифрованием должен быть отклонён")
	}

	t.Setenv("DB_ENCRYPT", "strict")
	if err := ValidateRuntime(); err != nil {
		t.Fatalf("strict-шифрование должно приниматься: %v", err)
	}
}

func TestBuildConnStringReflectsTLSSettings(t *testing.T) {
	t.Setenv("DB_SERVER", "sql.example.com")
	t.Setenv("DB_USER", "portal")
	t.Setenv("DB_PASSWORD", "irrelevant")
	t.Setenv("DB_PORT", "1433")

	t.Setenv("DB_ENCRYPT", "")
	t.Setenv("DB_TRUST_SERVER_CERT", "")
	if got := buildConnString("db"); !strings.Contains(got, "encrypt=true;") || !strings.Contains(got, "TrustServerCertificate=1;") {
		t.Fatalf("значения по умолчанию не попали в строку подключения: %q", got)
	}

	t.Setenv("DB_ENCRYPT", "strict")
	t.Setenv("DB_TRUST_SERVER_CERT", "false")
	if got := buildConnString("db"); !strings.Contains(got, "encrypt=strict;") || !strings.Contains(got, "TrustServerCertificate=0;") {
		t.Fatalf("явные настройки TLS не попали в строку подключения: %q", got)
	}
}

func TestValidateRuntimeAcceptsExplicitProductionConfig(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_AUTO_CREATE", "false")
	t.Setenv("CORS_ORIGINS", "https://portal.example.com")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.10, 10.0.0.11")
	t.Setenv("DB_TRUST_SERVER_CERT", "false")
	t.Setenv("DB_ENCRYPT", "true")
	if err := ValidateRuntime(); err != nil {
		t.Fatalf("безопасная production-конфигурация отклонена: %v", err)
	}
	if got := TrustedProxies(); len(got) != 2 || got[0] != "10.0.0.10" || got[1] != "10.0.0.11" {
		t.Fatalf("неверный список trusted proxies: %#v", got)
	}
}
