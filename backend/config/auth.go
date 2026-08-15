package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtIssuer        = "analytics-portal"
	jwtAudience      = "analytics-portal-api"
	accessTokenType  = "access"
	refreshTokenType = "refresh"
	minJWTSecretLen  = 32
)

var jwtSecret []byte

// InitAuth загружает JWT-конфигурацию после того, как main загрузил .env.
// Слабого fallback-секрета нет: приложение должно завершиться до старта HTTP-сервера.
func InitAuth() error {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if len(secret) < minJWTSecretLen {
		return fmt.Errorf("JWT_SECRET должен содержать не менее %d символов", minJWTSecretLen)
	}
	jwtSecret = []byte(secret)
	return nil
}

// IsProduction использует единую переменную окружения для production-настроек.
func IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

type Claims struct {
	Username  string `json:"username"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func signingSecret() ([]byte, error) {
	if len(jwtSecret) < minJWTSecretLen {
		return nil, errors.New("JWT-конфигурация не инициализирована")
	}
	return jwtSecret, nil
}

func GenerateAccessToken(username, role string) (string, error) {
	secret, err := signingSecret()
	if err != nil {
		return "", err
	}
	claims := Claims{
		Username:  username,
		Role:      role,
		TokenType: accessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func GenerateRefreshToken(username, role string) (string, error) {
	secret, err := signingSecret()
	if err != nil {
		return "", err
	}
	claims := Claims{
		Username:  username,
		Role:      role,
		TokenType: refreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// Deprecated: используйте GenerateAccessToken
func GenerateToken(username, role string) (string, error) {
	return GenerateAccessToken(username, role)
}

func validateToken(tokenStr, expectedType string) (*Claims, error) {
	secret, err := signingSecret()
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(t *jwt.Token) (interface{}, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != expectedType {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}

func ValidateAccessToken(tokenStr string) (*Claims, error) {
	return validateToken(tokenStr, accessTokenType)
}

// ValidateToken сохранён как совместимый алиас для проверки access-токена.
func ValidateToken(tokenStr string) (*Claims, error) {
	return ValidateAccessToken(tokenStr)
}

func ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return validateToken(tokenStr, refreshTokenType)
}
