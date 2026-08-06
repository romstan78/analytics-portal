# Аналитический портал — Документация проекта

**Дата:** 07.08.2026
**Стек:** Go (Gin) + React (Vite + MUI) + TypeScript + SQL Server (MSSQL)
**Репозиторий:** github.com/romstan78/analytics-portal

---

## Оглавление

1. [Архитектура](#архитектура)
2. [Структура файлов](#структура-файлов)
3. [Бэкенд — API эндпоинты](#бэкенд--api-эндпоинты)
4. [База данных](#база-данных)
5. [Готово (Done)](#готово-done)
6. [План доработок](#план-доработок)
7. [Архитектура работы с комментариями](#архитектура-работы-с-комментариями)
8. [Выполнено за сессию 07.08.2026](#выполнено-за-сессию-07082026)
9. [Запуск проекта](#запуск-проекта)

---

## Архитектура

```
┌─────────────────────────────────────────────────────────┐
│ Frontend (Vite + React + MUI + TanStack Query + TS)     │
│ localhost:5173                                          │
│                                                         │
│ src/pages/                                              │
│   Home.jsx — главная страница (6 карточек, CSS Grid)    │
│   Login.jsx — JWT-авторизация + Refresh Token cookie    │
│   InternetSales.jsx — интернет-продажи (DataTable)      │
│   PromoAnalysis.jsx — анализ промо (3 вкладки)          │
│   PromoForm.jsx — форма создания нового промо            │
│   PromoApproval.jsx — страница согласования              │
│     (переключатель Карточки/Таблица, mass-actions,       │
│     sticky-панель фильтров, настройка полей через        │
│     Drawer, предупреждение о смене статуса)              │
│                                                         │
│ src/components/                                         │
│   FilterPanel.jsx — панель фильтров (многоразовая)       │
│   DataTable.jsx — таблица с пагинацией + серверный поиск│
│   PromoEditDialog.jsx — диалог редактирования промо      │
│   ApprovalCard.jsx — карточка согласования               │
│     (localComment в useState, не триггерит ререндер      │
│     всей страницы, история comments, agreement1/2)       │
│   DrilldownModal.jsx — модал детализации                 │
│                                                         │
│ src/hooks/ (все на TypeScript)                          │
│   usePromoData.ts — данные промо (React Query)           │
│   usePromoFilters.ts — фильтры промо                     │
│   usePromoForm.ts — форма редактирования                 │
│   usePromoCalculations.ts — расчёт (через calcUtils)     │
│                                                         │
│ src/api/ (все на TypeScript)                            │
│   promo.ts — API-запросы (fetchWithAuth + auto-refresh) │
│   auth.ts — login/logout/refreshToken/saveSession       │
│                                                         │
│ src/utils/                                              │
│   cardFields.ts — FIELD_GROUPS, DEFAULT_VISIBLE_FIELDS   │
│   calcUtils.ts — calcPlan/calcActual (чистые функции)   │
│   calcUtils.test.ts — 16 тестов Vitest                  │
│                                                         │
│ src/types/                                              │
│   promo.ts — TypeScript-типы                             │
└─────────────────────────────────────────────────────────┘
    │ HTTP (CORS + AllowCredentials)
    ▼
┌─────────────────────────────────────────────────────────┐
│ Backend (Go + Gin)                                      │
│ localhost:8080                                          │
│ config/db.go — ensureTables (авто-DDL)                  │
│ handlers/                                               │
│   promo.go — SavePromo (аудит-лог), GetPromoComments    │
│   sales.go — buildSalesWhere (BrandExact для Drilldown) │
│ repository/                                              │
│   promo_repo.go — GetPromoRowsStream (Excel),           │
│     DiffPromoRows, InsertAuditLog, GetPromoComments      │
│ migrations/                                             │
│   003_create_tbl_promo_comments.sql                      │
│   004_create_tbl_audit_log.sql                           │
│ models/types.go — CommentRow, AuditLogRow               │
└─────────────────────────────────────────────────────────┘
    ▼
┌─────────────────────────────────────────────────────────┐
│ SQL Server (MSSQL)                                      │
│ dbo.tbl_PromoActivities                                 │
│ dbo.tbl_PromoComments (новая)                            │
│ dbo.tbl_AuditLog (новая)                                 │
│ dbo.tbl_Users, справочные таблицы                       │
└─────────────────────────────────────────────────────────┘
```

---

## Бэкенд — API эндпоинты

### Новые эндпоинты (добавлены 07.08.2026)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/promo/comments/:id` | Комментарии к промо из tbl_PromoComments |

---

## База данных

### Новые таблицы (миграции 003, 004)

**dbo.tbl_PromoComments**
| Column | Type | Description |
|--------|------|-------------|
| id | BIGINT (PK) | Auto-increment |
| promo_id | INT | FK → tbl_PromoActivities |
| user_name | NVARCHAR(100) | Кто написал |
| role | NVARCHAR(50) | КАМ / согласование1 / согласование2 |
| comment_text | NVARCHAR(MAX) | Текст комментария |
| created_at | DATETIME | Дата создания |

**dbo.tbl_AuditLog**
| Column | Type | Description |
|--------|------|-------------|
| id | BIGINT (PK) | Auto-increment |
| entity_type | NVARCHAR(50) | 'promo' |
| entity_id | INT | ID в tbl_PromoActivities |
| user_name | NVARCHAR(100) | Кто изменил |
| action_type | NVARCHAR(20) | CREATE/UPDATE/DELETE/APPROVE/REJECT |
| changed_fields | NVARCHAR(MAX) | JSON: {"field": {"old": val, "new": val}} |
| created_at | DATETIME | Дата изменения |

---

## Готово (Done) — обновлено 07.08.2026

### Этап 1: Стабилизация
- ✅ Drilldown SQL: `BrandExact`/`NetworkExact` вместо `strings.Replace`
- ✅ Streaming Excel: `excelize.StreamWriter` + `GetPromoRowsStream`
- ✅ Карточки: `localComment` в `useState` внутри `ApprovalCard`, убран `comments` стейт из `PromoApproval`
- ✅ Склейка комментариев: `\n` перед новой записью, `preserveStatus` для approved/rejected
- ✅ UI карточек: предупреждение о смене статуса, карточка уходит из очереди

### Этап 2: Audit Trail + Тесты
- ✅ `tbl_PromoComments` — нормализация комментариев (дублирование во все точки создания)
- ✅ `GET /api/promo/comments/:id`
- ✅ `tbl_AuditLog` — аудит изменений полей
- ✅ `DiffPromoRows` — сравнение old/new → JSON
- ✅ Интеграция в SavePromo (UPDATE), DeletePromo
- ✅ Авто-DDL: `ensureTables()` в `config/db.go`
- ✅ Vitest: 16 тестов `calcUtils.test.ts`
- ✅ `calcPlan`/`calcActual` в `utils/calcUtils.ts`

### Тесты
- **Бэкенд:** `cd backend && go test ./...` (интеграционные, требуют БД)
- **Фронтенд:** `cd frontend && npx vitest run` (16 unit-тестов)

---

## План доработок (оставшиеся задачи)

| # | Задача | Приоритет |
|---|--------|-----------|
| 1 | Полная миграция `.jsx` → `.tsx` | Средний |
| 2 | squirrel Query Builder вместо SQL-конкатенации | Средний |
| 3 | React Query refactor (убрать `setRows`) | Средний |
| 4 | `API_BASE` в env вместо хардкода | Низкий |
| 5 | E2E-тесты (Playwright) | Низкий |

---

## Выполнено за сессию 07.08.2026

### Бэкенд (`backend/`)
- `handlers/sales.go` — `salesFilter` расширен `BrandExact`/`NetworkExact`, `ExportSalesExcel` на StreamWriter
- `handlers/promo.go` — `ExportPromoExcel` на StreamWriter, аудит-лог в SavePromo/DeletePromo, `GetPromoCommentsHandler`, дублирование комментариев в новую таблицу
- `repository/promo_repo.go` — `GetPromoRowsStream`, `DiffPromoRows`, `InsertAuditLog`, `GetAuditLog`, `GetPromoComments`, `InsertComment`, `preserveStatus` в ApprovePromoWithStatus, исправлена склейка `\n` в комментариях
- `config/db.go` — `ensureTables()` (авто-DDL для `tbl_PromoComments` + `tbl_AuditLog`)
- `models/types.go` — `CommentRow`, `AuditLogRow`
- `main.go` — роут `GET /api/promo/comments/:id`
- `migrations/003_create_tbl_promo_comments.sql`
- `migrations/004_create_tbl_audit_log.sql`

### Фронтенд (`frontend/`)
- `components/ApprovalCard.jsx` — `localComment` в `useState`, новые сигнатуры колбэков
- `pages/PromoApproval.jsx` — убран `comments` стейт, предупреждение о смене статуса, `handleQuickAction`
- `components/PromoEditDialog.jsx` — синхронизирован `parseComments` с ApprovalCard
- `utils/calcUtils.ts` — чистые функции `calcPlan`/`calcActual`
- `utils/calcUtils.test.ts` — 16 тестов Vitest
- `hooks/usePromoCalculations.ts` — рефакторинг на использование `calcUtils`
- `package.json` — добавлен `vitest`

---

## Запуск проекта

### Бэкенд
```bash
cd backend
go build -o backend && ./backend
# → http://localhost:8080
```

### Фронтенд
```bash
cd frontend
npm run dev
# → http://localhost:5173
```

### Тесты бэкенда
```bash
cd backend && go test ./...
```

### Тесты фронтенда
```bash
cd frontend && npx vitest run
```

### Требования
- Go 1.21+
- Node.js 18+
- SQL Server (MSSQL)