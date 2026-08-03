This file is a merged representation of a subset of the codebase, containing files not matching ignore patterns, combined into a single document by Repomix.

# File Summary

## Purpose
This file contains a packed representation of a subset of the repository's contents that is considered the most important context.
It is designed to be easily consumable by AI systems for analysis, code review,
or other automated processes.

## File Format
The content is organized as follows:
1. This summary section
2. Repository information
3. Directory structure
4. Repository files (if enabled)
5. Multiple file entries, each consisting of:
  a. A header with the file path (## File: path/to/file)
  b. The full contents of the file in a code block

## Usage Guidelines
- This file should be treated as read-only. Any changes should be made to the
  original repository files, not this packed version.
- When processing this file, use the file path to distinguish
  between different files in the repository.
- Be aware that this file may contain sensitive information. Handle it with
  the same level of security as you would the original repository.

## Notes
- Some files may have been excluded based on .gitignore rules and Repomix's configuration
- Binary files are not included in this packed representation. Please refer to the Repository Structure section for a complete list of file paths, including binary files
- Files matching these patterns are excluded: node_modules/**, dist/**, build/**, .venv/**, __pycache__/**, vendor/**, **/*.md, upload/**, sync_script/**, **/*.xlsx, **/*.docx, **/*.csv, .clinerules, .clineignore, .gitignore, .env*, **/*.lock, **/package-lock.json, repomix-output.*, project_code.md, repomix.config.json, **/*.svg, **.*.png, **/*.jpg, **/*.jpeg, **/*.gif, **/*.ico, **/*.webp, **/*.woff, **/*.woff2, **/*.ttf, **/*.eot
- Files matching patterns in .gitignore are excluded
- Files matching default ignore patterns are excluded
- Files are sorted by Git change count (files with more changes are at the bottom)

# Directory Structure
```
backend/
  cmd/
    hash_password.go
  config/
    auth.go
    db.go
  handlers/
    auth.go
    promo_utils.go
    promo.go
    sales.go
  middleware/
    auth.go
  migrations/
    001_create_tbl_users.sql
    002_split_agreement_fields.sql
    seed_users.sql
  models/
    types.go
  repository/
    promo_repo.go
    user_repo.go
  services/
    promo_service.go
  .env.example
  go.mod
  main_test.go
  main.go
frontend/
  src/
    api/
      auth.js
      promo.js
    assets/
      hero.png
    components/
      ApprovalCard.jsx
      DataTable.jsx
      DrilldownModal.jsx
      FilterPanel.jsx
      PromoEditDialog.jsx
    hooks/
      usePromoCalculations.js
      usePromoData.js
      usePromoFilters.js
      usePromoForm.js
    pages/
      Home.jsx
      InternetSales.jsx
      Login.jsx
      PromoAnalysis.jsx
      PromoApproval.jsx
      PromoForm.jsx
    types/
      promo.ts
    App.css
    App.jsx
    index.css
    main.jsx
  .gitignore
  eslint.config.js
  index.html
  package.json
  vite.config.js
docker-compose.yml
```

# Files

## File: backend/cmd/hash_password.go
```go
//go:build ignore
// +build ignore

// Утилита для генерации bcrypt-хеша из пароля.
// Использование:
//   go run cmd/hash_password.go ваш_пароль
//
// Выводит bcrypt-хеш (cost=10), который можно вставить в tbl_Users.password_hash.

package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Использование: go run cmd/hash_password.go <пароль>")
		os.Exit(1)
	}

	password := os.Args[1]
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}
```

## File: backend/config/db.go
```go
package config

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"gopkg.in/natefinch/lumberjack.v2"
)

var DB *sql.DB
var Logger *slog.Logger

func buildConnString() string {
	return fmt.Sprintf(
		"server=%s;user id=%s;password=%s;database=%s;port=%s;TrustServerCertificate=1;",
		os.Getenv("DB_SERVER"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
}

func Init() {
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("Не удалось создать папку logs: %v", err)
	}
	logWriter := &lumberjack.Logger{
		Filename:   "logs/app.log",
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
	handler := slog.NewJSONHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelInfo})
	Logger = slog.New(handler)
	slog.SetDefault(Logger)

	var err error
	DB, err = sql.Open("mssql", buildConnString())
	if err != nil {
		Logger.Error("db_connection_failed", "error", err.Error())
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * 60 * 1e9)
	DB.SetConnMaxIdleTime(5 * time.Minute)

	if err = DB.Ping(); err != nil {
		Logger.Error("db_ping_failed", "error", err.Error())
		log.Fatalf("Нет соединения с БД: %v", err)
	}
	Logger.Info("db_connected", "host", os.Getenv("DB_SERVER"), "database", os.Getenv("DB_NAME"))
}
```

## File: backend/middleware/auth.go
```go
package middleware

import (
	"net/http"
	"strings"

	"backend/config"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "требуется авторизация"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "неверный формат токена"})
			c.Abort()
			return
		}

		claims, err := config.ValidateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "токен недействителен"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// Только для определённых ролей
func RoleRequired(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещён"})
		c.Abort()
	}
}
```

## File: backend/migrations/001_create_tbl_users.sql
```sql
-- Migration: создание tbl_Users и перенос хардкод-пользователей
-- Запустить вручную на БД.

IF OBJECT_ID('dbo.tbl_Users', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_Users (
        id          INT IDENTITY PRIMARY KEY,
        username    NVARCHAR(100) NOT NULL UNIQUE,
        password_hash NVARCHAR(255) NOT NULL,
        role        NVARCHAR(50) NOT NULL DEFAULT 'agreement1',
        created_at  DATETIME DEFAULT GETDATE(),
        updated_at  DATETIME DEFAULT GETDATE(),
        deleted_at  DATETIME NULL
    );
END
GO

-- Seed: создаём тестовых пользователей (если ещё нет).
-- Пароли захешированы bcrypt (cost=10):
--   manager1 / promo2024!   → $2a$10$... (сгенерируйте реальный хеш)
--   manager2 / promo2024!   → $2a$10$...
--   admin    / admin2024!   → $2a$10$...

-- Используйте Go-скрипт для генерации хешей:
--   go run backend/cmd/hash_password.go promo2024!
-- Ниже — пример c заранее сгенерированными хешами (замените на свои).

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager1' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager1', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement1');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager2' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager2', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement2');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'admin' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('admin', '$2a$10$jyr5S3OrcUK5UmgUwsnDCeUjhEHdcQji7tO.L.y0oAncVBA/YTzFO', 'admin');
```

## File: backend/migrations/002_split_agreement_fields.sql
```sql
-- Migration: разделение agreement1/agreement2 на status + comment
-- Заменить CHARINDEX-парсинг на нормальные колонки.

-- Добавляем новые колонки (если ещё не добавлены)
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement1_status') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement1_status NVARCHAR(20) NULL;
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement1_comment') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement1_comment NVARCHAR(MAX) NULL;
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement2_status') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement2_status NVARCHAR(20) NULL;
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement2_comment') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement2_comment NVARCHAR(MAX) NULL;
GO

-- Миграция существующих данных: парсим старые текстовые поля
-- approved: начинается с "согласовано"
UPDATE dbo.tbl_PromoActivities
SET agreement1_status = 'approved',
    agreement1_comment = CASE WHEN agreement1 LIKE N'согласовано: %'
      THEN SUBSTRING(agreement1, CHARINDEX(N':', agreement1) + 2, LEN(agreement1))
      ELSE NULL END
WHERE agreement1 IS NOT NULL
  AND CHARINDEX(N'согласовано', agreement1) = 1
  AND agreement1_status IS NULL;

-- rejected: начинается с "отклонено"
UPDATE dbo.tbl_PromoActivities
SET agreement1_status = 'rejected',
    agreement1_comment = CASE WHEN agreement1 LIKE N'отклонено: %'
      THEN SUBSTRING(agreement1, CHARINDEX(N':', agreement1) + 2, LEN(agreement1))
      ELSE NULL END
WHERE agreement1 IS NOT NULL
  AND CHARINDEX(N'отклонено', agreement1) = 1
  AND agreement1_status IS NULL;

-- commented: всё остальное не-NULL
UPDATE dbo.tbl_PromoActivities
SET agreement1_status = 'commented',
    agreement1_comment = agreement1
WHERE agreement1 IS NOT NULL
  AND agreement1_status IS NULL;

-- То же для agreement2
UPDATE dbo.tbl_PromoActivities
SET agreement2_status = 'approved',
    agreement2_comment = CASE WHEN agreement2 LIKE N'согласовано: %'
      THEN SUBSTRING(agreement2, CHARINDEX(N':', agreement2) + 2, LEN(agreement2))
      ELSE NULL END
WHERE agreement2 IS NOT NULL
  AND CHARINDEX(N'согласовано', agreement2) = 1
  AND agreement2_status IS NULL;

UPDATE dbo.tbl_PromoActivities
SET agreement2_status = 'rejected',
    agreement2_comment = CASE WHEN agreement2 LIKE N'отклонено: %'
      THEN SUBSTRING(agreement2, CHARINDEX(N':', agreement2) + 2, LEN(agreement2))
      ELSE NULL END
WHERE agreement2 IS NOT NULL
  AND CHARINDEX(N'отклонено', agreement2) = 1
  AND agreement2_status IS NULL;

UPDATE dbo.tbl_PromoActivities
SET agreement2_status = 'commented',
    agreement2_comment = agreement2
WHERE agreement2 IS NOT NULL
  AND agreement2_status IS NULL;

-- После миграции старые колонки можно оставить для обратной совместимости,
-- но бэкенд должен писать в новые поля.
```

## File: backend/migrations/seed_users.sql
```sql
-- Seed: пользователи с реальными bcrypt-хешами (cost=10)
-- Пароли: manager1/promo2024!, manager2/promo2024!, admin/admin2024!

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager1' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager1', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement1');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager2' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager2', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement2');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'admin' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('admin', '$2a$10$jyr5S3OrcUK5UmgUwsnDCeUjhEHdcQji7tO.L.y0oAncVBA/YTzFO', 'admin');
```

## File: backend/repository/user_repo.go
```go
package repository

import (
	"backend/config"
)

// UserRecord — запись из tbl_Users.
type UserRecord struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
}

// GetUserByUsername возвращает пользователя из БД по логину.
// Если пользователь не найден, возвращает nil, nil.
func GetUserByUsername(username string) (*UserRecord, error) {
	var u UserRecord
	err := config.DB.QueryRow(
		"SELECT id, username, password_hash, role FROM dbo.tbl_Users WHERE username = ? AND deleted_at IS NULL",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)
	if err != nil {
		// sql.ErrNoRows — не ошибка, просто пользователь не найден
		return nil, nil
	}
	return &u, nil
}
```

## File: backend/services/promo_service.go
```go
package services

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"backend/repository"
)

// PromoInputDTO — типизированная структура для входных данных промо-акции.
// Заменяет map[string]interface{} в SavePromo и calculatePromoFields.
type PromoInputDTO struct {
	// Основные поля
	NetworkName     string  `json:"network_name"`
	KAM             string  `json:"kam"`
	Brand           string  `json:"brand"`
	BrandAS         string  `json:"brand_as"`
	SKU             string  `json:"sku"`
	Year            int     `json:"year"`
	Month           int     `json:"month"`
	Quarter         int     `json:"quarter"`
	Mechanics       string  `json:"mechanics"`
	GTNOpex         string  `json:"gtn_opex"`
	IDDirectum      string  `json:"id_directum"`
	DSNumber        string  `json:"ds_number"`
	DiscountAmount  float64 `json:"discount_amount"`
	Conditions      string  `json:"conditions"`
	Comments        string  `json:"comments"`
	EcomSegment     string  `json:"ecom_segment"`
	TotalPharmacies int     `json:"total_pharmacies"`
	PromoPharmacies int     `json:"promo_pharmacies"`
	Status          string  `json:"status"`
	Date            string  `json:"date"`

	// Плановые показатели
	BaselineUnits      float64 `json:"baseline_units"`
	BaselineRub        float64 `json:"baseline_rub"`
	PlanPromoUnits     float64 `json:"plan_promo_units"`
	PlanPromoRub       float64 `json:"plan_promo_rub"`
	PlanInvestmentsRub float64 `json:"plan_investments_rub"`
	ContractPrice      float64 `json:"contract_price"`
	GM                 float64 `json:"gm"`

	// Ключевой регион и сегмент (из Geo-маппинга, если не заданы)
	KeyRegion    string `json:"key_region"`
	Top20Segment string `json:"top20_segment"`

	// OLAP цена
	OlapPrice float64 `json:"olap_price"`

	// Фактические показатели
	ActualPromoSalesUnits   float64 `json:"actual_promo_sales_units"`
	ActualPromoRub          float64 `json:"actual_promo_rub"`
	ActualInvestments       float64 `json:"actual_investments"`
	ActualPromoUpliftUnits  float64 `json:"actual_promo_uplift_units"`
	ActualPromoUpliftRub    float64 `json:"actual_promo_uplift_rub"`
	ActualExternalEcomUnits float64 `json:"actual_external_ecom_units"`
	ActualCorrectedBaseline float64 `json:"actual_corrected_baseline"`
	Agreement1              string  `json:"agreement1"`
	Agreement2              string  `json:"agreement2"`
}

// CalculatedFields — результат вычислений.
type CalculatedFields struct {
	Year                         int     `json:"year"`
	Month                        int     `json:"month"`
	Quarter                      int     `json:"quarter"`
	GM                           float64 `json:"gm"`
	PlanPromoRub                 float64 `json:"plan_promo_rub"`
	PlanPromoUpliftUnits         float64 `json:"plan_promo_uplift_units"`
	PlanPromoUpliftRub           float64 `json:"plan_promo_uplift_rub"`
	PlanPromoUpliftPctUnits      float64 `json:"plan_promo_uplift_pct_units"`
	PlanPromoUpliftPctRub        float64 `json:"plan_promo_uplift_pct_rub"`
	PlanInvestmentsPct           float64 `json:"plan_investments_pct"`
	PlanROI                      float64 `json:"plan_roi"`
	BaselineRub                  float64 `json:"baseline_rub"`
	NetPromoUpliftRub            float64 `json:"net_promo_uplift_rub"`
	NetPromoUpliftPct            float64 `json:"net_promo_uplift_pct"`
	ActualInvestmentsPct         float64 `json:"actual_investments_pct"`
	ActualROI                    float64 `json:"actual_roi"`
	ActualPromoRubWoEcom         float64 `json:"actual_promo_rub_wo_ecom"`
	ActualPromoUpliftUnitsWoEcom float64 `json:"actual_promo_uplift_units_wo_ecom"`
	ActualPromoUpliftRubWoEcom   float64 `json:"actual_promo_uplift_rub_wo_ecom"`
	NetPromoUpliftRubWoEcom      float64 `json:"net_promo_uplift_rub_wo_ecom"`
	NetPromoUpliftPctWoEcom      float64 `json:"net_promo_uplift_pct_wo_ecom"`
	ActualInvestmentsPctWoEcom   float64 `json:"actual_investments_pct_wo_ecom"`
	ActualROIWoEcom              float64 `json:"actual_roi_wo_ecom"`
	PlanVsFactRub                float64 `json:"plan_vs_fact_rub"`
	PlanVsFactInvestments        float64 `json:"plan_vs_fact_investments"`
	TurnoverPerPoint             float64 `json:"turnover_per_point"`
	TurnoverPerPointPromo        float64 `json:"turnover_per_point_promo"`
	PlanPromoCipOlap             float64 `json:"plan_promo_cip_olap"`
	FactPromoCipOlap             float64 `json:"fact_promo_cip_olap"`
	PlanPromoUpliftCipOlap       float64 `json:"plan_promo_uplift_cip_olap"`
	FactPromoUpliftCipOlap       float64 `json:"fact_promo_uplift_cip_olap"`
	Date                         string  `json:"date"`
	OlapPrice                    float64 `json:"olap_price"`
}

// CalculationContext — контекст с данными, полученными от репозитория,
// которые нужны для расчета, но не пришли от клиента.
type CalculationContext struct {
	GM           float64
	KeyRegion    string
	Top20Segment string
	OlapPrice    float64
}

// EnrichFromRepo — заполняет недостающие данные из БД через репозиторий.
// Раньше это делалось прямыми SQL-запросами в calculatePromoFields.
func EnrichFromRepo(input *PromoInputDTO) CalculationContext {
	ctx := CalculationContext{}

	// GM: если не задан — ищем последнюю запись по SKU
	if input.GM == 0 {
		lastData, err := repository.GetLastSKUData(input.SKU)
		if err == nil && lastData != nil && lastData.GM != 0 {
			ctx.GM = lastData.GM
		} else {
			ctx.GM = 1 // fallback по умолчанию
		}
	} else {
		ctx.GM = input.GM
	}

	// KeyRegion / Top20Segment: если не заданы — ищем из Geo-маппинга
	if input.KeyRegion == "" || input.Top20Segment == "" {
		geo, err := repository.GetNetworkGeoMapping(input.NetworkName)
		if err == nil && geo != nil {
			if input.KeyRegion == "" && geo.KeyRegion != "" {
				input.KeyRegion = geo.KeyRegion
			}
			if input.Top20Segment == "" && geo.Top20Segment != "" {
				input.Top20Segment = geo.Top20Segment
			}
		}
	}
	ctx.KeyRegion = input.KeyRegion
	ctx.Top20Segment = input.Top20Segment

	// OLAP price
	if input.OlapPrice == 0 {
		lastData, err := repository.GetLastSKUData(input.SKU)
		if err == nil && lastData != nil && lastData.OlapPrice != 0 {
			ctx.OlapPrice = lastData.OlapPrice
		}
	} else {
		ctx.OlapPrice = input.OlapPrice
	}

	return ctx
}

// CalculateFields — чистая функция расчета всех вычисляемых полей.
// Не делает запросов в БД, только математика.
func CalculateFields(input *PromoInputDTO, ctx CalculationContext) CalculatedFields {
	ppu := input.PlanPromoUnits
	cp := input.ContractPrice
	bu := input.BaselineUnits
	pir := input.PlanInvestmentsRub
	month := input.Month
	year := input.Year
	if year == 0 {
		year = time.Now().Year()
	}
	if month == 0 {
		month = int(time.Now().Month())
	}

	gm := ctx.GM
	olap := ctx.OlapPrice

	quarter := int(math.Ceil(float64(month) / 3))
	planPromoRub := ppu * cp
	planPromoUpliftUnits := ppu - bu
	planPromoUpliftRub := planPromoUpliftUnits * cp

	planPromoUpliftPctUnits := 0.0
	if ppu > 0 {
		planPromoUpliftPctUnits = (planPromoUpliftUnits / ppu) * 100
	}
	planPromoUpliftPctRub := 0.0
	if planPromoRub > 0 {
		planPromoUpliftPctRub = (planPromoUpliftRub / planPromoRub) * 100
	}
	planInvestmentsPct := 0.0
	if planPromoRub > 0 {
		planInvestmentsPct = (pir / planPromoRub) * 100
	}
	planROI := 0.0
	if pir > 0 {
		planROI = (planPromoUpliftRub/pir)*gm*100 - 100
	}
	baselineRub := bu * cp

	afu := input.ActualPromoSalesUnits
	afr := input.ActualPromoRub
	afi := input.ActualInvestments
	afupl := input.ActualPromoUpliftUnits
	afupr := input.ActualPromoUpliftRub
	afeu := input.ActualExternalEcomUnits
	acb := input.ActualCorrectedBaseline
	ph := float64(input.PromoPharmacies)
	if ph == 0 {
		ph = 1
	}

	netPromoUpliftRub := afupr * gm
	netPromoUpliftPct := 0.0
	if afr > 0 {
		netPromoUpliftPct = (netPromoUpliftRub / afr) * 100
	}
	actualInvestmentsPct := 0.0
	if afr > 0 {
		actualInvestmentsPct = (afi / afr) * 100
	}
	actualROI := 0.0
	if afi > 0 {
		actualROI = (afupr/afi)*gm*100 - 100
	}

	actualPromoRubWoEcom := afr - (afeu * cp)
	actualPromoUpliftUnitsWoEcom := afupl - afeu
	actualPromoUpliftRubWoEcom := actualPromoUpliftUnitsWoEcom * cp
	netPromoUpliftRubWoEcom := actualPromoUpliftRubWoEcom * gm
	netPromoUpliftPctWoEcom := 0.0
	if actualPromoRubWoEcom > 0 {
		netPromoUpliftPctWoEcom = (netPromoUpliftRubWoEcom / actualPromoRubWoEcom) * 100
	}
	actualInvestmentsPctWoEcom := 0.0
	if actualPromoRubWoEcom > 0 {
		actualInvestmentsPctWoEcom = (afi / actualPromoRubWoEcom) * 100
	}
	actualROIWoEcom := 0.0
	if afi > 0 {
		actualROIWoEcom = (actualPromoUpliftRubWoEcom/afi)*gm*100 - 100
	}

	planVsFactRub := 0.0
	if planPromoRub > 0 {
		planVsFactRub = (afr / planPromoRub) * 100
	}
	planVsFactInvestments := 0.0
	if pir > 0 {
		planVsFactInvestments = (afi / pir) * 100
	}

	turnoverPerPoint := acb / ph
	turnoverPerPointPromo := afu / ph
	planPromoCipOlap := ppu * olap
	factPromoCipOlap := afu * olap
	planPromoUpliftCipOlap := planPromoUpliftUnits * olap
	factPromoUpliftCipOlap := afupl * olap

	return CalculatedFields{
		Year:                         year,
		Month:                        month,
		Quarter:                      quarter,
		GM:                           gm,
		PlanPromoRub:                 planPromoRub,
		PlanPromoUpliftUnits:         planPromoUpliftUnits,
		PlanPromoUpliftRub:           planPromoUpliftRub,
		PlanPromoUpliftPctUnits:      planPromoUpliftPctUnits,
		PlanPromoUpliftPctRub:        planPromoUpliftPctRub,
		PlanInvestmentsPct:           planInvestmentsPct,
		PlanROI:                      planROI,
		BaselineRub:                  baselineRub,
		NetPromoUpliftRub:            netPromoUpliftRub,
		NetPromoUpliftPct:            netPromoUpliftPct,
		ActualInvestmentsPct:         actualInvestmentsPct,
		ActualROI:                    actualROI,
		ActualPromoRubWoEcom:         actualPromoRubWoEcom,
		ActualPromoUpliftUnitsWoEcom: actualPromoUpliftUnitsWoEcom,
		ActualPromoUpliftRubWoEcom:   actualPromoUpliftRubWoEcom,
		NetPromoUpliftRubWoEcom:      netPromoUpliftRubWoEcom,
		NetPromoUpliftPctWoEcom:      netPromoUpliftPctWoEcom,
		ActualInvestmentsPctWoEcom:   actualInvestmentsPctWoEcom,
		ActualROIWoEcom:              actualROIWoEcom,
		PlanVsFactRub:                planVsFactRub,
		PlanVsFactInvestments:        planVsFactInvestments,
		TurnoverPerPoint:             turnoverPerPoint,
		TurnoverPerPointPromo:        turnoverPerPointPromo,
		PlanPromoCipOlap:             planPromoCipOlap,
		FactPromoCipOlap:             factPromoCipOlap,
		PlanPromoUpliftCipOlap:       planPromoUpliftCipOlap,
		FactPromoUpliftCipOlap:       factPromoUpliftCipOlap,
		Date:                         PromoDate(year, month),
		OlapPrice:                    olap,
	}
}

// PromoDate — форматирует год и месяц в строку "YYYY-MM-01".
func PromoDate(year, month int) string {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// ToMap — преобразует CalculatedFields в map[string]interface{} для обратной
// совместимости с текущими repository.InsertPromo / UpdatePromo.
// TODO: удалить после полного перехода на типизированные структуры.
func (c CalculatedFields) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"year":                              c.Year,
		"month":                             c.Month,
		"quarter":                           c.Quarter,
		"gm":                                c.GM,
		"plan_promo_rub":                    c.PlanPromoRub,
		"plan_promo_uplift_units":           c.PlanPromoUpliftUnits,
		"plan_promo_uplift_rub":             c.PlanPromoUpliftRub,
		"plan_promo_uplift_pct_units":       c.PlanPromoUpliftPctUnits,
		"plan_promo_uplift_pct_rub":         c.PlanPromoUpliftPctRub,
		"plan_investments_pct":              c.PlanInvestmentsPct,
		"plan_roi":                          c.PlanROI,
		"baseline_rub":                      c.BaselineRub,
		"net_promo_uplift_rub":              c.NetPromoUpliftRub,
		"net_promo_uplift_pct":              c.NetPromoUpliftPct,
		"actual_investments_pct":            c.ActualInvestmentsPct,
		"actual_roi":                        c.ActualROI,
		"actual_promo_rub_wo_ecom":          c.ActualPromoRubWoEcom,
		"actual_promo_uplift_units_wo_ecom": c.ActualPromoUpliftUnitsWoEcom,
		"actual_promo_uplift_rub_wo_ecom":   c.ActualPromoUpliftRubWoEcom,
		"net_promo_uplift_rub_wo_ecom":      c.NetPromoUpliftRubWoEcom,
		"net_promo_uplift_pct_wo_ecom":      c.NetPromoUpliftPctWoEcom,
		"actual_investments_pct_wo_ecom":    c.ActualInvestmentsPctWoEcom,
		"actual_roi_wo_ecom":                c.ActualROIWoEcom,
		"plan_vs_fact_rub":                  c.PlanVsFactRub,
		"plan_vs_fact_investments":          c.PlanVsFactInvestments,
		"turnover_per_point":                c.TurnoverPerPoint,
		"turnover_per_point_promo":          c.TurnoverPerPointPromo,
		"plan_promo_cip_olap":               c.PlanPromoCipOlap,
		"fact_promo_cip_olap":               c.FactPromoCipOlap,
		"plan_promo_uplift_cip_olap":        c.PlanPromoUpliftCipOlap,
		"fact_promo_uplift_cip_olap":        c.FactPromoUpliftCipOlap,
		"date":                              c.Date,
		"olap_price":                        c.OlapPrice,
	}
}

// MapToDTO — преобразует map[string]interface{} в PromoInputDTO.
// Используется для обратной совместимости в SavePromo.
func MapToDTO(input map[string]interface{}) PromoInputDTO {
	return PromoInputDTO{
		NetworkName:             safeString(input, "network_name"),
		KAM:                     safeString(input, "kam"),
		Brand:                   safeString(input, "brand"),
		BrandAS:                 safeString(input, "brand_as"),
		SKU:                     safeString(input, "sku"),
		Year:                    safeInt(input, "year"),
		Month:                   safeInt(input, "month"),
		Quarter:                 safeInt(input, "quarter"),
		Mechanics:               safeString(input, "mechanics"),
		GTNOpex:                 safeString(input, "gtn_opex"),
		IDDirectum:              safeString(input, "id_directum"),
		DSNumber:                safeString(input, "ds_number"),
		DiscountAmount:          safeFloat(input, "discount_amount"),
		Conditions:              safeString(input, "conditions"),
		Comments:                safeString(input, "comments"),
		EcomSegment:             safeString(input, "ecom_segment"),
		TotalPharmacies:         safeInt(input, "total_pharmacies"),
		PromoPharmacies:         safeInt(input, "promo_pharmacies"),
		Status:                  safeString(input, "status"),
		Date:                    safeString(input, "date"),
		BaselineUnits:           safeFloat(input, "baseline_units"),
		BaselineRub:             safeFloat(input, "baseline_rub"),
		PlanPromoUnits:          safeFloat(input, "plan_promo_units"),
		PlanPromoRub:            safeFloat(input, "plan_promo_rub"),
		PlanInvestmentsRub:      safeFloat(input, "plan_investments_rub"),
		ContractPrice:           safeFloat(input, "contract_price"),
		GM:                      safeFloat(input, "gm"),
		KeyRegion:               safeString(input, "key_region"),
		Top20Segment:            safeString(input, "top20_segment"),
		OlapPrice:               safeFloat(input, "olap_price"),
		ActualPromoSalesUnits:   safeFloat(input, "actual_promo_sales_units"),
		ActualPromoRub:          safeFloat(input, "actual_promo_rub"),
		ActualInvestments:       safeFloat(input, "actual_investments"),
		ActualPromoUpliftUnits:  safeFloat(input, "actual_promo_uplift_units"),
		ActualPromoUpliftRub:    safeFloat(input, "actual_promo_uplift_rub"),
		ActualExternalEcomUnits: safeFloat(input, "actual_external_ecom_units"),
		ActualCorrectedBaseline: safeFloat(input, "actual_corrected_baseline"),
		Agreement1:              safeString(input, "agreement1"),
		Agreement2:              safeString(input, "agreement2"),
	}
}

// MergeCalculatedIntoMap — сливает CalculatedFields в существующий map.
func MergeCalculatedIntoMap(m map[string]interface{}, c CalculatedFields) {
	for k, v := range c.ToMap() {
		m[k] = v
	}
}

// ─── helpers (копия из promo_utils.go для независимости) ───

func safeFloat(input map[string]interface{}, key string) float64 {
	val, _ := strconv.ParseFloat(fmt.Sprint(input[key]), 64)
	return val
}

func safeInt(input map[string]interface{}, key string) int {
	val, _ := strconv.Atoi(fmt.Sprint(input[key]))
	return val
}

func safeString(input map[string]interface{}, key string) string {
	return fmt.Sprint(input[key])
}

// Выше нужны импорты "fmt" и "strconv", оставлены намеренно.
```

## File: frontend/src/hooks/usePromoCalculations.js
```javascript
import { useCallback } from 'react';

export function usePromoCalculations(form) {
  const recalcPlan = useCallback((updates) => {
    const f = { ...form, ...updates };
    const ppu = parseFloat(f.plan_promo_units) || 0;
    const cp = parseFloat(f.contract_price) || 0;
    const bu = parseFloat(f.baseline_units) || 0;
    const pir = parseFloat(f.plan_investments_rub) || 0;
    const gm = parseFloat(f.gm) || 1;
    const plan_rub = ppu * cp;
    const uplift_units = ppu - bu;
    const uplift_rub = uplift_units * cp;
    const roi = pir > 0 ? ((uplift_rub / pir) * gm * 100 - 100) : 0;
    const baseline_rub = bu * cp;
    return {
      plan_promo_rub: plan_rub.toFixed(2),
      plan_promo_uplift_units: uplift_units.toFixed(2),
      plan_promo_uplift_rub: uplift_rub.toFixed(2),
      plan_roi: roi.toFixed(1),
      baseline_rub: baseline_rub.toFixed(2),
    };
  }, [form]);

  const recalcActual = useCallback((updates) => {
    const f = { ...form, ...updates };
    const afu = parseFloat(f.actual_promo_sales_units) || 0;
    const cp = parseFloat(f.contract_price) || 0;
    const bu = parseFloat(f.baseline_units) || 0;
    const afi = parseFloat(f.actual_investments) || 0;
    const gm = parseFloat(f.gm) || 1;
    const afr = afu * cp;
    const afupl = afu - bu;
    const afupr = afupl * cp;
    const aroi = afi > 0 ? ((afupr / afi) * gm * 100 - 100) : 0;
    return {
      actual_promo_rub: afr.toFixed(2),
      actual_promo_uplift_units: afupl.toFixed(2),
      actual_promo_uplift_rub: afupr.toFixed(2),
      actual_roi: aroi.toFixed(1),
    };
  }, [form]);

  return { recalcPlan, recalcActual };
}
```

## File: frontend/src/types/promo.ts
```typescript
// Типы, соответствующие backend/models/types.go

export interface PromoRow {
  id: number;
  network_name: string | null;
  kam: string | null;
  id_directum: string | null;
  ds_number: string | null;
  year: number;
  month: number | null;
  quarter: number | null;
  sku: string | null;
  brand: string | null;
  brand_as: string | null;
  mechanics: string | null;
  discount_amount: number | null;
  gtn_opex: string | null;
  conditions: string | null;
  comments: string | null;
  baseline_units: number | null;
  baseline_rub: number | null;
  plan_promo_units: number | null;
  plan_promo_rub: number | null;
  plan_investments_rub: number | null;
  plan_promo_uplift_units: number | null;
  plan_promo_uplift_rub: number | null;
  plan_promo_uplift_pct_units: number | null;
  plan_promo_uplift_pct_rub: number | null;
  plan_investments_pct: number | null;
  plan_roi: number | null;
  contract_price: number | null;
  gm: number | null;
  total_pharmacies: number | null;
  promo_pharmacies: number | null;
  actual_promo_sales_units: number | null;
  actual_investments: number | null;
  status: string | null;
  actual_promo_rub: number | null;
  actual_promo_uplift_units: number | null;
  actual_promo_uplift_rub: number | null;
  actual_external_ecom_units: number | null;
  actual_corrected_baseline: number | null;
  actual_roi: number | null;
  plan_vs_fact_rub: number | null;
  plan_vs_fact_investments: number | null;
  channel: string | null;
  agreement1: string | null;
  agreement2: string | null;
  date: string | null;
  created_at: string | null;
  updated_at: string | null;
}

export interface ApprovalRow {
  id: number;
  network_name: string | null;
  brand_as: string | null;
  sku: string | null;
  mechanics: string | null;
  year: number;
  month: number | null;
  baseline_units: number | null;
  plan_promo_units: number | null;
  actual_promo_sales_units: number | null;
  plan_investments_rub: number | null;
  plan_roi: number | null;
  actual_roi: number | null;
  conditions: string | null;
  agreement1: string | null;
  agreement1_status: string | null;
  agreement1_comment: string | null;
  agreement2: string | null;
  agreement2_status: string | null;
  agreement2_comment: string | null;
  status: string | null;
  historical_count: number;
  avg_historical_roi: number | null;
}

export interface FilterOptions {
  kam: string[];
  brand: string[];
  sku: string[];
  network_name: string[];
  mechanics: string[];
  status: string[];
  channel: string[];
}

export interface PromoFormData {
  network_name?: string;
  kam?: string;
  brand?: string;
  brand_as?: string;
  sku?: string;
  year?: number;
  month?: number;
  mechanics?: string;
  gtn_opex?: string;
  id_directum?: string;
  ds_number?: string;
  discount_amount?: number;
  conditions?: string;
  comments?: string;
  ecom_segment?: string;
  total_pharmacies?: number;
  promo_pharmacies?: number;
  status?: string;
  baseline_units?: number;
  plan_promo_units?: number;
  plan_investments_rub?: number;
  contract_price?: number;
  gm?: number;
  key_region?: string;
  top20_segment?: string;
  actual_promo_sales_units?: number;
  actual_promo_rub?: number;
  actual_investments?: number;
  actual_promo_uplift_units?: number;
  actual_promo_uplift_rub?: number;
  actual_external_ecom_units?: number;
  actual_corrected_baseline?: number;
}
```

## File: frontend/src/App.css
```css
.counter {
  font-size: 16px;
  padding: 5px 10px;
  border-radius: 5px;
  color: var(--accent);
  background: var(--accent-bg);
  border: 2px solid transparent;
  transition: border-color 0.3s;
  margin-bottom: 24px;

  &:hover {
    border-color: var(--accent-border);
  }
  &:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
}

.hero {
  position: relative;

  .base,
  .framework,
  .vite {
    inset-inline: 0;
    margin: 0 auto;
  }

  .base {
    width: 170px;
    position: relative;
    z-index: 0;
  }

  .framework,
  .vite {
    position: absolute;
  }

  .framework {
    z-index: 1;
    top: 34px;
    height: 28px;
    transform: perspective(2000px) rotateZ(300deg) rotateX(44deg) rotateY(39deg)
      scale(1.4);
  }

  .vite {
    z-index: 0;
    top: 107px;
    height: 26px;
    width: auto;
    transform: perspective(2000px) rotateZ(300deg) rotateX(40deg) rotateY(39deg)
      scale(0.8);
  }
}

#center {
  display: flex;
  flex-direction: column;
  gap: 25px;
  place-content: center;
  place-items: center;
  flex-grow: 1;

  @media (max-width: 1024px) {
    padding: 32px 20px 24px;
    gap: 18px;
  }
}

#next-steps {
  display: flex;
  border-top: 1px solid var(--border);
  text-align: left;

  & > div {
    flex: 1 1 0;
    padding: 32px;
    @media (max-width: 1024px) {
      padding: 24px 20px;
    }
  }

  .icon {
    margin-bottom: 16px;
    width: 22px;
    height: 22px;
  }

  @media (max-width: 1024px) {
    flex-direction: column;
    text-align: center;
  }
}

#docs {
  border-right: 1px solid var(--border);

  @media (max-width: 1024px) {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}

#next-steps ul {
  list-style: none;
  padding: 0;
  display: flex;
  gap: 8px;
  margin: 32px 0 0;

  .logo {
    height: 18px;
  }

  a {
    color: var(--text-h);
    font-size: 16px;
    border-radius: 6px;
    background: var(--social-bg);
    display: flex;
    padding: 6px 12px;
    align-items: center;
    gap: 8px;
    text-decoration: none;
    transition: box-shadow 0.3s;

    &:hover {
      box-shadow: var(--shadow);
    }
    .button-icon {
      height: 18px;
      width: 18px;
    }
  }

  @media (max-width: 1024px) {
    margin-top: 20px;
    flex-wrap: wrap;
    justify-content: center;

    li {
      flex: 1 1 calc(50% - 8px);
    }

    a {
      width: 100%;
      justify-content: center;
      box-sizing: border-box;
    }
  }
}

#spacer {
  height: 88px;
  border-top: 1px solid var(--border);
  @media (max-width: 1024px) {
    height: 48px;
  }
}

.ticks {
  position: relative;
  width: 100%;

  &::before,
  &::after {
    content: '';
    position: absolute;
    top: -4.5px;
    border: 5px solid transparent;
  }

  &::before {
    left: 0;
    border-left-color: var(--border);
  }
  &::after {
    right: 0;
    border-right-color: var(--border);
  }
}
```

## File: frontend/src/index.css
```css
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

/* Сброс стилей и базовые настройки */
* {
  box-sizing: border-box;
}

html, body {
  margin: 0;
  padding: 0;
  width: 100%;
  height: 100%;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background-color: #f8fafc; /* Мягкий светлый фон */
  color: #0f172a; /* Глубокий сланцевый цвет текста вместо чистого черного */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

#root {
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  text-align: left;
}
```

## File: frontend/.gitignore
```
# Logs
logs
*.log
npm-debug.log*
yarn-debug.log*
yarn-error.log*
pnpm-debug.log*
lerna-debug.log*

node_modules
dist
dist-ssr
*.local

# Editor directories and files
.vscode/*
!.vscode/extensions.json
.idea
.DS_Store
*.suo
*.ntvs*
*.njsproj
*.sln
*.sw?
```

## File: frontend/eslint.config.js
```javascript
import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{js,jsx}'],
    extends: [
      js.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
  },
])
```

## File: frontend/index.html
```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <!-- Подключаем Inter -->
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet" />
    <title>frontend</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.jsx"></script>
  </body>
</html>
```

## File: frontend/vite.config.js
```javascript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
})
```

## File: docker-compose.yml
```yaml
version: '3.9'

services:
  mssql_db:
    image: mcr.microsoft.com/mssql/server:2022-latest
    container_name: my_local_mssql
    environment:
      ACCEPT_EULA: Y
      SA_PASSWORD: $#Pfchfytw_0378 # Очень важный пароль. Обязательно сложный!
    ports:
      - "1433:1433" # MSSQL стандартный порт
    volumes:
      - mssql_data_volume:/var/opt/mssql
    restart: unless-stopped

volumes:
  mssql_data_volume:
```

## File: backend/config/auth.go
```go
package config

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var JWTSecret = []byte(getEnvOrDefault("JWT_SECRET", "change-me-in-production-32bytes!!"))

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

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
```

## File: backend/handlers/promo_utils.go
```go
package handlers

import (
	"fmt"
	"strconv"
)

// safeFloat, safeInt, safeString — хелперы для безопасного извлечения
// типизированных значений из map[string]interface{}. Используются
// в promo.go для логирования и парсинга входных данных.

func safeFloat(input map[string]interface{}, key string) float64 {
	val, _ := strconv.ParseFloat(fmt.Sprint(input[key]), 64)
	return val
}

func safeInt(input map[string]interface{}, key string) int {
	val, _ := strconv.Atoi(fmt.Sprint(input[key]))
	return val
}

func safeString(input map[string]interface{}, key string) string {
	return fmt.Sprint(input[key])
}
```

## File: backend/handlers/sales.go
```go
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"backend/config"
	"backend/models"

	"github.com/gin-gonic/gin"
)

func GetFilterOptions(c *gin.Context) {
	getDistinct := func(query string) []string {
		rows, e := config.DB.Query(query)
		if e != nil {
			return []string{}
		}
		defer rows.Close()
		var vals []string
		for rows.Next() {
			var v sql.NullString
			if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
				vals = append(vals, v.String)
			}
		}
		return vals
	}

	result := gin.H{
		"brandName":   getDistinct("SELECT DISTINCT brandName FROM dbo.tbl_EcomSalesNormalized WHERE brandName IS NOT NULL ORDER BY brandName"),
		"networkName": getDistinct("SELECT DISTINCT networkName FROM dbo.tbl_EcomSalesNormalized WHERE networkName IS NOT NULL ORDER BY networkName"),
		"un_rub":      getDistinct("SELECT DISTINCT un_rub FROM dbo.tbl_EcomSalesNormalized WHERE un_rub IS NOT NULL ORDER BY un_rub"),
		"segment":     getDistinct("SELECT DISTINCT segment FROM dbo.tbl_EcomSalesNormalized WHERE segment IS NOT NULL ORDER BY segment"),
		"channel":     getDistinct("SELECT DISTINCT channel FROM dbo.tbl_EcomSalesNormalized WHERE channel IS NOT NULL ORDER BY channel"),
	}

	mappingQuery := `SELECT segment, channel FROM dbo.tbl_ChannelSegmentMapping WHERE segment IS NOT NULL AND channel IS NOT NULL GROUP BY segment, channel ORDER BY segment, channel`
	rows, e := config.DB.Query(mappingQuery)
	if e != nil {
		result["segmentChannelMap"] = make(map[string][]string)
		result["channelSegmentMap"] = make(map[string][]string)
	} else {
		defer rows.Close()
		segChanMap := make(map[string][]string)
		chanSegMap := make(map[string][]string)
		for rows.Next() {
			var seg, chanVal sql.NullString
			if err := rows.Scan(&seg, &chanVal); err == nil {
				if seg.Valid && chanVal.Valid && seg.String != "" && chanVal.String != "" {
					segChanMap[seg.String] = append(segChanMap[seg.String], chanVal.String)
					chanSegMap[chanVal.String] = append(chanSegMap[chanVal.String], seg.String)
				}
			}
		}
		result["segmentChannelMap"] = segChanMap
		result["channelSegmentMap"] = chanSegMap
	}
	c.JSON(http.StatusOK, result)
}

func GetData(c *gin.Context) {
	yearFromStr := c.Query("yearFrom")
	yearToStr := c.Query("yearTo")
	months := c.QueryArray("months")
	brandNames := c.QueryArray("brandName")
	networkNames := c.QueryArray("networkName")
	unRubs := c.QueryArray("un_rub")
	segments := c.QueryArray("segment")
	channels := c.QueryArray("channel")

	baseWhere := " WHERE n.metric_value != 0 AND n.metric_value IS NOT NULL"
	baseSelect := "SELECT n.id, n.[year], n.[month], n.brandName, n.productName, n.networkName, n.metric_type, n.metric_value, n.un_rub, n.segment, n.channel, n.updated_at FROM dbo.tbl_EcomSalesNormalized n"
	args := []interface{}{}

	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			baseWhere += " AND n.[year] >= ?"
			args = append(args, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			baseWhere += " AND n.[year] <= ?"
			args = append(args, y)
		}
	}
	if len(months) > 0 {
		placeholders := make([]string, 0, len(months))
		for _, m := range months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			baseWhere += " AND n.[month] IN (" + strings.Join(placeholders, ",") + ")"
		}
	}
	if len(brandNames) > 0 {
		conds := make([]string, 0, len(brandNames))
		for _, v := range brandNames {
			if v != "" {
				conds = append(conds, "n.brandName LIKE ?")
				args = append(args, "%"+v+"%")
			}
		}
		if len(conds) > 0 {
			baseWhere += " AND (" + strings.Join(conds, " OR ") + ")"
		}
	}
	if len(networkNames) > 0 {
		conds := make([]string, 0, len(networkNames))
		for _, v := range networkNames {
			if v != "" {
				conds = append(conds, "n.networkName LIKE ?")
				args = append(args, "%"+v+"%")
			}
		}
		if len(conds) > 0 {
			baseWhere += " AND (" + strings.Join(conds, " OR ") + ")"
		}
	}

	appendFilter := func(col string, values []string) {
		if len(values) > 0 {
			placeholders := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					placeholders = append(placeholders, "?")
					args = append(args, v)
				}
			}
			if len(placeholders) > 0 {
				baseWhere += " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}
	appendFilter("n.un_rub", unRubs)
	appendFilter("n.segment", segments)
	appendFilter("n.channel", channels)

	all := c.Query("all")

	if all == "true" {
		// Экспорт — возвращаем всё
		query := baseSelect + baseWhere + " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type"
		rows, err := config.DB.Query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
			return
		}
		defer rows.Close()

		var results []models.Row
		for rows.Next() {
			var r models.Row
			if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.BrandName, &r.ProductName, &r.NetworkName, &r.MetricType, &r.MetricValue, &r.UnRub, &r.Segment, &r.Channel, &r.UpdatedAt); err != nil {
				continue
			}
			results = append(results, r)
		}
		if results == nil {
			results = []models.Row{}
		}
		c.JSON(http.StatusOK, gin.H{"data": results})
		return
	}

	// Пагинация
	countQuery := "SELECT COUNT(*) FROM dbo.tbl_EcomSalesNormalized n" + baseWhere
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)

	var totalRows int
	if err := config.DB.QueryRow(countQuery, countArgs...).Scan(&totalRows); err != nil {
		totalRows = 0
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	offset := page * pageSize

	query := baseSelect + baseWhere + " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
	args = append(args, offset, pageSize)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	defer rows.Close()

	var results []models.Row
	for rows.Next() {
		var r models.Row
		if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.BrandName, &r.ProductName, &r.NetworkName, &r.MetricType, &r.MetricValue, &r.UnRub, &r.Segment, &r.Channel, &r.UpdatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil {
		results = []models.Row{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results, "totalRows": totalRows})
}

func GetDrilldown(c *gin.Context) {
	brandName := c.Query("brandName")
	networkName := c.Query("networkName")
	if brandName == "" || networkName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "brandName и networkName обязательны"})
		return
	}

	yearFromStr := c.Query("yearFrom")
	yearToStr := c.Query("yearTo")
	months := c.QueryArray("months")
	segments := c.QueryArray("segment")
	channels := c.QueryArray("channel")

	query := `SELECT n.[year], n.[month], n.metric_type, SUM(n.metric_value) as total_value, n.un_rub, n.segment, n.channel FROM dbo.tbl_EcomSalesNormalized n WHERE n.brandName = ? AND n.networkName = ? AND n.metric_value != 0 AND n.metric_value IS NOT NULL`
	args := []interface{}{brandName, networkName}

	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			query += " AND n.[year] >= ?"
			args = append(args, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			query += " AND n.[year] <= ?"
			args = append(args, y)
		}
	}
	if len(months) > 0 {
		placeholders := make([]string, 0, len(months))
		for _, m := range months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			query += " AND n.[month] IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	appendFilter := func(col string, values []string) {
		if len(values) > 0 {
			placeholders := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					placeholders = append(placeholders, "?")
					args = append(args, v)
				}
			}
			if len(placeholders) > 0 {
				query += " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}
	appendFilter("n.segment", segments)
	appendFilter("n.channel", channels)

	query += " GROUP BY n.[year], n.[month], n.metric_type, n.un_rub, n.segment, n.channel ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
		return
	}
	defer rows.Close()

	var results []models.DrilldownRow
	for rows.Next() {
		var r models.DrilldownRow
		if err := rows.Scan(&r.Year, &r.Month, &r.MetricType, &r.TotalValue, &r.UnRub, &r.Segment, &r.Channel); err != nil {
			continue
		}
		results = append(results, r)
	}
	c.JSON(http.StatusOK, gin.H{"brandName": brandName, "networkName": networkName, "data": results})
}
```

## File: backend/repository/promo_repo.go
```go
package repository

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"
)

// ─── Filters ────────────────────────────────────────────────────────────────

type PromoFilterParams struct {
	YearFromStr, YearToStr                            string
	Months                                            []string
	Kams, Brands, SKUs, Networks, Mechanics, Statuses []string
}

func BuildBaseWhere(params PromoFilterParams) (string, []interface{}) {
	where := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	if params.YearFromStr != "" {
		if y, _ := strconv.Atoi(params.YearFromStr); true {
			where += " AND year >= ?"
			args = append(args, y)
		}
	}
	if params.YearToStr != "" {
		if y, _ := strconv.Atoi(params.YearToStr); true {
			where += " AND year <= ?"
			args = append(args, y)
		}
	}
	if len(params.Months) > 0 {
		placeholders := make([]string, 0, len(params.Months))
		for _, m := range params.Months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			where += " AND month IN (" + strings.Join(placeholders, ",") + ")"
		}
	}
	return where, args
}

func AddFilter(col string, values []string) (string, []interface{}) {
	if len(values) == 0 {
		return "", nil
	}
	placeholders := make([]string, 0, len(values))
	newArgs := []interface{}{}
	for _, v := range values {
		if v != "" {
			placeholders = append(placeholders, "?")
			newArgs = append(newArgs, v)
		}
	}
	if len(placeholders) > 0 {
		return " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")", newArgs
	}
	return "", nil
}

func ExecDistinct(query string, args []interface{}) []string {
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
			vals = append(vals, v.String)
		}
	}
	return vals
}

// GetFilterValues возвращает список уникальных значений для конкретной колонки
func GetFilterValues(col string, baseWhere string, baseArgs []interface{}, excludeCol string, filters map[string][]string) []string {
	where := baseWhere
	args := make([]interface{}, len(baseArgs))
	copy(args, baseArgs)
	for filterCol, values := range filters {
		if filterCol != excludeCol {
			cond, newArgs := AddFilter(filterCol, values)
			if cond != "" {
				where += cond
				args = append(args, newArgs...)
			}
		}
	}
	query := "SELECT DISTINCT " + col + " FROM dbo.tbl_PromoActivities " + where + " AND " + col + " IS NOT NULL ORDER BY " + col
	return ExecDistinct(query, args)
}

// GetChannelFilterValues — специальный запрос для канала через JOIN
func GetChannelFilterValues(baseWhere string, baseArgs []interface{}, filters map[string][]string) []string {
	where := baseWhere
	args := make([]interface{}, len(baseArgs))
	copy(args, baseArgs)
	for filterCol, values := range filters {
		cond, newArgs := AddFilter("p."+filterCol, values)
		if cond != "" {
			where += cond
			args = append(args, newArgs...)
		}
	}
	query := "SELECT DISTINCT m.channel FROM dbo.tbl_PromoActivities p LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON p.mechanics = m.mechanics " + where + " AND m.channel IS NOT NULL ORDER BY m.channel"
	return ExecDistinct(query, args)
}

// ─── Promo Data ─────────────────────────────────────────────────────────────

func GetPromoRows(params PromoFilterParams, channels []string, page, pageSize int, getAll bool) ([]models.PromoRow, error) {
	query := `SELECT p.id, p.network_name, p.kam, p.id_directum, p.ds_number, p.year, p.month, p.quarter, p.sku, p.brand, p.brand_as, p.mechanics, p.discount_amount, p.gtn_opex, p.conditions, p.comments, p.total_pharmacies, p.promo_pharmacies, p.baseline_units, p.baseline_rub, p.plan_promo_units, p.plan_promo_rub, p.plan_investments_rub, p.plan_promo_uplift_units, p.plan_promo_uplift_rub, p.plan_promo_uplift_pct_units, p.plan_promo_uplift_pct_rub, p.plan_investments_pct, p.plan_roi, p.contract_price, p.gm, p.actual_promo_sales_units, p.actual_investments, p.status, p.actual_promo_rub, p.actual_promo_uplift_units, p.actual_promo_uplift_rub, p.actual_external_ecom_units, p.actual_corrected_baseline, p.actual_roi, p.plan_vs_fact_rub, p.plan_vs_fact_investments, p.agreement1, p.agreement2, p.date, p.created_at, p.updated_at, m.channel FROM dbo.tbl_PromoActivities p LEFT JOIN dbo.tbl_MechanicsChannelMapping m ON p.mechanics = m.mechanics WHERE p.deleted_at IS NULL`
	args := []interface{}{}

	appendFilter := func(col string, values []string) {
		if len(values) > 0 {
			placeholders := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					placeholders = append(placeholders, "?")
					args = append(args, v)
				}
			}
			if len(placeholders) > 0 {
				query += " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}

	if params.YearFromStr != "" {
		if y, _ := strconv.Atoi(params.YearFromStr); true {
			query += " AND p.year >= ?"
			args = append(args, y)
		}
	}
	if params.YearToStr != "" {
		if y, _ := strconv.Atoi(params.YearToStr); true {
			query += " AND p.year <= ?"
			args = append(args, y)
		}
	}
	if len(params.Months) > 0 {
		placeholders := make([]string, 0, len(params.Months))
		for _, m := range params.Months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			query += " AND p.month IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	appendFilter("p.kam", params.Kams)
	appendFilter("p.brand_as", params.Brands)
	appendFilter("p.sku", params.SKUs)
	appendFilter("p.network_name", params.Networks)
	appendFilter("p.mechanics", params.Mechanics)
	appendFilter("p.status", params.Statuses)
	appendFilter("m.channel", channels)

	if getAll {
		query += " ORDER BY p.year DESC, p.month DESC"
	} else {
		if pageSize <= 0 {
			pageSize = 100
		}
		if pageSize > 1000 {
			pageSize = 1000
		}
		offset := page * pageSize
		query += " ORDER BY p.year DESC, p.month DESC OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
		args = append(args, offset, pageSize)
	}

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.PromoRow
	for rows.Next() {
		var r models.PromoRow
		if err := rows.Scan(&r.ID, &r.NetworkName, &r.KAM, &r.IDDirectum, &r.DSNumber, &r.Year, &r.Month, &r.Quarter, &r.SKU, &r.Brand, &r.BrandAS, &r.Mechanics, &r.DiscountAmount, &r.GTNOpex, &r.Conditions, &r.Comments, &r.TotalPharmacies, &r.PromoPharmacies, &r.BaselineUnits, &r.BaselineRub, &r.PlanPromoUnits, &r.PlanPromoRub, &r.PlanInvestmentsRub, &r.PlanPromoUpliftUnits, &r.PlanPromoUpliftRub, &r.PlanPromoUpliftPctUnits, &r.PlanPromoUpliftPctRub, &r.PlanInvestmentsPct, &r.PlanROI, &r.ContractPrice, &r.GM, &r.ActualPromoSalesUnits, &r.ActualInvestments, &r.Status, &r.ActualPromoRub, &r.ActualPromoUpliftUnits, &r.ActualPromoUpliftRub, &r.ActualExternalEcomUnits, &r.ActualCorrectedBaseline, &r.ActualROI, &r.PlanVsFactRub, &r.PlanVsFactInvestments, &r.Agreement1, &r.Agreement2, &r.Date, &r.CreatedAt, &r.UpdatedAt, &r.PromoChannel); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil {
		results = []models.PromoRow{}
	}
	return results, nil
}

// ─── SKU / Lookups ──────────────────────────────────────────────────────────

func GetSKUsByBrand(brand string) ([]string, error) {
	rows, err := config.DB.Query("SELECT DISTINCT sku FROM dbo.tbl_PromoActivities WHERE brand_as = ? AND sku IS NOT NULL AND deleted_at IS NULL ORDER BY sku", brand)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skus []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			skus = append(skus, s)
		}
	}
	return skus, nil
}

func GetLastContractPrice(sku string) (*float64, error) {
	var price sql.NullFloat64
	err := config.DB.QueryRow("SELECT TOP 1 contract_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND contract_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC", sku).Scan(&price)
	if err != nil {
		return nil, err
	}
	if price.Valid {
		return &price.Float64, nil
	}
	return nil, nil
}

func GetKAMsByNetwork(network string) ([]string, error) {
	rows, err := config.DB.Query("SELECT DISTINCT kam FROM dbo.tbl_KAMNetworkMapping WHERE network_name = ? AND kam IS NOT NULL ORDER BY kam", network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var kams []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			kams = append(kams, k)
		}
	}
	return kams, nil
}

func GetLastNetworkData(network string) (*int64, error) {
	var totalPharmacies sql.NullInt64
	err := config.DB.QueryRow("SELECT TOP 1 total_pharmacies FROM dbo.tbl_PromoActivities WHERE network_name = ? AND total_pharmacies IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC", network).Scan(&totalPharmacies)
	if err != nil {
		return nil, err
	}
	if totalPharmacies.Valid {
		return &totalPharmacies.Int64, nil
	}
	return nil, nil
}

func GetNetworkGeoMapping(network string) (*models.NetworkGeo, error) {
	var kam, networkType, top20Segment, keyRegion sql.NullString
	err := config.DB.QueryRow(
		"SELECT kam, network_type, top20_segment, key_region FROM dbo.tbl_NetworkGeoMapping WHERE network_name = ?",
		network,
	).Scan(&kam, &networkType, &top20Segment, &keyRegion)
	if err != nil {
		return nil, err
	}
	return &models.NetworkGeo{
		KAM:          kam.String,
		NetworkType:  networkType.String,
		Top20Segment: top20Segment.String,
		KeyRegion:    keyRegion.String,
	}, nil
}

func GetSKUInfo(sku string) (brand, brandAs string, found bool) {
	var b, ba sql.NullString
	err := config.DB.QueryRow("SELECT brand, brand_as FROM dbo.tbl_SKUMapping WHERE sku = ?", sku).Scan(&b, &ba)
	if err != nil {
		return "", "", false
	}
	return b.String, ba.String, true
}

func GetLastSKUData(sku string) (*models.LastSKUData, error) {
	var contractPrice, gm, olapPrice sql.NullFloat64
	var totalPharmacies sql.NullInt64
	var keyRegion, top20Segment sql.NullString

	err := config.DB.QueryRow(
		"SELECT TOP 1 contract_price, gm, total_pharmacies, key_region, top20_segment, olap_price FROM dbo.tbl_PromoActivities WHERE sku = ? AND contract_price IS NOT NULL AND deleted_at IS NULL ORDER BY year DESC, month DESC",
		sku,
	).Scan(&contractPrice, &gm, &totalPharmacies, &keyRegion, &top20Segment, &olapPrice)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return &models.LastSKUData{
		ContractPrice:   contractPrice.Float64,
		GM:              gm.Float64,
		TotalPharmacies: totalPharmacies.Int64,
		KeyRegion:       keyRegion.String,
		Top20Segment:    top20Segment.String,
		OlapPrice:       olapPrice.Float64,
	}, nil
}

// ─── History ────────────────────────────────────────────────────────────────

func GetPromoHistory(sku, network, mechanics, yearFrom, yearTo string) ([]models.HistoryRow, error) {
	query := "SELECT TOP 10 id, network_name, year, month, mechanics, sku, baseline_units, plan_promo_units, actual_promo_sales_units, plan_promo_uplift_units, actual_promo_uplift_units, plan_roi, actual_roi FROM dbo.tbl_PromoActivities WHERE deleted_at IS NULL"
	args := []interface{}{}
	if sku != "" {
		query += " AND sku = ?"
		args = append(args, sku)
	}
	if network != "" {
		query += " AND network_name = ?"
		args = append(args, network)
	}
	if mechanics != "" {
		query += " AND mechanics = ?"
		args = append(args, mechanics)
	}
	if yearFrom != "" {
		if y, _ := strconv.Atoi(yearFrom); true {
			query += " AND year >= ?"
			args = append(args, y)
		}
	}
	if yearTo != "" {
		if y, _ := strconv.Atoi(yearTo); true {
			query += " AND year <= ?"
			args = append(args, y)
		}
	}
	query += " ORDER BY year DESC, month DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.HistoryRow
	for rows.Next() {
		var r models.HistoryRow
		if err := rows.Scan(&r.ID, &r.NetworkName, &r.Year, &r.Month, &r.Mechanics, &r.SKU, &r.BaselineUnits, &r.PlanPromoUnits, &r.ActualPromoSalesUnits, &r.PlanPromoUpliftUnits, &r.ActualPromoUpliftUnits, &r.PlanROI, &r.ActualROI); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// ─── Save / Delete ──────────────────────────────────────────────────────────

func FetchExistingRow(id int) (map[string]interface{}, error) {
	allPromoFields := []string{
		"network_name", "kam", "brand", "brand_as", "sku",
		"year", "month", "quarter", "mechanics", "gtn_opex",
		"baseline_units", "baseline_rub",
		"plan_promo_units", "plan_promo_rub", "plan_investments_rub",
		"plan_promo_uplift_units", "plan_promo_uplift_rub",
		"plan_promo_uplift_pct_units", "plan_promo_uplift_pct_rub",
		"plan_investments_pct", "plan_roi",
		"contract_price", "gm",
		"id_directum", "ds_number", "discount_amount",
		"conditions", "comments", "ecom_segment",
		"total_pharmacies", "promo_pharmacies",
		"status", "date",
		"key_region", "top20_segment", "olap_price",
		"plan_promo_cip_olap", "fact_promo_cip_olap",
		"plan_promo_uplift_cip_olap", "fact_promo_uplift_cip_olap",
		"actual_promo_sales_units", "actual_investments",
		"actual_promo_rub", "actual_promo_uplift_units", "actual_promo_uplift_rub",
		"actual_external_ecom_units", "actual_corrected_baseline",
		"agreement1", "agreement2",
		"net_promo_uplift_rub", "net_promo_uplift_pct",
		"actual_investments_pct", "actual_roi",
		"actual_promo_rub_wo_ecom", "actual_promo_uplift_units_wo_ecom",
		"actual_promo_uplift_rub_wo_ecom",
		"net_promo_uplift_rub_wo_ecom", "net_promo_uplift_pct_wo_ecom",
		"actual_investments_pct_wo_ecom", "actual_roi_wo_ecom",
		"plan_vs_fact_rub", "plan_vs_fact_investments",
		"turnover_per_point", "turnover_per_point_promo",
		"updated_at",
	}

	row := config.DB.QueryRow(
		"SELECT "+strings.Join(allPromoFields, ", ")+" FROM dbo.tbl_PromoActivities WHERE id = ? AND deleted_at IS NULL",
		id,
	)

	existing := make(map[string]interface{})
	dest := make([]interface{}, len(allPromoFields))
	for i := range allPromoFields {
		var v sql.NullString
		dest[i] = &v
	}

	if err := row.Scan(dest...); err != nil {
		return nil, err
	}

	for i, field := range allPromoFields {
		ns := dest[i].(*sql.NullString)
		if ns.Valid {
			if f, err := strconv.ParseFloat(ns.String, 64); err == nil {
				existing[field] = f
			} else if i, err := strconv.Atoi(ns.String); err == nil {
				existing[field] = i
			} else {
				existing[field] = ns.String
			}
		}
	}
	return existing, nil
}

// UpdatePromo возвращает rowsAffected (0 = конфликт версий)
func UpdatePromo(id int, existing map[string]interface{}, updatedAt string) (int64, error) {
	allPromoFields := []string{
		"network_name", "kam", "brand", "brand_as", "sku",
		"year", "month", "quarter", "mechanics", "gtn_opex",
		"baseline_units", "baseline_rub",
		"plan_promo_units", "plan_promo_rub", "plan_investments_rub",
		"plan_promo_uplift_units", "plan_promo_uplift_rub",
		"plan_promo_uplift_pct_units", "plan_promo_uplift_pct_rub",
		"plan_investments_pct", "plan_roi",
		"contract_price", "gm",
		"id_directum", "ds_number", "discount_amount",
		"conditions", "comments", "ecom_segment",
		"total_pharmacies", "promo_pharmacies",
		"status", "date",
		"key_region", "top20_segment", "olap_price",
		"plan_promo_cip_olap", "fact_promo_cip_olap",
		"plan_promo_uplift_cip_olap", "fact_promo_uplift_cip_olap",
		"actual_promo_sales_units", "actual_investments",
		"actual_promo_rub", "actual_promo_uplift_units", "actual_promo_uplift_rub",
		"actual_external_ecom_units", "actual_corrected_baseline",
		"agreement1", "agreement2",
		"net_promo_uplift_rub", "net_promo_uplift_pct",
		"actual_investments_pct", "actual_roi",
		"actual_promo_rub_wo_ecom", "actual_promo_uplift_units_wo_ecom",
		"actual_promo_uplift_rub_wo_ecom",
		"net_promo_uplift_rub_wo_ecom", "net_promo_uplift_pct_wo_ecom",
		"actual_investments_pct_wo_ecom", "actual_roi_wo_ecom",
		"plan_vs_fact_rub", "plan_vs_fact_investments",
		"turnover_per_point", "turnover_per_point_promo",
		"updated_at",
	}

	setClauses := []string{}
	values := []interface{}{}
	for _, field := range allPromoFields {
		if field == "updated_at" {
			continue
		}
		if val, ok := existing[field]; ok {
			setClauses = append(setClauses, field+" = ?")
			values = append(values, val)
		}
	}
	setClauses = append(setClauses, "updated_at = GETDATE()")
	values = append(values, id)

	query := "UPDATE dbo.tbl_PromoActivities SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	if updatedAt != "" {
		query += " AND updated_at = ?"
		values = append(values, updatedAt)
	}

	result, err := config.DB.Exec(query, values...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func InsertPromo(input map[string]interface{}) (int64, error) {
	allPromoFields := []string{
		"network_name", "kam", "brand", "brand_as", "sku",
		"year", "month", "quarter", "mechanics", "gtn_opex",
		"baseline_units", "baseline_rub",
		"plan_promo_units", "plan_promo_rub", "plan_investments_rub",
		"plan_promo_uplift_units", "plan_promo_uplift_rub",
		"plan_promo_uplift_pct_units", "plan_promo_uplift_pct_rub",
		"plan_investments_pct", "plan_roi",
		"contract_price", "gm",
		"id_directum", "ds_number", "discount_amount",
		"conditions", "comments", "ecom_segment",
		"total_pharmacies", "promo_pharmacies",
		"status", "date",
		"key_region", "top20_segment", "olap_price",
		"plan_promo_cip_olap", "fact_promo_cip_olap",
		"plan_promo_uplift_cip_olap", "fact_promo_uplift_cip_olap",
		"actual_promo_sales_units", "actual_investments",
		"actual_promo_rub", "actual_promo_uplift_units", "actual_promo_uplift_rub",
		"actual_external_ecom_units", "actual_corrected_baseline",
		"agreement1", "agreement2",
		"net_promo_uplift_rub", "net_promo_uplift_pct",
		"actual_investments_pct", "actual_roi",
		"actual_promo_rub_wo_ecom", "actual_promo_uplift_units_wo_ecom",
		"actual_promo_uplift_rub_wo_ecom",
		"net_promo_uplift_rub_wo_ecom", "net_promo_uplift_pct_wo_ecom",
		"actual_investments_pct_wo_ecom", "actual_roi_wo_ecom",
		"plan_vs_fact_rub", "plan_vs_fact_investments",
		"turnover_per_point", "turnover_per_point_promo",
		"updated_at",
	}

	placeholders := make([]string, len(allPromoFields))
	values := make([]interface{}, len(allPromoFields))
	for i, f := range allPromoFields {
		placeholders[i] = "?"
		if val, ok := input[f]; ok {
			values[i] = val
		} else {
			values[i] = nil
		}
	}

	var newID int64
	err := config.DB.QueryRow(
		fmt.Sprintf("INSERT INTO dbo.tbl_PromoActivities (%s) OUTPUT INSERTED.id VALUES (%s)",
			strings.Join(allPromoFields, ", "),
			strings.Join(placeholders, ", ")),
		values...,
	).Scan(&newID)
	return newID, err
}

func SoftDeletePromo(id int) (int64, error) {
	result, err := config.DB.Exec("UPDATE dbo.tbl_PromoActivities SET deleted_at = GETDATE(), updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL", id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ─── Approvals ──────────────────────────────────────────────────────────────

type ApprovalParams struct {
	Role              string
	KAM               string
	ApprovalStatus    string
	YearStr, MonthStr string
}

func GetApprovals(params ApprovalParams) ([]models.ApprovalRow, error) {
	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	query := `
		SELECT TOP 500
			p.id, p.network_name, p.brand_as, p.sku, p.mechanics, p.year, p.month,
			p.baseline_units, p.plan_promo_units, p.actual_promo_sales_units,
			p.plan_investments_rub, p.plan_roi, p.actual_roi,
			p.conditions, p.agreement1, p.agreement2, p.status,
			p.agreement1_status, p.agreement1_comment,
			p.agreement2_status, p.agreement2_comment,
			0 as historical_count,
			CAST(NULL AS FLOAT) as avg_historical_roi
		FROM dbo.tbl_PromoActivities p
		WHERE p.deleted_at IS NULL
	`

	args := []interface{}{}

	if params.YearStr != "" {
		y, _ := strconv.Atoi(params.YearStr)
		query += " AND p.year = ?"
		args = append(args, y)
	} else if params.MonthStr != "" {
		query += " AND p.year >= ?"
		args = append(args, currentYear)
	} else {
		query += " AND (p.year > ? OR (p.year = ? AND p.month >= ?))"
		args = append(args, currentYear, currentYear, currentMonth)
	}

	if params.MonthStr != "" {
		m, _ := strconv.Atoi(params.MonthStr)
		query += " AND p.month = ?"
		args = append(args, m)
	}

	if params.KAM != "" {
		query += " AND p.kam = ?"
		args = append(args, params.KAM)
	}

	// Используем agreement1_status/agreement2_status вместо CHARINDEX-парсинга
	statusField := "p.agreement1_status"
	if params.Role == "agreement2" {
		statusField = "p.agreement2_status"
	}

	switch params.ApprovalStatus {
	case "pending":
		query += fmt.Sprintf(" AND %s IS NULL", statusField)
	case "commented":
		query += fmt.Sprintf(" AND %s = 'commented'", statusField)
	case "approved":
		query += fmt.Sprintf(" AND %s = 'approved'", statusField)
	case "rejected":
		query += fmt.Sprintf(" AND %s = 'rejected'", statusField)
	}

	query += " ORDER BY p.year DESC, p.month DESC, p.network_name"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ApprovalRow
	for rows.Next() {
		var r models.ApprovalRow
		if err := rows.Scan(
			&r.ID, &r.NetworkName, &r.BrandAS, &r.SKU, &r.Mechanics, &r.Year, &r.Month,
			&r.BaselineUnits, &r.PlanPromoUnits, &r.ActualPromoSalesUnits,
			&r.PlanInvestmentsRub, &r.PlanROI, &r.ActualROI,
			&r.Conditions, &r.Agreement1, &r.Agreement2, &r.Status,
			&r.Agreement1Status, &r.Agreement1Comment,
			&r.Agreement2Status, &r.Agreement2Comment,
			&r.HistoricalCount, &r.AvgHistoricalROI,
		); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil {
		results = []models.ApprovalRow{}
	}
	return results, nil
}

// ApprovePromoWithStatus — обновляет agreement1/agreement2 и новые поля _status/_comment
func ApprovePromoWithStatus(agreementNum int, id int, status string, comment string, legacyValue string) error {
	statusField := fmt.Sprintf("agreement%d_status", agreementNum)
	commentField := fmt.Sprintf("agreement%d_comment", agreementNum)
	agreementField := fmt.Sprintf("agreement%d", agreementNum)

	query := fmt.Sprintf(
		"UPDATE dbo.tbl_PromoActivities SET %s = ?, %s = ?, %s = ?, updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL",
		agreementField, statusField, commentField,
	)
	_, err := config.DB.Exec(query, legacyValue, status, comment, id)
	return err
}

// Deprecated: используйте ApprovePromoWithStatus
func ApprovePromo(field string, id int, value string) error {
	_, err := config.DB.Exec(
		fmt.Sprintf("UPDATE dbo.tbl_PromoActivities SET %s = ?, updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL", field),
		value, id,
	)
	return err
}

// ─── Approval Filters ───────────────────────────────────────────────────────

type ApprovalFilterParams struct {
	ApprovalStatus, KAM, Network, Brand, MechFilter, YearStr, MonthStr, Role string
}

func GetApprovalFilters(params ApprovalFilterParams) (networks, brands, mechanics, kams []string, err error) {
	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	query := `
		SELECT DISTINCT p.network_name, p.brand_as, p.mechanics, p.kam
		FROM dbo.tbl_PromoActivities p
		WHERE p.deleted_at IS NULL
	`
	args := []interface{}{}

	if params.YearStr != "" {
		y, _ := strconv.Atoi(params.YearStr)
		query += " AND p.year = ?"
		args = append(args, y)
	} else {
		query += " AND (p.year > ? OR (p.year = ? AND p.month >= ?))"
		args = append(args, currentYear, currentYear, currentMonth)
	}

	if params.MonthStr != "" {
		m, _ := strconv.Atoi(params.MonthStr)
		query += " AND p.month = ?"
		args = append(args, m)
	}

	if params.KAM != "" {
		query += " AND p.kam = ?"
		args = append(args, params.KAM)
	}

	if params.Network != "" {
		query += " AND p.network_name = ?"
		args = append(args, params.Network)
	}

	if params.Brand != "" {
		query += " AND p.brand_as = ?"
		args = append(args, params.Brand)
	}

	if params.MechFilter != "" {
		query += " AND p.mechanics = ?"
		args = append(args, params.MechFilter)
	}

	// Фильтруем по статусу конкретной роли (а не по OR двух полей)
	filterStatusField := "p.agreement1_status"
	if params.Role == "agreement2" {
		filterStatusField = "p.agreement2_status"
	}

	switch params.ApprovalStatus {
	case "pending":
		query += fmt.Sprintf(" AND %s IS NULL", filterStatusField)
	case "commented":
		query += fmt.Sprintf(" AND %s = 'commented'", filterStatusField)
	case "approved":
		query += fmt.Sprintf(" AND %s = 'approved'", filterStatusField)
	case "rejected":
		query += fmt.Sprintf(" AND %s = 'rejected'", filterStatusField)
	}

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer rows.Close()

	networkSet := make(map[string]bool)
	brandSet := make(map[string]bool)
	mechSet := make(map[string]bool)
	kamSet := make(map[string]bool)

	for rows.Next() {
		var nw, br, mech, k sql.NullString
		if rows.Scan(&nw, &br, &mech, &k) == nil {
			if nw.Valid {
				networkSet[nw.String] = true
			}
			if br.Valid {
				brandSet[br.String] = true
			}
			if mech.Valid {
				mechSet[mech.String] = true
			}
			if k.Valid {
				kamSet[k.String] = true
			}
		}
	}

	for v := range networkSet {
		networks = append(networks, v)
	}
	for v := range brandSet {
		brands = append(brands, v)
	}
	for v := range mechSet {
		mechanics = append(mechanics, v)
	}
	for v := range kamSet {
		kams = append(kams, v)
	}

	sort.Strings(networks)
	sort.Strings(brands)
	sort.Strings(mechanics)
	sort.Strings(kams)

	return networks, brands, mechanics, kams, nil
}

func GetApprovalKAMs(field string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT p.kam 
		FROM dbo.tbl_PromoActivities p 
		WHERE p.deleted_at IS NULL AND %s IS NULL AND p.kam IS NOT NULL
		ORDER BY p.kam
	`, field)

	rows, err := config.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kams []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			kams = append(kams, k)
		}
	}
	if kams == nil {
		kams = []string{}
	}
	return kams, nil
}

