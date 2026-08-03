# Аналитический портал — Документация проекта

**Дата:** 03.08.2026  
**Коммит:** текущий рабочий каталог (незакоммиченные изменения)  
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
│ Frontend (Vite + React + MUI)                           │
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
│     (agreement1/2 только read-only чипы)                │
│   ApprovalCard.jsx — карточка согласования + skeleton    │
│   DrilldownModal.jsx — модал детализации                 │
│                                                         │
│ src/hooks/                                              │
│   usePromoData.js — загрузка данных промо                │
│   usePromoFilters.js — фильтры промо                     │
│   usePromoForm.js — форма редактирования                 │
│   usePromoCalculations.js — расчёт плановых/фактических  │
│                                                         │
│ src/api/                                                │
│   promo.js — API-запросы (fetchWithAuth + auto-refresh) │
│   auth.js — login/logout/refreshToken/saveSession       │
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
│   promo.go (~330 строк) — тонкие обработчики HTTP       │
│   promo_utils.go — safeFloat/safeInt/calculatePromoFields│
│   sales.go — интернет-продажи                           │
│   auth.go — логин + рефреш токен                        │
│ repository/           ← НОВЫЙ СЛОЙ                      │
│   promo_repo.go (~850 строк) — все SQL-запросы          │
│ middleware/                                              │
│   auth.go — AuthRequired + RoleRequired                 │
│ models/                                                 │
│   types.go — Row, PromoRow, ApprovalRow, NetworkGeo,    │
│     LastSKUData, HistoryRow, DrilldownRow               │
└─────────────────────────────────────────────────────────┘
    │ database/sql
    ▼
┌─────────────────────────────────────────────────────────┐
│ SQL Server (MSSQL)                                      │
│                                                         │
│ dbo.tbl_PromoActivities — промо-акции                    │
│ dbo.tbl_EcomSalesNormalized — интернет-продажи           │
│ dbo.tbl_MechanicsChannelMapping — механика → канал       │
│ dbo.tbl_ChannelSegmentMapping — канал ↔ сегмент          │
│ dbo.tbl_KAMNetworkMapping — KAM ↔ сеть                  │
│ dbo.tbl_NetworkGeoMapping — сеть → гео-данные            │
│ dbo.tbl_SKUMapping — SKU → бренд                        │
│ dbo.tbl_Users — пользователи (JWT)                      │
└─────────────────────────────────────────────────────────┘
```

---

## Структура файлов

### Backend
```
backend/
├── main.go                 (~160 строк) — сервер, роутинг, CORS, rate limiter
├── main_test.go            — тесты
├── config/
│   ├── auth.go             — JWT: GenerateAccessToken (15 мин), GenerateRefreshToken (7 дней),
│   │                         ValidateToken, ValidateRefreshToken
│   └── db.go               — подключение к SQL Server, пул соединений
├── handlers/
│   ├── auth.go             — POST /api/auth/login, POST /api/auth/refresh
│   ├── promo.go            (~330 строк) — тонкие обработчики, делегируют в repository
│   ├── promo_utils.go      (202 строки) — safeFloat/safeInt/calculatePromoFields
│   └── sales.go            (293 строки) — GetData / GetFilterOptions / GetDrilldown
├── repository/             ← НОВЫЙ СЛОЙ
│   └── promo_repo.go       (~850 строк) — все SQL-запросы: CRUD, фильтры, согласование
├── middleware/
│   └── auth.go             — AuthRequired + RoleRequired
└── models/
    └── types.go            — Row, PromoRow, ApprovalRow (с brand_as), NetworkGeo,
                              LastSKUData, HistoryRow, DrilldownRow
```

### Frontend
```
frontend/src/
├── App.jsx                — роутинг, тема MUI (Modern)
├── main.jsx               — точка входа
├── api/
│   ├── auth.js            — login/logout, refreshToken, saveSession
│   └── promo.js           — fetchWithAuth (авто-рефреш при 401), 18 методов API
├── components/
│   ├── ApprovalCard.jsx   — карточка согласования + LinearProgress + CircularProgress overlay
│   ├── DataTable.jsx      — таблица MUI DataGrid
│   ├── DrilldownModal.jsx — модал детализации
│   ├── FilterPanel.jsx    — панель фильтров с Autocomplete
│   └── PromoEditDialog.jsx — диалог редактирования (agreement1/2 — только read-only чипы)
├── hooks/
│   ├── usePromoCalculations.js — расчёт плановых/фактических
│   ├── usePromoData.js         — загрузка данных
│   ├── usePromoFilters.js      — фильтры с debounce
│   └── usePromoForm.js         — форма + сохранение
└── pages/
    ├── Home.jsx            — главная страница
    ├── InternetSales.jsx   — интернет-продажи
    ├── Login.jsx           — страница входа (credentials: 'include')
    ├── PromoAnalysis.jsx   — анализ промо (3 вкладки)
    ├── PromoApproval.jsx   — согласование (год по умолчанию — текущий)
    └── PromoForm.jsx       — создание нового промо
