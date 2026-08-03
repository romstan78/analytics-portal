# Аналитический портал — Документация проекта

**Дата:** 03.08.2026
**Коммит:** fix: Roadmap аудита — 0 vs NULL, аудит-лог, фильтры согласования, Confirm Dialog, TS-хук
**Стек:** Go (Gin) + React (Vite + MUI) + SQL Server (MSSQL)
**Репозиторий:** github.com/romstan78/analytics-portal

---

## Оглавление

1. [Архитектура](#архитектура)
2. [Структура файлов](#структура-файлов)
3. [Бэкенд — API эндпоинты](#бэкенд--api-эндпоинты)
4. [База данных](#база-данных)
5. [Готово (Done)](#готово-done)
6. [План доработок](#план-доработок)

---

## Архитектура

```
┌─────────────────────────────────────────────────────────┐
│ Frontend (Vite + React + MUI + TanStack Query)          │
│ localhost:5173                                          │
│                                                         │
│ src/pages/                                              │
│   Home.jsx — главная страница (6 карточек, CSS Grid)    │
│   Login.jsx — JWT-авторизация + Refresh Token cookie    │
│   InternetSales.jsx — интернет-продажи (DataTable)       │
│   PromoAnalysis.jsx — анализ промо (3 вкладки)          │
│   PromoForm.jsx — форма создания нового промо            │
│   PromoApproval.jsx — страница согласования              │
│                                                         │
│ src/components/                                         │
│   FilterPanel.jsx — панель фильтров (многоразовая)       │
│   DataTable.jsx — таблица с пагинацией                   │
│   PromoEditDialog.jsx — диалог редактирования промо      │
│     (agreement1/2 только read-only чипы,                 │
│     кнопки Закрыть/Сохранить, без авто-закрытия,        │
│     Confirm Dialog для согласующих)                     │
│   ApprovalCard.jsx — карточка согласования + skeleton    │
│   DrilldownModal.jsx — модал детализации                 │
│                                                         │
│ src/hooks/                                              │
│   usePromoData.ts — данные промо (React Query)           │
│   usePromoFilters.js — фильтры промо                     │
│   usePromoForm.ts — форма редактирования                 │
│   usePromoCalculations.ts — расчёт плановых/фактических  │
│                                                         │
│ src/api/                                                │
│   promo.js — API-запросы (fetchWithAuth + auto-refresh) │
│   auth.js — login/logout/refreshToken/saveSession       │
│                                                         │
│ src/types/                                              │
│   promo.ts — TypeScript-типы: PromoRow, ApprovalRow,    │
│     FilterOptions, PromoFormData                         │
│ tsconfig.json — конфигурация TypeScript (allowJs)       │
└─────────────────────────────────────────────────────────┘
    │ HTTP (CORS: из env CORS_ORIGINS, AllowCredentials)
    │ Access token: 15 мин в localStorage
    │ Refresh token: 7 дней в httpOnly cookie (Secure на проде)
    ▼
┌─────────────────────────────────────────────────────────┐
│ Backend (Go + Gin)                                      │
│ localhost:8080                                          │
│                                                         │
│ main.go — сервер, роуты, CORS, rate limiter (RWMutex)   │
│ config/                                                 │
│   db.go — подключение к SQL Server (25 connections)     │
│   auth.go — JWT: Access (15 мин) + Refresh (7 дней)     │
│   cache.go — InMemoryCache, FilterCacheTTL (5 мин)      │
│ handlers/                                               │
│   promo.go — тонкие обработчики HTTP + applyJSONToRow   │
│     (защита Mass Assignment, аудит-лог с username)       │
│   sales.go — интернет-продажи                           │
│   auth.go — логин + рефреш (БД + bcrypt, Secure-флаг)   │
│ services/                                               │
│   promo_service.go — PromoInputDTO, CalculatedFields,   │
│     EnrichFromRepo, CalculateFields,                     │
│     MergeCalculatedIntoDBRow, DTOToDBRow, DBRowToDTO,   │
│     MapToDTO, DBRowToMap                                │
│ repository/                                              │
│   promo_repo.go — все SQL-запросы промо (типизирован)   │
│     GetApprovalFilters: 4 горутины errgroup +            │
│     buildApprovalWhere(excludeCol) перекрёстная          │
│     фильтрация                                          │
│   user_repo.go — запросы к tbl_Users                    │
│ middleware/                                              │
│   auth.go — AuthRequired + RoleRequired                 │
│ models/                                                 │
│   types.go — Row, PromoRow, PromoRowDB (числовые поля   │
│     — указатели *float64/*int для NULL vs 0),           │
│     PtrFloat/PtrInt/ValFloat/ValInt хелперы,            │
│     ApprovalRow, NetworkGeo, LastSKUData,                │
│     HistoryRow, DrilldownRow                            │
│ migrations/                                             │
│   001_create_tbl_users.sql — таблица пользователей       │
│   002_split_agreement_fields.sql — разделение полей      │
│     согласования на status + comment                    │
│   seed_users.sql — seed с реальными bcrypt-хешами       │
│ cmd/                                                    │
│   hash_password.go — утилита генерации bcrypt-хешей     │
└─────────────────────────────────────────────────────────┘
    │ database/sql
    ▼
┌─────────────────────────────────────────────────────────┐
│ SQL Server (MSSQL)                                      │
│                                                         │
│ dbo.tbl_PromoActivities — промо-акции                    │
│   (новые поля: agreement1_status, agreement1_comment,   │
│    agreement2_status, agreement2_comment)               │
│ dbo.tbl_EcomSalesNormalized — интернет-продажи           │
│ dbo.tbl_MechanicsChannelMapping — механика → канал       │
│ dbo.tbl_ChannelSegmentMapping — канал ↔ сегмент          │
│ dbo.tbl_KAMNetworkMapping — KAM ↔ сеть                  │
│ dbo.tbl_NetworkGeoMapping — сеть → гео-данные            │
│ dbo.tbl_SKUMapping — SKU → бренд                        │
│ dbo.tbl_Users — пользователи (bcrypt-хеши, soft-delete)  │
└─────────────────────────────────────────────────────────┘
```

---

## Структура файлов

### Backend
```
backend/
├── main.go                 — сервер, роутинг, CORS, rate limiter
├── main_test.go            — тесты
├── cmd/
│   └── hash_password.go    — утилита: go run cmd/hash_password.go <пароль>
├── config/
│   ├── auth.go             — JWT: GenerateAccessToken, GenerateRefreshToken, ValidateToken
│   ├── cache.go            — InMemoryCache (RWMutex, TTL 5 мин) для фильтров
│   └── db.go               — подключение к SQL Server, пул соединений, slog
├── handlers/
│   ├── auth.go             — POST /api/auth/login (БД + bcrypt), POST /api/auth/refresh
│   ├── promo.go            — тонкие обработчики + applyJSONToRow (защита Mass Assignment, аудит-лог)
│   └── sales.go            — GetData / GetFilterOptions / GetDrilldown
├── services/
│   └── promo_service.go    — PromoInputDTO, CalculatedFields, EnrichFromRepo, CalculateFields,
│                             MergeCalculatedIntoDBRow, DTOToDBRow, DBRowToDTO, MapToDTO, DBRowToMap
├── repository/
│   ├── promo_repo.go       — все SQL-запросы: FetchExistingRow (*PromoRowDB), UpdatePromo, InsertPromo,
│   │                         фильтры, согласование (GetApprovals с фильтрацией в SQL,
│   │                         GetApprovalFilters: 4 горутины + buildApprovalWhere(excludeCol))
│   └── user_repo.go        — GetUserByUsername (из tbl_Users)
├── middleware/
│   └── auth.go             — AuthRequired + RoleRequired
├── migrations/
│   ├── 001_create_tbl_users.sql
│   ├── 002_split_agreement_fields.sql
│   └── seed_users.sql      — seed с реальными bcrypt-хешами
└── models/
    └── types.go            — Row, PromoRow, PromoRowDB (числовые поля — указатели для NULL vs 0),
                              ApprovalRow (agreement1_status/comment), NetworkGeo,
                              LastSKUData, HistoryRow, DrilldownRow,
                              PtrFloat/PtrInt/ValFloat/ValInt (хелперы указателей)
```

### Frontend
```
frontend/src/
├── App.jsx                — роутинг, тема MUI (Modern)
├── main.jsx               — точка входа + QueryClientProvider
├── tsconfig.json          — TypeScript-конфигурация (allowJs, strict)
├── api/
│   ├── auth.js            — login/logout, refreshToken, saveSession
│   └── promo.js           — fetchWithAuth (авто-рефреш при 401), 18 методов API
├── components/
│   ├── ApprovalCard.jsx   — карточка согласования + LinearProgress + CircularProgress overlay
│   ├── DataTable.jsx      — таблица MUI DataGrid
│   ├── DrilldownModal.jsx — модал детализации
│   ├── FilterPanel.jsx    — панель фильтров с Autocomplete
│   └── PromoEditDialog.jsx — диалог редактирования (Закрыть/Сохранить, Confirm Dialog для согласующих)
├── hooks/
│   ├── usePromoCalculations.ts — расчёт плановых/фактических (типизирован, PlanFields/ActualFields)
│   ├── usePromoData.ts         — загрузка данных (React Query, типизирован)
│   ├── usePromoFilters.js      — фильтры с debounce
│   └── usePromoForm.ts         — форма + сохранение (типизирован, 0 vs NULL исправлен)
├── types/
│   └── promo.ts           — TypeScript-типы: PromoRow, ApprovalRow, FilterOptions, PromoFormData
└── pages/
    ├── Home.jsx            — главная страница
    ├── InternetSales.jsx   — интернет-продажи
    ├── Login.jsx           — страница входа (credentials: 'include')
    ├── PromoAnalysis.jsx   — анализ промо (3 вкладки)
    ├── PromoApproval.jsx   — согласование (перекрёстная фильтрация, без клиентского .filter())
    └── PromoForm.jsx       — создание нового промо
```

---

## Бэкенд — API эндпоинты

### Auth
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/login` | No | JWT login (БД + bcrypt) → access token (JSON) + refresh token (httpOnly cookie, Secure на проде) |
| POST | `/api/auth/refresh` | No (cookie) | Обновление access + refresh токенов |

### Internet Sales
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/data` | JWT | Paginated data |
| GET | `/api/data?all=true` | JWT | All data (export) |
| GET | `/api/filters` | JWT | Filter options + segment↔channel mapping |
| GET | `/api/drilldown` | JWT | Drilldown by brand+network |

### Promo — Read
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/promo/filters` | JWT | Distinct filter options (7 параллельных запросов через errgroup, кэш 5 мин) |
| GET | `/api/promo/data` | JWT | All promo rows with filtering, pagination |
| GET | `/api/promo/sku-by-brand` | JWT | SKUs for brand |
| GET | `/api/promo/last-contract-price` | JWT | Last contract price for SKU |
| GET | `/api/promo/investment-types` | JWT | Fixed list: GTN, GTN в ОС, OPEX, OPEX Marketing |
| GET | `/api/promo/kam-by-network` | JWT | KAM for network |
| GET | `/api/promo/last-network-data` | JWT | Pharmacy count for network |
| GET | `/api/promo/network-geo` | JWT | Geo mapping for network |
| GET | `/api/promo/history` | JWT | Top-10 history rows |
| GET | `/api/promo/sku-info` | JWT | Brand for SKU |
| GET | `/api/promo/last-sku-data` | JWT | Latest contract_price, gm, olap_price, key_region, top20_segment |

### Promo — Approval
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/promo/approvals` | JWT | Approval list (фильтрация в SQL: kam, network_name, brand, mechanics, year, month) |
| GET | `/api/promo/approval-filters` | JWT | Networks/brands/mechanics/kams (4 горутины errgroup, перекрёстная фильтрация excludeCol) |
| GET | `/api/promo/approval-kams` | JWT | KAMs with pending approval |
| GET | `/api/promo/approval-networks` | JWT | Networks for KAM |
| GET | `/api/promo/approval-brands` | JWT | Brands for KAM+network |
| POST | `/api/promo/approve` | JWT | Approve/reject/comment (пишет в agreementN + agreementN_status + agreementN_comment) |

### Promo — Write
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/promo/save` | JWT + Roles | Create/Update через типизированные структуры PromoRowDB, защита от Mass Assignment |
| DELETE | `/api/promo/:id` | JWT + admin | Soft-delete promo |

---

## База данных

### dbo.tbl_PromoActivities (основная таблица)
| Column | Type | Description |
|--------|------|-------------|
| id | int (PK) | Auto-increment |
| network_name | nvarchar | Торговая сеть |
| kam | nvarchar | Key Account Manager |
| brand / brand_as | nvarchar | Бренд |
| sku | nvarchar | SKU |
| year / month / quarter | int | Период |
| mechanics | nvarchar | Механика |
| gtn_opex | nvarchar | Тип инвестиций |
| baseline_units / baseline_rub | float | Baseline |
| plan_promo_units / plan_promo_rub | float | План |
| actual_promo_sales_units / actual_promo_rub | float | Факт |
| plan_investments_rub / actual_investments | float | Инвестиции |
| plan_roi / actual_roi | float | ROI |
| agreement1 / agreement2 | nvarchar | Статус согласования (legacy, для обратной совместимости) |
| **agreement1_status / agreement2_status** | nvarchar(20) | Статус: pending/approved/rejected/commented |
| **agreement1_comment / agreement2_comment** | nvarchar(max) | Текст комментария |
| status | nvarchar | Статус промо |
| deleted_at | datetime | Soft delete |
| updated_at | datetime | Обновлён (используется для Optimistic Locking) |
| created_at | datetime | Создан |

### dbo.tbl_Users (пользователи)
| Column | Type | Description |
|--------|------|-------------|
| id | int (PK) | Auto-increment |
| username | nvarchar(100) | Уникальный логин |
| password_hash | nvarchar(255) | bcrypt-хеш (cost=10) |
| role | nvarchar(50) | Роль: admin/agreement1/agreement2 |
| created_at / updated_at | datetime | Даты |
| deleted_at | datetime | Soft delete |

### Вспомогательные таблицы
- `tbl_EcomSalesNormalized` — интернет-продажи
- `tbl_MechanicsChannelMapping` — механика → канал
- `tbl_ChannelSegmentMapping` — канал ↔ сегмент
- `tbl_KAMNetworkMapping` — KAM ↔ сеть
- `tbl_NetworkGeoMapping` — сеть → key_region, top20_segment, kam, network_type
- `tbl_SKUMapping` — SKU → brand, brand_as

---

## Готово (Done)

### Безопасность
- ✅ JWT-авторизация с ролями: admin, agreement1, agreement2
- ✅ Middleware AuthRequired + RoleRequired
- ✅ **Access Token (15 мин) + Refresh Token (7 дней, httpOnly cookie)**
- ✅ **Secure-флаг куки: `ENV=production` — только HTTPS**
- ✅ **Защита от Mass Assignment: поля agreement* и status исключены в applyJSONToRow**
- ✅ Авто-рефреш токена на фронтенде (fetchWithAuth при 401)
- ✅ **Аудит-лог: promo_updated/created/deleted/approved — реальный username из JWT**
- ✅ Пользователи в БД (tbl_Users) + bcrypt-хеширование паролей (cost=10)
- ✅ Утилита cmd/hash_password.go для генерации хешей
- ✅ Rate limiter (100 запросов/мин на IP, sync.RWMutex)
- ✅ Structured logging (slog + lumberjack, ротация логов)
- ✅ **CORS из переменной окружения (`CORS_ORIGINS`) + AllowCredentials: true**

### Целостность данных
- ✅ **0 vs NULL: usePromoForm.ts — `x !== '' ? parseFloat(x) : null` вместо `parseFloat(x) \|\| null`**
- ✅ **PromoRowDB: 40+ числовых полей переведены на указатели (`*float64`, `*int`)**
- ✅ **FetchExistingRow: сканирование через `sql.Null*` с проверкой `.Valid`**
- ✅ **applyJSONToRow: присваивание числовых полей через `&val`**
- ✅ **Хелперы PtrFloat/PtrInt/ValFloat/ValInt в models/types.go**

### Просмотр и редактирование промо
- ✅ DataGrid с пагинацией, сортировкой, поиском, экспортом CSV
- ✅ Фильтры с Autocomplete + debounce 300ms
- ✅ Сохранение состояния фильтров (sessionStorage, localStorage)
- ✅ CRUD промо: создание, редактирование, удаление (soft delete)
- ✅ **Optimistic Locking: WHERE updated_at = ?, при конфликте → HTTP 409**
- ✅ Автообновление UI после редактирования/удаления
- ✅ **agreement1/agreement2 — только read-only чипы в диалоге, защита на бэкенде**
- ✅ **PromoEditDialog: кнопки Закрыть/Сохранить, форма остаётся после сохранения**
- ✅ **PromoEditDialog: Confirm Dialog для согласующих (agreement1/agreement2) при сохранении**

### Согласование
- ✅ Карточки промо в CSS Grid (React.memo для производительности)
- ✅ Три действия: Комментарий / Согласовано / Отклонено
- ✅ **Перекрёстная фильтрация: 4 горутины errgroup, buildApprovalWhere(excludeCol)**
- ✅ **GetApprovals фильтрует network_name/brand/mechanics в SQL (не на клиенте)**
- ✅ **Фронтенд передаёт все applied-фильтры в API, без клиентского .filter()**
- ✅ Фильтр «Состояние согласования» (5 состояний)
- ✅ Кнопка «Применить» (контроль момента загрузки)
- ✅ Защита от загрузки без фильтров
- ✅ Отображение комментариев обоих согласующих в карточке
- ✅ **Индикация загрузки: LinearProgress + CircularProgress overlay при отправке**
- ✅ **Ролевая фильтрация: только свой статус (agreement1_status/agreement2_status)**
- ✅ **Новые поля: agreementN_status + agreementN_comment (вместо CHARINDEX-парсинга)**
- ✅ Год по умолчанию = текущий

### Архитектура
- ✅ **Четырёхслойная архитектура: handlers → services → repository → DB**
- ✅ **PromoRowDB — типизированная структура (0 map[string]interface{} в Save/Update)**
- ✅ **services/promo_service.go: DTOToDBRow, DBRowToDTO, MergeCalculatedIntoDBRow, MapToDTO, DBRowToMap**
- ✅ **config/cache.go: InMemoryCache (RWMutex, TTL 5 мин) для GetPromoFilters**
- ✅ repository/promo_repo.go — все SQL-запросы (FetchExistingRow/UpdatePromo/InsertPromo на структурах)
- ✅ repository/user_repo.go — запросы к tbl_Users + bcrypt
- ✅ GetPromoFilters: 7 параллельных запросов через errgroup
- ✅ GetApprovalFilters: 4 параллельных запроса через errgroup + excludeCol
- ✅ Среднее время загрузки страницы ~2.5 сек (без учёта MSSQL-планов)
- ✅ Модели: NetworkGeo, LastSKUData, ApprovalRow, PromoRowDB

### Фронтенд-архитектура
- ✅ **TanStack Query (React Query) в usePromoData.ts**
- ✅ **TypeScript-типы в types/promo.ts**
- ✅ **tsconfig.json: strict-режим, allowJs для постепенной миграции**
- ✅ **Хуки usePromoData.ts, usePromoForm.ts и usePromoCalculations.ts типизированы**
- ✅ **QueryClientProvider в main.jsx**

### Удалённый код
- ✅ `promo_utils.go` удалён (safeFloat/safeInt/safeString не нужны)
- ✅ `usePromoCalculations.js` удалён (заменён на .ts)

---

## План доработок

### Ближайшие
| # | Задача | Оценка |
|---|--------|--------|
| 1 | Счётчик обработанных промо в согласовании | 15 мин |
| 2 | Сортировка карточек по ROI / дате / сети | 20 мин |
| 3 | Цветовая индикация убыточных промо | 10 мин |
| 4 | Экспорт в CSV из согласования | 15 мин |

### Технический долг
| # | Задача | Оценка |
|---|--------|--------|
| 5 | Тесты на фронтенд (Jest + React Testing Library) | 3-4 дня |
| 6 | Миграция оставшихся JSX-файлов на TypeScript | 1-2 дня |

### Будущий функционал
| # | Задача | Оценка |
|---|--------|--------|
| 7 | Дашборд с графиками (ROI, uplift, план/факт) | TBD |
| 8 | Мобильная версия | TBD |
| 9 | Нормализация словарей (ID вместо строк) | TBD |

---

## Запуск проекта

### Бэкенд
```bash
cd backend
go build -o backend
./backend
# → http://localhost:8080
```

### Фронтенд
```bash
cd frontend
npm run dev
# → http://localhost:5173
```

### Генерация bcrypt-хеша
```bash
cd backend
go run cmd/hash_password.go promo2024!
# → $2a$10$...
```

### Переменные окружения (`backend/.env`)
```
DB_SERVER=localhost
DB_USER=sa
DB_PASSWORD=your_password_here
DB_NAME=local_project_db
DB_PORT=1433
CORS_ORIGINS=http://localhost:5173
JWT_SECRET=your-secret-here
ENV=development
```

### Требования
- Go 1.21+
- Node.js 18+
- SQL Server (MSSQL)

### Учётные записи
| Логин | Пароль | Роль |
|-------|--------|------|
| admin | admin2024! | admin |
| manager1 | promo2024! | agreement1 |
| manager2 | promo2024! | agreement2 |