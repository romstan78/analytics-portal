package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"backend/config"
	"backend/handlers"

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
	// Фоновая очистка мёртвых IP каждые 5 минут
	go rl.periodicCleanup(5 * time.Minute)
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Оставляем только записи внутри окна
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

// periodicCleanup удаляет IP с пустыми списками.
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

	// Rate limiter: 100 запросов в минуту
	limiter := NewRateLimiter(100, 1*time.Minute)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:5173"},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))
	r.Use(RateLimitMiddleware(limiter))

	// Интернет-продажи
	r.GET("/api/data", handlers.GetData)
	r.GET("/api/filters", handlers.GetFilterOptions)
	r.GET("/api/drilldown", handlers.GetDrilldown)

	// Промо
	r.GET("/api/promo/filters", handlers.GetPromoFilters)
	r.GET("/api/promo/data", handlers.GetPromoData)
	r.GET("/api/promo/sku-by-brand", handlers.GetSKUByBrand)
	r.GET("/api/promo/last-contract-price", handlers.GetLastContractPrice)
	r.GET("/api/promo/investment-types", handlers.GetInvestmentTypes)
	r.GET("/api/promo/kam-by-network", handlers.GetKAMByNetwork)
	r.GET("/api/promo/last-network-data", handlers.GetLastNetworkData)
	r.GET("/api/promo/history", handlers.GetPromoHistoryFiltered)
	r.GET("/api/promo/sku-info", handlers.GetSKUInfo)
	r.GET("/api/promo/last-sku-data", handlers.GetLastSKUData)
	r.GET("/api/promo/network-geo", handlers.GetNetworkGeoMapping)
	r.POST("/api/promo/save", handlers.SavePromo)
	r.DELETE("/api/promo/:id", handlers.DeletePromo)

	config.Logger.Info("server_starting", "port", "8080")
	if err := r.Run(":8080"); err != nil {
		config.Logger.Error("server_failed", "error", err.Error())
		log.Fatalf("Ошибка запуска: %v", err)
	}
}