func GetApprovalNetworks(field, kam string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT p.network_name 
		FROM dbo.tbl_PromoActivities p 
		WHERE p.deleted_at IS NULL 
		  AND %s IS NULL 
		  AND p.kam = ? 
		  AND p.network_name IS NOT NULL
		ORDER BY p.network_name
	`, field)

	rows, err := config.DB.Query(query, kam)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var networks []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			networks = append(networks, n)
		}
	}
	if networks == nil {
		networks = []string{}
	}
	return networks, nil
}

func GetApprovalBrands(field, kam, network string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT p.brand_as 
		FROM dbo.tbl_PromoActivities p 
		WHERE p.deleted_at IS NULL 
		  AND %s IS NULL 
		  AND p.kam = ? 
		  AND p.brand_as IS NOT NULL
	`, field)
	args := []interface{}{kam}

	if network != "" {
		query += " AND p.network_name = ?"
		args = append(args, network)
	}

	query += " ORDER BY p.brand_as"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var brands []string
	for rows.Next() {
		var b string
		if rows.Scan(&b) == nil {
			brands = append(brands, b)
		}
	}
	if brands == nil {
		brands = []string{}
	}
	return brands, nil
}
```

## File: backend/.env.example
```
DB_SERVER=localhost
DB_USER=sa
DB_PASSWORD=your_password_here
DB_NAME=local_project_db
DB_PORT=1433
CORS_ORIGINS=http://localhost:5173
```

## File: frontend/src/api/auth.js
```javascript
export const getToken = () => localStorage.getItem('token');
export const getUsername = () => localStorage.getItem('username');
export const getRole = () => localStorage.getItem('role');

