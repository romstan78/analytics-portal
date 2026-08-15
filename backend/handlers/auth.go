package handlers

import (
	"net/http"
	"time"

	"backend/config"
	"backend/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}

	// Ищем пользователя в БД
	user, err := repository.GetUserByUsername(req.Username)
	if err != nil {
		config.Logger.Error("login_db_error", "error", err.Error(), "username", req.Username)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}

	if user == nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
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

	// Refresh token в httpOnly cookie. SameSite явно задан для защиты refresh endpoint.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"refresh_token",
		refreshToken,
		int(7*24*time.Hour.Seconds()), // 7 дней
		"/api/auth",                   // доступен только для /api/auth/*
		"",                            // domain (текущий)
		config.IsProduction(),         // secure (true только на проде)
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
	user, err := repository.GetUserByUsername(claims.Username)
	if err != nil {
		config.Logger.Error("refresh_user_lookup_failed", "error", err.Error(), "username", claims.Username)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}
	if user == nil {
		clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "пользователь деактивирован"})
		return
	}

	newAccessToken, err := config.GenerateAccessToken(user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка генерации токена"})
		return
	}

	newRefreshToken, err := config.GenerateRefreshToken(user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка генерации токена"})
		return
	}

	// Обновляем refresh cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"refresh_token",
		newRefreshToken,
		int(7*24*time.Hour.Seconds()),
		"/api/auth",
		"",
		config.IsProduction(),
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"token":    newAccessToken,
		"username": user.Username,
		"role":     user.Role,
	})
}

func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"refresh_token",
		"",
		-1,
		"/api/auth",
		"",
		config.IsProduction(),
		true,
	)
}

// Logout очищает refresh cookie. Серверное хранение и отзыв refresh-сессий
// будут добавлены отдельным этапом; текущий endpoint закрывает браузерную сессию.
func Logout(c *gin.Context) {
	clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}
