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