export const logout = () => {
  localStorage.removeItem('token');
  localStorage.removeItem('username');
  localStorage.removeItem('role');
};

// Сохраняет данные сессии из ответа логина/рефреша
export const saveSession = (data) => {
  if (data.token) localStorage.setItem('token', data.token);
  if (data.username) localStorage.setItem('username', data.username);
  if (data.role) localStorage.setItem('role', data.role);
};

// Пытается обновить access token через refresh cookie
// Возвращает true если успешно
export const refreshToken = async () => {
  try {
    const res = await fetch('/api/auth/refresh', {
      method: 'POST',
      credentials: 'include', // отправляем httpOnly cookie
    });
    if (!res.ok) return false;
    const data = await res.json();
    saveSession(data);
    return true;
  } catch {
    return false;
  }
};
```

## File: frontend/src/components/ApprovalCard.jsx
```javascript
import { memo } from 'react';
import {
  Box, Typography, Card, CardContent, CardActions,
  Button, Chip, Collapse, Grid, TextField,
  LinearProgress, CircularProgress,
} from '@mui/material';
import {
  ExpandMore as ExpandMoreIcon,
  CheckCircle as ApproveIcon,
  Cancel as RejectIcon,
  Comment as CommentIcon,
} from '@mui/icons-material';

const fmtNum = (v, decimals = 0) => {
  if (v == null) return '—';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
};

const roiColor = (roi) => {
  if (roi == null) return '#94a3b8';
  return roi >= 0 ? '#16a34a' : '#dc2626';
};

const MONTHS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 },
  { label: 'Апрель', value: 4 }, { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 },
  { label: 'Июль', value: 7 }, { label: 'Август', value: 8 }, { label: 'Сентябрь', value: 9 },
  { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

const ApprovalCard = memo(function ApprovalCard({
  item, expanded, submitting, onCommentRef,
  onToggleExpand, onOpenConfirm, onCommentOnly,
}) {
  const id = item.id;
  const isSubmitting = submitting[id] || false;

  return (
    <Box sx={{ position: 'relative' }}>
      {isSubmitting && <LinearProgress sx={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 2, borderTopLeftRadius: 12, borderTopRightRadius: 12 }} />}
      <Card elevation={2} sx={{ borderRadius: 3, transition: 'all 0.2s', '&:hover': { boxShadow: 6 }, height: '100%', display: 'flex', flexDirection: 'column', opacity: isSubmitting ? 0.7 : 1 }}>
        {isSubmitting && (
          <Box sx={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1, bgcolor: 'rgba(255,255,255,0.4)', borderRadius: 3 }}>
            <CircularProgress size={32} />
          </Box>
        )}
        <CardContent sx={{ flex: 1, pb: 1 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 0.5 }}>
            {item.network_name || '—'}
          </Typography>
          <Box sx={{ display: 'flex', gap: 0.5, mb: 1, flexWrap: 'wrap' }}>
            <Chip label={item.sku || '—'} size="small" variant="outlined" />
            <Chip label={item.mechanics || '—'} size="small" color="primary" variant="outlined" />
          </Box>
          {item.year && item.month && (
            <Typography variant="caption" color="text.secondary" sx={{ mb: 1, display: 'block' }}>
              Период: {MONTHS.find(m => m.value === item.month)?.label || item.month} {item.year}
            </Typography>
          )}
          <Grid container spacing={1} sx={{ mb: 1 }}>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">Baseline</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.baseline_units)} уп</Typography>
            </Grid>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">План</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.plan_promo_units)} уп</Typography>
            </Grid>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">Факт продаж</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.actual_promo_sales_units)} уп</Typography>
            </Grid>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">Инвестиции</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.plan_investments_rub, 2)} ₽</Typography>
            </Grid>
          </Grid>
          <Box sx={{ display: 'flex', gap: 2, mb: 1 }}>
            <Box>
              <Typography variant="caption" color="text.secondary">ROI план</Typography>
              <Typography variant="body2" sx={{ fontWeight: 700, color: roiColor(item.plan_roi) }}>
                {item.plan_roi != null ? `${Number(item.plan_roi).toFixed(1)}%` : '—'}
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">ROI факт</Typography>
              <Typography variant="body2" sx={{ fontWeight: 700, color: roiColor(item.actual_roi) }}>
                {item.actual_roi != null ? `${Number(item.actual_roi).toFixed(1)}%` : '—'}
              </Typography>
            </Box>
          </Box>
          <Box sx={{ bgcolor: '#f1f5f9', borderRadius: 1.5, p: 1, mb: 1, display: 'flex', gap: 2 }}>
            <Typography variant="caption" color="text.secondary">История: {item.historical_count} промо</Typography>
            <Typography variant="caption" color="text.secondary">
              Средний ROI: {item.avg_historical_roi != null ? `${Number(item.avg_historical_roi).toFixed(1)}%` : '—'}
            </Typography>
          </Box>
          {(item.agreement1 || item.agreement2) && (
            <Box sx={{ mb: 1, display: 'flex', flexDirection: 'column', gap: 0.5 }}>
              {item.agreement1 && (
                <Typography variant="caption" sx={{ 
                  p: 0.75, borderRadius: 1, fontSize: '0.72rem',
                  bgcolor: String(item.agreement1).startsWith('согласовано') ? '#f0fdf4' : 
                           String(item.agreement1).startsWith('отклонено') ? '#fef2f2' : '#eef2ff',
                  color: String(item.agreement1).startsWith('согласовано') ? '#16a34a' : 
                         String(item.agreement1).startsWith('отклонено') ? '#dc2626' : '#6366f1',
                }}>
                  <b>Согл. 1:</b> {item.agreement1}
                </Typography>
              )}
              {item.agreement2 && (
                <Typography variant="caption" sx={{ 
                  p: 0.75, borderRadius: 1, fontSize: '0.72rem',
                  bgcolor: String(item.agreement2).startsWith('согласовано') ? '#f0fdf4' : 
                           String(item.agreement2).startsWith('отклонено') ? '#fef2f2' : '#eef2ff',
                  color: String(item.agreement2).startsWith('согласовано') ? '#16a34a' : 
                         String(item.agreement2).startsWith('отклонено') ? '#dc2626' : '#6366f1',
                }}>
                  <b>Согл. 2:</b> {item.agreement2}
                </Typography>
              )}
            </Box>
          )}
          {item.conditions && (
            <Box sx={{ mb: 1 }}>
              <Button size="small" onClick={() => onToggleExpand(id)}
                endIcon={<ExpandMoreIcon sx={{ transform: expanded ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 0.2s' }} />}
                sx={{ color: '#64748b', textTransform: 'none', p: 0 }}>Условия</Button>
              <Collapse in={expanded}>
                <Typography variant="body2" sx={{ mt: 0.5, p: 1, bgcolor: '#f8fafc', borderRadius: 1, fontSize: '0.8rem', color: '#475569' }}>
                  {item.conditions}
                </Typography>
              </Collapse>
            </Box>
          )}
          <TextField size="small" fullWidth multiline minRows={1} maxRows={3}
            placeholder="Комментарий (необязательно)"
            inputRef={(el) => { if (el && onCommentRef) onCommentRef(id, el); }}
            sx={{ mb: 1 }} />
        </CardContent>
        <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2, gap: 0.5, mt: 'auto' }}>
          <Button size="small" variant="outlined" startIcon={<CommentIcon />}
            onClick={() => onCommentOnly(id)} disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>Комментарий</Button>
          <Button size="small" variant="contained" color="success" startIcon={<ApproveIcon />}
            onClick={() => onOpenConfirm(id, 'согласовано')} disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>Согласовано</Button>
          <Button size="small" variant="contained" color="error" startIcon={<RejectIcon />}
            onClick={() => onOpenConfirm(id, 'отклонено')} disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>Отклонено</Button>
        </CardActions>
      </Card>
    </Box>
  );
});

export default ApprovalCard;
```

## File: frontend/src/components/DrilldownModal.jsx
```javascript
import { useState, useEffect } from 'react';
import {
  Dialog, DialogTitle, DialogContent, IconButton, Typography,
  Box, Tabs, Tab, CircularProgress, Alert
} from '@mui/material';
import { Close as CloseIcon } from '@mui/icons-material';
import { DataGrid } from '@mui/x-data-grid';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend,
  ResponsiveContainer
} from 'recharts';

const API_BASE = 'http://localhost:8080';

const COLORS = [
  '#8884d8', '#82ca9d', '#ffc658', '#ff7300', '#a4de6c',
  '#d0ed57', '#83a6ed', '#8dd1e1', '#82ca9d', '#a4de6c',
  '#d0ed57', '#ffc658', '#ff7300', '#8884d8', '#83a6ed',
];

