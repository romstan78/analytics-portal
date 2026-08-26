package handlers

import (
	"errors"
	"net/http"
	"strings"
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

	// Регистрируем сессию: без записи в реестре refresh-токен не сработает,
	// поэтому ошибку здесь нельзя проглатывать.
	if err := repository.CreateRefreshSession(req.Username, refreshToken, time.Now().Add(config.RefreshTokenTTL)); err != nil {
		config.Logger.Error("refresh_session_create_failed", "error", err.Error(), "username", req.Username)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}
	// Обслуживание реестра: убираем записи, истёкшие более суток назад.
	if err := repository.DeleteExpiredRefreshSessions(24 * time.Hour); err != nil {
		config.Logger.Error("refresh_session_cleanup_failed", "error", err.Error())
	}

	setRefreshCookie(c, refreshToken)

	c.JSON(http.StatusOK, gin.H{
		"token":    accessToken,
		"username": req.Username,
		"role":     user.Role,
		// Отображаемое имя: у КАМа это имя из справочника, у остальных —
		// логин. Интерфейс подписывает пользователя им, а не логином.
		"display_name": displayName(user.Username, user.KAM),
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

	// Токен одноразовый: гасим сессию до выдачи новой пары. Если активной
	// сессии нет, значит этот refresh уже использовали или отозвали — при
	// корректной работе клиента такого не бывает, поэтому считаем это признаком
	// компрометации и гасим все сессии пользователя.
	sessionUser, err := repository.ConsumeRefreshSession(refreshToken)
	if errors.Is(err, repository.ErrSessionNotActive) {
		revoked, revokeErr := repository.RevokeAllUserSessions(claims.Username, repository.RevokeCauseReuseDetected)
		if revokeErr != nil {
			config.Logger.Error("refresh_revoke_all_failed", "error", revokeErr.Error(), "username", claims.Username)
		}
		config.Logger.Warn("refresh_token_reuse_detected", "username", claims.Username, "revoked_sessions", revoked)
		clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "сессия завершена, войдите заново"})
		return
	}
	if err != nil {
		config.Logger.Error("refresh_session_consume_failed", "error", err.Error(), "username", claims.Username)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}
	if sessionUser != claims.Username {
		// Подпись и реестр разошлись — доверять такой паре нельзя.
		config.Logger.Warn("refresh_session_user_mismatch", "token_user", claims.Username, "session_user", sessionUser)
		clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "сессия завершена, войдите заново"})
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
	if err := repository.CreateRefreshSession(user.Username, newRefreshToken, time.Now().Add(config.RefreshTokenTTL)); err != nil {
		config.Logger.Error("refresh_session_rotate_failed", "error", err.Error(), "username", user.Username)
		clearRefreshCookie(c)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}

	setRefreshCookie(c, newRefreshToken)

	c.JSON(http.StatusOK, gin.H{
		"token":        newAccessToken,
		"username":     user.Username,
		"role":         user.Role,
		"display_name": displayName(user.Username, user.KAM),
	})
}

// setRefreshCookie кладёт refresh-токен в httpOnly cookie, доступную только
// эндпоинтам /api/auth/*. SameSite задан явно.
// displayName — как подписывать пользователя в интерфейсе. Логин собран
// транслитерацией и читается плохо, поэтому у закреплённого КАМа показывается
// его имя из справочника.
func displayName(username, kam string) string {
	if trimmed := strings.TrimSpace(kam); trimmed != "" {
		return trimmed
	}
	return username
}

func setRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"refresh_token",
		token,
		int(config.RefreshTokenTTL.Seconds()),
		"/api/auth",
		"",
		config.IsProduction(), // secure только на проде
		true,                  // httpOnly
	)
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

// Logout отзывает refresh-сессию на сервере и очищает cookie. Отзыв означает,
// что даже перехваченный ранее токен после выхода бесполезен.
func Logout(c *gin.Context) {
	if refreshToken, err := c.Cookie("refresh_token"); err == nil && refreshToken != "" {
		if err := repository.RevokeRefreshSession(refreshToken, repository.RevokeCauseLogout); err != nil {
			// Выход не должен падать из-за недоступной базы: cookie всё равно
			// очищаем, но факт неудачного отзыва фиксируем.
			config.Logger.Error("refresh_session_revoke_failed", "error", err.Error())
		}
	}
	clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}
