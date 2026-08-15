package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"backend/config"
	"backend/handlers"
	"backend/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

// ─── Rate Limiter ───────────────────────────────────────────────────────────

// IPRateLimiter — Token Bucket rate limiter на базе golang.org/x/time/rate.
type IPRateLimiter struct {
	limit rate.Limit
	burst int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{limit: r, burst: b}
}

func (rl *IPRateLimiter) RateLimitMiddleware() gin.HandlerFunc {
	// Очистка старых лимитеров раз в 5 минут
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	clients := make(map[string]*client)
	var mu sync.Mutex

	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 5*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		v, exists := clients[ip]
		if !exists {
			v = &client{limiter: rate.NewLimiter(rl.limit, rl.burst)}
			clients[ip] = v
		}
		v.lastSeen = time.Now()
		mu.Unlock()

		if !v.limiter.Allow() {
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
	if err := config.InitAuth(); err != nil {
		log.Fatalf("Ошибка конфигурации авторизации: %v", err)
	}

	config.Init()
	defer config.DB.Close()

	limiter := NewIPRateLimiter(100.0/60.0, 20) // 100 запросов в минуту, burst 20

	r := gin.Default()
	corsOrigins := []string{"http://localhost:5173"}
	if env := os.Getenv("CORS_ORIGINS"); env != "" {
		corsOrigins = strings.Split(env, ",")
		for i := range corsOrigins {
			corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	r.Use(limiter.RateLimitMiddleware())

	// ─── Публичный роут (без авторизации) ────────────────────────────────
	r.POST("/api/auth/login", handlers.Login)
	r.POST("/api/auth/refresh", handlers.RefreshToken)
	r.POST("/api/auth/logout", handlers.Logout)

	// ─── Защищённые роуты (требуется JWT) ────────────────────────────────
	api := r.Group("/api")
	api.Use(middleware.AuthRequired())
	{
		// Интернет-продажи
		api.GET("/data", handlers.GetData)
		api.GET("/data/export-xlsx", handlers.ExportSalesExcel)
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
		api.GET("/promo/comments/:id", handlers.GetPromoCommentsHandler)
		api.GET("/promo/export-xlsx", handlers.ExportPromoExcel)

		// Промо — запись (agreement1, agreement2, admin)
		api.POST("/promo/save", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.SavePromo)
		api.GET("/promo/approvals", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.GetApprovals)
		api.GET("/promo/approval-filters", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.GetApprovalFilters)
		api.GET("/promo/approval-kams", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.GetApprovalKAMs)
		api.GET("/promo/approval-networks", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.GetApprovalNetworks)
		api.GET("/promo/approval-brands", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.GetApprovalBrands)
		api.POST("/promo/approve", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.ApprovePromo)
		api.POST("/promo/approve/batch", middleware.RoleRequired("admin", "agreement1", "agreement2"), handlers.BatchApprovePromo)

		// Промо — удаление/восстановление (только admin)
		api.DELETE("/promo/:id", middleware.RoleRequired("admin"), handlers.DeletePromo)
		api.PATCH("/promo/:id/restore", middleware.RoleRequired("admin"), handlers.RestorePromo)
	}

	config.Logger.Info("server_starting", "port", "8080")
	if err := r.Run(":8080"); err != nil {
		config.Logger.Error("server_failed", "error", err.Error())
		log.Fatalf("Ошибка запуска: %v", err)
	}
}