export default function DrilldownModal({ open, onClose, rowData, appliedFilters = {} }) {
  const [tab, setTab] = useState(0);
  const [viewType, setViewType] = useState('total');
  const [data, setData] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (open && rowData) fetchDrilldownData();
  }, [open, rowData, appliedFilters]);

  const fetchDrilldownData = async () => {
    if (!rowData?.brandName || !rowData?.networkName) return;
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        brandName: rowData.brandName,
        networkName: rowData.networkName,
      });
      if (appliedFilters.yearFrom) params.set('yearFrom', appliedFilters.yearFrom);
      if (appliedFilters.yearTo) params.set('yearTo', appliedFilters.yearTo);
      if (appliedFilters.months?.length > 0) appliedFilters.months.forEach(m => params.append('months', String(m)));
      if (appliedFilters.segment?.length > 0) appliedFilters.segment.forEach(s => params.append('segment', s));
      if (appliedFilters.channel?.length > 0) appliedFilters.channel.forEach(c => params.append('channel', c));

      const response = await fetch(`${API_BASE}/api/drilldown?${params}`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const json = await response.json();
      setData(json.data || []);
    } catch (err) { setError(err.message); } finally { setLoading(false); }
  };

  const chartData = prepareChartData(data);

  const columns = [
    { field: 'year', headerName: 'Год', width: 80, type: 'number', valueFormatter: (value) => value },
    { field: 'month', headerName: 'Месяц', width: 80, type: 'number' },
    { field: 'metricType', headerName: 'Показатель', width: 130 },
    { field: 'totalValue', headerName: 'Значение', width: 140, type: 'number',
      valueFormatter: (value) => value != null ? Number(value).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '' },
    { field: 'un_rub', headerName: 'Уп/Руб', width: 90 },
    { field: 'segment', headerName: 'Сегмент', width: 140 },
    { field: 'channel', headerName: 'Канал', width: 140 },
  ];

  if (!rowData) return null;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth PaperProps={{ sx: { height: '80vh' } }}>
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography variant="h6">Детализация: {rowData.brandName}</Typography>
          <Typography variant="body2" color="text.secondary">
            Сеть: {rowData.networkName}
            {appliedFilters.yearFrom && ` • Годы: ${appliedFilters.yearFrom}${appliedFilters.yearTo ? `–${appliedFilters.yearTo}` : '+'}`}
          </Typography>
        </Box>
        <IconButton onClick={onClose} size="small"><CloseIcon /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
          <Tabs value={tab} onChange={(_, v) => setTab(v)}>
            <Tab label="График" />
            <Tab label="Таблица" />
          </Tabs>
          {tab === 0 && chartData.length > 0 && (
            <Tabs value={viewType} onChange={(_, v) => setViewType(v)}
              sx={{ minHeight: 40, '& .MuiTab-root': { minHeight: 40, py: 0.5 } }}>
              <Tab label="Уп/Руб" value="total" />
              <Tab label="По сегментам" value="segments" />
              <Tab label="По каналам" value="channels" />
            </Tabs>
          )}
        </Box>

        {loading && <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}><CircularProgress /></Box>}
        {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

        {!loading && !error && (
          <>
            {tab === 0 && (
              <Box sx={{ flex: 1, minHeight: 400, height: '100%' }}>
                {chartData.length > 0 ? (
                  <ResponsiveContainer width="100%" height={400}>
                    {viewType === 'total' ? (
                      <BarChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="period" tick={{ fontSize: 12 }} angle={-45} textAnchor="end" height={60} />
                        <YAxis />
                        <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} />
                        <Legend />
                        <Bar dataKey="упаковки" fill="#8884d8" radius={[4, 4, 0, 0]} />
                        <Bar dataKey="рубли" fill="#82ca9d" radius={[4, 4, 0, 0]} />
                      </BarChart>
                    ) : viewType === 'segments' ? (
                      <BarChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="period" tick={{ fontSize: 12 }} angle={-45} textAnchor="end" height={60} />
                        <YAxis />
                        <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} />
                        <Legend />
                        {getUniqueKeys(chartData, 'segments').map((segKey, idx) => {
                          const originalName = segKey.replace(/_/g, '.');
                          return <Bar key={segKey} dataKey={`segments.${segKey}`} name={originalName} fill={COLORS[idx % COLORS.length]} radius={[4, 4, 0, 0]} />;
                        })}
                      </BarChart>
                    ) : (
                      <BarChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="period" tick={{ fontSize: 12 }} angle={-45} textAnchor="end" height={60} />
                        <YAxis />
                        <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} />
                        <Legend />
                        {getUniqueKeys(chartData, 'channels').map((chKey, idx) => {
                          const originalName = chKey.replace(/_/g, '.');
                          return <Bar key={chKey} dataKey={`channels.${chKey}`} name={originalName} fill={COLORS[idx % COLORS.length]} radius={[4, 4, 0, 0]} />;
                        })}
                      </BarChart>
                    )}
                  </ResponsiveContainer>
                ) : (
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                    <Typography color="text.secondary">Нет данных для отображения графика</Typography>
                  </Box>
                )}
              </Box>
            )}

            {tab === 1 && (
              <Box sx={{ flex: 1 }}>
                <DataGrid
                  rows={data.map((row, idx) => ({ ...row, id: idx }))} columns={columns}
                  initialState={{ pagination: { paginationModel: { pageSize: 50 } }, sorting: { sortModel: [{ field: 'year', sort: 'desc' }, { field: 'month', sort: 'asc' }] } }}
                  pageSizeOptions={[25, 50, 100]} disableRowSelectionOnClick sx={{ height: '100%' }} />
              </Box>
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function prepareChartData(data) {
  const grouped = {};
  data.forEach((row) => {
    const key = `${row.year}-${String(row.month).padStart(2, '0')}`;
    if (!grouped[key]) grouped[key] = { period: key, упаковки: 0, рубли: 0, segments: {}, channels: {} };
    if (row.un_rub === 'уп') grouped[key].упаковки += row.totalValue;
    else if (row.un_rub === 'руб') grouped[key].рубли += row.totalValue;
    const segmentKey = (row.segment || 'Без сегмента').replace(/\./g, '_');
    if (!grouped[key].segments[segmentKey]) grouped[key].segments[segmentKey] = 0;
    grouped[key].segments[segmentKey] += row.totalValue;
    const channelKey = (row.channel || 'Без канала').replace(/\./g, '_');
    if (!grouped[key].channels[channelKey]) grouped[key].channels[channelKey] = 0;
    grouped[key].channels[channelKey] += row.totalValue;
  });
  return Object.values(grouped).sort((a, b) => a.period.localeCompare(b.period));
}

function getUniqueKeys(data, type) {
  const keys = new Set();
  data.forEach(item => { Object.keys(item[type] || {}).forEach(key => keys.add(key)); });
  return Array.from(keys).sort();
}
```

## File: frontend/src/hooks/usePromoFilters.js
```javascript
import { useState, useEffect, useCallback, useRef } from 'react';
import { promoAPI } from '../api/promo';

export function usePromoFilters(initialFilters, storageKey, persistFlagKey) {
  const [meta, setMeta] = useState({
    kam: [], brand: [], sku: [], network_name: [], mechanics: [], channel: [], status: [],
    loading: true, error: null
  });
  
  const [filters, setFilters] = useState(() => {
    try {
      if (localStorage.getItem(persistFlagKey) === 'true') {
        const saved = sessionStorage.getItem(storageKey);
        if (saved) return JSON.parse(saved);
      }
    } catch (e) {}
    return { ...initialFilters };
  });

  const [appliedFilters, setAppliedFilters] = useState(filters);
  const [persistFilters, setPersistFilters] = useState(
    () => localStorage.getItem(persistFlagKey) === 'true'
  );
  const debounceRef = useRef(null);

  // Загрузка метаданных
  const fetchMeta = useCallback(async (currentFilters) => {
    setMeta(prev => ({ ...prev, loading: true }));
    try {
      const json = await promoAPI.getFilters(currentFilters);
      setMeta({
        kam: json.kam || [], brand: json.brand || [], sku: json.sku || [],
        network_name: json.network_name || [], mechanics: json.mechanics || [],
        channel: json.channel || [], status: json.status || [],
        loading: false, error: null
      });
    } catch (err) {
      setMeta(prev => ({ ...prev, loading: false, error: err.message }));
    }
  }, []);

  // Первичная загрузка
  useEffect(() => { fetchMeta(filters); }, []);

  // Обновление с debounce при изменении фильтров
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => fetchMeta(filters), 300);
    return () => clearTimeout(debounceRef.current);
  }, [filters, fetchMeta]);

  const handleSearch = useCallback(() => {
    setAppliedFilters({ ...filters });
    // Если галочка включена — сохраняем фильтры в sessionStorage
    if (localStorage.getItem(persistFlagKey) === 'true') {
      sessionStorage.setItem(storageKey, JSON.stringify(filters));
    }
  }, [filters, persistFlagKey, storageKey]);

  const handleReset = useCallback(() => {
    const empty = { ...initialFilters };
    setFilters(empty);
    setAppliedFilters(empty);
    sessionStorage.removeItem(storageKey);
  }, [initialFilters, storageKey]);

  const handlePersistChange = useCallback((checked) => {
    setPersistFilters(checked);
    localStorage.setItem(persistFlagKey, String(checked));
    if (checked) {
      // Сразу сохраняем текущие фильтры при включении
      sessionStorage.setItem(storageKey, JSON.stringify(filters));
    } else {
      sessionStorage.removeItem(storageKey);
    }
  }, [persistFlagKey, storageKey, filters]);

  return {
    meta, filters, setFilters, appliedFilters,
    persistFilters, handleSearch, handleReset, handlePersistChange,
    fetchMeta
  };
}
```

## File: frontend/src/pages/InternetSales.jsx
```javascript
import { useState, useEffect, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Stack, Box, Typography, CircularProgress } from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import FilterPanel from '../components/FilterPanel';
import DataTable from '../components/DataTable';
import DrilldownModal from '../components/DrilldownModal';
import { salesAPI } from '../api/promo';

const API_BASE = 'http://localhost:8080';
const FILTERS_STORAGE_KEY = 'internet_sales_filters_v7';
const PERSIST_FLAG_KEY = 'internet_sales_persist_v7';

const COLUMNS = [
  { field: 'year', headerName: 'Год', width: 90, type: 'number', valueFormatter: (v) => v },
  { field: 'month', headerName: 'Месяц', width: 80, type: 'number' },
  { field: 'brandName', headerName: 'Бренд', width: 150 },
  { field: 'productName', headerName: 'Продукт', width: 250 },
  { field: 'networkName', headerName: 'Сеть', width: 200 },
  { field: 'metricType', headerName: 'Показатель', width: 140 },
  { field: 'metricValue', headerName: 'Значение', width: 130, type: 'number',
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '' },
  { field: 'un_rub', headerName: 'Уп/Руб', width: 100 },
  { field: 'segment', headerName: 'Сегмент', width: 150 },
  { field: 'channel', headerName: 'Канал', width: 150 },
  { field: 'updated_at', headerName: 'Обновлено', width: 160 },
  { field: 'id', headerName: 'ID', width: 70, type: 'number' },
];

const EMPTY_FILTERS = {
  yearFrom: '', yearTo: '', months: [],
  brandName: [], networkName: [], un_rub: [], segment: [], channel: []
};
const EXTRA_FILTERS = [
  { type: 'year', field: 'yearFrom', label: 'Год от' },
  { type: 'year', field: 'yearTo', label: 'Год до' },
  { type: 'months', field: 'months', label: 'Месяцы' }
];

export default function InternetSales() {
  const navigate = useNavigate();

  const [meta, setMeta] = useState({
    brandName: [], networkName: [], un_rub: [], segment: [], channel: [],
    segmentChannelMap: {}, channelSegmentMap: {},
    loading: true, error: null
  });

  const [filters, setFilters] = useState(() => {
    try {
      if (localStorage.getItem(PERSIST_FLAG_KEY) === 'true') {
        const saved = sessionStorage.getItem(FILTERS_STORAGE_KEY);
        if (saved) {
          const parsed = JSON.parse(saved);
          if (parsed && Array.isArray(parsed.months)) return parsed;
        }
      }
    } catch (e) {}
    return { ...EMPTY_FILTERS };
  });

  const [persistFilters, setPersistFilters] = useState(
    () => localStorage.getItem(PERSIST_FLAG_KEY) === 'true'
  );
  const [appliedFilters, setAppliedFilters] = useState(filters);
  const [rowCount, setRowCount] = useState(0);
  const [drilldownRow, setDrilldownRow] = useState(null);

  // Загрузка справочников через API-слой
  useEffect(() => {
    salesAPI.getFilters()
      .then(data => setMeta({
        brandName: data.brandName || [],
        networkName: data.networkName || [],
        un_rub: data.un_rub || [],
        segment: data.segment || [],
        channel: data.channel || [],
        segmentChannelMap: data.segmentChannelMap || {},
        channelSegmentMap: data.channelSegmentMap || {},
        loading: false,
        error: null
      }))
      .catch(err => setMeta(prev => ({ ...prev, loading: false, error: err.message })));
  }, []);

  // Видимые опции фильтров с учётом каскада
  const filterOptions = useMemo(() => {
    let segments = [...meta.segment];
    const channels = [...meta.channel];

    if (filters.channel.length > 0) {
      const allowed = new Set();
      filters.channel.forEach(ch => {
        (meta.channelSegmentMap[ch] || []).forEach(seg => allowed.add(seg));
      });
      segments = segments.filter(seg => allowed.has(seg));
    }

    return {
      brandName: meta.brandName,
      networkName: meta.networkName,
      un_rub: meta.un_rub,
      channel: channels,
      segment: segments,
    };
  }, [meta, filters.channel]);

  // Каскадная фильтрация
  const handleFiltersChange = useCallback((newFilters) => {
    let updated = { ...newFilters };

    if (JSON.stringify(newFilters.segment) !== JSON.stringify(filters.segment)) {
      const addedSegments = newFilters.segment.filter(seg => !filters.segment.includes(seg));
      const removedSegments = filters.segment.filter(seg => !newFilters.segment.includes(seg));

      if (addedSegments.length > 0) {
        const channelsToAdd = new Set(updated.channel);
        addedSegments.forEach(seg => {
          (meta.segmentChannelMap[seg] || []).forEach(ch => channelsToAdd.add(ch));
        });
        updated.channel = Array.from(channelsToAdd);
      }

      if (removedSegments.length > 0) {
        const channelsToRemove = new Set();
        removedSegments.forEach(seg => {
          const linked = meta.segmentChannelMap[seg] || [];
          linked.forEach(ch => {
            const allSegs = meta.channelSegmentMap[ch] || [];
            if (allSegs.filter(s => updated.segment.includes(s)).length === 0) {
              channelsToRemove.add(ch);
            }
          });
        });
        updated.channel = updated.channel.filter(ch => !channelsToRemove.has(ch));
      }
    }

    if (JSON.stringify(newFilters.channel) !== JSON.stringify(filters.channel)) {
      const removedChannels = filters.channel.filter(ch => !newFilters.channel.includes(ch));
      const addedChannels = newFilters.channel.filter(ch => !filters.channel.includes(ch));

      if (removedChannels.length > 0) {
        const segsToRemove = new Set();
        removedChannels.forEach(ch => {
          (meta.channelSegmentMap[ch] || []).forEach(seg => segsToRemove.add(seg));
        });
        updated.segment = updated.segment.filter(seg => !segsToRemove.has(seg));
      }

      if (addedChannels.length > 0) {
        const segsToAdd = new Set(updated.segment);
        addedChannels.forEach(ch => {
          (meta.channelSegmentMap[ch] || []).forEach(seg => segsToAdd.add(seg));
        });
        updated.segment = Array.from(segsToAdd);
      }
    }

    setFilters(updated);
  }, [filters, meta]);

  const handleSearch = useCallback(() => {
    setAppliedFilters({ ...filters });
    if (persistFilters) sessionStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
  }, [filters, persistFilters]);

  const handleReset = useCallback(() => {
    const empty = { ...EMPTY_FILTERS };
    setFilters(empty);
    setAppliedFilters(empty);
    sessionStorage.removeItem(FILTERS_STORAGE_KEY);
    setRowCount(0);
    setDrilldownRow(null);
  }, []);

  const handlePersistChange = useCallback((checked) => {
    setPersistFilters(checked);
    localStorage.setItem(PERSIST_FLAG_KEY, String(checked));
    if (checked) {
      sessionStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
    } else {
      sessionStorage.removeItem(FILTERS_STORAGE_KEY);
    }
  }, [filters]);

  const handleDataLoaded = useCallback((data) => setRowCount(data.length), []);
  const handleRowClick = useCallback((params) => {
    if (params.row.networkName && params.row.brandName) setDrilldownRow(params.row);
  }, []);
  const handleCloseDrilldown = useCallback(() => setDrilldownRow(null), []);

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2 }}>
      <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 2 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>Интернет-продажи</Typography>
        {meta.loading && <CircularProgress size={20} />}
        {rowCount > 0 && (
          <Typography variant="body2" color="text.secondary">
            Загружено: {rowCount.toLocaleString('ru-RU')} строк
          </Typography>
        )}
      </Stack>

      <Box sx={{ mb: 2 }}>
        <FilterPanel
          filters={filters}
          filterOptions={filterOptions}
          onFiltersChange={handleFiltersChange}
          onSearch={handleSearch}
          onReset={handleReset}
          extraFilters={EXTRA_FILTERS}
          persistFilters={persistFilters}
          onPersistChange={handlePersistChange}
        />
      </Box>

      {meta.error && (
        <Button
          variant="outlined" color="warning"
          onClick={() => window.location.reload()}
          sx={{ mb: 2, alignSelf: 'flex-start' }}
        >
          Ошибка загрузки справочников
        </Button>
      )}

      <Box sx={{ flex: 1, overflow: 'hidden' }}>
        <DataTable
          columns={COLUMNS}
          apiUrl={`${API_BASE}/api/data`}
          filters={appliedFilters}
          exportFileName="internet-sales"
          onDataLoaded={handleDataLoaded}
          onRowClick={handleRowClick}
        />
      </Box>

      <DrilldownModal
        open={!!drilldownRow}
        onClose={handleCloseDrilldown}
        rowData={drilldownRow}
        appliedFilters={appliedFilters}
      />
    </Box>
  );
}
```

## File: frontend/src/pages/Login.jsx
```javascript
import { useState } from 'react';
import { Box, Card, TextField, Button, Typography, Alert } from '@mui/material';
import { Lock as LockIcon } from '@mui/icons-material';
import { saveSession } from '../api/auth';

export default function Login({ onLogin }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!username || !password) {
      setError('Заполните все поля');
      return;
    }
    setLoading(true); setError('');
    try {
      const response = await fetch('http://localhost:8080/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
        credentials: 'include', // получаем httpOnly refresh cookie
      });
      const data = await response.json();
      if (!response.ok) {
        setError(data.error || 'Ошибка входа');
        return;
      }
      saveSession(data);
      onLogin(data);
    } catch (err) {
      setError('Сервер недоступен');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box sx={{ 
      minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center',
      bgcolor: '#f1f5f9'
    }}>
      <Card elevation={3} sx={{ p: 5, width: 400, borderRadius: 4 }}>
        <Box sx={{ textAlign: 'center', mb: 3 }}>
          <Box sx={{ 
            width: 56, height: 56, borderRadius: '16px', bgcolor: '#6366f115',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            mx: 'auto', mb: 2, color: '#6366f1'
          }}>
            <LockIcon sx={{ fontSize: 28 }} />
          </Box>
          <Typography variant="h5" sx={{ fontWeight: 700 }}>Вход в систему</Typography>
          <Typography variant="body2" color="text.secondary">Аналитический портал</Typography>
        </Box>

        {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

        <form onSubmit={handleSubmit}>
          <TextField label="Логин" fullWidth size="small" value={username}
            onChange={(e) => setUsername(e.target.value)} sx={{ mb: 2 }} />
          <TextField label="Пароль" type="password" fullWidth size="small" value={password}
            onChange={(e) => setPassword(e.target.value)} sx={{ mb: 3 }} />
          <Button variant="contained" fullWidth type="submit" disabled={loading}
            sx={{ py: 1.2, fontWeight: 600 }}>
            {loading ? 'Вход...' : 'Войти'}
          </Button>
        </form>
      </Card>
    </Box>
  );
}
```

## File: frontend/src/main.jsx
```javascript
import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App.jsx';
import './index.css';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 минут — данные не перезапрашиваются
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

createRoot(document.getElementById('root')).render(
  //<React.StrictMode>
    <BrowserRouter>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </BrowserRouter>
  //</React.StrictMode>,
);
```

## File: frontend/package.json
```json
{
  "name": "frontend",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "lint": "eslint .",
    "preview": "vite preview"
  },
  "dependencies": {
    "@emotion/react": "^11.14.0",
    "@emotion/styled": "^11.14.1",
    "@mui/icons-material": "^9.2.0",
    "@mui/material": "^9.2.0",
    "@mui/x-data-grid": "^9.10.1",
    "@tanstack/react-query": "^5.101.4",
    "react": "^19.2.7",
    "react-dom": "^19.2.7",
    "react-router-dom": "^7.11.0",
    "recharts": "^3.10.0"
  },
  "devDependencies": {
    "@eslint/js": "^10.0.1",
    "@types/react": "^19.2.17",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^6.0.3",
    "eslint": "^10.6.0",
    "eslint-plugin-react-hooks": "^7.1.1",
    "eslint-plugin-react-refresh": "^0.5.3",
    "globals": "^17.7.0",
    "vite": "^8.1.1"
  }
}
```

## File: backend/handlers/auth.go
```go
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
```

## File: backend/main_test.go
```go
package main

import (
	"backend/config"
	"backend/handlers"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
	config.Init()
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	// Промо
	r.POST("/api/promo/save", handlers.SavePromo)
	r.DELETE("/api/promo/:id", handlers.DeletePromo)
	r.GET("/api/promo/data", handlers.GetPromoData)
	r.GET("/api/promo/filters", handlers.GetPromoFilters)
	// Интернет-продажи
	r.GET("/api/data", handlers.GetData)
	r.GET("/api/filters", handlers.GetFilterOptions)
	return r
}

// cleanupTestData удаляет тестовые записи после прогона
func cleanupTestData() {
	config.DB.Exec("DELETE FROM dbo.tbl_PromoActivities WHERE sku LIKE 'TEST-%'")
}

func TestMain(m *testing.M) {
	code := m.Run()
	cleanupTestData()
	os.Exit(code)
}

// ==================== СОЗДАНИЕ ====================

func TestSavePromo_Create(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{
		"network_name": "Тестовая сеть", "sku": "TEST-SKU-001",
		"year": 2026, "month": 1, "mechanics": "Скидка", "gtn_opex": "GTN",
		"baseline_units": 100, "plan_promo_units": 150, "plan_investments_rub": 5000,
		"contract_price": 200, "id_directum": "DIR-001", "ds_number": "DS-001",
		"discount_amount": 15.5, "conditions": "Тестовые условия",
		"ecom_segment":     "есть, не убирают из отчета",
		"total_pharmacies": 1000, "promo_pharmacies": 500, "status": "Планируется",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d: %s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["message"] != "Created" {
		t.Errorf("Ожидалось 'Created', получено '%v'", response["message"])
	}
	data := response["data"].(map[string]interface{})
	if data["plan_promo_rub"].(float64) != 30000 {
		t.Errorf("plan_promo_rub: ожидалось 30000, получено %f", data["plan_promo_rub"])
	}
	if data["quarter"].(float64) != 1 {
		t.Errorf("quarter: ожидался 1, получен %f", data["quarter"])
	}
}

func TestSavePromo_MissingFields(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{"network_name": "Тестовая сеть", "year": 2026}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}
}

func TestSavePromo_ZeroValues(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{
		"network_name": "Тестовая сеть", "sku": "TEST-ZERO-001",
		"year": 2026, "month": 6, "baseline_units": 0, "plan_promo_units": 0,
		"plan_investments_rub": 0, "contract_price": 0,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})
	if data["plan_promo_rub"].(float64) != 0 {
		t.Error("plan_promo_rub должен быть 0 при нулевых входных данных")
	}
	if data["plan_roi"].(float64) != 0 {
		t.Error("plan_roi должен быть 0 при нулевых инвестициях")
	}
}

func TestSavePromo_NegativeValues(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{
		"network_name": "Тест", "sku": "TEST-NEG-001",
		"year": 2026, "month": 3, "baseline_units": 100, "plan_promo_units": 80,
		"plan_investments_rub": 5000, "contract_price": 200,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})
	if data["plan_promo_uplift_units"].(float64) != -20 {
		t.Error("plan_promo_uplift_units должен быть -20")
	}
}

func TestSavePromo_QuarterCalculation(t *testing.T) {
	router := setupRouter()
	months := []int{1, 4, 7, 10}
	expectedQuarters := []float64{1, 2, 3, 4}
	for i, month := range months {
		payload := map[string]interface{}{
			"network_name": "Тест", "sku": fmt.Sprintf("TEST-Q%d", month),
			"year": 2026, "month": month, "plan_promo_units": 100, "contract_price": 100,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		if data["quarter"].(float64) != expectedQuarters[i] {
			t.Errorf("Месяц %d: ожидался квартал %.0f, получен %.0f", month, expectedQuarters[i], data["quarter"])
		}
	}
}

// ==================== ОБНОВЛЕНИЕ ====================

// saveTestPromo создаёт тестовое промо и возвращает его ID
func saveTestPromo(t *testing.T, router *gin.Engine, sku string) int {
	t.Helper()
	payload := map[string]interface{}{
		"network_name": "Тест-Update", "sku": sku, "year": 2026, "month": 5,
		"baseline_units": 100, "plan_promo_units": 200, "plan_investments_rub": 10000,
		"contract_price": 150, "mechanics": "Скидка", "gtn_opex": "GTN",
		"status": "Планируется",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	return int(response["id"].(float64))
}

func TestUpdatePromo_RecalculatesFields(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-UPDATE-001")

	// Меняем plan_promo_units: 200 → 300
	payload := map[string]interface{}{
		"id": id, "plan_promo_units": 300,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["message"] != "Updated" {
		t.Errorf("Ожидалось 'Updated', получено '%v'", response["message"])
	}

	data := response["data"].(map[string]interface{})

	// plan_promo_rub = 300 * 150 = 45000
	if planRub, ok := data["plan_promo_rub"]; !ok || math.Abs(planRub.(float64)-45000) > 0.01 {
		t.Errorf("plan_promo_rub: ожидалось 45000, получено %v", data["plan_promo_rub"])
	}
	// uplift_units = 300 - 100 = 200
	if uplift, ok := data["plan_promo_uplift_units"]; !ok || math.Abs(uplift.(float64)-200) > 0.01 {
		t.Errorf("plan_promo_uplift_units: ожидалось 200, получено %v", data["plan_promo_uplift_units"])
	}
}

func TestUpdatePromo_ChangesQuarterOnMonthChange(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-UPDATE-Q")

	// Меняем месяц: 5 (Q2) → 11 (Q4)
	payload := map[string]interface{}{"id": id, "month": 11}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	if data["quarter"].(float64) != 4 {
		t.Errorf("quarter: ожидался 4 (месяц 11), получен %v", data["quarter"])
	}
}

func TestUpdatePromo_PreservesUnchangedFields(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-UPDATE-PRES")

	// Меняем только status
	payload := map[string]interface{}{"id": id, "status": "Завершено"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	// network_name должен сохраниться
	if data["network_name"] != "Тест-Update" {
		t.Errorf("network_name: ожидалось 'Тест-Update', получено '%v'", data["network_name"])
	}
	// contract_price должен сохраниться и использоваться в расчётах
	if cp, ok := data["contract_price"]; !ok || math.Abs(cp.(float64)-150) > 0.01 {
		t.Errorf("contract_price: ожидалось 150, получено %v", data["contract_price"])
	}
}

func TestUpdatePromo_UpdateNonExistent(t *testing.T) {
	router := setupRouter()
	payload := map[string]interface{}{"id": 99999999, "status": "Завершено"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// fetchExistingRow вернёт ошибку → 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Ожидался статус 500 для несуществующего ID, получен %d", w.Code)
	}
}

// ==================== УДАЛЕНИЕ ====================

func TestDeletePromo_NotFound(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("DELETE", "/api/promo/99999999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Ожидался статус 200 или 404, получен %d", w.Code)
	}
}

func TestDeletePromo_InvalidID(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("DELETE", "/api/promo/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Ожидался статус 400, получен %d", w.Code)
	}
}

func TestDeletePromo_ThenVerifyGone(t *testing.T) {
	router := setupRouter()
	id := saveTestPromo(t, router, "TEST-DELETE-001")

	// Удаляем
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/promo/%d", id), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200 при удалении, получен %d", w.Code)
	}

	// Пробуем обновить удалённое → 500
	payload := map[string]interface{}{"id": id, "status": "Обновлено"}
	body, _ := json.Marshal(payload)
	req2, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("Ожидался статус 500 при обновлении удалённой записи, получен %d", w2.Code)
	}
}

// ==================== ФИЛЬТРЫ ПРОМО ====================

func TestGetPromoFilters_All(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/filters", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Все ключи должны присутствовать
	for _, key := range []string{"kam", "brand", "sku", "network_name", "mechanics", "channel", "status"} {
		if _, ok := response[key]; !ok {
			t.Errorf("В ответе отсутствует ключ '%s'", key)
		}
	}
}

func TestGetPromoFilters_ByYear(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/filters?yearFrom=2025&yearTo=2026", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Фильтр по году: ожидался 200, получен %d", w.Code)
	}
}

func TestGetPromoFilters_ByMonth(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/filters?months=1&months=6&months=12", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Фильтр по месяцам: ожидался 200, получен %d", w.Code)
	}
}

func TestGetPromoFilters_Cascading(t *testing.T) {
	router := setupRouter()
	// Выбираем одного KAM → список брендов должен сузиться
	req, _ := http.NewRequest("GET", "/api/promo/filters?kam=%D0%98%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Даже если такого KAM нет — фильтр не должен падать
	if w.Code != http.StatusOK {
		t.Errorf("Каскадный фильтр: ожидался 200, получен %d", w.Code)
	}
}

// ==================== ДАННЫЕ ПРОМО ====================

func TestGetPromoData_Empty(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/data?all=true&yearFrom=1900&yearTo=1901", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("Ожидался пустой массив, получено %d записей", len(data))
	}
}

func TestGetPromoData_Pagination(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/promo/data?page=0&pageSize=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Пагинация: ожидался 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})
	if len(data) > 5 {
		t.Errorf("Пагинация: ожидалось не более 5 записей, получено %d", len(data))
	}
}

// ==================== ИНТЕРНЕТ-ПРОДАЖИ: ФИЛЬТРЫ ====================

func TestGetFilterOptions_ReturnsAllKeys(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/filters", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", w.Code)
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	for _, key := range []string{"brandName", "networkName", "un_rub", "segment", "channel", "segmentChannelMap", "channelSegmentMap"} {
		if _, ok := response[key]; !ok {
			t.Errorf("В ответе отсутствует ключ '%s'", key)
		}
	}
}

func TestGetData_WithFilters(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/data?yearFrom=2024&yearTo=2025&page=0&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d: %s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if _, ok := response["totalRows"]; !ok {
		t.Error("Ответ должен содержать 'totalRows' при пагинации")
	}
}

func TestGetData_AllForExport(t *testing.T) {
	router := setupRouter()
	req, _ := http.NewRequest("GET", "/api/data?all=true&yearFrom=2025&yearTo=2025", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("all=true: ожидался 200, получен %d", w.Code)
	}
}

// ==================== ИНТЕГРАЦИОННЫЙ: Create → Read → Update → Delete ====================

func TestIntegration_PromoCRUD(t *testing.T) {
	router := setupRouter()
	timestamp := time.Now().UnixNano()

	// CREATE
	createPayload := map[string]interface{}{
		"network_name": "Интеграция-Сеть",
		"sku":          fmt.Sprintf("TEST-INT-%d", timestamp),
		"year":         2026, "month": 7,
		"mechanics": "Скидка 15%", "gtn_opex": "GTN",
		"baseline_units": 500, "plan_promo_units": 800,
		"plan_investments_rub": 25000, "contract_price": 300,
		"status": "Планируется",
	}
	body, _ := json.Marshal(createPayload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CREATE: ожидался 200, получен %d: %s", w.Code, w.Body.String())
	}
	var createResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	id := int(createResp["id"].(float64))
	t.Logf("Создано промо ID=%d", id)

	// READ через GetPromoData с фильтром по SKU
	req2, _ := http.NewRequest("GET",
		fmt.Sprintf("/api/promo/data?all=true&sku=%s", createPayload["sku"]), nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("READ: ожидался 200, получен %d", w2.Code)
	}
	var readResp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &readResp)
	rows := readResp["data"].([]interface{})
	if len(rows) == 0 {
		t.Fatal("READ: запись не найдена после создания")
	}
	t.Logf("Прочитано %d записей по SKU", len(rows))

	// UPDATE
	updatePayload := map[string]interface{}{
		"id": id, "plan_promo_units": 1000, "status": "Согласовано",
	}
	body3, _ := json.Marshal(updatePayload)
	req3, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body3))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("UPDATE: ожидался 200, получен %d: %s", w3.Code, w3.Body.String())
	}
	var updateResp map[string]interface{}
	json.Unmarshal(w3.Body.Bytes(), &updateResp)
	if updateResp["message"] != "Updated" {
		t.Errorf("UPDATE: ожидалось 'Updated', получено '%v'", updateResp["message"])
	}
	// Проверяем пересчёт
	updatedData := updateResp["data"].(map[string]interface{})
	if math.Abs(updatedData["plan_promo_rub"].(float64)-300000) > 0.01 {
		t.Errorf("UPDATE: plan_promo_rub ожидалось 300000 (1000*300), получено %v", updatedData["plan_promo_rub"])
	}

	// DELETE
	req4, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/promo/%d", id), nil)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("DELETE: ожидался 200, получен %d", w4.Code)
	}
	t.Log("Удалено успешно")

	// VERIFY DELETION — обновление удалённой записи должно вернуть 500
	body5, _ := json.Marshal(map[string]interface{}{"id": id, "status": "После удаления"})
	req5, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body5))
	req5.Header.Set("Content-Type", "application/json")
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusInternalServerError {
		t.Errorf("VERIFY: обновление удалённой записи должно вернуть 500, получен %d", w5.Code)
	}
}

// ==================== МАТЕМАТИКА ====================

func TestROICalculation(t *testing.T) {
	upliftRub := 50000.0
	investments := 200000.0
	gm := 0.56
	expected := (upliftRub/investments)*gm*100 - 100
	roi := (upliftRub/investments)*gm*100 - 100
	if math.Abs(roi-expected) > 0.001 {
		t.Errorf("ROI: ожидалось %f, получено %f", expected, roi)
	}
}

