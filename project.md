# Аналитический портал — Документация проекта

**Дата:** 03.08.2026
**Коммит:** текущий рабочий каталог (аудит + рефакторинг)
**Стек:** Go (Gin) + React (Vite + MUI) + SQL Server (MSSQL)
**Репозиторий:** github.com/romstan78/analytics-portal

---

## Оглавление

1. [Архитектура](#архитектура)
2. [Структура файлов](#структура-файлов)
3. [Бэкенд — API эндпоинты](#бэкенд--api-эндпоинты)
4. [База данных](#база-данных)
5. [Фронтенд — страницы](#фронтенд--страницы)
6. [Фронтенд — компоненты и хуки](#фронтенд--компоненты-и-хуки)
7. [Готово (Done)](#готово-done)
8. [Известные проблемы (Bugs)](#известные-проблемы-bugs)
9. [План доработок](#план-доработок)

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
│   PromoAnalysis.jsx — анализ промо (таблица + форма +    │
│     согласование, 3 вкладки)                             │
│   PromoForm.jsx — форма создания нового промо            │
│   PromoApproval.jsx — страница согласования              │
│                                                         │
│ src/components/                                         │
│   FilterPanel.jsx — панель фильтров (многоразовая)       │
│   DataTable.jsx — таблица с пагинацией                   │
│   PromoEditDialog.jsx — диалог редактирования промо      │
│     (agreement1/2 только read-only чипы,                 │
│     кнопки Закрыть/Сохранить, без авто-закрытия)        │
│   ApprovalCard.jsx — карточка согласования + skeleton    │
│   DrilldownModal.jsx — модал детализации                 │
│                                                         │
│ src/hooks/                                              │
│   usePromoData.js — данные промо (React Query)           │
│   usePromoFilters.js — фильтры промо                     │
│   usePromoForm.js — форма редактирования                 │
│   usePromoCalculations.js — расчёт плановых/фактических  │
│                                                         │
│ src/api/                                                │
│   promo.js — API-запросы (fetchWithAuth + auto-refresh) │
│   auth.js — login/logout/refreshToken/saveSession       │
│                                                         │
│ src/types/                                              │
│   promo.ts — TypeScript-типы: PromoRow, ApprovalRow,    │
│     FilterOptions, PromoFormData                         │
└─────────────────────────────────────────────────────────┘
    │ HTTP (CORS: из env CORS_ORIGINS, AllowCredentials)
    │ Access token: 15 мин в localStorage
    │ Refresh token: 7 дней в httpOnly cookie
    ▼
┌─────────────────────────────────────────────────────────┐
│ Backend (Go + Gin)                                      │
│ localhost:8080                                          │
│                                                         │
│ main.go — сервер, роуты, CORS, rate limiter (RWMutex)   │
│ config/                                                 │
│   db.go — подключение к SQL Server (25 connections)     │
│   auth.go — JWT: Access (15 мин) + Refresh (7 дней)     │
│ handlers/                                               │
│   promo.go — тонкие обработчики HTTP (делегируют в      │
│              services + repository)                     │
│   promo_utils.go — safeFloat/safeInt/safeString         │
│   sales.go — интернет-продажи                           │
│   auth.go — логин + рефреш (через БД + bcrypt)          │
│ services/             ← НОВЫЙ СЛОЙ                      │
│   promo_service.go — PromoInputDTO, EnrichFromRepo,     │
│     CalculateFields, MapToDTO, MergeCalculatedIntoMap   │
│ repository/                                              │
│   promo_repo.go — все SQL-запросы промо                 │
│   user_repo.go — запросы к tbl_Users                    │
│ middleware/                                              │
│   auth.go — AuthRequired + RoleRequired                 │
│ models/                                                 │
│   types.go — Row, PromoRow, ApprovalRow (с              │
│     agreement1_status/comment), NetworkGeo,             │
│     LastSKUData, HistoryRow, DrilldownRow               │
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
│   └── db.go               — подключение к SQL Server, пул соединений, slog
├── handlers/
│   ├── auth.go             — POST /api/auth/login (БД + bcrypt), POST /api/auth/refresh
│   ├── promo.go            — тонкие обработчики, делегируют в services + repository
│   ├── promo_utils.go      — safeFloat/safeInt/safeString (хелперы для map[string]interface{})
│   └── sales.go            — GetData / GetFilterOptions / GetDrilldown
├── services/                ← НОВЫЙ СЛОЙ
│   └── promo_service.go    — PromoInputDTO, CalculatedFields, EnrichFromRepo, CalculateFields
├── repository/
│   ├── promo_repo.go       — все SQL-запросы: CRUD, фильтры, согласование (agreement_status)
│   └── user_repo.go        — GetUserByUsername (из tbl_Users)
├── middleware/
│   └── auth.go             — AuthRequired + RoleRequired
├── migrations/
│   ├── 001_create_tbl_users.sql
│   ├── 002_split_agreement_fields.sql
│   └── seed_users.sql      — seed с реальными bcrypt-хешами
└── models/
    └── types.go            — Row, PromoRow, ApprovalRow (agreement1_status/comment),
                              NetworkGeo, LastSKUData, HistoryRow, DrilldownRow
```

### Frontend
```
frontend/src/
├── App.jsx                — роутинг, тема MUI (Modern)
├── main.jsx               — точка входа + QueryClientProvider
├── api/
│   ├── auth.js            — login/logout, refreshToken, saveSession
│   └── promo.js           — fetchWithAuth (авто-рефреш при 401), 18 методов API
├── components/
│   ├── ApprovalCard.jsx   — карточка согласования + LinearProgress + CircularProgress overlay
│   ├── DataTable.jsx      — таблица MUI DataGrid
│   ├── DrilldownModal.jsx — модал детализации
│   ├── FilterPanel.jsx    — панель фильтров с Autocomplete
│   └── PromoEditDialog.jsx — диалог редактирования (Закрыть/Сохранить, без авто-закрытия)
├── hooks/
│   ├── usePromoCalculations.js — расчёт плановых/фактических
│   ├── usePromoData.js         — загрузка данных (React Query)
│   ├── usePromoFilters.js      — фильтры с debounce
│   └── usePromoForm.js         — форма + сохранение
├── types/
│   └── promo.ts           — TypeScript-типы: PromoRow, ApprovalRow, FilterOptions, PromoFormData
└── pages/
    ├── Home.jsx            — главная страница
    ├── InternetSales.jsx   — интернет-продажи
    ├── Login.jsx           — страница входа (credentials: 'include')
    ├── PromoAnalysis.jsx   — анализ промо (3 вкладки)
    ├── PromoApproval.jsx   — согласование (фильтры перезапрашиваются после действий)
    └── PromoForm.jsx       — создание нового промо
```

---

## Бэкенд — API эндпоинты

### Auth
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/login` | No | JWT login (БД + bcrypt) → access token (JSON) + refresh token (httpOnly cookie) |
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
| GET | `/api/promo/filters` | JWT | Distinct filter options (7 параллельных запросов через errgroup) |
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
| GET | `/api/promo/approvals` | JWT | Approval list (фильтр по agreementN_status) |
| GET | `/api/promo/approval-filters` | JWT | Networks/brands/mechanics/kams (ролевая фильтрация) |
| GET | `/api/promo/approval-kams` | JWT | KAMs with pending approval |
| GET | `/api/promo/approval-networks` | JWT | Networks for KAM |
| GET | `/api/promo/approval-brands` | JWT | Brands for KAM+network |
| POST | `/api/promo/approve` | JWT | Approve/reject/comment (пишет в agreementN + agreementN_status + agreementN_comment) |

### Promo — Write
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/promo/save` | JWT + Roles | Create/Update (расчёт полей через services/promo_service.go) |
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

### Авторизация и безопасность
- ✅ JWT-авторизация с ролями: admin, agreement1, agreement2
- ✅ Middleware AuthRequired + RoleRequired
- ✅ **Access Token (15 мин) + Refresh Token (7 дней, httpOnly cookie)**
- ✅ **Авто-рефреш токена на фронтенде (fetchWithAuth при 401)**
- ✅ **Пользователи в БД (tbl_Users) + bcrypt-хеширование паролей (cost=10)**
- ✅ **Утилита cmd/hash_password.go для генерации хешей**
- ✅ Rate limiter (100 запросов/мин на IP, sync.RWMutex)
- ✅ Structured logging (slog + lumberjack, ротация логов)
- ✅ **CORS из переменной окружения (`CORS_ORIGINS`) + AllowCredentials: true**

### Просмотр и редактирование промо
- ✅ DataGrid с пагинацией, сортировкой, поиском, экспортом CSV
- ✅ Фильтры с Autocomplete + debounce 300ms
- ✅ Сохранение состояния фильтров (sessionStorage, localStorage)
- ✅ CRUD промо: создание, редактирование, удаление (soft delete)
- ✅ **Optimistic Locking: WHERE updated_at = ?, при конфликте → HTTP 409**
- ✅ Автообновление UI после редактирования/удаления
- ✅ **agreement1/agreement2 — только read-only чипы в диалоге, защита на бэкенде**
- ✅ **PromoEditDialog: кнопки Закрыть/Сохранить, форма остаётся после сохранения**

### Согласование
- ✅ Карточки промо в CSS Grid (React.memo для производительности)
- ✅ Три действия: Комментарий / Согласовано / Отклонено
- ✅ Перекрёстная каскадная фильтрация (справочники перезапрашиваются после действий)
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
- ✅ **services/promo_service.go: PromoInputDTO, EnrichFromRepo, CalculateFields**
- ✅ **promo_utils.go очищен от бизнес-логики и прямых SQL-запросов**
- ✅ repository/promo_repo.go — все SQL-запросы
- ✅ repository/user_repo.go — запросы к tbl_Users + bcrypt
- ✅ GetPromoFilters: 7 параллельных запросов через errgroup
- ✅ Модели: NetworkGeo, LastSKUData, ApprovalRow с agreementN_status/comment

### Фронтенд-архитектура
- ✅ **TanStack Query (React Query) в usePromoData.js**
- ✅ **TypeScript-типы в types/promo.ts**
- ✅ **QueryClientProvider в main.jsx**

### Другое
- ✅ Главная страница (6 карточек, CSS Grid)
- ✅ Интернет-продажи (FilterPanel + DataTable + DrilldownModal)
- ✅ Основная документация в project.md
- ✅ **Миграции с реальными bcrypt-хешами**

---

## Известные проблемы (Bugs)

### Исправлены ✅
1. ~~usePromoData: JSON.stringify в зависимостях → лишние HTTP-запросы~~ → React Query
2. ~~Rate limiter: глобальный sync.Mutex~~ → sync.RWMutex
3. ~~Memory leak: commentRefs в PromoApproval~~ → исправлено
4. ~~409 Conflict~~ → Optimistic Locking с корректной обработкой
5. ~~GetPromoFilters: 7 последовательных запросов~~ → errgroup
6. ~~ApprovalCard: преждевременное скрытие~~ → skeleton overlay
7. ~~Фильтр по бренду: .includes(sku)~~ → точное совпадение
8. ~~CORS: хардкод~~ → env CORS_ORIGINS
9. ~~JWT без refresh token~~ → Access + Refresh httpOnly cookie
10. ~~agreement1/2 редактируемы~~ → read-only чипы
11. ~~Хардкод пользователей в handlers/auth.go~~ → БД + bcrypt
12. ~~SQL-запросы в calculatePromoFields~~ → services/promo_service.go
13. ~~CHARINDEX-парсинг согласований~~ → agreementN_status + agreementN_comment
14. ~~Авто-закрытие диалога после сохранения~~ → остаётся открытым
15. ~~Фильтры не обновлялись после согласования~~ → setRefreshFilters

### Остаются (P2)
16. **Нет тестов на фронтенд** — только бэкенд main_test.go
17. **map[string]interface{} в Save/Update** — нужен полный переход на PromoInputDTO в репозитории
18. **GetApprovalFilters всё ещё принимает brand/network/mechanics параметры** — фронтенд их не шлёт, но бэкенд обрабатывает

---

## План доработок

### Ближайшие
| # | Задача | Оценка |
|---|--------|--------|
| 1 | Полный переход на PromoInputDTO в repository (убрать map[string]interface{}) | 1 час |
| 2 | Счётчик обработанных промо в согласовании | 15 мин |
| 3 | Сортировка карточек по ROI / дате / сети | 20 мин |
| 4 | Цветовая индикация убыточных промо | 10 мин |
| 5 | Экспорт в CSV из согласования | 15 мин |

### Технический долг
| # | Задача | Оценка |
|---|--------|--------|
| 6 | Тесты на фронтенд (Jest + React Testing Library) | 3-4 дня |
| 7 | Миграция оставшихся JSX-файлов на TypeScript | 1-2 дня |
| 8 | Удалить старый ApprovePromo, оставить только ApprovePromoWithStatus | 15 мин |

### Будущий функционал
| # | Задача | Оценка |
|---|--------|--------|
| 9 | Дашборд с графиками (ROI, uplift, план/факт) | TBD |
| 10 | Мобильная версия | TBD |
| 11 | Нормализация словарей (ID вместо строк) | TBD |

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