```

---

## Бэкенд — API эндпоинты

### Auth
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/login` | No | JWT login → access token (JSON) + refresh token (httpOnly cookie) |
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
| GET | `/api/promo/approvals` | JWT | Approval list (returns brand_as) |
| GET | `/api/promo/approval-filters` | JWT | Networks/brands/mechanics/kams (cross-filtered) |
| GET | `/api/promo/approval-kams` | JWT | KAMs with pending approval |
| GET | `/api/promo/approval-networks` | JWT | Networks for KAM |
| GET | `/api/promo/approval-brands` | JWT | Brands for KAM+network |
| POST | `/api/promo/approve` | JWT | Approve/reject/comment |

### Promo — Write
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/promo/save` | JWT + Roles | Create/Update (agreement1/2 — только через approve, игнорируются при save) |
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
| agreement1 / agreement2 | nvarchar | Статус согласования (только через approve) |
| status | nvarchar | Статус промо |
| deleted_at | datetime | Soft delete |
| updated_at | datetime | Обновлён (используется для Optimistic Locking) |
| created_at | datetime | Создан |

### Вспомогательные таблицы
- `tbl_EcomSalesNormalized` — интернет-продажи (brandName, productName, networkName, metric_type, metric_value, un_rub, segment, channel)
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

### Согласование
- ✅ Карточки промо в CSS Grid (React.memo для производительности)
- ✅ Три действия: Комментарий / Согласовано / Отклонено
- ✅ Перекрёстная каскадная фильтрация
- ✅ Фильтр «Состояние согласования» (5 состояний)
- ✅ Кнопка «Применить» (контроль момента загрузки)
- ✅ Защита от загрузки без фильтров
- ✅ Отображение комментариев обоих согласующих в карточке
- ✅ **Индикация загрузки: LinearProgress + CircularProgress overlay при отправке**
- ✅ **Фильтр по бренду через brand_as (точное совпадение)**
- ✅ **Год по умолчанию = текущий**
- ✅ SQL: CHARINDEX для Unicode-поиска, TOP 500, фильтр по дате

### Архитектура
- ✅ **Трёхслойная архитектура: handlers → repository → DB**
- ✅ **promo.go сокращён с 1087 до ~330 строк**
- ✅ **repository/promo_repo.go — все SQL-запросы**
- ✅ **GetPromoFilters: 7 параллельных запросов через errgroup**
- ✅ **Модели: NetworkGeo, LastSKUData, ApprovalRow с brand_as**

### Другое
- ✅ Главная страница (6 карточек, CSS Grid)
- ✅ Интернет-продажи (FilterPanel + DataTable + DrilldownModal)
- ✅ Основная документация в project.md

---

## Известные проблемы (Bugs)

### Исправлены ✅
1. ~~usePromoData: JSON.stringify в зависимостях → лишние HTTP-запросы~~ → исправлено
2. ~~Rate limiter: глобальный sync.Mutex~~ → исправлено (sync.RWMutex)
3. ~~Memory leak: commentRefs в PromoApproval~~ → исправлено
4. ~~Форма редактирования: 409 Conflict~~ → исправлено (убран WHERE updated_at, потом возвращён с корректной обработкой)
5. ~~GetPromoFilters: 7 последовательных SQL-запросов~~ → исправлено (errgroup, параллельные горутины)
6. ~~ApprovalCard: преждевременное скрытие~~ → исправлено (дождаться HTTP 200 + skeleton overlay)
7. ~~Фильтр по бренду: .includes(sku)~~ → исправлено (точное совпадение brand_as)
8. ~~CORS: хардкод localhost:5173~~ → исправлено (переменная окружения CORS_ORIGINS + AllowCredentials)
9. ~~JWT без refresh token~~ → исправлено (Access 15мин + Refresh 7дней httpOnly cookie)
10. ~~agreement1/2 редактируемы в форме~~ → исправлено (read-only чипы + защита на бэкенде)

### Остаются (P2)
11. **Нет тестов на фронтенд** — только бэкенд main_test.go
12. **Бизнес-логика в promo_utils.go** — calculatePromoFields всё ещё в handlers (не в services/)

---

## План доработок

### Ближайшие
| # | Задача | Оценка |
|---|--------|--------|
| 1 | Вынести calculatePromoFields в services/ | 30 мин |
| 2 | Счётчик обработанных промо в согласовании | 15 мин |
| 3 | Сортировка карточек по ROI / дате / сети | 20 мин |
| 4 | Цветовая индикация убыточных промо | 10 мин |
| 5 | Экспорт в CSV из согласования | 15 мин |

### Технический долг
| # | Задача | Оценка |
|---|--------|--------|
| 6 | TypeScript для фронтенда | 2-3 дня |
| 7 | Тесты на фронтенд (Jest + React Testing Library) | 3-4 дня |
| 8 | Вынести auth users в БД tbl_Users | 1 час |

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