func TestBaselineRubCalculation(t *testing.T) {
	tests := []struct {
		baseline, price, expected float64
	}{
		{100, 200, 20000}, {0, 200, 0}, {100, 0, 0}, {150.5, 300.75, 45262.875},
	}
	for _, tt := range tests {
		result := tt.baseline * tt.price
		if math.Abs(result-tt.expected) > 0.001 {
			t.Errorf("baseline_rub(%f*%f): ожидалось %f, получено %f", tt.baseline, tt.price, tt.expected, result)
		}
	}
}

func TestUpliftCalculation(t *testing.T) {
	tests := []struct{ plan, baseline, expected float64 }{
		{150, 100, 50}, {80, 100, -20}, {0, 100, -100}, {100, 0, 100},
	}
	for _, tt := range tests {
		result := tt.plan - tt.baseline
		if math.Abs(result-tt.expected) > 0.001 {
			t.Errorf("uplift(%f-%f): ожидалось %f, получено %f", tt.plan, tt.baseline, tt.expected, result)
		}
	}
}

func TestQuarterCalculation(t *testing.T) {
	tests := []struct{ month, expected int }{
		{1, 1}, {2, 1}, {3, 1}, {4, 2}, {5, 2}, {6, 2},
		{7, 3}, {8, 3}, {9, 3}, {10, 4}, {11, 4}, {12, 4},
	}
	for _, tt := range tests {
		q := int(math.Ceil(float64(tt.month) / 3))
		if q != tt.expected {
			t.Errorf("Месяц %d: ожидался квартал %d, получен %d", tt.month, tt.expected, q)
		}
	}
}

// ==================== RATE LIMITING ====================

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(5, 1*time.Second)
	for i := 0; i < 5; i++ {
		if !limiter.Allow("127.0.0.1") {
			t.Errorf("Запрос %d должен быть разрешён", i+1)
		}
	}
	if limiter.Allow("127.0.0.1") {
		t.Error("6-й запрос должен быть отклонён")
	}
	if !limiter.Allow("192.168.1.1") {
		t.Error("Запрос с другого IP должен быть разрешён")
	}
	time.Sleep(1 * time.Second)
	if !limiter.Allow("127.0.0.1") {
		t.Error("После сброса окна запрос должен быть разрешён")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := NewRateLimiter(3, 100*time.Millisecond)
	limiter.Allow("ip1")
	limiter.Allow("ip2")
	limiter.Allow("ip1")
	if len(limiter.visitors) != 2 {
		t.Errorf("Ожидалось 2 посетителя, получено %d", len(limiter.visitors))
	}
	time.Sleep(150 * time.Millisecond)
	limiter.Allow("ip1")
	if len(limiter.visitors["ip1"]) > 1 {
		t.Error("Старые записи должны были очиститься")
	}
}

func saveTestPromoWithMeta(t *testing.T, router *gin.Engine, sku string) (int, string) {
	t.Helper()
	payload := map[string]interface{}{
		"network_name": "Тест-OPTILOCK", "sku": sku, "year": 2026, "month": 5,
		"baseline_units": 100, "plan_promo_units": 200, "plan_investments_rub": 10000,
		"contract_price": 150, "mechanics": "Скидка", "gtn_opex": "GTN",
		"status": "Планируется",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["message"] != "Created" {
		t.Fatalf("Не удалось создать тестовое промо: %v", response)
	}

	id := int(response["id"].(float64))

	// Достаём реальный updated_at из БД
	var updatedAt string
	config.DB.QueryRow("SELECT CONVERT(NVARCHAR, updated_at, 121) FROM dbo.tbl_PromoActivities WHERE id = ? AND deleted_at IS NULL", id).Scan(&updatedAt)

	return id, updatedAt
}

func TestSavePromo_OptimisticLocking(t *testing.T) {
	router := setupRouter()
	id, updatedAt := saveTestPromoWithMeta(t, router, "TEST-OPTILOCK-001")

	// Первое обновление — передаём актуальный updated_at (должно пройти)
	payload1 := map[string]interface{}{
		"id": id, "status": "Первое изменение",
		"updated_at": updatedAt,
	}
	body1, _ := json.Marshal(payload1)
	req1, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		var errResp map[string]interface{}
		json.Unmarshal(w1.Body.Bytes(), &errResp)
		t.Fatalf("Первое обновление должно пройти: %v", errResp)
	}

	var resp1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	data1 := resp1["data"].(map[string]interface{})
	newUpdatedAt := data1["updated_at"]

	// Второе обновление с тем же старым updated_at — должно дать 409
	payload2 := map[string]interface{}{
		"id": id, "status": "Конфликт",
		"updated_at": updatedAt, // тот же, что и в первом запросе — уже не актуален
	}
	body2, _ := json.Marshal(payload2)
	req2, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Ожидался 409 Conflict, получен %d. Ответ: %s", w2.Code, w2.Body.String())
	} else {
		t.Logf("✅ Optimistic locking работает: первый OK с новым updated_at=%v, второй 409 со старым=%v", newUpdatedAt, updatedAt)
	}
}
```

## File: frontend/src/components/FilterPanel.jsx
```javascript
import {
  TextField, Stack, Autocomplete,
  FormControlLabel, Checkbox, ListItemText, Button, Box
} from '@mui/material';

const DEFAULT_MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 },
  { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 },
  { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 },
  { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

export default function FilterPanel({
  filters,
  filterOptions = {},
  onFiltersChange,
  onSearch,
  onReset,
  loading = false,
  extraFilters = [],
  persistFilters = false,
  onPersistChange = null,
  visibleFilters = null,
  labels = {},
}) {

  const handleTextChange = (field) => (e) => onFiltersChange({ ...filters, [field]: e.target.value });
  const handleArrayChange = (field) => (_, newValue) => onFiltersChange({ ...filters, [field]: newValue });

  const renderCheckboxOption = (props, option, { selected }) => {
    const { key, item, ...rest } = props;
    return (
      <li key={key} {...rest} style={{ padding: '2px 8px' }}>
        <Checkbox size="small" checked={selected} sx={{ mr: 1 }} />
        <ListItemText 
          primary={option?.label ?? option} 
          primaryTypographyProps={{ fontSize: 13 }} 
        />
      </li>
    );
  };

  const filterKeys = visibleFilters || Object.keys(filterOptions);

  const defaultLabels = {
    brandName: 'Бренд', brand: 'Бренд', networkName: 'Сеть', network_name: 'Сеть',
    un_rub: 'Уп/Руб', segment: 'Сегмент', channel: 'Канал',
    productName: 'Продукт', metricType: 'Показатель', sku: 'SKU',
    mechanics: 'Механика', status: 'Статус', kam: 'KAM', gtn_opex: 'GTN/OPEX',
  };

  const getLabel = (key) => labels[key] || defaultLabels[key] || key;

  return (
    <Stack spacing={1.5}>
      {/* Строка фильтров */}
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>

        {/* Годы и месяцы — один проход */}
        {extraFilters.map((filter) => {
          // Год
          if (filter.type === 'year') {
            return (
              <TextField 
                key={filter.field} 
                label={filter.label} 
                size="small" 
                type="number"
                value={filters[filter.field] || ''} 
                onChange={handleTextChange(filter.field)}
                sx={{ width: 90 }} 
                slotProps={{ htmlInput: { min: 2018, max: 2030 } }} 
              />
            );
          }

          // Месяцы
          if (filter.type === 'months') {
            const selectedMonths = filters[filter.field] || [];
            const monthOptions = filter.options || DEFAULT_MONTH_OPTIONS;
            
            const monthDisplayText = selectedMonths.length === 0 
              ? '' 
              : selectedMonths.length === 1 
                ? monthOptions.find(m => m.value === selectedMonths[0])?.label || ''
                : `Выбрано: ${selectedMonths.length}`;

            return (
              <Autocomplete 
                key={filter.field} 
                multiple 
                disableCloseOnSelect 
                size="small"
                options={monthOptions}
                getOptionLabel={(opt) => opt.label}
                isOptionEqualToValue={(opt, val) => opt.value === val?.value}
                value={monthOptions.filter(m => selectedMonths.includes(m.value))}
                onChange={(_, newVal) => {
                  const values = newVal.map(v => v.value);
                  onFiltersChange({ ...filters, [filter.field]: values });
                }}
                renderTags={() => null}
                renderOption={renderCheckboxOption}
                renderInput={(params) => (
                  <TextField 
                    {...params} 
                    label={filter.label} 
                    placeholder={monthDisplayText}
                    InputLabelProps={{ shrink: true }} 
                  />
                )}
                slotProps={{ 
                  listbox: { style: { maxHeight: 300 } }, 
                  paper: { sx: { minWidth: 300 } } 
                }}
                sx={{ minWidth: 170, '& .MuiAutocomplete-tag': { display: 'none' } }} 
              />
            );
          }

          return null;
        })}

        {/* Остальные фильтры */}
        {filterKeys.map((key) => {
          const options = filterOptions[key];
          if (!options || options.length === 0) return null;

          const selected = filters[key] || [];
          const displayText = selected.length === 0 
            ? '' 
            : selected.length === 1 
              ? selected[0] 
              : `Выбрано: ${selected.length}`;

          return (
            <Autocomplete key={key} multiple disableCloseOnSelect size="small"
              options={options} value={selected}
              onChange={handleArrayChange(key)}
              renderOption={renderCheckboxOption}
              renderTags={() => null}
              limitTags={0}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label={getLabel(key)}
                  placeholder={displayText}
                  InputLabelProps={{ shrink: true }}
                />
              )}
              slotProps={{ listbox: { style: { maxHeight: 300 } }, paper: { sx: { minWidth: 350 } } }}
              sx={{ minWidth: 170, '& .MuiAutocomplete-tag': { display: 'none' } }} />
          );
        })}
      </Stack>

      {/* Кнопки */}
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        <Button variant="contained" onClick={onSearch} disabled={loading} size="small">
          {loading ? '...' : 'Применить'}
        </Button>
        <Button variant="outlined" onClick={onReset} disabled={loading} size="small">
          Сброс
        </Button>

        {onPersistChange && (
          <FormControlLabel
            control={
              <Checkbox 
                size="small" 
                checked={persistFilters} 
                onChange={(e) => onPersistChange(e.target.checked)} 
              />
            }
            label="Сохранять" 
            sx={{ ml: 1, '& .MuiTypography-root': { fontSize: 13 } }} 
          />
        )}
      </Stack>
    </Stack>
  );
}
```

## File: frontend/src/pages/Home.jsx
```javascript
import { useNavigate } from 'react-router-dom';
import { Box, Typography, Card, CardActionArea, CardContent, Button } from '@mui/material';
import { 
  BarChart as BarChartIcon, 
  ListAlt as ListAltIcon, 
  ShoppingCart as CartIcon, 
  Refresh as RefreshIcon,
  Campaign as CampaignIcon,
  CompareArrows as CompareIcon,
} from '@mui/icons-material';

const blocks = [
  { 
    title: 'Анализ продаж', 
    path: '/sales-analysis', 
    icon: <BarChartIcon sx={{ fontSize: 48 }} />, 
    desc: 'Динамика продаж по периодам',
    color: '#6366f1',
  },
  { 
    title: 'Реестр сетей', 
    path: '/network-registry', 
    icon: <ListAltIcon sx={{ fontSize: 48 }} />, 
    desc: 'Справочник торговых сетей',
    color: '#10b981',
  },
  { 
    title: 'Интернет-продажи', 
    path: '/internet-sales', 
    icon: <CartIcon sx={{ fontSize: 48 }} />, 
    desc: 'Детализация онлайн-заказов',
    color: '#f59e0b',
  },
  { 
    title: 'Оборачиваемость', 
    path: '/turnover', 
    icon: <RefreshIcon sx={{ fontSize: 48 }} />, 
    desc: 'Анализ оборотов запасов',
    color: '#8b5cf6',
  },
  { 
    title: 'Анализ промо', 
    path: '/promo-analysis', 
    icon: <CampaignIcon sx={{ fontSize: 48 }} />, 
    desc: 'Эффективность промо-акций',
    color: '#f43f5e',
  },
  { 
    title: 'Продажи Like For Like', 
    path: '/like-for-like', 
    icon: <CompareIcon sx={{ fontSize: 48 }} />, 
    desc: 'Сравнение продаж LFL',
    color: '#0ea5e9',
  },
];

