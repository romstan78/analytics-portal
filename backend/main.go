package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"backend/config"
	"backend/handlers"
	"backend/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// ─── Rate Limiter ───────────────────────────────────────────────────────────

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	go rl.periodicCleanup(5 * time.Minute)
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var valid []time.Time
	for _, t := range rl.visitors[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.visitors[ip] = valid
		return false
	}

	valid = append(valid, now)
	rl.visitors[ip] = valid
	return true
}

func (rl *RateLimiter) periodicCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, times := range rl.visitors {
			if len(times) == 0 {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func RateLimitMiddleware(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много запросов. Попробуйте позже."})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ─── Main ───────────────────────────────────────────────────────────────────

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env файл не найден")
	}

	config.Init()
	defer config.DB.Close()

	limiter := NewRateLimiter(100, 1*time.Minute)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:5173"},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))
	r.Use(RateLimitMiddleware(limiter))

	// ─── Публичный роут (без авторизации) ────────────────────────────────
	r.POST("/api/auth/login", handlers.Login)

	// ─── Защищённые роуты (требуется JWT) ────────────────────────────────
	api := r.Group("/api")
	api.Use(middleware.AuthRequired())
	{
		// Интернет-продажи
		api.GET("/data", handlers.GetData)
		api.GET("/filters", handlers.GetFilterOptions)
		api.GET("/drilldown", handlers.GetDrilldown)

		// Промо — чтение
		api.GET("/promo/filters", handlers.GetPromoFilters)
		api.GET("/promo/data", handlers.GetPromoData)
		api.GET("/promo/sku-by-brand", handlers.GetSKUByBrand)
		api.GET("/promo/last-contract-price", handlers.GetLastContractPrice)
		api.GET("/promo/investment-types", handlers.GetInvestmentTypes)
		api.GET("/promo/kam-by-network", handlers.GetKAMByNetwork)
		api.GET("/promo/last-network-data", handlers.GetLastNetworkData)
		api.GET("/promo/network-geo", handlers.GetNetworkGeoMapping)
		api.GET("/promo/history", handlers.GetPromoHistoryFiltered)
		api.GET("/promo/sku-info", handlers.GetSKUInfo)
		api.GET("/promo/last-sku-data", handlers.GetLastSKUData)

		// Промо — запись (agreement1, agreement2, admin)
		api.POST("/promo/save", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.SavePromo)
		api.GET("/promo/approvals", handlers.GetApprovals)
		api.GET("/promo/approval-kams", handlers.GetApprovalKAMs)
		api.POST("/promo/approve", handlers.ApprovePromo)

		// Промо — удаление (только admin)
		api.DELETE("/promo/:id", middleware.RoleRequired("admin"), handlers.DeletePromo)
	}

	config.Logger.Info("server_starting", "port", "8080")
	if err := r.Run(":8080"); err != nil {
		config.Logger.Error("server_failed", "error", err.Error())
		log.Fatalf("Ошибка запуска: %v", err)
	}
}
