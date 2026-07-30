package handlers

import (
	"net/http"

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

	token, err := config.GenerateToken(req.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка генерации токена"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":    token,
		"username": req.Username,
		"role":     user.Role,
	})
}