export default function Home({ onLogout }) {
  const navigate = useNavigate();

  return (
    <Box sx={{ p: { xs: 3, md: 6 }, maxWidth: 1400, mx: 'auto', w: '100%' }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
        <Box>
          <Typography variant="h3" gutterBottom>
            Аналитический портал
          </Typography>
          <Typography variant="subtitle1" color="text.secondary" sx={{ mb: 5 }}>
            Добро пожаловать. Выберите нужный раздел для начала работы.
          </Typography>
        </Box>
        {onLogout && (
          <Button 
            variant="outlined" 
            onClick={onLogout} 
            size="small"
            sx={{ mt: 1 }}
          >
            Выйти ({localStorage.getItem('username')})
          </Button>
        )}
      </Box>
      
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr' },
        gap: 3,
        justifyContent: 'center',
      }}>
        {blocks.map((block) => (
          <Card 
            key={block.path}
            elevation={1} 
            sx={{ 
              borderRadius: 4,
              transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
              border: '1px solid #f1f5f9',
              '&:hover': { 
                transform: 'translateY(-6px)',
                boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1)',
                borderColor: 'transparent'
              }
            }}
          >
            <CardActionArea 
              onClick={() => navigate(block.path)} 
              sx={{ p: 4, display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center' }}
            >
              <Box 
                sx={{ 
                  width: 80, 
                  height: 80, 
                  borderRadius: '20px', 
                  mb: 3,
                  display: 'flex', 
                  alignItems: 'center', 
                  justifyContent: 'center',
                  backgroundColor: `${block.color}15`,
                  color: block.color,
                }}
              >
                {block.icon}
              </Box>
              <Typography variant="h5" gutterBottom sx={{ fontWeight: 700 }}>
                {block.title}
              </Typography>
              <Typography variant="body1" color="text.secondary" sx={{ lineHeight: 1.6 }}>
                {block.desc}
              </Typography>
            </CardActionArea>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
```

## File: frontend/src/pages/PromoForm.jsx
```javascript
import { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Button, Stack, Box, Typography, TextField, Autocomplete, Grid, Paper, Alert, Snackbar,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow
} from '@mui/material';
import { Save as SaveIcon } from '@mui/icons-material';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { promoAPI } from '../api/promo';

const MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 }, { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

const ECOM_SEGMENT_OPTIONS = [
  'есть, не убирают из отчета',
  'есть, не убирают из отчета, засчитывается в промо',
  'есть, не убирают из отчетов, не засчитывается в промо',
  'есть, убирают из отчета',
  'нет внешнего е-ком',
  'нет данных',
];

const REQUIRED_FIELDS = [
  'network_name', 'sku', 'year', 'month', 'mechanics', 'gtn_opex', 'contract_price',
  'baseline_units', 'plan_promo_units', 'plan_investments_rub', 'id_directum', 'ds_number',
  'discount_amount', 'conditions', 'ecom_segment', 'total_pharmacies', 'promo_pharmacies'
];

const EMPTY_FORM = {
  id: null, network_name: '', kam: '', brand: '', sku: '',
  year: '', month: '', mechanics: '', gtn_opex: '', baseline_units: '',
  plan_promo_units: '', plan_investments_rub: '', contract_price: '',
  id_directum: '', ds_number: '', discount_amount: '',
  conditions: '', comments: '', ecom_segment: '',
  total_pharmacies: '', promo_pharmacies: '',
  actual_promo_sales_units: '', actual_investments: '', actual_promo_rub: '',
  actual_promo_uplift_units: '', actual_promo_uplift_rub: '',
  actual_external_ecom_units: '', actual_corrected_baseline: '',
  key_region: '', top20_segment: '',
  status: 'Планируется',
};

const fmt = (v) => {
  if (v == null || v === '') return '';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

const cleanNumber = (v) => v.replace(/\s/g, '').replace(',', '.');

const safeNumber = (val) => {
  const n = parseInt(val);
  return isNaN(n) ? null : n;
};

const safeFloatNull = (val) => {
  const n = parseFloat(val);
  return isNaN(n) ? null : n;
};

const requiredLabel = (label) => `${label} *`;

const NumberField = ({ label, value, onChange, ...props }) => (
  <TextField
    label={label}
    type="text"
    size="small"
    fullWidth
    value={value != null && value !== '' ? Number(value).toLocaleString('ru-RU') : ''}
    onChange={(e) => onChange(cleanNumber(e.target.value))}
    slotProps={{ htmlInput: { inputMode: 'decimal' } }}
    {...props}
  />
);

export default function PromoForm({ onSave }) {
  const [form, setForm] = useState({ ...EMPTY_FORM });
  const [allSkuOptions, setAllSkuOptions] = useState([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState([]);
  const [mechanicsOptions, setMechanicsOptions] = useState([]);
  const [investmentTypes, setInvestmentTypes] = useState([]);
  const [history, setHistory] = useState([]);
  const [saving, setSaving] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });
  const [lastSKUData, setLastSKUData] = useState({});

  // Загрузка справочников
  useEffect(() => {
    promoAPI.getFilters().then(data => {
      setAllSkuOptions(data.sku || []);
      setAllNetworkOptions(data.network_name || []);
      setMechanicsOptions(data.mechanics || []);
    }).catch(() => {});
    
    promoAPI.getInvestmentTypes().then(data => {
      setInvestmentTypes(data.data || []);
    }).catch(() => setInvestmentTypes(['GTN', 'GTN в ОС', 'OPEX', 'OPEX Marketing']));
  }, []);

  // При выборе SKU
  useEffect(() => {
    if (form.sku) {
      promoAPI.getSKUInfo(form.sku).then(data => {
        if (data.brand) setForm(prev => ({ ...prev, brand: data.brand }));
      }).catch(() => {});
      
      promoAPI.getLastSKUData(form.sku).then(data => setLastSKUData(data)).catch(() => {});
    }
  }, [form.sku]);

  // При выборе сети — подтягиваем KAM, регион, сегмент, аптеки
  useEffect(() => {
    if (form.network_name) {
      promoAPI.getNetworkGeo(form.network_name).then(data => {
        const updates = {};
        if (data.kam) updates.kam = data.kam;
        if (data.key_region) updates.key_region = data.key_region;
        if (data.top20_segment) updates.top20_segment = data.top20_segment;
        if (Object.keys(updates).length > 0) setForm(prev => ({ ...prev, ...updates }));
      }).catch(() => {});
      
      promoAPI.getLastNetworkData(form.network_name).then(data => {
        if (data.total_pharmacies) setForm(prev => ({ ...prev, total_pharmacies: data.total_pharmacies }));
      }).catch(() => {});
    }
  }, [form.network_name]);

  // Автозаполнение из lastSKUData
  useEffect(() => {
    if (lastSKUData.contract_price) setForm(prev => ({ ...prev, contract_price: lastSKUData.contract_price }));
  }, [lastSKUData]);

  // История
  const fetchHistory = useCallback(async () => {
    if (!form.network_name || !form.sku || !form.mechanics) return;
    try {
      const data = await promoAPI.getHistory({
        network_name: form.network_name,
        sku: form.sku,
        mechanics: form.mechanics,
      });
      setHistory(data.data || []);
    } catch (e) { setHistory([]); }
  }, [form.network_name, form.sku, form.mechanics]);

  useEffect(() => { fetchHistory(); }, [form.network_name, form.sku, form.mechanics, fetchHistory]);

  // Расчёты
  const calculated = useMemo(() => {
    const ppu = parseFloat(form.plan_promo_units) || 0;
    const cp = parseFloat(form.contract_price) || 0;
    const bu = parseFloat(form.baseline_units) || 0;
    const pir = parseFloat(form.plan_investments_rub) || 0;
    const gm = parseFloat(lastSKUData.gm) || 1;
    const month = parseInt(form.month) || 1;

    const plan_promo_rub = ppu * cp;
    const plan_promo_uplift_units = ppu - bu;
    const plan_promo_uplift_rub = plan_promo_uplift_units * cp;
    const plan_roi = pir > 0 ? ((plan_promo_uplift_rub / pir) * gm * 100 - 100) : 0;
    const promo_date = form.year && form.month ? `${form.year}-${String(month).padStart(2, '0')}-01` : '';

    return { plan_promo_rub, plan_promo_uplift_units, plan_promo_uplift_rub, plan_roi, promo_date };
  }, [form.plan_promo_units, form.contract_price, form.baseline_units, form.plan_investments_rub, form.year, form.month, lastSKUData.gm]);

  const missingFields = REQUIRED_FIELDS.filter(f => !form[f] || form[f] === '');

  const handleSave = async () => {
    if (missingFields.length > 0) {
      setSnackbar({ open: true, message: `⚠️ Заполните: ${missingFields.slice(0, 5).join(', ')}`, severity: 'warning' });
      return;
    }
    setSaving(true);
    try {
      const payload = {
        network_name: form.network_name, kam: form.kam, brand: form.brand, brand_as: form.brand,
        sku: form.sku, year: parseInt(form.year), month: parseInt(form.month),
        mechanics: form.mechanics, gtn_opex: form.gtn_opex,
        baseline_units: parseFloat(form.baseline_units),
        plan_promo_units: parseFloat(form.plan_promo_units),
        plan_promo_rub: calculated.plan_promo_rub,
        plan_promo_uplift_units: calculated.plan_promo_uplift_units,
        plan_promo_uplift_rub: calculated.plan_promo_uplift_rub,
        plan_investments_rub: parseFloat(form.plan_investments_rub) || null,
        plan_roi: calculated.plan_roi,
        contract_price: parseFloat(form.contract_price),
        id_directum: form.id_directum, ds_number: form.ds_number,
        discount_amount: parseFloat(form.discount_amount) || null,
        conditions: form.conditions, comments: form.comments ?? null,
        ecom_segment: form.ecom_segment,
        total_pharmacies: safeNumber(form.total_pharmacies),
        promo_pharmacies: safeNumber(form.promo_pharmacies),
        key_region: form.key_region || null,
        top20_segment: form.top20_segment || null,
        actual_promo_sales_units: parseFloat(form.actual_promo_sales_units) || null,
        actual_investments: parseFloat(form.actual_investments) || null,
        actual_promo_rub: parseFloat(form.actual_promo_rub) || null,
        actual_promo_uplift_units: parseFloat(form.actual_promo_uplift_units) || null,
        actual_promo_uplift_rub: parseFloat(form.actual_promo_uplift_rub) || null,
        actual_external_ecom_units: safeFloatNull(form.actual_external_ecom_units),
        actual_corrected_baseline: safeFloatNull(form.actual_corrected_baseline),
        status: form.status, date: calculated.promo_date,
      };

      await promoAPI.save(payload);
      setSnackbar({ open: true, message: '✅ Сохранено', severity: 'success' });
      if (onSave) onSave();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + err.message, severity: 'error' });
    } finally { setSaving(false); }
  };

  const handleReset = () => {
    setForm({ ...EMPTY_FORM });
    setHistory([]);
    setLastSKUData({});
  };

  const updateForm = (field) => (value) => setForm(prev => ({ ...prev, [field]: value }));

  const chartData = useMemo(() => {
    return history.map(row => ({
      period: `${row.year}-${String(row.month).padStart(2, '0')}`,
      plan: row.plan_promo_units || 0,
      fact: row.actual_promo_sales_units || 0,
    })).reverse();
  }, [history]);

  return (
    <Box sx={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
      <Grid container spacing={2} sx={{ flex: 1, overflow: 'hidden' }}>
        <Grid item xs={12} md={6} sx={{ height: '100%', overflow: 'auto' }}>
          <Paper sx={{ p: 2, height: '100%' }}>
            <Typography variant="h6" sx={{ mb: 1 }}>Новое промо</Typography>
            <Grid container spacing={1.5}>
              <Grid item xs={6}>
                <Stack spacing={1.5}>
                  <Autocomplete size="small" freeSolo options={allNetworkOptions} value={form.network_name || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, network_name: v || '', kam: '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, network_name: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Сеть')} size="small" />} />
                  <Autocomplete size="small" freeSolo options={allSkuOptions} value={form.sku || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, sku: v || '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, sku: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('SKU')} size="small" />} />
                  <Autocomplete size="small" options={MONTH_OPTIONS} getOptionLabel={o => o.label}
                    value={MONTH_OPTIONS.find(m => m.value === parseInt(form.month)) || null}
                    onChange={(_, v) => setForm(prev => ({ ...prev, month: v?.value || '' }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Месяц')} size="small" />} />
                  <TextField label={requiredLabel('Год')} type="number" size="small" fullWidth value={form.year}
                    onChange={(e) => setForm(prev => ({ ...prev, year: e.target.value }))}
                    slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />
                  <Autocomplete size="small" freeSolo options={mechanicsOptions} value={form.mechanics || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, mechanics: v || '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, mechanics: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Механика')} size="small" />} />
                  <NumberField label={requiredLabel('Сумма скидки')} value={form.discount_amount} onChange={updateForm('discount_amount')} />
                  <Autocomplete size="small" freeSolo options={investmentTypes} value={form.gtn_opex || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, gtn_opex: v || '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, gtn_opex: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Тип инвест.')} size="small" />} />
                  <Autocomplete size="small" options={ECOM_SEGMENT_OPTIONS} value={form.ecom_segment || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, ecom_segment: v || '' }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('E-com сегмент')} size="small" />} />
                  <NumberField label={requiredLabel('Аптек ТОТАЛ')} value={form.total_pharmacies} onChange={updateForm('total_pharmacies')} />
                  <NumberField label={requiredLabel('Аптек в промо')} value={form.promo_pharmacies} onChange={updateForm('promo_pharmacies')} />
                  <TextField label={requiredLabel('ID Директум')} size="small" fullWidth value={form.id_directum}
                    onChange={(e) => setForm(prev => ({ ...prev, id_directum: e.target.value }))} />
                  <TextField label={requiredLabel('№ ДС')} size="small" fullWidth value={form.ds_number}
                    onChange={(e) => setForm(prev => ({ ...prev, ds_number: e.target.value }))} />
                  <NumberField label={requiredLabel('Цена контракта')} value={form.contract_price} onChange={updateForm('contract_price')} />
                </Stack>
              </Grid>
              <Grid item xs={6}>
                <Stack spacing={1.5}>
                  <TextField label={requiredLabel('Условия')} size="small" fullWidth multiline rows={4} value={form.conditions}
                    onChange={(e) => setForm(prev => ({ ...prev, conditions: e.target.value }))} />
                  <TextField label="Комментарии" size="small" fullWidth multiline rows={3} value={form.comments}
                    onChange={(e) => setForm(prev => ({ ...prev, comments: e.target.value }))} />
                  <Paper variant="outlined" sx={{ p: 1.5, bgcolor: '#f8f9fa' }}>
                    <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>📊 Baseline и План</Typography>
                    <Stack spacing={1.5}>
                      <NumberField label={requiredLabel('Baseline (уп)')} value={form.baseline_units} onChange={updateForm('baseline_units')} />
                      <NumberField label={requiredLabel('План промо (уп)')} value={form.plan_promo_units} onChange={updateForm('plan_promo_units')} />
                      <TextField label="План (руб)" size="small" fullWidth value={fmt(calculated.plan_promo_rub)} slotProps={{ input: { readOnly: true } }} />
                      <NumberField label={requiredLabel('Инвестиции (руб)')} value={form.plan_investments_rub} onChange={updateForm('plan_investments_rub')} />
                      <TextField label="Uplift (уп)" size="small" fullWidth value={fmt(calculated.plan_promo_uplift_units)} slotProps={{ input: { readOnly: true } }} />
                      <TextField label="Uplift (руб)" size="small" fullWidth value={fmt(calculated.plan_promo_uplift_rub)} slotProps={{ input: { readOnly: true } }} />
                      <TextField label="ROI план %" size="small" fullWidth value={calculated.plan_roi.toFixed(1)} slotProps={{ input: { readOnly: true } }} />
                    </Stack>
                  </Paper>
                  <Stack direction="row" spacing={1}>
                    <Button variant="contained" startIcon={<SaveIcon />} onClick={handleSave} disabled={saving} fullWidth size="small">
                      {saving ? 'Сохранение...' : 'Сохранить'}
                    </Button>
                    <Button variant="outlined" onClick={handleReset} size="small" sx={{ minWidth: 90 }}>Сброс</Button>
                  </Stack>
                </Stack>
              </Grid>
            </Grid>
          </Paper>
        </Grid>
        <Grid item xs={12} md={6} sx={{ height: '100%' }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', gap: 1 }}>
            <Paper sx={{ p: 2, flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
                📋 История: {form.network_name || 'сеть'} / {form.sku || 'SKU'} / {form.mechanics || 'механика'}
              </Typography>
              <TableContainer sx={{ flex: 1 }}>
                <Table size="small" stickyHeader>
                  <TableHead>
                    <TableRow>
                      <TableCell>Период</TableCell>
                      <TableCell align="right">Baseline</TableCell>
                      <TableCell align="right">План (уп)</TableCell>
                      <TableCell align="right">Факт (уп)</TableCell>
                      <TableCell align="right">Uplift план</TableCell>
                      <TableCell align="right">Uplift факт</TableCell>
                      <TableCell align="right">ROI план %</TableCell>
                      <TableCell align="right">ROI факт %</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {history.map((row) => (
                      <TableRow key={row.id} hover>
                        <TableCell>{row.year}/{String(row.month).padStart(2, '0')}</TableCell>
                        <TableCell align="right">{row.baseline_units != null ? Number(row.baseline_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.plan_promo_units != null ? Number(row.plan_promo_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.actual_promo_sales_units != null ? Number(row.actual_promo_sales_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.plan_promo_uplift_units != null ? Number(row.plan_promo_uplift_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.actual_promo_uplift_units != null ? Number(row.actual_promo_uplift_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.plan_roi != null ? Number(row.plan_roi).toFixed(1) : '-'}</TableCell>
                        <TableCell align="right">{row.actual_roi != null ? Number(row.actual_roi).toFixed(1) : '-'}</TableCell>
                      </TableRow>
                    ))}
                    {history.length === 0 && (
                      <TableRow><TableCell colSpan={8} align="center">Выберите сеть, SKU и механику</TableCell></TableRow>
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>
            <Paper sx={{ p: 2, height: 250, flexShrink: 0 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>📈 План / Факт</Typography>
              {history.length > 0 ? (
                <Box sx={{ height: 190 }}>
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="period" tick={{ fontSize: 10 }} />
                      <YAxis tick={{ fontSize: 10 }} />
                      <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU')} />
                      <Legend wrapperStyle={{ fontSize: 11 }} />
                      <Bar dataKey="plan" name="План (уп)" fill="#8884d8" radius={[3, 3, 0, 0]} />
                      <Bar dataKey="fact" name="Факт (уп)" fill="#82ca9d" radius={[3, 3, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </Box>
              ) : (
                <Box sx={{ height: 190, display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: '#fafafa', borderRadius: 1 }}>
                  <Typography color="text.disabled">Нет данных</Typography>
                </Box>
              )}
            </Paper>
          </Box>
        </Grid>
      </Grid>
      <Snackbar open={snackbar.open} autoHideDuration={4000} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>{snackbar.message}</Alert>
      </Snackbar>
    </Box>
  );
}
```

## File: backend/models/types.go
```go
package models

type Row struct {
	ID          int     `json:"id"`
	Year        int     `json:"year"`
	Month       int     `json:"month"`
	BrandName   string  `json:"brandName"`
	ProductName string  `json:"productName"`
	NetworkName string  `json:"networkName"`
	MetricType  string  `json:"metricType"`
	MetricValue float64 `json:"metricValue"`
	UnRub       *string `json:"un_rub"`
	Segment     *string `json:"segment"`
	Channel     *string `json:"channel"`
	UpdatedAt   *string `json:"updated_at"`
}

type PromoRow struct {
	ID                      int      `json:"id"`
	NetworkName             *string  `json:"network_name"`
	KAM                     *string  `json:"kam"`
	IDDirectum              *string  `json:"id_directum"`
	DSNumber                *string  `json:"ds_number"`
	Year                    int      `json:"year"`
	Month                   *int     `json:"month"`
	Quarter                 *int     `json:"quarter"`
	SKU                     *string  `json:"sku"`
	Brand                   *string  `json:"brand"`
	BrandAS                 *string  `json:"brand_as"`
	Mechanics               *string  `json:"mechanics"`
	DiscountAmount          *float64 `json:"discount_amount"`
	GTNOpex                 *string  `json:"gtn_opex"`
	Conditions              *string  `json:"conditions"`
	Comments                *string  `json:"comments"`
	BaselineUnits           *float64 `json:"baseline_units"`
	BaselineRub             *float64 `json:"baseline_rub"`
	PlanPromoUnits          *float64 `json:"plan_promo_units"`
	PlanPromoRub            *float64 `json:"plan_promo_rub"`
	PlanInvestmentsRub      *float64 `json:"plan_investments_rub"`
	PlanPromoUpliftUnits    *float64 `json:"plan_promo_uplift_units"`
	PlanPromoUpliftRub      *float64 `json:"plan_promo_uplift_rub"`
	PlanPromoUpliftPctUnits *float64 `json:"plan_promo_uplift_pct_units"`
	PlanPromoUpliftPctRub   *float64 `json:"plan_promo_uplift_pct_rub"`
	PlanInvestmentsPct      *float64 `json:"plan_investments_pct"`
	PlanROI                 *float64 `json:"plan_roi"`
	ContractPrice           *float64 `json:"contract_price"`
	GM                      *float64 `json:"gm"`
	TotalPharmacies         *int     `json:"total_pharmacies"`
	PromoPharmacies         *int     `json:"promo_pharmacies"`
	ActualPromoSalesUnits   *float64 `json:"actual_promo_sales_units"`
	ActualInvestments       *float64 `json:"actual_investments"`
	Status                  *string  `json:"status"`
	ActualPromoRub          *float64 `json:"actual_promo_rub"`
	ActualPromoUpliftUnits  *float64 `json:"actual_promo_uplift_units"`
	ActualPromoUpliftRub    *float64 `json:"actual_promo_uplift_rub"`
	ActualExternalEcomUnits *float64 `json:"actual_external_ecom_units"`
	ActualCorrectedBaseline *float64 `json:"actual_corrected_baseline"`
	ActualROI               *float64 `json:"actual_roi"`
	PlanVsFactRub           *float64 `json:"plan_vs_fact_rub"`
	PlanVsFactInvestments   *float64 `json:"plan_vs_fact_investments"`
	PromoChannel            *string  `json:"channel"`
	Agreement1              *string  `json:"agreement1"`
	Agreement2              *string  `json:"agreement2"`
	Date                    *string  `json:"date"`
	CreatedAt               *string  `json:"created_at"`
	UpdatedAt               *string  `json:"updated_at"`
}

type HistoryRow struct {
	ID                     int      `json:"id"`
	NetworkName            *string  `json:"network_name"`
	Year                   int      `json:"year"`
	Month                  int      `json:"month"`
	Mechanics              *string  `json:"mechanics"`
	SKU                    *string  `json:"sku"`
	BaselineUnits          *float64 `json:"baseline_units"`
	PlanPromoUnits         *float64 `json:"plan_promo_units"`
	ActualPromoSalesUnits  *float64 `json:"actual_promo_sales_units"`
	PlanPromoUpliftUnits   *float64 `json:"plan_promo_uplift_units"`
	ActualPromoUpliftUnits *float64 `json:"actual_promo_uplift_units"`
	PlanROI                *float64 `json:"plan_roi"`
	ActualROI              *float64 `json:"actual_roi"`
}

type DrilldownRow struct {
	Year       int     `json:"year"`
	Month      int     `json:"month"`
	MetricType string  `json:"metricType"`
	TotalValue float64 `json:"totalValue"`
	UnRub      *string `json:"un_rub"`
	Segment    *string `json:"segment"`
	Channel    *string `json:"channel"`
}

type NetworkGeo struct {
	KAM          string `json:"kam"`
	NetworkType  string `json:"network_type"`
	Top20Segment string `json:"top20_segment"`
	KeyRegion    string `json:"key_region"`
}

type LastSKUData struct {
	ContractPrice   float64 `json:"contract_price"`
	GM              float64 `json:"gm"`
	TotalPharmacies int64   `json:"total_pharmacies"`
	KeyRegion       string  `json:"key_region"`
	Top20Segment    string  `json:"top20_segment"`
	OlapPrice       float64 `json:"olap_price"`
}

type ApprovalRow struct {
	ID                    int      `json:"id"`
	NetworkName           *string  `json:"network_name"`
	BrandAS               *string  `json:"brand_as"`
	SKU                   *string  `json:"sku"`
	Mechanics             *string  `json:"mechanics"`
	Year                  int      `json:"year"`
	Month                 *int     `json:"month"`
	BaselineUnits         *float64 `json:"baseline_units"`
	PlanPromoUnits        *float64 `json:"plan_promo_units"`
	ActualPromoSalesUnits *float64 `json:"actual_promo_sales_units"`
	PlanInvestmentsRub    *float64 `json:"plan_investments_rub"`
	PlanROI               *float64 `json:"plan_roi"`
	ActualROI             *float64 `json:"actual_roi"`
	Conditions            *string  `json:"conditions"`
	Agreement1            *string  `json:"agreement1"`         // обратная совместимость
	Agreement1Status      *string  `json:"agreement1_status"`  // pending/approved/rejected/commented
	Agreement1Comment     *string  `json:"agreement1_comment"` // текст комментария
	Agreement2            *string  `json:"agreement2"`         // обратная совместимость
	Agreement2Status      *string  `json:"agreement2_status"`
	Agreement2Comment     *string  `json:"agreement2_comment"`
	Status                *string  `json:"status"`
	HistoricalCount       int      `json:"historical_count"`
	AvgHistoricalROI      *float64 `json:"avg_historical_roi"`
}
```

## File: backend/go.mod
```
module backend

go 1.25.0

require (
	github.com/denisenkom/go-mssqldb v0.12.3
	github.com/gin-contrib/cors v1.7.7
	github.com/gin-gonic/gin v1.12.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/joho/godotenv v1.5.1
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	golang.org/x/arch v0.23.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
```

## File: frontend/src/components/DataTable.jsx
```javascript
import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { DataGrid } from '@mui/x-data-grid';
import { 
  Box, Alert, TextField, Button, Menu, MenuItem, 
  Checkbox, ListItemText, Typography, Divider 
} from '@mui/material';
import { 
  ViewColumn as ColumnsIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
} from '@mui/icons-material';

export default function DataTable({ 
  columns, apiUrl, filters = {}, defaultPageSize = 100, 
  exportFileName = 'export', onDataLoaded = null, 
  onRowClick = null, refreshKey 
}) {
  const [rawRows, setRawRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const filtersKey = useMemo(() => JSON.stringify(filters), [filters]);

  // Серверная пагинация
  const [paginationModel, setPaginationModel] = useState({
    page: 0,
    pageSize: defaultPageSize,
  });
  const [totalRows, setTotalRows] = useState(0);

  // Тулбар
  const [searchText, setSearchText] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const map = {};
    columns.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  // Уникальные ключи
  const rows = useMemo(() => {
    return rawRows.map((row, idx) => ({
      ...row,
      _rowId: `${row.id ?? 'row'}_${paginationModel.page}_${idx}`,
    }));
  }, [rawRows, paginationModel.page]);

  // Клиентский поиск (по текущей странице)
  const filteredRows = useMemo(() => {
    if (!searchText.trim()) return rows;
    const lower = searchText.toLowerCase();
    return rows.filter(row =>
      Object.values(row).some(val =>
        val != null && String(val).toLowerCase().includes(lower)
      )
    );
  }, [rows, searchText]);

  const visibleCols = useMemo(
    () => columns.filter(c => visibleColumns[c.field] !== false),
    [columns, visibleColumns]
  );

  const toggleColumn = (field) => {
    setVisibleColumns(prev => ({ ...prev, [field]: !prev[field] }));
  };

  const showAllColumns = () => {
    const map = {};
    columns.forEach(c => { map[c.field] = true; });
    setVisibleColumns(map);
  };

  const hideAllColumns = () => {
    const map = {};
    columns.forEach(c => { map[c.field] = false; });
    setVisibleColumns(map);
  };

  const handleExport = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.set('all', 'true');
      Object.entries(filters).forEach(([key, value]) => { 
        if (Array.isArray(value)) { 
          value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); }); 
        } else if (value !== '' && value != null) { 
          params.set(key, String(value)); 
        } 
      });
      const url = `${apiUrl}?${params.toString()}`;
      const response = await fetch(url, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json();
      const data = json.data || [];

      const headers = visibleCols.map(c => c.headerName || c.field);
      const fields = visibleCols.map(c => c.field);

      let csv = '\uFEFF' + headers.join(';') + '\n';
      data.forEach(row => {
        const line = fields.map(f => {
          let val = row[f];
          if (val == null) return '';
          val = String(val);
          if (val.includes(';') || val.includes('"') || val.includes('\n')) {
            val = '"' + val.replace(/"/g, '""') + '"';
          }
          return val;
        }).join(';');
        csv += line + '\n';
      });

      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `${exportFileName}.csv`;
      link.click();
      URL.revokeObjectURL(link.href);
    } catch (err) {
      console.error('Ошибка экспорта:', err);
    } finally {
      setLoading(false);
    }
  };

  // Загрузка данных с пагинацией
  const fetchData = useCallback(async () => {
    setLoading(true); setError(null);
    try {
      const params = new URLSearchParams();
      params.set('page', String(paginationModel.page));
      params.set('pageSize', String(paginationModel.pageSize));
      
      Object.entries(filters).forEach(([key, value]) => { 
        if (Array.isArray(value)) { 
          value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); }); 
        } else if (value !== '' && value != null) { 
          params.set(key, String(value)); 
        } 
      });
      const qs = params.toString();
      const url = `${apiUrl}?${qs}`;
      const response = await fetch(url, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json();
      const data = json.data || [];
      setRawRows(data);
      setTotalRows(json.totalRows || data.length);
      if (onDataLoaded) onDataLoaded(data);
    } catch (err) { setError(err.message); } finally { setLoading(false); }
  }, [apiUrl, filtersKey, paginationModel.page, paginationModel.pageSize]);

  // Сброс страницы при смене фильтров
  useEffect(() => {
    setPaginationModel(prev => ({ ...prev, page: 0 }));
  }, [filtersKey]);

  useEffect(() => { fetchData(); }, [fetchData, refreshKey]);

  if (error) return <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>Ошибка загрузки: {error}</Alert>;

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%' }}>
      
      {/* Тулбар */}
      <Box sx={{ 
        display: 'flex', alignItems: 'center', gap: 1, 
        px: 2, py: 1, bgcolor: '#f1f5f9', 
        borderRadius: '12px 12px 0 0',
        border: '1px solid #e2e8f0', borderBottom: 'none',
      }}>
        <Button size="small" startIcon={<ColumnsIcon />}
          onClick={(e) => setColumnsAnchor(e.currentTarget)}
          sx={{ color: '#475569', fontWeight: 500 }}>Колонки</Button>
        <Menu anchorEl={columnsAnchor} open={Boolean(columnsAnchor)}
          onClose={() => setColumnsAnchor(null)}
          slotProps={{ paper: { sx: { maxHeight: 400, minWidth: 220 } } }}>
          <MenuItem dense onClick={showAllColumns}>
            <Typography variant="caption" color="primary" sx={{ fontWeight: 600 }}>Показать все</Typography></MenuItem>
          <MenuItem dense onClick={hideAllColumns}>
            <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>Скрыть все</Typography></MenuItem>
          <Divider />
          {columns.map(col => (
            <MenuItem key={col.field} dense onClick={() => toggleColumn(col.field)}>
              <Checkbox size="small" checked={visibleColumns[col.field] !== false} />
              <ListItemText primary={col.headerName || col.field} primaryTypographyProps={{ fontSize: 13 }} />
            </MenuItem>
          ))}
        </Menu>

        <TextField size="small" placeholder="Поиск по странице..." value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          InputProps={{ startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> }}
          sx={{ width: 240, '& .MuiOutlinedInput-root': { bgcolor: '#fff', borderRadius: 2 }, '& .MuiInputBase-input': { fontSize: '0.875rem', py: 0.75 } }} />

        <Box sx={{ flex: 1 }} />

        {totalRows > 0 && (
          <Typography variant="caption" color="text.secondary" sx={{ mr: 1 }}>
            {totalRows.toLocaleString('ru-RU')} строк
          </Typography>
        )}

        <Button size="small" startIcon={<ExportIcon />} onClick={handleExport}
          sx={{ color: '#475569', fontWeight: 500 }}>CSV</Button>
      </Box>

      {/* Таблица */}
      <DataGrid 
        apiRef={apiRef}
        rows={filteredRows} 
        columns={visibleCols}
        getRowId={(row) => row._rowId}
        loading={loading} 
        sortingMode="server"
        paginationMode="server"
        rowCount={totalRows}
        paginationModel={paginationModel}
        onPaginationModelChange={setPaginationModel}
        disableColumnFilter
        onRowClick={onRowClick}
        pageSizeOptions={[25, 50, 100]} 
        disableRowSelectionOnClick
        sx={{ 
          flex: 1, border: '1px solid #e2e8f0', borderTop: 'none',
          borderRadius: '0 0 12px 12px',
          '& .MuiDataGrid-columnHeaders': { borderRadius: 0 },
          '& .MuiDataGrid-row': { cursor: onRowClick ? 'pointer' : 'default' } 
        }} 
      />
    </Box>
  );
}
```

## File: frontend/src/hooks/usePromoData.js
```javascript
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useCallback } from 'react';

/**
 * Хук для получения данных промо с использованием React Query.
 * Заменяет ручной AbortController, JSON.stringify сравнение фильтров,
 * и state-машину loading/error.
 *
 * Возвращает совместимый интерфейс: { rows, setRows, loading, error, refetch }
 * чтобы не ломать PromoAnalysis.jsx.
 */
export function usePromoData(filters, refreshTrigger) {
  const queryClient = useQueryClient();
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  // Стабильный queryKey на основе фильтров и refreshTrigger
  const queryKey = ['promoData', filters, refreshTrigger];

  const fetchPromoData = useCallback(async () => {
    const currentFilters = filtersRef.current;
    const params = new URLSearchParams();
    Object.entries(currentFilters).forEach(([key, value]) => {
      if (Array.isArray(value)) {
        value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
      } else if (value !== '' && value != null) {
        params.set(key, String(value));
      }
    });

    const qs = params.toString();
    const response = await fetch(
      `http://localhost:8080/api/promo/data?all=true${qs ? '&' + qs : ''}`,
      {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      }
    );

    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const json = await response.json();
    return json.data || [];
  }, []);

  const { data: rows = [], isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: fetchPromoData,
  });

  // setRows — для обратной совместимости: обновляет кеш React Query
  const setRows = useCallback((newRowsOrUpdater) => {
    queryClient.setQueryData(queryKey, (old) => {
      if (typeof newRowsOrUpdater === 'function') {
        return newRowsOrUpdater(old || []);
      }
      return newRowsOrUpdater;
    });
  }, [queryClient, queryKey]);

  return {
    rows,
    setRows,
    loading: isLoading,
    error: error?.message || null,
    refetch,
  };
}
```

## File: frontend/src/hooks/usePromoForm.js
```javascript
import { useState, useCallback } from 'react';
import { promoAPI } from '../api/promo';

// ─── Пустая форма ──────────────────────────────────────────────────────────
const EMPTY_FORM = {
  id: null, network_name: '', kam: '', brand: '', sku: '',
  year: new Date().getFullYear(), month: new Date().getMonth() + 1,
  mechanics: '', gtn_opex: '', baseline_units: '', baseline_rub: '',
  plan_promo_units: '', plan_promo_rub: '', plan_promo_uplift_units: '',
  plan_promo_uplift_rub: '', plan_investments_rub: '', contract_price: '',
  plan_roi: '', gm: '', discount_amount: '',
  actual_promo_sales_units: '', actual_investments: '', actual_promo_rub: '',
  actual_promo_uplift_units: '', actual_promo_uplift_rub: '', actual_roi: '',
  actual_external_ecom_units: '', actual_corrected_baseline: '',
  agreement1: '', agreement2: '',
  conditions: '', comments: '',
  id_directum: '', ds_number: '',
  total_pharmacies: '', promo_pharmacies: '',
  status: '',
  updated_at: null, // ← для optimistic locking
};

// ─── Хук ────────────────────────────────────────────────────────────────────
// Колбэки:
//   onEditSuccess(id, data) — после успешного редактирования
//   onDeleteSuccess(id)     — после успешного удаления
//   onCreateSuccess()       — после создания нового промо
export function usePromoForm({ onEditSuccess, onDeleteSuccess, onCreateSuccess }) {
  const [form, setForm] = useState({ ...EMPTY_FORM });
  const [editMode, setEditMode] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // ─── Загрузка строки в форму (клик по таблице) ──────────────────────────
  const handleRowClick = useCallback((row) => {
    setForm({
      id: row.id,
      network_name: row.network_name ?? '',
      kam: row.kam ?? '',
      brand: row.brand_as ?? row.brand ?? '',
      sku: row.sku ?? '',
      year: row.year,
      month: row.month,
      mechanics: row.mechanics ?? '',
      gtn_opex: row.gtn_opex ?? '',
      baseline_units: row.baseline_units ?? '',
      baseline_rub: row.baseline_rub ?? '',
      plan_promo_units: row.plan_promo_units ?? '',
      plan_promo_rub: row.plan_promo_rub ?? '',
      plan_promo_uplift_units: row.plan_promo_uplift_units ?? '',
      plan_promo_uplift_rub: row.plan_promo_uplift_rub ?? '',
      plan_investments_rub: row.plan_investments_rub ?? '',
      contract_price: row.contract_price ?? '',
      discount_amount: row.discount_amount ?? '',
      plan_roi: row.plan_roi ?? '',
      gm: row.gm ?? '',
      total_pharmacies: row.total_pharmacies ?? '',
      promo_pharmacies: row.promo_pharmacies ?? '',
      actual_promo_sales_units: row.actual_promo_sales_units ?? '',
      actual_investments: row.actual_investments ?? '',
      actual_promo_rub: row.actual_promo_rub ?? '',
      actual_promo_uplift_units: row.actual_promo_uplift_units ?? '',
      actual_promo_uplift_rub: row.actual_promo_uplift_rub ?? '',
      actual_roi: row.actual_roi ?? '',
      actual_external_ecom_units: row.actual_external_ecom_units ?? '',
      actual_corrected_baseline: row.actual_corrected_baseline ?? '',
      agreement1: row.agreement1 ?? '',
      agreement2: row.agreement2 ?? '',
      conditions: row.conditions ?? '',
      comments: row.comments ?? '',
      id_directum: row.id_directum ?? '',
      ds_number: row.ds_number ?? '',
      status: row.status ?? '',
      updated_at: row.updated_at ?? null, // ← критично для optimistic locking
    });
    setEditMode(true);
  }, []);

  // ─── Сохранение (INSERT или UPDATE) ─────────────────────────────────────
  const handleSave = useCallback(async () => {
    if (!form.sku || !form.network_name) {
      return { success: false, message: '⚠️ Заполните Сеть и SKU' };
    }
    setSaving(true);
    try {
      const payload = {
        id: form.id || undefined,
        network_name: form.network_name, kam: form.kam, brand: form.brand, brand_as: form.brand,
        sku: form.sku, year: parseInt(form.year), month: parseInt(form.month),
        mechanics: form.mechanics, gtn_opex: form.gtn_opex,
        baseline_units: parseFloat(form.baseline_units) || null,
        plan_promo_units: parseFloat(form.plan_promo_units) || null,
        plan_investments_rub: parseFloat(form.plan_investments_rub) || null,
        contract_price: parseFloat(form.contract_price) || null,
        gm: parseFloat(form.gm) || null,
        id_directum: form.id_directum ?? null,
        ds_number: form.ds_number ?? null,
        discount_amount: parseFloat(form.discount_amount) || null,
        conditions: form.conditions ?? null,
        comments: form.comments ?? null,
        ecom_segment: form.ecom_segment,
        total_pharmacies: form.total_pharmacies !== '' ? parseInt(form.total_pharmacies) : null,
        promo_pharmacies: form.promo_pharmacies !== '' ? parseInt(form.promo_pharmacies) : null,
        actual_promo_sales_units: parseFloat(form.actual_promo_sales_units) || null,
        actual_investments: parseFloat(form.actual_investments) || null,
        actual_promo_rub: parseFloat(form.actual_promo_rub) || null,
        actual_promo_uplift_units: parseFloat(form.actual_promo_uplift_units) || null,
        actual_promo_uplift_rub: parseFloat(form.actual_promo_uplift_rub) || null,
        actual_external_ecom_units: form.actual_external_ecom_units !== '' ? parseFloat(form.actual_external_ecom_units) : null,
        actual_corrected_baseline: form.actual_corrected_baseline !== '' ? parseFloat(form.actual_corrected_baseline) : null,
        agreement1: form.agreement1 ?? null,
        agreement2: form.agreement2 ?? null,
        status: form.status,
        updated_at: form.updated_at, // ← для optimistic locking
      };

      const result = await promoAPI.save(payload);

      if (result.data) {
        setForm(prev => ({ ...prev, ...result.data, id: result.id }));
      }

      if (form.id && onEditSuccess && result.data) {
        onEditSuccess(form.id, result.data);
      } else if (!form.id && onCreateSuccess) {
        onCreateSuccess();
      }

      return { success: true, message: '✅ Сохранено' };
    } catch (err) {
      // 409 — конфликт версий (optimistic locking)
      if (err.status === 409) {
        return { success: false, message: '⚠️ Запись изменена другим пользователем. Обновите страницу.' };
      }
      return { success: false, message: '❌ ' + (err.message || 'Ошибка сохранения') };
    } finally {
      setSaving(false);
    }
  }, [form, onEditSuccess, onCreateSuccess]);

  // ─── Удаление (soft-delete) ─────────────────────────────────────────────
  const handleDelete = useCallback(async () => {
    if (!form.id) return { success: false, message: 'Нет ID' };
    setDeleting(true);
    try {
      await promoAPI.delete(form.id);
      if (onDeleteSuccess) onDeleteSuccess(form.id);
      return { success: true, message: '🗑️ Удалено' };
    } catch (err) {
      return { success: false, message: '❌ ' + (err.message || 'Ошибка удаления') };
    } finally {
      setDeleting(false);
    }
  }, [form.id, onDeleteSuccess]);

  // ─── Сброс формы ────────────────────────────────────────────────────────
  const resetForm = useCallback(() => {
    setForm({ ...EMPTY_FORM });
    setEditMode(false);
  }, []);

  return {
    form, setForm, editMode, saving, deleting,
    handleRowClick, handleSave, handleDelete, resetForm
  };
}

export { EMPTY_FORM };
```

## File: frontend/src/App.jsx
```javascript
import { useState } from 'react';
import { Routes, Route, useNavigate } from 'react-router-dom';
import { createTheme, ThemeProvider, CssBaseline, Box, Typography, Button } from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import Login from './pages/Login';
import Home from './pages/Home';
import InternetSales from './pages/InternetSales';
import PromoAnalysis from './pages/PromoAnalysis';
import { getToken, logout } from './api/auth';

const modernTheme = createTheme({
  palette: {
    mode: 'light',
    primary: { 
      main: '#6366f1',
      light: '#818cf8',
      dark: '#4f46e5',
      contrastText: '#ffffff',
    },
    background: { default: '#f8fafc', paper: '#ffffff' },
    text: { primary: '#0f172a', secondary: '#64748b' },
    divider: '#e2e8f0',
  },
  typography: {
    fontFamily: '"Inter", "Helvetica", "Arial", sans-serif',
    fontSize: 14,
    h3: { fontWeight: 700, letterSpacing: '-0.02em', color: '#0f172a' },
    h5: { fontWeight: 600, letterSpacing: '-0.01em', color: '#0f172a' },
    h6: { fontWeight: 600, letterSpacing: '-0.01em', color: '#0f172a' },
    subtitle1: { fontWeight: 600 },
    subtitle2: { fontWeight: 600 },
    button: { textTransform: 'none', fontWeight: 600, letterSpacing: '0em' },
  },
  shape: { borderRadius: 12 },
  components: {
    MuiButton: {
      styleOverrides: {
        root: {
          borderRadius: 10,
          padding: '8px 20px',
          boxShadow: 'none',
          transition: 'all 0.2s ease-in-out',
          '&:hover': { boxShadow: 'none', transform: 'translateY(-1px)' }
        },
        contained: {
          boxShadow: '0 4px 6px -1px rgba(99, 102, 241, 0.2), 0 2px 4px -2px rgba(99, 102, 241, 0.2)',
          '&:hover': { boxShadow: '0 10px 15px -3px rgba(99, 102, 241, 0.3), 0 4px 6px -4px rgba(99, 102, 241, 0.3)' }
        }
      }
    },
    MuiPaper: {
      styleOverrides: {
        root: { backgroundImage: 'none' },
        elevation1: { boxShadow: '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1)' },
        elevation2: { boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1)' },
        outlined: { borderColor: '#e2e8f0', borderRadius: 16 },
      }
    },
    MuiInputLabel: {
      styleOverrides: {
        root: {
          color: '#475569',
          '&.Mui-focused': { color: '#6366f1' },
        },
        shrink: { color: '#6366f1' },
      },
    },
    MuiTextField: { defaultProps: { size: 'small' } },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          borderRadius: 8,
          backgroundColor: '#ffffff',
          transition: 'all 0.2s',
          '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: '#94a3b8' },
          '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderWidth: '1px' },
        },
        notchedOutline: { borderColor: '#cbd5e1' },
      }
    },
    MuiDataGrid: {
      styleOverrides: {
        root: {
          border: 'none',
          backgroundColor: '#ffffff',
          boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.05)',
          borderRadius: 16,
          overflow: 'hidden',
          '& .MuiDataGrid-columnHeaders': { backgroundColor: '#f1f5f9', borderBottom: '1px solid #e2e8f0' },
          '& .MuiDataGrid-cell': { borderBottom: '1px solid #f8fafc' },
          '& .MuiDataGrid-row:hover': { backgroundColor: '#f1f5f9' },
          '& .MuiDataGrid-columnSeparator': { display: 'none' },
        },
      },
    },
    MuiDialog: {
      styleOverrides: {
        paper: { borderRadius: 20, boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)' }
      }
    },
    MuiTabs: {
      styleOverrides: { indicator: { height: 3, borderRadius: '3px 3px 0 0' } }
    },
    MuiTab: {
      styleOverrides: { root: { textTransform: 'none', fontWeight: 600, fontSize: '0.95rem' } }
    }
  },
});

function PlaceholderPage({ title, description }) {
  const navigate = useNavigate();

  return (
    <Box sx={{ p: 4 }}>
      <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')} sx={{ mb: 4 }}>
        На главную
      </Button>
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '50vh', textAlign: 'center' }}>
        <Typography variant="h3" gutterBottom sx={{ fontWeight: 700, color: 'text.secondary' }}>{title}</Typography>
        <Typography variant="h6" color="text.secondary" sx={{ mb: 3 }}>{description}</Typography>
        <Typography variant="body1" color="text.disabled">🚧 Раздел находится в разработке</Typography>
      </Box>
    </Box>
  );
}

export default function App() {
  const [auth, setAuth] = useState(() => ({
    token: getToken(),
    username: localStorage.getItem('username'),
    role: localStorage.getItem('role'),
  }));

  const handleLogin = (data) => {
    setAuth({ token: data.token, username: data.username, role: data.role });
  };

  const handleLogout = () => {
    logout();
    setAuth({ token: null, username: null, role: null });
  };

  if (!auth.token) {
    return (
      <ThemeProvider theme={modernTheme}>
        <CssBaseline />
        <Login onLogin={handleLogin} />
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={modernTheme}>
      <CssBaseline />
      <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100vh', bgcolor: 'background.default' }}>
        <Routes>
          <Route path="/" element={<Home onLogout={handleLogout} />} />
          <Route path="/internet-sales" element={<InternetSales />} />
          <Route path="/promo-analysis" element={<PromoAnalysis role={auth.role} />} />
          <Route path="/sales-analysis" element={<PlaceholderPage title="Анализ продаж" description="Динамика продаж по периодам" />} />
          <Route path="/network-registry" element={<PlaceholderPage title="Реестр сетей" description="Справочник торговых сетей" />} />
          <Route path="/turnover" element={<PlaceholderPage title="Оборачиваемость" description="Анализ оборотов запасов" />} />
          <Route path="/like-for-like" element={<PlaceholderPage title="Продажи Like For Like" description="Сравнение продаж LFL" />} />
        </Routes>
      </Box>
    </ThemeProvider>
  );
}
```

## File: frontend/src/components/PromoEditDialog.jsx
```javascript
import { useState, useEffect } from 'react';
import {
  Button, Box, Typography, TextField, Grid, Paper, Dialog, DialogTitle,
  DialogContent, DialogActions, IconButton, MenuItem, Tooltip, Chip
} from '@mui/material';
import { Save as SaveIcon, Close as CloseIcon, Delete as DeleteIcon } from '@mui/icons-material';
import { promoAPI } from '../api/promo';

// ─── Месяцы ────────────────────────────────────────────────────────────────
const MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 }, { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

// ─── Форматирование ────────────────────────────────────────────────────────
const fmtDisplay = (v) => {
  if (v == null || v === '') return '';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

// ─── Чип статуса согласования (для KAM — компактный, с Tooltip и скроллингом) ─
const AgreementChip = ({ label, value }) => {
  const text = value || '';
  if (!text || text === '0') return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
      <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.2 }}>{label}</Typography>
      <Chip label="Ожидает" size="small" variant="outlined" sx={{ borderColor: '#94a3b8', color: '#64748b', fontWeight: 500, height: 28 }} />
    </Box>
  );

  const lower = text.toLowerCase();
  const isApproved = lower.startsWith('согласовано');
  const isRejected = lower.startsWith('отклонено');
  const color = isApproved ? '#16a34a' : isRejected ? '#dc2626' : '#6366f1';
  const bg = isApproved ? '#f0fdf4' : isRejected ? '#fef2f2' : '#eef2ff';
  const shortLabel = isApproved ? '✓ Согласовано' : isRejected ? '✗ Отклонено' : '💬 Комментарий';

  const chip = (
    <Chip
      label={shortLabel}
      size="small"
      variant="filled"
      sx={{
        bgcolor: bg, color, fontWeight: 600, height: 28, maxWidth: '100%',
        '& .MuiChip-label': { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
      }}
    />
  );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.2 }}>{label}</Typography>
      <Tooltip title={text} arrow placement="top" slotProps={{ tooltip: { sx: { maxWidth: 320, whiteSpace: 'pre-wrap', wordBreak: 'break-word' } } }}>
        {chip}
      </Tooltip>
    </Box>
  );
};

