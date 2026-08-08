package config

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func requireSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		if os.Getenv("GO_ENV") == "production" {
			panic("JWT_SECRET is required in production environment")
		}
		secret = "change-me-in-development-only"
	}
	return []byte(secret)
}

var JWTSecret = requireSecret()

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(username, role string) (string, error) {
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "analytics-portal",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

func GenerateRefreshToken(username, role string) (string, error) {
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "analytics-portal",
			ID:        "refresh",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// Deprecated: используйте GenerateAccessToken
func GenerateToken(username, role string) (string, error) {
	return GenerateAccessToken(username, role)
}

func ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return JWTSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func ValidateRefreshToken(tokenStr string) (*Claims, error) {
	claims, err := ValidateToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.ID != "refresh" {
		return nil, errors.New("not a refresh token")
	}
	return claims, nil
}
