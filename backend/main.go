package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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
	limit   rate.Limit
	burst   int
	message string
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{limit: r, burst: b, message: "Слишком много запросов. Попробуйте позже."}
}

// NewLoginRateLimiter — отдельный, куда более жёсткий лимит для входа.
//
// Общий лимит портала (100 запросов в минуту) для формы входа означает сотню
// попыток пароля в минуту с одного адреса: за сутки это 144 000 паролей по
// каждой учётной записи. Обычному человеку хватает нескольких попыток, поэтому
// счёт здесь идёт на единицы: 5 попыток в минуту, всплеск — те же 5.
func NewLoginRateLimiter() *IPRateLimiter {
	return &IPRateLimiter{
		limit:   5.0 / 60.0,
		burst:   5,
		message: "Слишком много попыток входа. Повторите через минуту.",
	}
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
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": rl.message})
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
	if err := config.ValidateRuntime(); err != nil {
		log.Fatalf("Ошибка production-конфигурации: %v", err)
	}

	if err := config.Init(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer config.DB.Close()

	// Файлы фоновых выгрузок от прошлого запуска: карта заданий после
	// перезапуска пуста, и убрать их по ней уже невозможно.
	handlers.CleanupSalesExportDir()
	// Фиктивный хеш пароля — до первого запроса, иначе первый вход по
	// несуществующему логину выдал бы себя временем ответа.
	handlers.WarmUpPasswordHashing()

	limiter := NewIPRateLimiter(100.0/60.0, 20) // 100 запросов в минуту, burst 20
	// Вход считается отдельно от остального API: подбор пароля не должен
	// прикрываться общим лимитом, рассчитанным на работу с интерфейсом.
	loginLimiter := NewLoginRateLimiter()

	if config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	// По умолчанию не доверяем X-Forwarded-For от произвольного клиента.
	// В production принимаем заголовок только от явно заданных proxy.
	if err := r.SetTrustedProxies(config.TrustedProxies()); err != nil {
		log.Fatalf("Ошибка настройки доверенных proxy: %v", err)
	}
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
	r.GET("/health", handlers.Liveness)
	r.GET("/ready", handlers.Readiness)
	r.POST("/api/auth/login", loginLimiter.RateLimitMiddleware(), handlers.Login)
	r.POST("/api/auth/refresh", handlers.RefreshToken)
	r.POST("/api/auth/logout", handlers.Logout)

	// ─── Защищённые роуты (требуется JWT) ────────────────────────────────
	api := r.Group("/api")
	api.Use(middleware.AuthRequired())
	{
		// Интернет-продажи
		api.GET("/data", handlers.GetData)
		api.GET("/sales/dashboard", handlers.GetSalesDashboard)
		api.GET("/sales/pivot", handlers.GetSalesPivot)
		api.GET("/sales/pivot/export-xlsx", handlers.ExportSalesPivotExcel)
		api.GET("/sales/network-options", handlers.GetSalesNetworkOptions)
		api.GET("/data/export-xlsx", handlers.ExportSalesExcel)
		api.POST("/data/export-jobs", handlers.StartSalesExcelExport)
		api.GET("/data/export-jobs/:id", handlers.GetSalesExcelExportJob)
		api.GET("/data/export-jobs/:id/download", handlers.DownloadSalesExcelExport)
		api.GET("/filters", handlers.GetFilterOptions)
		api.GET("/drilldown", handlers.GetDrilldown)

		// Администрирование мастер-справочников. Ключевые названия у уже
		// созданных записей не переименовываются этим API: так история промо и
		// продаж не теряет связи с мастер-данными.
		api.GET("/admin/dictionaries", middleware.RoleRequired("admin"), handlers.GetDictionaries)
		api.POST("/admin/dictionaries/skus", middleware.RoleRequired("admin"), handlers.CreateSKUReference)
		api.PATCH("/admin/dictionaries/skus/:id", middleware.RoleRequired("admin"), handlers.UpdateSKUReference)
		api.POST("/admin/dictionaries/networks", middleware.RoleRequired("admin"), handlers.CreateNetworkReference)
		api.PATCH("/admin/dictionaries/networks/:id", middleware.RoleRequired("admin"), handlers.UpdateNetworkReference)
		api.POST("/admin/dictionaries/kam-networks", middleware.RoleRequired("admin"), handlers.CreateKAMNetworkReference)
		api.PATCH("/admin/dictionaries/kam-networks/:id", middleware.RoleRequired("admin"), handlers.UpdateKAMNetworkReference)
		api.POST("/admin/dictionaries/mechanics", middleware.RoleRequired("admin"), handlers.CreateMechanicReference)
		api.PATCH("/admin/dictionaries/mechanics/:id", middleware.RoleRequired("admin"), handlers.UpdateMechanicReference)

		// Промо — чтение
		api.GET("/promo/filters", handlers.GetPromoFilters)
		api.GET("/promo/data", handlers.GetPromoData)
		api.GET("/promo/dashboard", handlers.GetPromoDashboard)
		api.GET("/promo/:id", handlers.GetPromoByID)
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
		// Пересчёт черновика карточки: формулы живут только в services.
		api.POST("/promo/calculate", handlers.PreviewPromoCalculations)

		// Промо — запись. Промо ведёт КАМ, поэтому его роль здесь обязательна;
		// кем именно можно править и заводить, решает область ведения в
		// обработчике, а не список ролей.
		api.POST("/promo/save", middleware.RoleRequired("admin", "agreement1", "agreement2", "kam"), handlers.SavePromo)
		// Доступ к согласованию зависит от закрепления, а не только от роли,
		// поэтому его выясняет отдельный запрос без ограничения по роли.
		api.GET("/promo/approval-access", handlers.GetApprovalAccess)
		api.GET("/promo/approvals", middleware.RoleRequired("admin", "agreement1", "agreement2", "kam"), handlers.GetApprovals)
		api.GET("/promo/approval-filters", middleware.RoleRequired("admin", "agreement1", "agreement2", "kam"), handlers.GetApprovalFilters)
		api.POST("/promo/approve", middleware.RoleRequired("admin", "agreement1", "agreement2", "kam"), handlers.ApprovePromo)
		api.POST("/promo/approve/batch", middleware.RoleRequired("admin", "agreement1", "agreement2", "kam"), handlers.BatchApprovePromo)

		// Реестр сетей — чтение
		api.GET("/networks", handlers.GetNetworks)
		api.GET("/networks/brands", handlers.GetNetworkBrands)
		api.GET("/networks/kams", handlers.GetNetworkKAMs)
		// Витрина реестра: собственная область видимости внутри обработчика.
		api.GET("/networks/dashboard", handlers.GetNetworkDashboard)
		api.GET("/networks/:id/plan", handlers.NetworkAccessRequired(), handlers.GetNetworkPlan)
		api.GET("/networks/:id/forecast", handlers.NetworkAccessRequired(), handlers.GetNetworkForecast)
		api.GET("/networks/:id/prices", handlers.NetworkAccessRequired(), handlers.GetNetworkPrices)
		api.GET("/networks/:id/comments", handlers.NetworkAccessRequired(), handlers.GetNetworkComments)
		api.GET("/networks/:id/audit", handlers.NetworkAccessRequired(), handlers.GetNetworkAudit)

		// Реестр сетей — запись (планы вносят КАМы)
		api.POST("/networks", middleware.RoleRequired("admin", "kam"), handlers.CreateNetwork)
		api.PATCH("/networks/:id", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.UpdateNetwork)
		api.POST("/networks/:id/plan", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.SaveNetworkPlan)
		api.POST("/networks/:id/investment-payment-modes", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.UpdateNetworkInvestmentPaymentModes)
		api.POST("/networks/:id/forecast", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.SaveNetworkForecast)
		api.POST("/networks/:id/forecast/import/preview", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.PreviewNetworkForecastImport)
		api.POST("/networks/:id/forecast/import", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.ImportNetworkForecast)
		api.POST("/networks/:id/forecast/clear", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.ClearNetworkForecast)
		api.POST("/networks/:id/entry-mode", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.UpdateNetworkEntryMode)
		api.POST("/networks/:id/prices", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.SaveNetworkPrices)
		// Пересчёт черновика: расчёт живёт только на бэкенде, в БД не пишет.
		api.POST("/networks/:id/plan/preview", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam"), handlers.PreviewNetworkPlan)
		api.POST("/networks/:id/comments", handlers.NetworkAccessRequired(), middleware.RoleRequired("admin", "kam", "agreement1", "agreement2"), handlers.AddNetworkComment)

		// Промо — удаление/восстановление (только admin)
		api.DELETE("/promo/:id", middleware.RoleRequired("admin"), handlers.DeletePromo)
		api.PATCH("/promo/:id/restore", middleware.RoleRequired("admin"), handlers.RestorePromo)
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	config.Logger.Info("server_starting", "port", port)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Minute, // большие Excel-выгрузки
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			config.Logger.Error("server_failed", "error", err.Error())
			log.Fatalf("Ошибка запуска: %v", err)
		}
	case <-shutdownSignal.Done():
		config.Logger.Info("server_shutdown_started")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		config.Logger.Error("server_shutdown_failed", "error", err.Error())
		if closeErr := server.Close(); closeErr != nil {
			config.Logger.Error("server_force_close_failed", "error", closeErr.Error())
		}
	}
	config.Logger.Info("server_stopped")
}