// ─── Подсветка согласований ────────────────────────────────────────────────
const renderAgreementSx = (value) => {
  if (value == null || value === '' || value === '0') return {};
  const v = String(value).toLowerCase();
  if (v.startsWith('согласовано')) return { 
    '& .MuiOutlinedInput-input': { color: '#16a34a', fontWeight: 600 },
    '& .MuiOutlinedInput-notchedOutline': { borderColor: '#16a34a' },
    '& .MuiInputBase-root': { bgcolor: '#f0fdf4' },
  };
  if (v.startsWith('отклонено')) return { 
    '& .MuiOutlinedInput-input': { color: '#dc2626', fontWeight: 600 },
    '& .MuiOutlinedInput-notchedOutline': { borderColor: '#dc2626' },
    '& .MuiInputBase-root': { bgcolor: '#fef2f2' },
  };
  return { 
    '& .MuiOutlinedInput-input': { color: '#6366f1', fontStyle: 'italic' },
    '& .MuiInputBase-root': { bgcolor: '#eef2ff' },
  };
};

// ─── Компонент ─────────────────────────────────────────────────────────────
export default function PromoEditDialog({
  open, onClose, form, setForm, recalcPlan, recalcActual,
  onSave, onDelete, saving, deleting,
  meta, allSkuOptions, allNetworkOptions, investmentTypes,
  role,
}) {
  const [editingFields, setEditingFields] = useState({});

  const fetchSKUInfoForEdit = async (sku) => {
    try { const data = await promoAPI.getSKUInfo(sku); if (data.brand) setForm(prev => ({ ...prev, brand: data.brand })); } catch (e) {}
  };

  useEffect(() => { setEditingFields({}); }, [open]);

  if (!form) return null;

  const updateField = (field) => (e) => setForm(prev => ({ ...prev, [field]: e.target.value }));

  const planTriggers = ['baseline_units', 'plan_promo_units', 'contract_price', 'plan_investments_rub'];
  const actualTriggers = ['actual_promo_sales_units', 'actual_investments', 'actual_promo_uplift_units'];
  const textFields = ['network_name', 'kam', 'brand', 'sku', 'mechanics', 'gtn_opex', 'conditions', 'comments', 'ecom_segment', 'status', 'id_directum', 'ds_number'];

  const handleFieldChange = (field) => (e) => {
    const rawValue = e.target.value;
    const cleanValue = rawValue.replace(/\s/g, '').replace(',', '.');
    if (planTriggers.includes(field)) {
      setForm(prev => { const calc = recalcPlan({ [field]: cleanValue }); return { ...prev, [field]: cleanValue, ...calc }; });
    } else if (actualTriggers.includes(field)) {
      setForm(prev => { const calc = recalcActual({ [field]: cleanValue }); return { ...prev, [field]: cleanValue, ...calc }; });
    } else {
      setForm(prev => ({ ...prev, [field]: textFields.includes(field) ? rawValue : cleanValue }));
    }
  };

  const handleFocus = (field) => () => setEditingFields(prev => ({ ...prev, [field]: true }));
  const handleBlur = (field) => () => setEditingFields(prev => ({ ...prev, [field]: false }));

  const getDisplayValue = (field, editable) => {
    if (!editable) return form[field] != null ? fmtDisplay(form[field]) : '';
    if (editingFields[field]) return form[field] != null ? String(form[field]) : '';
    return form[field] != null ? fmtDisplay(form[field]) : '';
  };

  const isKAM = role === 'agreement1' || role === 'agreement2';

  return (
    <Dialog 
      open={open} 
      onClose={onClose} 
      maxWidth="lg" 
      fullWidth 
      // Растягиваем окно почти на всю высоту экрана
      PaperProps={{ sx: { height: '96vh', maxHeight: '96vh', bgcolor: '#f5f7fa' } }}
    >
      
      {/* Чуть уменьшили отступы (py: 1.5) в шапке */}
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', bgcolor: '#ffffff', py: 1.5, px: 3 }}>
        <Typography component="span" sx={{ fontSize: '1.25rem', fontWeight: 600 }}>
          Редактирование: {form.network_name || 'Промо'}
        </Typography>
        <IconButton onClick={onClose} size="small"><CloseIcon /></IconButton>
      </DialogTitle>
  
      {/* Уменьшили внутренний отступ формы (p: 2 вместо 3) */}
      <DialogContent dividers sx={{ p: 2, overflow: 'auto' }}>
        
        {/* Уменьшили расстояние между тремя блоками (gap: 1.5 вместо 3) */}
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
  
          {(() => {
            const gridStyles = { 
              display: 'grid', 
              gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', 
              gap: 1.5, // Уменьшили расстояние между полями (было 2.5)
            };
  
            const paperStyles = {
              p: 2, // Уменьшили отступы внутри белых блоков (было 3)
              borderRadius: 2, 
              boxShadow: '0 2px 8px rgba(0,0,0,0.05)',
            };
  
            // Стиль для заголовков блоков (mb: 1.5 вместо 2.5)
            const titleStyles = { fontWeight: 600, mb: 1.5 };
  
            return (
              <>
                {/* ─── Блок 1: Основные данные ──────────────────── */}
                <Paper sx={{ ...paperStyles, bgcolor: '#ffffff' }}>
                  <Typography variant="subtitle1" sx={{ ...titleStyles }}>📋 Основные данные</Typography>
                  
                  <Box sx={gridStyles}>
                    <TextField label="ID Директум" size="small" fullWidth value={form.id_directum || ''} onChange={updateField('id_directum')} />
                    <TextField label="№ ДС" size="small" fullWidth value={form.ds_number || ''} onChange={updateField('ds_number')} />
                    <TextField select size="small" fullWidth label="Месяц" value={form.month || ''} onChange={updateField('month')}>
                      {MONTH_OPTIONS.map(m => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
                    </TextField>
                    <TextField label="Год" type="number" size="small" fullWidth value={form.year || ''} onChange={updateField('year')} slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />
  
                    <TextField select size="small" fullWidth label="SKU" value={form.sku || ''}
                      onChange={(e) => { const v = e.target.value; setForm(prev => ({ ...prev, sku: v })); if (v) fetchSKUInfoForEdit(v); }}>
                      {allSkuOptions.map(s => <MenuItem key={s} value={s}>{s}</MenuItem>)}
                    </TextField>
                    <TextField select size="small" fullWidth label="Механика" value={form.mechanics || ''} onChange={updateField('mechanics')}>
                      {meta.mechanics?.map(m => <MenuItem key={m} value={m}>{m}</MenuItem>)}
                    </TextField>
                    <TextField select size="small" fullWidth label="Тип инвест." value={form.gtn_opex || ''} onChange={updateField('gtn_opex')}>
                      {investmentTypes.map(t => <MenuItem key={t} value={t}>{t}</MenuItem>)}
                    </TextField>
                    <TextField select size="small" fullWidth label="Статус" value={form.status || ''} onChange={updateField('status')}>
                      {(() => { const opts = [...(meta.status || [])]; if (form.status && !opts.includes(form.status)) opts.push(form.status); return opts.map(s => <MenuItem key={s} value={s}>{s}</MenuItem>); })()}
                    </TextField>
  
                    <TextField label="Аптек ТОТАЛ" type="number" size="small" fullWidth value={form.total_pharmacies || ''} onChange={updateField('total_pharmacies')} slotProps={{ htmlInput: { min: 0 } }} />
                    <TextField label="Аптек в промо" type="number" size="small" fullWidth value={form.promo_pharmacies || ''} onChange={updateField('promo_pharmacies')} slotProps={{ htmlInput: { min: 0 } }} />
                    <AgreementChip label="Согласование 1" value={form.agreement1} />
                    <AgreementChip label="Согласование 2" value={form.agreement2} />
                  </Box>
  
                  {/* Поля Условия и Комментарии: minRows={1} экономит место, но позволяет расширяться */}
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 1.5 }}>
                    <TextField label="Условия" size="small" fullWidth multiline minRows={1} maxRows={3}
                      value={form.conditions || ''} onChange={updateField('conditions')} />
                    <TextField label="Комментарии" size="small" fullWidth multiline minRows={1} maxRows={3}
                      value={form.comments || ''} onChange={updateField('comments')} />
                  </Box>
                </Paper>
  
                {/* ─── Блок 2: Плановые показатели ──────────────────── */}
                <Paper sx={{ ...paperStyles, bgcolor: '#f8faff', border: '1px solid #e0e7ff' }}>
                  <Typography variant="subtitle1" sx={{ ...titleStyles, color: '#1a237e' }}>📊 Плановые показатели</Typography>
                  <Box sx={gridStyles}>
                    {[
                      { label: 'Baseline (уп)', field: 'baseline_units', editable: true },
                      { label: 'Baseline (руб)', field: 'baseline_rub', editable: true },
                      { label: 'План промо (уп)', field: 'plan_promo_units', editable: true },
                      { label: 'План промо (руб)', field: 'plan_promo_rub', editable: true },
                      { label: 'Сумма скидки', field: 'discount_amount', editable: true },
                      { label: 'План инвестиций (руб)', field: 'plan_investments_rub', editable: true },
                      { label: 'Цена контракта', field: 'contract_price', editable: true },
                      { label: 'Uplift (уп)', field: 'plan_promo_uplift_units', editable: true },
                      { label: 'Uplift (руб)', field: 'plan_promo_uplift_rub', editable: true },
                      { label: 'ROI план %', field: 'plan_roi', editable: false },
                    ].map(({ label, field, editable }) => (
                      <TextField key={field} label={label} type="text" size="small" fullWidth
                        value={getDisplayValue(field, editable)}
                        onChange={editable ? handleFieldChange(field) : undefined}
                        onFocus={editable ? handleFocus(field) : undefined}
                        onBlur={editable ? handleBlur(field) : undefined}
                        slotProps={{ input: editable ? {} : { readOnly: true }, htmlInput: { inputMode: 'text' } }} 
                        sx={{ bgcolor: editable ? '#ffffff' : '#f0f0f0' }} 
                      />
                    ))}
                  </Box>
                </Paper>
  
                {/* ─── Блок 3: Фактические показатели ──────────────────── */}
                <Paper sx={{ ...paperStyles, bgcolor: '#f2fbf4', border: '1px solid #d4ebd9' }}>
                  <Typography variant="subtitle1" sx={{ ...titleStyles, color: '#1b5e20' }}>✅ Фактические показатели</Typography>
                  <Box sx={gridStyles}>
                    {[
                      { label: 'Факт продажи (уп)', field: 'actual_promo_sales_units', editable: true },
                      { label: 'Факт промо (руб)', field: 'actual_promo_rub', editable: true },
                      { label: 'Факт инвестиции', field: 'actual_investments', editable: true },
                      { label: 'Факт Uplift (уп)', field: 'actual_promo_uplift_units', editable: true },
                      { label: 'Факт Uplift (руб)', field: 'actual_promo_uplift_rub', editable: true },
                      { label: 'Факт ROI %', field: 'actual_roi', editable: false },
                      { label: 'Внешний e-com (уп)', field: 'actual_external_ecom_units', editable: true },
                      { label: 'Скорр. Baseline', field: 'actual_corrected_baseline', editable: true },
                    ].map(({ label, field, editable }) => (
                      <TextField key={field} label={label} type="text" size="small" fullWidth
                        value={getDisplayValue(field, editable)}
                        onChange={editable ? handleFieldChange(field) : undefined}
                        onFocus={editable ? handleFocus(field) : undefined}
                        onBlur={editable ? handleBlur(field) : undefined}
                        slotProps={{ input: editable ? {} : { readOnly: true }, htmlInput: { inputMode: 'text' } }} 
                        sx={{ bgcolor: editable ? '#ffffff' : '#e9ecea' }} 
                      />
                    ))}
                  </Box>
                </Paper>
              </>
            );
          })()}
  
        </Box>
      </DialogContent>
  
      {/* Уменьшили отступы в подвале (py: 1.5) */}
      <DialogActions sx={{ justifyContent: 'space-between', px: 3, py: 1.5, bgcolor: '#ffffff' }}>
        <Button color="error" startIcon={<DeleteIcon />} onClick={onDelete}>Удалить</Button>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button variant="outlined" onClick={onClose}>Закрыть</Button>
          <Button variant="contained" startIcon={<SaveIcon />} onClick={onSave} disabled={saving}>
            {saving ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
}
```

## File: frontend/src/pages/PromoAnalysis.jsx
```javascript
import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { 
  Button, Stack, Box, Typography, CircularProgress, Tabs, Tab, 
  Alert, Snackbar, Dialog, DialogTitle, DialogContent, DialogActions,
  TextField, Menu, MenuItem, Checkbox, ListItemText, Divider,
  Tooltip, Chip,
} from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import { 
  ViewColumn as ColumnsIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
} from '@mui/icons-material';
import { DataGrid } from '@mui/x-data-grid';
import FilterPanel from '../components/FilterPanel';
import PromoForm from './PromoForm';
import PromoEditDialog from '../components/PromoEditDialog';
import PromoApproval from './PromoApproval';
import { promoAPI } from '../api/promo';
import { usePromoFilters } from '../hooks/usePromoFilters';
import { usePromoData } from '../hooks/usePromoData';
import { usePromoForm } from '../hooks/usePromoForm';
import { usePromoCalculations } from '../hooks/usePromoCalculations';

const FILTERS_STORAGE_KEY = 'promo_filters_v20';
const PERSIST_FLAG_KEY = 'promo_persist_v20';

const renderAgreement = (value) => {
  if (value == null || value === '' || value === '0') return '';
  const v = String(value);
  const lower = v.toLowerCase();

  const isApproved = lower.startsWith('согласовано');
  const isRejected = lower.startsWith('отклонено');

  if (isApproved) {
    const comment = v.substring('согласовано'.length).replace(/^[:\s]+/, '');
    return (
      <Tooltip title={comment || 'Согласовано'} arrow placement="top" disableHoverListener={!comment}>
        <Chip
          label={comment ? '✓ Согласовано + комм.' : '✓ Согласовано'}
          size="small"
          variant="filled"
          sx={{ bgcolor: '#f0fdf4', color: '#16a34a', fontWeight: 600, height: 24, fontSize: '0.75rem' }}
        />
      </Tooltip>
    );
  }

  if (isRejected) {
    const comment = v.substring('отклонено'.length).replace(/^[:\s]+/, '');
    return (
      <Tooltip title={comment || 'Отклонено'} arrow placement="top" disableHoverListener={!comment}>
        <Chip
          label={comment ? '✗ Отклонено + комм.' : '✗ Отклонено'}
          size="small"
          variant="filled"
          sx={{ bgcolor: '#fef2f2', color: '#dc2626', fontWeight: 600, height: 24, fontSize: '0.75rem' }}
        />
      </Tooltip>
    );
  }

  // Только комментарий
  return (
    <Tooltip title={v} arrow placement="top">
      <Chip
        label="💬 Комментарий"
        size="small"
        variant="filled"
        sx={{ bgcolor: '#eef2ff', color: '#6366f1', fontWeight: 600, height: 24, fontSize: '0.75rem' }}
      />
    </Tooltip>
  );
};

// ─── Колонки таблицы просмотра данных ──────────────────────────────────────
const COLUMNS = [
  { field: 'year', headerName: 'Год', width: 70, type: 'number', valueFormatter: (v) => v },
  { field: 'month', headerName: 'Мес', width: 60, type: 'number' }, 
  { field: 'channel', headerName: 'Канал', width: 90 },
  { field: 'network_name', headerName: 'Сеть', width: 180 }, 
  { field: 'brand_as', headerName: 'Бренд', width: 130 },
  { field: 'sku', headerName: 'SKU', width: 130 }, 
  { field: 'mechanics', headerName: 'Механика', width: 180 },
  { field: 'plan_promo_units', headerName: 'План (уп)', width: 110, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0 }) : '' },
  { field: 'actual_promo_sales_units', headerName: 'Факт (уп)', width: 110, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0 }) : '' },
  { field: 'plan_investments_rub', headerName: 'План инвест.', width: 130, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'actual_investments', headerName: 'Факт инвест.', width: 130, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'agreement1', headerName: 'Согласование 1', width: 160,
    renderCell: (params) => renderAgreement(params.value) },
  { field: 'agreement2', headerName: 'Согласование 2', width: 160,
    renderCell: (params) => renderAgreement(params.value) },
  { field: 'status', headerName: 'Статус', width: 140 },
];

const EMPTY_FILTERS = { 
  yearFrom: '', yearTo: '', months: [], kam: [], brand: [], sku: [], 
  network_name: [], mechanics: [], channel: [], status: [] 
};
const EXTRA_FILTERS = [
  { type: 'year', field: 'yearFrom', label: 'Год от' }, 
  { type: 'year', field: 'yearTo', label: 'Год до' }, 
  { type: 'months', field: 'months', label: 'Месяцы' }
];
const PROMO_VISIBLE_FILTERS = ['kam', 'brand', 'sku', 'network_name', 'mechanics', 'channel', 'status'];

// ─── Компонент ─────────────────────────────────────────────────────────────
// role — передаётся из App.jsx (admin / agreement1 / agreement2)
export default function PromoAnalysis({ role }) {
  const navigate = useNavigate();
  const [tab, setTab] = useState(0);
  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const [allSkuOptions, setAllSkuOptions] = useState([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState([]);
  const [investmentTypes, setInvestmentTypes] = useState([]);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  // ─── Пользовательский тулбар таблицы ──────────────────────────────────
  const [searchText, setSearchText] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const map = {};
    COLUMNS.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  // ─── Фильтры и данные ─────────────────────────────────────────────────
  const { meta, filters, setFilters, appliedFilters, persistFilters, handleSearch, handleReset, handlePersistChange, fetchMeta } = 
    usePromoFilters(EMPTY_FILTERS, FILTERS_STORAGE_KEY, PERSIST_FLAG_KEY);
  const { rows, setRows, loading: dataLoading, error: dataError, refetch } = usePromoData(appliedFilters, refreshTrigger);

  // ─── Локальные обновления UI (без перезагрузки всей таблицы) ──────────
  // После редактирования: заменяем одну строку в массиве
  const handleEditSuccess = useCallback((editedId, updatedData) => {
    setRows(prev => prev.map(row => 
      row.id === editedId ? { ...row, ...updatedData } : row
    ));
  }, [setRows]);

  // После удаления: убираем строку из массива
  const handleDeleteSuccess = useCallback((deletedId) => {
    setRows(prev => prev.filter(row => row.id !== deletedId));
  }, [setRows]);

  // После создания нового промо: перезагружаем таблицу полностью
  const handleCreateSuccess = useCallback(() => {
    setRefreshTrigger(prev => prev + 1);
  }, []);

  // ─── Форма редактирования ─────────────────────────────────────────────
  const { form, setForm, saving, deleting, handleRowClick: formHandleRowClick, handleSave: formHandleSave, handleDelete: formHandleDelete, resetForm } = 
    usePromoForm({ onEditSuccess: handleEditSuccess, onDeleteSuccess: handleDeleteSuccess, onCreateSuccess: handleCreateSuccess });
  const { recalcPlan, recalcActual } = usePromoCalculations(form);

  // ─── Загрузка справочников ────────────────────────────────────────────
  useEffect(() => { promoAPI.getInvestmentTypes().then(data => setInvestmentTypes(data.data || [])); }, []);
  useEffect(() => { 
    promoAPI.getFilters().then(data => { 
      setAllSkuOptions(data.sku || []); 
      setAllNetworkOptions(data.network_name || []); 
    }); 
  }, []);

  const filterOptions = useMemo(() => ({
    kam: meta.kam || [], brand: meta.brand || [], sku: meta.sku || [],
    network_name: meta.network_name || [], mechanics: meta.mechanics || [],
    channel: meta.channel || [], status: meta.status || []
  }), [meta]);

  // ─── Обработчики действий ─────────────────────────────────────────────
  const handleRowClick = (params) => { formHandleRowClick(params.row); setEditDialogOpen(true); };

  const handleSave = async () => { 
    const result = await formHandleSave(); 
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
  };

  const handleDelete = async () => { 
    const result = await formHandleDelete(); 
    if (result.success) { setDeleteDialogOpen(false); setEditDialogOpen(false); }
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
  };

  const handlePromoFormSave = () => {
    setRefreshTrigger(prev => prev + 1);
    setSnackbar({ open: true, message: '✅ Сохранено', severity: 'success' });
  };

  // ─── Поиск по таблице (клиентский) ────────────────────────────────────
  const filteredRows = useMemo(() => {
    if (!searchText.trim()) return rows;
    const lower = searchText.toLowerCase();
    return rows.filter(row =>
      Object.values(row).some(val =>
        val != null && String(val).toLowerCase().includes(lower)
      )
    );
  }, [rows, searchText]);

  const visibleCols = useMemo(
    () => COLUMNS.filter(c => visibleColumns[c.field] !== false),
    [visibleColumns]
  );

  const toggleColumn = (f) => setVisibleColumns(prev => ({ ...prev, [f]: !prev[f] }));

  // ─── Экспорт CSV ──────────────────────────────────────────────────────
  const handleExport = async () => {
    try {
      const params = new URLSearchParams();
      params.set('all', 'true');
      Object.entries(appliedFilters).forEach(([k, v]) => {
        if (Array.isArray(v)) v.forEach(x => { if (x) params.append(k, String(x)); });
        else if (v) params.set(k, String(v));
      });
      const res = await fetch(`http://localhost:8080/api/promo/data?${params}`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });
      const json = await res.json();
      const data = json.data || [];
      const headers = visibleCols.map(c => c.headerName || c.field);
      const fields = visibleCols.map(c => c.field);
      let csv = '\uFEFF' + headers.join(';') + '\n';
      data.forEach(row => {
        csv += fields.map(f => {
          let v = row[f]; if (v == null) return '';
          v = String(v);
          if (v.includes(';') || v.includes('"') || v.includes('\n')) v = '"' + v.replace(/"/g, '""') + '"';
          return v;
        }).join(';') + '\n';
      });
      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = 'promo-analysis.csv';
      link.click();
      URL.revokeObjectURL(link.href);
    } catch (e) { console.error('Export error:', e); }
  };

  // ─── Рендер ───────────────────────────────────────────────────────────
  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2 }}>
      {/* Шапка */}
      <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 2 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>Анализ промо</Typography>
        {meta.loading && <CircularProgress size={20} />}
        {rows.length > 0 && tab === 0 && 
          <Typography variant="body2" color="text.secondary">Загружено: {rows.length} записей</Typography>}
      </Stack>

      {/* ─── Вкладки ─────────────────────────────────────────────────── */}
      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 2 }}>
        <Tab label="Просмотр данных" />
        <Tab label="Новое промо" />
        {(role === 'agreement1' || role === 'agreement2' || role === 'admin') && (
          <Tab label="Согласование" />
        )}
      </Tabs>

      {/* ─── Tab 0: Просмотр данных ──────────────────────────────────── */}
      {tab === 0 && (<>
        <Box sx={{ mb: 2 }}>
          <FilterPanel filters={filters} filterOptions={filterOptions} onFiltersChange={setFilters}
            onSearch={handleSearch} onReset={handleReset} extraFilters={EXTRA_FILTERS}
            persistFilters={persistFilters} onPersistChange={handlePersistChange} 
            visibleFilters={PROMO_VISIBLE_FILTERS} />
        </Box>
        {meta.error && 
          <Button variant="outlined" color="warning" onClick={() => fetchMeta(filters)} sx={{ mb: 2 }}>
            Ошибка загрузки справочников. Повторить
          </Button>}

          <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minHeight: 0 }}>
          {/* Пользовательский тулбар */}
          <Box sx={{ 
            display: 'flex', alignItems: 'center', gap: 1, px: 2, py: 1,
            bgcolor: '#f1f5f9', borderRadius: '12px 12px 0 0',
            border: '1px solid #e2e8f0', borderBottom: 'none',
          }}>
            <Button size="small" startIcon={<ColumnsIcon />}
              onClick={(e) => setColumnsAnchor(e.currentTarget)}
              sx={{ color: '#475569', fontWeight: 500 }}>Колонки</Button>
            <Menu anchorEl={columnsAnchor} open={Boolean(columnsAnchor)}
              onClose={() => setColumnsAnchor(null)}
              slotProps={{ paper: { sx: { maxHeight: 400, minWidth: 220 } } }}>
              <MenuItem dense onClick={() => setVisibleColumns(Object.fromEntries(COLUMNS.map(c => [c.field, true])))}>
                <Typography variant="caption" color="primary" sx={{ fontWeight: 600 }}>Показать все</Typography></MenuItem>
              <MenuItem dense onClick={() => setVisibleColumns(Object.fromEntries(COLUMNS.map(c => [c.field, false])))}>
                <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>Скрыть все</Typography></MenuItem>
              <Divider />
              {COLUMNS.map(col => (
                <MenuItem key={col.field} dense onClick={() => toggleColumn(col.field)}>
                  <Checkbox size="small" checked={visibleColumns[col.field] !== false} />
                  <ListItemText primary={col.headerName || col.field} primaryTypographyProps={{ fontSize: 13 }} />
                </MenuItem>
              ))}
            </Menu>
            <TextField size="small" placeholder="Поиск по таблице..." value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              InputProps={{ startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> }}
              sx={{ width: 240, '& .MuiOutlinedInput-root': { bgcolor: '#fff', borderRadius: 2 }, '& .MuiInputBase-input': { fontSize: '0.875rem', py: 0.75 } }} />
            <Box sx={{ flex: 1 }} />
            {rows.length > 0 && (
              <Typography variant="caption" color="text.secondary" sx={{ mr: 1 }}>
                {rows.length.toLocaleString('ru-RU')} строк
              </Typography>
            )}
            <Button size="small" startIcon={<ExportIcon />} onClick={handleExport}
              sx={{ color: '#475569', fontWeight: 500 }}>CSV</Button>
          </Box>

          <DataGrid 
            apiRef={apiRef}
            rows={filteredRows} 
            columns={visibleCols} 
            loading={dataLoading} 
            sortingMode="client" 
            disableColumnFilter
            onRowClick={handleRowClick}
            initialState={{ 
              pagination: { paginationModel: { pageSize: 100 } }, 
              sorting: { sortModel: [{ field: 'year', sort: 'desc' }] } 
            }}
            pageSizeOptions={[25, 50, 100]} 
            disableRowSelectionOnClick 
            sx={{ 
              flex: 1, border: '1px solid #e2e8f0', borderTop: 'none',
              borderRadius: '0 0 12px 12px',
              '& .MuiDataGrid-columnHeaders': { borderRadius: 0 },
              '& .MuiDataGrid-row': { cursor: 'pointer' } 
            }} 
          />
        </Box>

        {/* Диалог редактирования */}
        <PromoEditDialog 
          open={editDialogOpen} onClose={() => setEditDialogOpen(false)}
          form={form} setForm={setForm} recalcPlan={recalcPlan} recalcActual={recalcActual}
          onSave={handleSave} onDelete={() => setDeleteDialogOpen(true)}
          saving={saving} deleting={deleting} meta={meta} 
          allSkuOptions={allSkuOptions} allNetworkOptions={allNetworkOptions} 
          investmentTypes={investmentTypes}
          role={role}
        />

        {/* Диалог подтверждения удаления */}
        <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
          <DialogTitle>Удалить промо #{form.id}?</DialogTitle>
          <DialogContent><Typography>Это действие нельзя отменить.</Typography></DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteDialogOpen(false)}>Отмена</Button>
            <Button color="error" variant="contained" onClick={handleDelete} disabled={deleting}>
              {deleting ? 'Удаление...' : 'Удалить'}
            </Button>
          </DialogActions>
        </Dialog>
      </>)}

      {/* ─── Tab 1: Новое промо ────────────────────────────────────────── */}
      {tab === 1 && <PromoForm onSave={handlePromoFormSave} />}

      {/* ─── Tab 2: Согласование ──────────────────────────────────────── */}
      {tab === 2 && <PromoApproval role={role} onDataChanged={() => setRefreshTrigger(prev => prev + 1)} />}

      {/* Снекбар уведомлений */}
      <Snackbar open={snackbar.open} autoHideDuration={3000} 
        onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}
