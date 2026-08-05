# Аналитический портал — Документация проекта

**Дата:** 05.08.2026
**Коммит:** feat: Excel-экспорт, настройка карточек, лимит 50, фильтр комментариев
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
7. [Архитектура работы с комментариями](#архитектура-работы-с-комментариями)
8. [Актуальные изменения (working tree)](#актуальные-изменения-working-tree)

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
│   InternetSales.jsx — интернет-продажи (DataTable)      │
│   PromoAnalysis.jsx — анализ промо (3 вкладки)          │
│   PromoForm.jsx — форма создания нового промо            │
│   PromoApproval.jsx — страница согласования              │
│     (переключатель Карточки/Таблица, mass-actions,       │
│     sticky-панель фильтров, лимит 50 карточек,           │
│     настройка полей через Drawer, фильтр has_comments)   │
│                                                         │
│ src/components/                                         │
│   FilterPanel.jsx — панель фильтров (многоразовая)       │
│   DataTable.jsx — таблица с пагинацией + серверный поиск│
│     (Excel/CSV экспорт, предупреждение при >10000 строк) │
│   PromoEditDialog.jsx — диалог редактирования промо      │
│     (agreement1/2 editable, чипы статуса)                │
│   ApprovalCard.jsx — карточка согласования               │
│     (динамические поля, история comments, agreement1/2,   │
│     conditions всегда видимы)                            │
│   DrilldownModal.jsx — модал детализации                 │
│                                                         │
│ src/hooks/                                              │
│   usePromoData.ts — данные промо (React Query +         │
│     AbortController signal для отмены устаревших запросов)│
│   usePromoFilters.js — фильтры промо                     │
│   usePromoForm.ts — форма редактирования                 │
│   usePromoCalculations.ts — расчёт плановых/фактических  │
│                                                         │
│ src/api/                                                │
│   promo.js — API-запросы (fetchWithAuth + auto-refresh) │
│     + batchApprove (массовое согласование)               │
│     + has_comments фильтр                                │
│   auth.js — login/logout/refreshToken/saveSession       │
│                                                         │
│ src/utils/                                              │
│   cardFields.js — FIELD_GROUPS, DEFAULT_VISIBLE_FIELDS   │
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
│     (agreement1/2 editable, защита Mass Assignment,      │
│     аудит-лог с username,                               │
│     BatchApprovePromo для массового согласования)        │
│   sales.go — интернет-продажи (серверный поиск LIKE)    │
│   auth.go — логин + рефреш (БД + bcrypt, Secure-флаг)   │
│ services/                                               │
│   promo_service.go — PromoInputDTO, CalculatedFields,   │
│     EnrichFromRepo, CalculateFields,                     │
│     MergeCalculatedIntoDBRow, DTOToDBRow, DBRowToDTO,   │
│     MapToDTO, DBRowToMap                                │
│ repository/                                              │
│   promo_repo.go — все SQL-запросы промо                 │
│     GetApprovalFilters: 4 горутины errgroup +            │
│     buildApprovalWhere(excludeCol),                      │
│     ApprovePromoWithStatus: фиксирует статус в           │
│     agreementN, дописывает комментарий в comments        │
│     с пометкой [дата автор], фильтр has_comments         │
│   user_repo.go — запросы к tbl_Users                    │
│ middleware/                                              │
│   auth.go — AuthRequired + RoleRequired                 │
│ models/                                                 │
│   types.go — Row, PromoRow, PromoRowDB,                 │
│     ApprovalRow (добавлено Comments),                   │
│     PtrFloat/PtrInt/ValFloat/ValInt хелперы             │
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
│   (agreement1_status, agreement1_comment,               │
│    agreement2_status, agreement2_comment,               │
│    comments — история переписки КАМ + согласующих)       │
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
│   ├── promo.go            — обработчики + applyJSONToRow (agreement1/2 editable,
│   │                         agreementN_status/_comment защищены от прямого редактирования,
│   │                         аудит-лог, BatchApprovePromo для массового согласования)
│   └── sales.go            — GetData / GetFilterOptions / GetDrilldown (серверный поиск LIKE)
├── services/
│   └── promo_service.go    — PromoInputDTO, CalculatedFields, EnrichFromRepo, CalculateFields,
│                             MergeCalculatedIntoDBRow, DTOToDBRow, DBRowToDTO, MapToDTO, DBRowToMap
├── repository/
│   ├── promo_repo.go       — все SQL-запросы: FetchExistingRow (*PromoRowDB), UpdatePromo, InsertPromo,
│   │                         фильтры, GetApprovals (Comments + has_comments),
│   │                         GetApprovalFilters: 4 горутины + buildApprovalWhere(excludeCol),
│   │                         ApprovePromoWithStatus (append в comments), BatchApprove
│   └── user_repo.go        — GetUserByUsername (из tbl_Users)
├── middleware/
│   └── auth.go             — AuthRequired + RoleRequired
├── migrations/
│   ├── 001_create_tbl_users.sql
│   ├── 002_split_agreement_fields.sql
│   └── seed_users.sql      — seed с реальными bcrypt-хешами
└── models/
    └── types.go            — Row, PromoRow, PromoRowDB, ApprovalRow (+Comments),
                              NetworkGeo, LastSKUData, HistoryRow, DrilldownRow,
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
│   └── promo.js           — fetchWithAuth (авто-рефреш при 401), 19 методов API
│                            (вкл. batchApprove, has_comments)
├── components/
│   ├── ApprovalCard.jsx   — карточка: динамические поля, agreement1/2, conditions,
│   │                        история comments (💬 Комментарии), цветная рамка по ROI
│   ├── DataTable.jsx      — таблица MUI DataGrid + Export CSV/Excel (ExcelJS),
│   │                        предупреждение при >10000 строк
│   ├── DrilldownModal.jsx — модал детализации
│   ├── FilterPanel.jsx    — панель фильтров с Autocomplete
│   └── PromoEditDialog.jsx — диалог редактирования (agreement1/2 editable,
│                              Закрыть/Сохранить, Confirm Dialog для согласующих)
├── hooks/
│   ├── usePromoCalculations.ts — расчёт плановых/фактических
│   ├── usePromoData.ts         — загрузка данных (React Query, AbortController signal)
│   ├── usePromoData.js         — JS-версия
│   ├── usePromoFilters.js      — фильтры с debounce
│   └── usePromoForm.ts         — форма + сохранение (0 vs NULL исправлен)
├── utils/
│   └── cardFields.js           — FIELD_GROUPS (Основные, Плановые, Фактические, Исторические)
│                                  DEFAULT_VISIBLE_FIELDS, ALL_FIELDS_FLAT, FIELDS_MAP
├── types/
│   └── promo.ts           — TypeScript-типы
└── pages/
    ├── Home.jsx            — главная страница
    ├── InternetSales.jsx   — интернет-продажи
    ├── Login.jsx           — страница входа
    ├── PromoAnalysis.jsx   — анализ промо (3 вкладки, Export CSV/Excel)
    ├── PromoApproval.jsx   — согласование (переключатель Карточки/Таблица через startTransition,
    │                         лимит 50 карточек, sticky-фильтры, Drawer настройки полей,
    │                         фильтр has_comments, массовое согласование)
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
| GET | `/api/data` | JWT | Paginated data + серверный поиск (`search` param → LIKE в SQL) |
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
| GET | `/api/promo/approvals` | JWT | Approval list (фильтрация: kam, network_name, brand, mechanics, year, month, has_comments). Возвращает comments в каждой строке. |
| GET | `/api/promo/approval-filters` | JWT | Networks/brands/mechanics/kams (4 горутины errgroup, перекрёстная фильтрация excludeCol) |
| GET | `/api/promo/approval-kams` | JWT | KAMs with pending approval |
| GET | `/api/promo/approval-networks` | JWT | Networks for KAM |
| GET | `/api/promo/approval-brands` | JWT | Brands for KAM+network |
| POST | `/api/promo/approve` | JWT | Approve/reject/comment (пишет в agreementN + _status + _comment + дописывает комментарий в comments с пометкой автора) |
| POST | `/api/promo/approve/batch` | JWT | Массовое согласование массива ID |

### Promo — Write
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/promo/save` | JWT + Roles | Create/Update. agreement1/agreement2 — editable; agreementN_status/_comment — защищены (только через согласование). Refetch после UPDATE. |
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
| agreement1 / agreement2 | nvarchar | Статус согласования (Согласовано/Отклонено/комментарий) |
| agreement1_status / agreement2_status | nvarchar(20) | Статус: approved/rejected/commented (управляется только через /approve) |
| agreement1_comment / agreement2_comment | nvarchar(max) | Текст комментария согласующего |
| **comments** | nvarchar(max) | **История переписки: комментарии КАМ + согласующих с пометками [дата автор]** |
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
- ✅ **Защита Mass Assignment: agreementN_status/_comment исключены из applyJSONToRow**
- ✅ Авто-рефреш токена на фронтенде (fetchWithAuth при 401)
- ✅ **Аудит-лог: promo_updated/created/deleted/approved — реальный username из JWT**
- ✅ Пользователи в БД (tbl_Users) + bcrypt-хеширование паролей
- ✅ Rate limiter (100 запросов/мин на IP, sync.RWMutex)
- ✅ Structured logging (slog + lumberjack, ротация логов)
- ✅ **CORS из переменной окружения (`CORS_ORIGINS`) + AllowCredentials: true**

### Целостность данных
- ✅ **0 vs NULL: usePromoForm.ts — `x !== '' ? parseFloat(x) : null`**
- ✅ **PromoRowDB: числовые поля — указатели (`*float64`, `*int`)**
- ✅ **FetchExistingRow: сканирование через `sql.Null*` с проверкой `.Valid`**
- ✅ **applyJSONToRow: защита от `"<nil>"`, agreementN редактируемы, _status/_comment защищены**
- ✅ **Хелперы PtrFloat/PtrInt/ValFloat/ValInt**
- ✅ **stringVal: nil → '' вместо '<nil>' (agreement1/2 при создании промо)**
- ✅ **Optimistic Locking: строковое сравнение updated_at, refetch после UPDATE**

### Просмотр и редактирование промо
- ✅ DataGrid с пагинацией, сортировкой, поиском
- ✅ Фильтры с Autocomplete + debounce 300ms
- ✅ Сохранение состояния фильтров (sessionStorage, localStorage)
- ✅ CRUD промо: создание, редактирование, удаление (soft delete)
- ✅ **agreement1/2 — редактируемые поля в диалоге (agreementN_status/_comment — только через согласование)**
- ✅ PromoEditDialog: кнопки Закрыть/Сохранить, Confirm Dialog для согласующих
- ✅ **Экспорт CSV + Excel (ExcelJS + file-saver), предупреждение при >10000 строк**
- ✅ Серверный поиск в DataTable (debounce 400ms, LIKE в SQL)

### Согласование
- ✅ Карточки промо в CSS Grid (React.memo)
- ✅ Три действия: Комментарий / Согласовано / Отклонено
- ✅ **Перекрёстная фильтрация: 4 горутины errgroup, buildApprovalWhere(excludeCol)**
- ✅ Фильтр «Состояние согласования» (5 состояний)
- ✅ **Фильтр «Есть комментарии» (has_comments)**
- ✅ Кнопка «Применить» (контроль момента загрузки)
- ✅ **ApprovePromoWithStatus: комментарий согласующего → comments с пометкой `[дата автор]: текст`**
- ✅ **История comments отображается в карточке (💬 Комментарии)**
- ✅ **Динамические поля карточек: настройка через Drawer с группировкой**
- ✅ **brand_as, kam, conditions, agreement1/2 — всегда видимы на карточке**
- ✅ **Лимит 50 карточек + информирование об остатке**
- ✅ **Переключатель Карточки/Таблица через startTransition (быстрое переключение)**
- ✅ **Sticky-панель фильтров + переключатель вида**
- ✅ **Массовое согласование (только в табличном виде)**
- ✅ **Ролевая фильтрация: только свой статус (agreement1_status/agreement2_status)**
- ✅ Цветовая индикация ROI (зелёный/красный), цветная левая рамка по ROI

### Архитектура
- ✅ **Четырёхслойная архитектура: handlers → services → repository → DB**
- ✅ InMemoryCache (RWMutex, TTL 5 мин) для GetPromoFilters
- ✅ GetPromoFilters: 7 параллельных запросов через errgroup
- ✅ GetApprovalFilters: 4 параллельных запроса через errgroup + excludeCol
- ✅ TanStack Query (React Query) в usePromoData.ts с AbortController signal
- ✅ TypeScript-типы в types/promo.ts, хуки типизированы

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
| 3 | Полная миграция на TypeScript (.jsx → .tsx) | 1-2 дня |

### Технический долг
| # | Задача | Оценка |
|---|--------|--------|
| 4 | Тесты на фронтенд (Vitest + React Testing Library) | 3-4 дня |
| 5 | ~~Починить запись комментариев согласующего в comments БД~~ ✅ исправлено | 1-2 часа |

### Будущий функционал
| # | Задача | Оценка |
|---|--------|--------|
| 6 | Дашбордизация InternetSales (KPI-карточки + графики) | 1 неделя |
| 7 | Мобильная версия | TBD |
| 8 | Нормализация словарей (ID вместо строк) | TBD |
| 9 | E2E-тесты (Playwright) | 3-4 дня |

---

## Архитектура работы с комментариями

### Как сохраняются

| Роль | Метод | Формат |
|------|-------|--------|
| КАМ (admin) | `PUT /api/promo` с `comments: "текст"` | Бэкенд оборачивает в `[ДД.ММ.ГГГГ КАМ|username]: текст` |
| Согласующий 1/2 | `POST /api/promo/approve` (статус `comment`/`согласовано`/`отклонено`) | Бэкенд добавляет строку `[ДД.ММ.ГГГГ согласованиеN|username]: текст` |
| Массовое согласование | `POST /api/promo/approve/batch` | Строка `[ДД.ММ.ГГГГ согласованиеN|batch]: текст` |

### Как отображаются

- **PromoEditDialog**: `parseComments(form.comments)` разбирает строки, группирует по ролям, показывает дату/автора с цветовой схемой.
- **ApprovalCard**: Popover «История» — аналогичный разбор, компактное отображение в карточке.

### Ключевое правило (исправлено)

При обновлении промо через `PUT /api/promo`:
- **Добавление**: если новый комментарий непустой → извлекается текущая история (**все** строки, не только структурированные), добавляется новая строка с датой и ролью КАМ.
- **Не-затирание**: если новый комментарий пуст/`null` (обычное редактирование параметров) → история **не трогается**.
- **INSERT**: при создании нового промо первый комментарий тоже оформляется как строка истории `[ДД.ММ.ГГГГ КАМ|автор]: текст`.

---

## Актуальные изменения (working tree)

Ниже перечислены **незакоммиченные** изменения в рабочей директории (в том числе по исправлению истории комментариев):

| Файл | Что изменено |
|------|--------------|
| `backend/handlers/promo.go` | Исправлена история комментариев: при UPDATE извлекается **вся** история (все непустые строки, а не только `[ДД.ММ...]`); при PUT с пустым `comments` история больше **не затирается** (удалена ветка «Комментарий очищен»); при INSERT первый комментарий оформляется как строка истории `[ДД.ММ.ГГГГ КАМ|автор]: текст` |
| `backend/repository/promo_repo.go` | Снят `TOP 500` в GetApprovals; комментарии согласующих записываются в формате `[дд.мм.гггг согласованиеN|автор]` |
| `backend/main_test.go` | Защита cleanupTestData от случайного DELETE в боевой БД |
| `frontend/src/api/auth.ts` | Logout event (для синхронизации штатов), абсолютный URL в refresh |
| `frontend/src/api/promo.ts` | Обработка 401 → logout + заглушка JSON |
| `frontend/src/App.jsx` | Слушатель события `auth:logout` |
| `frontend/src/components/ApprovalCard.jsx` | Popover «История» с цветовой разбивкой по ролям |
| `frontend/src/components/PromoEditDialog.jsx` | Отображение истории (только чтение) + поле «Новый комментарий» для КАМ |
| `frontend/src/components/DataTable.jsx` | Удалён ExcelJS (только CSV) |
| `frontend/src/hooks/usePromoForm.ts` | Поддержка `commentOverride` для отдельного комментария |
| `frontend/vite.config.js` | Прокси /api → localhost:8080 |

**Итог исправления бага «затирается комментарий на вновь созданном промо»:**
1. При INSERT первый комментарий КАМ записывался «сырым» текстом (без формата `[дата...]`).
2. При следующем UPDATE извлечение истории фильтровало строки по префиксу `[ДД.ММ...]` и **отбрасывало** «сырой» первый комментарий → оставался только последний.
3. Исправление: извлечение сохраняет **все** непустые строки; INSERT дополнительно оформляет первый комментарий в структурированном виде.
4. История теперь накапливается корректно (КАМ + согласующие), сценарий «первые два комментария на новом промо» больше не теряет предыдущий.

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