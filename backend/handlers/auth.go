package handlers

import (
	"net/http"
	"time"

	"backend/config"

	"github.com/gin-gonic/gin"
)

// Простая таблица пользователей. В будущем — из БД tbl_Users.
var users = map[string]struct {
	Password string
	Role     string
}{
	"manager1": {Password: "promo2024!", Role: "agreement1"},
	"manager2": {Password: "promo2024!", Role: "agreement2"},
	"admin":    {Password: "admin2024!", Role: "admin"},
}

func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}

	user, ok := users[req.Username]
	if !ok || user.Password != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неверный логин или пароль"})
		return
	}

	accessToken, err := config.GenerateAccessToken(req.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка генерации токена"})
		return
	}

	refreshToken, err := config.GenerateRefreshToken(req.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка генерации токена"})
		return
	}

	// Refresh token в httpOnly secure cookie
	c.SetCookie(
		"refresh_token",
		refreshToken,
		int(7*24*time.Hour.Seconds()), // 7 дней
		"/api/auth",                   // доступен только для /api/auth/*
		"",                            // domain (текущий)
		false,                         // secure (false для localhost)
		true,                          // httpOnly
	)

	c.JSON(http.StatusOK, gin.H{
		"token":    accessToken,
		"username": req.Username,
		"role":     user.Role,
	})
}

func RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token не найден"})
		return
	}

	claims, err := config.ValidateRefreshToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "недействительный refresh token"})
		return
	}

	newAccessToken, err := config.GenerateAccessToken(claims.Username, claims.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка генерации токена"})
		return
	}

	newRefreshToken, err := config.GenerateRefreshToken(claims.Username, claims.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка генерации токена"})
		return
	}

	// Обновляем refresh cookie
	c.SetCookie(
		"refresh_token",
		newRefreshToken,
		int(7*24*time.Hour.Seconds()),
		"/api/auth",
		"",
		false,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"token":    newAccessToken,
		"username": claims.Username,
		"role":     claims.Role,
	})
}