```

## File: backend/main.go
```go
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
)

// ─── Rate Limiter ───────────────────────────────────────────────────────────

type RateLimiter struct {
	mu       sync.RWMutex
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
	corsOrigins := []string{"http://localhost:5173"}
	if env := os.Getenv("CORS_ORIGINS"); env != "" {
		corsOrigins = strings.Split(env, ",")
		for i := range corsOrigins {
			corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	r.Use(RateLimitMiddleware(limiter))

	// ─── Публичный роут (без авторизации) ────────────────────────────────
	r.POST("/api/auth/login", handlers.Login)
	r.POST("/api/auth/refresh", handlers.RefreshToken)

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
		api.GET("/promo/approval-filters", handlers.GetApprovalFilters)
		api.GET("/promo/approval-kams", handlers.GetApprovalKAMs)
		api.GET("/promo/approval-networks", handlers.GetApprovalNetworks)
		api.GET("/promo/approval-brands", handlers.GetApprovalBrands)
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
```

## File: frontend/src/api/promo.js
```javascript
import { refreshToken } from './auth';

const API_BASE = 'http://localhost:8080';

// ─── Утилита: fetch с авторизацией и таймаутом ──────────────────────────────
async function fetchWithAuth(url, options = {}, timeout = 15000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  const doFetch = () => {
    const token = localStorage.getItem('token');
    const headers = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    return fetch(url, { ...options, headers, signal: controller.signal });
  };

  let res = await doFetch();

  // При 401 пробуем обновить токен и повторить
  if (res.status === 401) {
    const refreshed = await refreshToken();
    if (refreshed) {
      res = await doFetch();
    }
  }

  clearTimeout(timer);
  return res;
}

// ─── Утилита: построение query string ──────────────────────────────────────
function buildParams(filters) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (Array.isArray(value)) {
      value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
    } else if (value !== '' && value != null) {
      params.set(key, String(value));
    }
  });
  return params.toString();
}

// ─── API: Промо ────────────────────────────────────────────────────────────
export const promoAPI = {
  // Справочники фильтров
  getFilters: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/filters?${buildParams(filters)}`).then(r => r.json()),

  // Данные промо
  getData: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/data?all=true&${buildParams(filters)}`).then(r => r.json()),

  // История промо по SKU/сети/механике
  getHistory: (params = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/history?${new URLSearchParams(params)}`).then(r => r.json()),

  // Сохранение (INSERT / UPDATE)
  save: (data) =>
    fetchWithAuth(`${API_BASE}/api/promo/save`, {
      method: 'POST',
      body: JSON.stringify(data),
    }).then(async r => {
      const json = await r.json();
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка сохранения' };
      return json;
    }),

  // Удаление (soft-delete)
  delete: (id) =>
    fetchWithAuth(`${API_BASE}/api/promo/${id}`, { method: 'DELETE' }).then(async r => {
      if (!r.ok) {
        const data = await r.json().catch(() => ({}));
        throw new Error(data.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),

  // SKU по бренду
  getSKUByBrand: (brand) =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-by-brand?brand=${encodeURIComponent(brand)}`).then(r => r.json()),

  // Информация о SKU (бренд)
  getSKUInfo: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-info?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // Последние данные по SKU
  getLastSKUData: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-sku-data?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // KAM по сети
  getKAMByNetwork: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/kam-by-network?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Гео-маппинг сети (KAM, регион, сегмент ТОП-20)
  getNetworkGeo: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/network-geo?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Последние данные по сети (аптеки)
  getLastNetworkData: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-network-data?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Типы инвестиций
  getInvestmentTypes: () =>
    fetchWithAuth(`${API_BASE}/api/promo/investment-types`).then(r => r.json()),

  // Последняя цена контракта по SKU
  getLastContractPrice: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-contract-price?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // ─── Согласование ──────────────────────────────────────────────────────

  // Список KAM'ов с промо на согласовании
  getApprovalKAMs: () =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-kams`).then(r => r.json()),

  // Сети для выбранного KAM (в согласовании)
  getApprovalNetworks: (kam) =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-networks?kam=${encodeURIComponent(kam)}`).then(r => r.json()),

  // Бренды для KAM + сети (в согласовании)
  getApprovalBrands: (kam, network = '') =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-brands?kam=${encodeURIComponent(kam)}&network_name=${encodeURIComponent(network)}`).then(r => r.json()),

  // Список промо на согласование
  getApprovals: (params = {}) => {
    const qs = new URLSearchParams();
    if (params.kam) qs.set('kam', params.kam);
    if (params.approval_status) qs.set('approval_status', params.approval_status);
    else qs.set('approval_status', 'pending');
    if (params.year) qs.set('year', params.year);
    if (params.month) qs.set('month', params.month);
    return fetchWithAuth(`${API_BASE}/api/promo/approvals?${qs}`).then(r => r.json());
  },

  // Справочники сетей/брендов/механик для страницы согласования
  getApprovalFilters: (params = {}) => {
    const qs = new URLSearchParams();
    qs.set('approval_status', params.approval_status || 'pending');
    if (params.kam) qs.set('kam', params.kam);
    if (params.network_name) qs.set('network_name', params.network_name);
    if (params.brand) qs.set('brand', params.brand);
    if (params.mechanics) qs.set('mechanics', params.mechanics);
    if (params.year) qs.set('year', params.year);
    if (params.month) qs.set('month', params.month);
    return fetchWithAuth(`${API_BASE}/api/promo/approval-filters?${qs}`).then(r => r.json());
  },

  // Действие согласования: comment / согласовано / отклонено
  approve: (id, status, comment = '') =>
    fetchWithAuth(`${API_BASE}/api/promo/approve`, {
      method: 'POST',
      body: JSON.stringify({ id, status, comment }),
    }).then(async r => {
      const json = await r.json();
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка' };
      return json;
    }),
};

// ─── API: Интернет-продажи ─────────────────────────────────────────────────
export const salesAPI = {
  getFilters: () =>
    fetchWithAuth(`${API_BASE}/api/filters`).then(r => r.json()),

  getData: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/data?${buildParams(filters)}`).then(r => r.json()),

  getDrilldown: (params = {}) =>
    fetchWithAuth(`${API_BASE}/api/drilldown?${new URLSearchParams(params)}`).then(r => r.json()),
};
```

## File: backend/handlers/promo.go
```go
// ─── Обработчики ────────────────────────────────────────────────────────────

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// ─── Read ──────────────────────────────────────────────────────────────────

func GetPromoFilters(c *gin.Context) {
	params := repository.PromoFilterParams{
		YearFromStr: c.Query("yearFrom"),
		YearToStr:   c.Query("yearTo"),
		Months:      c.QueryArray("months"),
		Kams:        c.QueryArray("kam"),
		Brands:      c.QueryArray("brand"),
		SKUs:        c.QueryArray("sku"),
		Networks:    c.QueryArray("network_name"),
		Mechanics:   c.QueryArray("mechanics"),
		Statuses:    c.QueryArray("status"),
	}

	baseWhere, baseArgs := repository.BuildBaseWhere(params)

	mainFilters := map[string][]string{
		"kam": params.Kams, "brand_as": params.Brands, "sku": params.SKUs,
		"network_name": params.Networks, "mechanics": params.Mechanics, "status": params.Statuses,
	}

	var (
		resKam, resBrand, resSKU, resNetwork, resMechanics, resStatus, resChannel []string
	)

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		resKam = repository.GetFilterValues("kam", baseWhere, baseArgs, "kam", mainFilters)
		return nil
	})
	g.Go(func() error {
		resBrand = repository.GetFilterValues("brand_as", baseWhere, baseArgs, "brand_as", mainFilters)
		return nil
	})
	g.Go(func() error {
		resSKU = repository.GetFilterValues("sku", baseWhere, baseArgs, "sku", mainFilters)
		return nil
	})
	g.Go(func() error {
		resNetwork = repository.GetFilterValues("network_name", baseWhere, baseArgs, "network_name", mainFilters)
		return nil
	})
	g.Go(func() error {
		resMechanics = repository.GetFilterValues("mechanics", baseWhere, baseArgs, "mechanics", mainFilters)
		return nil
	})
	g.Go(func() error {
		resStatus = repository.GetFilterValues("status", baseWhere, baseArgs, "status", mainFilters)
		return nil
	})
	g.Go(func() error {
		resChannel = repository.GetChannelFilterValues(baseWhere, baseArgs, mainFilters)
		return nil
	})

	_ = g.Wait()

	c.JSON(http.StatusOK, gin.H{
		"kam":          resKam,
		"brand":        resBrand,
		"sku":          resSKU,
		"network_name": resNetwork,
		"mechanics":    resMechanics,
		"status":       resStatus,
		"channel":      resChannel,
	})
}

func GetPromoData(c *gin.Context) {
	params := repository.PromoFilterParams{
		YearFromStr: c.Query("yearFrom"),
		YearToStr:   c.Query("yearTo"),
		Months:      c.QueryArray("months"),
		Kams:        c.QueryArray("kam"),
		Brands:      c.QueryArray("brand"),
		SKUs:        c.QueryArray("sku"),
		Networks:    c.QueryArray("network_name"),
		Mechanics:   c.QueryArray("mechanics"),
		Statuses:    c.QueryArray("status"),
	}
	channels := c.QueryArray("channel")

	all := c.Query("all")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("limit", "100")))

	results, err := repository.GetPromoRows(params, channels, page, pageSize, all == "true")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func GetSKUByBrand(c *gin.Context) {
	brand := c.Query("brand")
	if brand == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	skus, _ := repository.GetSKUsByBrand(brand)
	if skus == nil {
		skus = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": skus})
}

func GetLastContractPrice(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{"price": nil})
		return
	}
	price, _ := repository.GetLastContractPrice(sku)
	c.JSON(http.StatusOK, gin.H{"price": price})
}

func GetInvestmentTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []string{"GTN", "GTN в ОС", "OPEX", "OPEX Marketing"}})
}

func GetKAMByNetwork(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	kams, _ := repository.GetKAMsByNetwork(network)
	if kams == nil {
		kams = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": kams})
}

func GetLastNetworkData(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	total, _ := repository.GetLastNetworkData(network)
	if total != nil {
		c.JSON(http.StatusOK, gin.H{"total_pharmacies": *total})
	} else {
		c.JSON(http.StatusOK, gin.H{"total_pharmacies": 0})
	}
}

func GetNetworkGeoMapping(c *gin.Context) {
	network := c.Query("network")
	if network == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "network is required"})
		return
	}
	geo, err := repository.GetNetworkGeoMapping(network)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"kam": nil, "network_type": nil, "top20_segment": nil, "key_region": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"kam":           geo.KAM,
		"network_type":  geo.NetworkType,
		"top20_segment": geo.Top20Segment,
		"key_region":    geo.KeyRegion,
	})
}

func GetPromoHistoryFiltered(c *gin.Context) {
	sku := c.Query("sku")
	network := c.Query("network_name")
	mechanics := c.Query("mechanics")
	yearFrom := c.Query("yearFrom")
	yearTo := c.Query("yearTo")

	results, err := repository.GetPromoHistory(sku, network, mechanics, yearFrom, yearTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func GetSKUInfo(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{"brand": nil, "brand_as": nil})
		return
	}
	brand, brandAs, found := repository.GetSKUInfo(sku)
	if !found {
		c.JSON(http.StatusOK, gin.H{"brand": nil, "brand_as": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"brand": brand, "brand_as": brandAs})
}

func GetLastSKUData(c *gin.Context) {
	sku := c.Query("sku")
	if sku == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	data, err := repository.GetLastSKUData(sku)
	if err != nil && err.Error() != "sql: no rows in result set" {
		config.Logger.Error("get_last_sku_data_failed", "error", err.Error(), "sku", sku)
	}
	if data == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"contract_price":   data.ContractPrice,
		"gm":               data.GM,
		"total_pharmacies": data.TotalPharmacies,
		"key_region":       data.KeyRegion,
		"top20_segment":    data.Top20Segment,
		"olap_price":       data.OlapPrice,
	})
}

// ─── Save / Delete ─────────────────────────────────────────────────────────

func SavePromo(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// UPDATE
	if id, ok := input["id"]; ok && id != nil {
		idFloat, _ := strconv.ParseFloat(fmt.Sprint(id), 64)
		idInt := int(idFloat)
		if idInt > 0 {
			existing, err := repository.FetchExistingRow(idInt)
			if err != nil {
				config.Logger.Error("promo_update_fetch_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Запись не найдена"})
				return
			}

			// Сохраняем текущий updated_at для Optimistic Locking
			updatedAt := safeString(existing, "updated_at")

			for k, v := range input {
				if k != "id" && k != "deleted_at" && k != "updated_at" && k != "agreement1" && k != "agreement2" {
					existing[k] = v
				}
			}

			dto := services.MapToDTO(existing)
			calcCtx := services.EnrichFromRepo(&dto)
			calc := services.CalculateFields(&dto, calcCtx)
			services.MergeCalculatedIntoMap(existing, calc)

			rowsAffected, err := repository.UpdatePromo(idInt, existing, updatedAt)
			if err != nil {
				config.Logger.Error("promo_update_failed", "error", err.Error(), "id", idInt)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if rowsAffected == 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "Данные были изменены другим пользователем. Обновите страницу и попробуйте снова."})
				return
			}

			existing["updated_at"] = time.Now().Format("2006-01-02T15:04:05.9999999-07:00")

			config.Logger.Info("promo_updated",
				"id", idInt,
				"sku", fmt.Sprint(existing["sku"]),
				"network", fmt.Sprint(existing["network_name"]),
				"user", "system",
				"timestamp", time.Now().Format(time.RFC3339),
			)
			c.JSON(http.StatusOK, gin.H{"message": "Updated", "id": idInt, "data": existing})
			return
		}
	}

	// INSERT
	dto := services.MapToDTO(input)
	calcCtx := services.EnrichFromRepo(&dto)
	calc := services.CalculateFields(&dto, calcCtx)
	services.MergeCalculatedIntoMap(input, calc)
	delete(input, "id")

	newID, err := repository.InsertPromo(input)
	if err != nil {
		config.Logger.Error("promo_insert_failed",
			"error", err.Error(),
			"sku", fmt.Sprint(input["sku"]),
			"network", fmt.Sprint(input["network_name"]),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	config.Logger.Info("promo_created",
		"id", newID,
		"sku", fmt.Sprint(input["sku"]),
		"network", fmt.Sprint(input["network_name"]),
		"year", safeInt(input, "year"),
		"month", safeInt(input, "month"),
		"plan_units", safeFloat(input, "plan_promo_units"),
		"plan_rub", safeFloat(input, "plan_promo_rub"),
		"user", "system",
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Created", "id": newID, "data": input})
}

func DeletePromo(c *gin.Context) {
	id := c.Param("id")

	if _, err := strconv.Atoi(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	idInt, _ := strconv.Atoi(id)
	rows, err := repository.SoftDeletePromo(idInt)
	if err != nil {
		config.Logger.Error("promo_delete_failed", "id", id, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена или уже удалена"})
		return
	}

	config.Logger.Info("promo_deleted", "id", id, "user", "system", "timestamp", time.Now().Format(time.RFC3339))
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ─── Approvals ─────────────────────────────────────────────────────────────

func GetApprovals(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	params := repository.ApprovalParams{
		Role:           roleStr,
		KAM:            c.Query("kam"),
		ApprovalStatus: c.DefaultQuery("approval_status", "pending"),
		YearStr:        c.Query("year"),
		MonthStr:       c.Query("month"),
	}

	results, err := repository.GetApprovals(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func ApprovePromo(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	field := "agreement1"
	if roleStr == "agreement2" {
		field = "agreement2"
	}

	var req struct {
		ID      int    `json:"id"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}

	var status string
	var comment string
	var legacyValue string
	switch req.Status {
	case "comment":
		if req.Comment == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "комментарий не может быть пустым"})
			return
		}
		status = "commented"
		comment = req.Comment
		legacyValue = req.Comment
	case "согласовано":
		status = "approved"
		comment = req.Comment
		legacyValue = "согласовано"
		if req.Comment != "" {
			legacyValue = "согласовано: " + req.Comment
		}
	case "отклонено":
		status = "rejected"
		comment = req.Comment
		legacyValue = "отклонено"
		if req.Comment != "" {
			legacyValue = "отклонено: " + req.Comment
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "допустимые status: comment, согласовано, отклонено"})
		return
	}

	agreementNum := 1
	if roleStr == "agreement2" {
		agreementNum = 2
	}

	if err := repository.ApprovePromoWithStatus(agreementNum, req.ID, status, comment, legacyValue); err != nil {
		config.Logger.Error("approve_failed", "error", err.Error(), "id", req.ID, "field", field)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
		return
	}

	config.Logger.Info("promo_approved",
		"id", req.ID,
		"field", field,
		"status", status,
		"comment", comment,
		"user", "system",
		"timestamp", time.Now().Format(time.RFC3339),
	)
	c.JSON(http.StatusOK, gin.H{"message": "Обновлено"})
}

// ─── Approval Filters ─────────────────────────────────────────────────────

func GetApprovalFilters(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)

	params := repository.ApprovalFilterParams{
		ApprovalStatus: c.DefaultQuery("approval_status", "pending"),
		KAM:            c.Query("kam"),
		Network:        c.Query("network_name"),
		Brand:          c.Query("brand"),
		MechFilter:     c.Query("mechanics"),
		YearStr:        c.Query("year"),
		MonthStr:       c.Query("month"),
		Role:           roleStr,
	}

	networks, brands, mechanics, kams, err := repository.GetApprovalFilters(params)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"networks": []string{}, "brands": []string{}, "mechanics": []string{}, "kams": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"networks": networks, "brands": brands, "mechanics": mechanics, "kams": kams})
}

func GetApprovalKAMs(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)
	field := "agreement1"
	if roleStr == "agreement2" {
		field = "agreement2"
	}

	kams, err := repository.GetApprovalKAMs(field)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": kams})
}

func GetApprovalNetworks(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)
	kam := c.Query("kam")
	if kam == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}

	field := "p.agreement1"
	if roleStr == "agreement2" {
		field = "p.agreement2"
	}

	networks, err := repository.GetApprovalNetworks(field, kam)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": networks})
}

func GetApprovalBrands(c *gin.Context) {
	role, _ := c.Get("role")
	roleStr := fmt.Sprint(role)
	kam := c.Query("kam")
	network := c.Query("network_name")
	if kam == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}

	field := "p.agreement1"
	if roleStr == "agreement2" {
		field = "p.agreement2"
	}

	brands, err := repository.GetApprovalBrands(field, kam, network)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": brands})
}

// ─── Log helpers (используются для логирования, оставлены для совместимости) ─

func init() {
	// Проверка что middleware и models импортируются (избегаем unused import)
	_ = middleware.RoleRequired
	_ = models.ApprovalRow{}
}
```

## File: frontend/src/pages/PromoApproval.jsx
```javascript
import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Box, Typography, CircularProgress, Alert, Snackbar,
  TextField, MenuItem, Dialog,
  DialogTitle, DialogContent, DialogActions,
  Paper, Stack, Button,
} from '@mui/material';
import ApprovalCard from '../components/ApprovalCard';
import { promoAPI } from '../api/promo';

const MONTHS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 },
  { label: 'Апрель', value: 4 }, { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 },
  { label: 'Июль', value: 7 }, { label: 'Август', value: 8 }, { label: 'Сентябрь', value: 9 },
  { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

const APPROVAL_STATUSES = [
  { label: 'На согласовании', value: 'pending' },
  { label: 'С комментариями', value: 'commented' },
  { label: 'Согласовано', value: 'approved' },
  { label: 'Отклонено', value: 'rejected' },
  { label: 'Все', value: 'all' },
];

export default function PromoApproval({ role, onDataChanged }) {
  // Черновики фильтров (меняются сразу)
  const [draftKam, setDraftKam] = useState('');
  const [draftNetwork, setDraftNetwork] = useState('');
  const [draftBrand, setDraftBrand] = useState('');
  const [draftMechanics, setDraftMechanics] = useState('');
  const [draftStatus, setDraftStatus] = useState('pending');
  const [draftYear, setDraftYear] = useState(String(new Date().getFullYear()));
  const [draftMonth, setDraftMonth] = useState('');

  // Флаг: была ли нажата кнопка «Применить»
  const [hasApplied, setHasApplied] = useState(false);

  // Применённые фильтры (по кнопке «Применить»)
  const [appliedKam, setAppliedKam] = useState('');
  const [appliedNetwork, setAppliedNetwork] = useState('');
  const [appliedBrand, setAppliedBrand] = useState('');
  const [appliedMechanics, setAppliedMechanics] = useState('');
  const [appliedStatus, setAppliedStatus] = useState('pending');
  const [appliedYear, setAppliedYear] = useState(String(new Date().getFullYear()));
  const [appliedMonth, setAppliedMonth] = useState('');

  // Справочники (зависят от appliedStatus)
  const [kams, setKams] = useState([]);
  const [networks, setNetworks] = useState([]);
  const [brands, setBrands] = useState([]);
  const [mechanicsOptions, setMechanicsOptions] = useState([]);

  const [approvals, setApprovals] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [expandedCards, setExpandedCards] = useState({});
  const [submitting, setSubmitting] = useState({});

  const commentRefs = useRef({});
  const [confirmDialog, setConfirmDialog] = useState({ open: false, id: null, status: '' });
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });
  const [refreshFilters, setRefreshFilters] = useState(0);
  const fetchIdRef = useRef(0);

  // Загрузка справочников при смене фильтров (включая KAM из того же запроса)
  useEffect(() => {
    // Не передаём brand/network/mechanics — иначе фильтруем сами себя
    promoAPI.getApprovalFilters({
      approval_status: appliedStatus,
      kam: appliedKam,
      year: appliedYear,
      month: appliedMonth,
    })
      .then(data => {
        setKams(data.kams || []);
        setNetworks(data.networks || []);
        setBrands(data.brands || []);
        setMechanicsOptions(data.mechanics || []);
      })
      .catch(err => console.error('Ошибка справочников:', err));
  }, [appliedStatus, appliedKam, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth, refreshFilters]);

  // Загрузка данных при изменении применённых фильтров (только после «Применить»)
  const fetchApprovals = useCallback(async () => {
    if (!hasApplied || (!appliedKam && !appliedNetwork && !appliedBrand && !appliedMechanics && !appliedYear && !appliedMonth)) return;
    const currentFetchId = ++fetchIdRef.current;
    setLoading(true);
    setError(null);

    try {
      const data = await promoAPI.getApprovals({
        kam: appliedKam || undefined,
        approval_status: appliedStatus,
        year: appliedYear,
        month: appliedMonth,
      });
      if (currentFetchId !== fetchIdRef.current) return;

      let filtered = data.data || [];
      if (appliedNetwork) filtered = filtered.filter(a => a.network_name === appliedNetwork);
      if (appliedBrand) filtered = filtered.filter(a => a.brand_as === appliedBrand);
      if (appliedMechanics) filtered = filtered.filter(a => a.mechanics === appliedMechanics);
      if (appliedYear) filtered = filtered.filter(a => a.year === parseInt(appliedYear));
      if (appliedMonth) filtered = filtered.filter(a => a.month === parseInt(appliedMonth));

      // Очищаем старые DOM-рефы перед установкой новых данных
      commentRefs.current = {};
      setApprovals(filtered);
    } catch (err) {
      if (currentFetchId !== fetchIdRef.current) return;
      setError(err.message || 'Ошибка загрузки');
    } finally {
      if (currentFetchId === fetchIdRef.current) setLoading(false);
    }
  }, [hasApplied, appliedKam, appliedStatus, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth]);

  useEffect(() => {
    fetchApprovals();
  }, [fetchApprovals]);

  // Кнопка «Применить» — только если выбран хоть один фильтр
  const handleApply = () => {
    const hasAnyFilter = draftKam || draftNetwork || draftBrand || draftMechanics || draftYear || draftMonth;
    if (!hasAnyFilter) return;
    setHasApplied(true);
    setAppliedKam(draftKam);
    setAppliedNetwork(draftNetwork);
    setAppliedBrand(draftBrand);
    setAppliedMechanics(draftMechanics);
    setAppliedStatus(draftStatus);
    setAppliedYear(draftYear);
    setAppliedMonth(draftMonth);
  };

  const handleReset = () => {
    setDraftKam(''); setDraftNetwork(''); setDraftBrand(''); setDraftMechanics('');
    setDraftStatus('pending'); setDraftYear(String(new Date().getFullYear())); setDraftMonth('');
    setAppliedKam(''); setAppliedNetwork(''); setAppliedBrand(''); setAppliedMechanics('');
    setAppliedStatus('pending'); setAppliedYear(String(new Date().getFullYear())); setAppliedMonth('');
    setApprovals([]);
    setHasApplied(false);
  };

  const handleCommentRef = useCallback((id, el) => { commentRefs.current[id] = el; }, []);
  const openConfirm = (id, status) => setConfirmDialog({ open: true, id, status });

  const handleConfirmedAction = async () => {
    const { id, status } = confirmDialog;
    setConfirmDialog({ open: false, id: null, status: '' });
    if (!id) return;
    const inputEl = commentRefs.current[id];
    const comment = inputEl ? inputEl.value : '';
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, status, comment);
      setApprovals(prev => prev.filter(a => a.id !== id));
      delete commentRefs.current[id];
      setSnackbar({ open: true, message: status === 'согласовано' ? '✅ Согласовано' : status === 'отклонено' ? '❌ Отклонено' : '💬 Комментарий сохранён', severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      if (onDataChanged) onDataChanged();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (err.message || 'не удалось'), severity: 'error' });
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };

  const handleCommentOnly = (id) => {
    const inputEl = commentRefs.current[id];
    if (!inputEl || !inputEl.value.trim()) return;
    openConfirm(id, 'comment');
  };
  const toggleExpand = (id) => setExpandedCards(prev => ({ ...prev, [id]: !prev[id] }));

  return (
    <Box sx={{ flex: 1, overflow: 'auto', px: 2, pb: 4 }}>
      <Paper variant="outlined" sx={{ p: 2, mb: 3, borderRadius: 3 }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>🔍 Фильтры</Typography>
        <Stack direction="row" spacing={1.5} flexWrap="wrap" useFlexGap alignItems="center">
          <TextField select size="small" label="KAM" value={draftKam}
            onChange={(e) => setDraftKam(e.target.value)} sx={{ minWidth: 180 }}>
            <MenuItem value="">Все</MenuItem>
            {kams.map(k => <MenuItem key={k} value={k}>{k}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Сеть" value={draftNetwork}
            onChange={(e) => setDraftNetwork(e.target.value)} sx={{ minWidth: 180 }}>
            <MenuItem value="">Все</MenuItem>
            {networks.map(n => <MenuItem key={n} value={n}>{n}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Бренд" value={draftBrand}
            onChange={(e) => setDraftBrand(e.target.value)} sx={{ minWidth: 160 }}>
            <MenuItem value="">Все</MenuItem>
            {brands.map(b => <MenuItem key={b} value={b}>{b}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Механика" value={draftMechanics}
            onChange={(e) => setDraftMechanics(e.target.value)} sx={{ minWidth: 160 }}>
            <MenuItem value="">Все</MenuItem>
            {mechanicsOptions.map(m => <MenuItem key={m} value={m}>{m}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Состояние" value={draftStatus}
            onChange={(e) => setDraftStatus(e.target.value)} sx={{ minWidth: 170 }}>
            {APPROVAL_STATUSES.map(s => <MenuItem key={s.value} value={s.value}>{s.label}</MenuItem>)}
          </TextField>
          <TextField label="Год" type="number" size="small" value={draftYear}
            onChange={(e) => setDraftYear(e.target.value)} sx={{ width: 90 }}
            slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />
          <TextField select size="small" label="Месяц" value={draftMonth}
            onChange={(e) => setDraftMonth(e.target.value)} sx={{ minWidth: 120 }}>
            <MenuItem value="">Все</MenuItem>
            {MONTHS.map(m => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
          </TextField>
          <Button variant="contained" size="small" onClick={handleApply} sx={{ alignSelf: 'center' }}>
            Применить
          </Button>
          <Button variant="outlined" size="small" onClick={handleReset} sx={{ alignSelf: 'center' }}>
            Сброс
          </Button>
        </Stack>
      </Paper>

      {loading && <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress /></Box>}
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}

      {!loading && !error && approvals.length === 0 && (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography color="text.secondary" variant="h6">
            {appliedKam || appliedNetwork || appliedBrand ? 'Ничего не найдено' : 'Выберите фильтры и нажмите «Применить»'}
          </Typography>
        </Box>
      )}

      {!loading && approvals.length > 0 && (
        <>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Найдено: {approvals.length} промо</Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr', lg: '1fr 1fr 1fr 1fr' }, gap: 2 }}>
            {approvals.map(a => (
              <ApprovalCard key={a.id} item={a}
                expanded={expandedCards[a.id] || false} submitting={submitting}
                onCommentRef={handleCommentRef} onToggleExpand={toggleExpand}
                onOpenConfirm={openConfirm} onCommentOnly={handleCommentOnly} />
            ))}
          </Box>
        </>
      )}

      <Dialog open={confirmDialog.open} onClose={() => setConfirmDialog({ open: false, id: null, status: '' })}>
        <DialogTitle>{confirmDialog.status === 'comment' ? 'Сохранить комментарий?' : 'Подтвердите действие'}</DialogTitle>
        <DialogContent>
          <Typography>
            {confirmDialog.status === 'согласовано' && 'Вы уверены, что хотите СОГЛАСОВАТЬ это промо?'}
            {confirmDialog.status === 'отклонено' && 'Вы уверены, что хотите ОТКЛОНИТЬ это промо?'}
            {confirmDialog.status === 'comment' && 'Комментарий будет сохранён, решение не принято.'}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialog({ open: false, id: null, status: '' })}>Отмена</Button>
          <Button variant="contained"
            color={confirmDialog.status === 'отклонено' ? 'error' : confirmDialog.status === 'comment' ? 'primary' : 'success'}
            onClick={handleConfirmedAction}>
            {confirmDialog.status === 'comment' ? 'Отправить комментарий' : confirmDialog.status === 'согласовано' ? 'Согласовать' : 'Отклонить'}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar open={snackbar.open} autoHideDuration={3000} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>{snackbar.message}</Alert>
      </Snackbar>
    </Box>
  );
}
```
