package handlers

import (
	"backend/config"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func Readiness(c *gin.Context) {
	if config.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := config.DB.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	// Состояние пула отдаётся вместе с готовностью: исчерпание видно только по
	// нему. wait_count — сколько раз запрос ждал свободного соединения; растущее
	// значение и есть тот самый «пул исчерпан», который иначе проявляется лишь
	// задержками на стороне пользователя.
	stats := config.DB.Stats()
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"db_pool": gin.H{
			"max_open":            stats.MaxOpenConnections,
			"open":                stats.OpenConnections,
			"in_use":              stats.InUse,
			"idle":                stats.Idle,
			"wait_count":          stats.WaitCount,
			"wait_duration_ms":    stats.WaitDuration.Milliseconds(),
			"max_idle_closed":     stats.MaxIdleClosed,
			"max_lifetime_closed": stats.MaxLifetimeClosed,
		},
	})
}
