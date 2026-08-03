# Аналитический портал — Документация проекта

**Дата:** 03.08.2026  
**Коммит:** 8441c01  
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
│   Login.jsx — JWT-авторизация                           │
│   InternetSales.jsx — интернет-продажи (DataTable)       │
│   PromoAnalysis.jsx — анализ промо (таблица + форма +    │
│     согласование, 3 вкладки)                             │
│   PromoForm.jsx — форма создания нового промо            │
│   PromoApproval.jsx — страница согласования (200 строк)  │
│                                                         │
│ src/components/                                         │
│   FilterPanel.jsx — панель фильтров (многоразовая)       │
│   DataTable.jsx — таблица с пагинацией                   │
│   PromoEditDialog.jsx — диалог редактирования промо      │
│   ApprovalCard.jsx — карточка согласования               │
│   DrilldownModal.jsx — модал детализации                 │
│                                                         │
│ src/hooks/                                              │
│   usePromoData.js — загрузка данных промо                │
│   usePromoFilters.js — фильтры промо                     │
│   usePromoForm.js — форма редактирования                 │
│   usePromoCalculations.js — расчёт плановых/фактических  │
│                                                         │
│ src/api/                                                │
│   promo.js — все API-запросы (fetchWithAuth)             │
│   auth.js — логин/логаут                                 │
└─────────────────────────────────────────────────────────┘
    │ HTTP (CORS: localhost:5173)
    ▼
┌─────────────────────────────────────────────────────────┐
│ Backend (Go + Gin)                                      │
│ localhost:8080                                          │
│                                                         │
│ main.go — сервер, роуты, rate limiter (RWMutex)         │
│ config/                                                 │
│   db.go — подключение к SQL Server (25 connections)     │
│   auth.go — JWT генерация/валидация (8 часов)           │
│ handlers/                                               │
│   promo.go — основные обработчики (1087 строк)          │
│   promo_utils.go — утилиты + calculatePromoFields       │
│   sales.go — интернет-продажи                           │
│   auth.go — логин                                       │
│ middleware/                                              │
│   auth.go — AuthRequired + RoleRequired                 │
│ models/                                                 │
│   types.go — Row, PromoRow, ApprovalRow, ...             │
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
├── main.go                 (150 строк) — сервер, роутинг, rate limiter
├── main_test.go            — тесты
├── config/
│   ├── auth.go             — JWT GenerateToken / ValidateToken
│   └── db.go               — подключение к SQL Server, пул соединений
├── handlers/
│   ├── auth.go             — POST /api/auth/login
│   ├── promo.go            (1087 строк) — CRUD + фильтры + согласование
│   ├── promo_utils.go      (182 строки) — safeFloat/safeInt/calculatePromoFields
│   └── sales.go            (293 строки) — GetData / GetFilterOptions / GetDrilldown
├── middleware/
│   └── auth.go             — AuthRequired + RoleRequired
└── models/
    └── types.go            — Row, PromoRow, ApprovalRow, HistoryRow, DrilldownRow
```

### Frontend
```
frontend/src/
├── App.jsx                — роутинг, тема MUI (Modern)
├── main.jsx               — точка входа
├── api/
│   ├── auth.js            — login/logout/getToken
│   └── promo.js           — все API-запросы (fetchWithAuth, 12 методов)
├── components/
│   ├── ApprovalCard.jsx   (159 строк) — карточка согласования
│   ├── DataTable.jsx      (240 строк) — таблица MUI DataGrid
│   ├── DrilldownModal.jsx — модал детализации
│   ├── FilterPanel.jsx    (184 строки) — панель фильтров с Autocomplete
│   └── PromoEditDialog.jsx (220 строк) — диалог редактирования
├── hooks/
│   ├── usePromoCalculations.js (45 строк) — расчёт плановых/фактических
│   ├── usePromoData.js         (80 строк) — загрузка данных
│   ├── usePromoFilters.js      (83 строки) — фильтры с debounce
│   └── usePromoForm.js         (168 строк) — форма + сохранение
└── pages/
    ├── Home.jsx            (120 строк) — главная страница
    ├── InternetSales.jsx   (248 строк) — интернет-продажи
    ├── Login.jsx           — страница входа
    ├── PromoAnalysis.jsx   (386 строк) — анализ промо (3 вкладки)
    ├── PromoApproval.jsx   (200 строк) — согласование
    └── PromoForm.jsx       — создание нового промо
```

---

## Бэкенд — API эндпоинты

### Auth
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/login` | No | JWT login |

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
| GET | `/api/promo/filters` | JWT | Distinct filter options (kam, brand, sku, network_name, mechanics, channel, status) |
| GET | `/api/promo/data` | JWT | All promo rows with filtering |
| GET | `/api/promo/sku-by-brand` | JWT | SKUs for brand |
| GET | `/api/promo/last-contract-price` | JWT | Last contract price for SKU |
| GET | `/api/promo/investment-types` | JWT | Fixed list: GTN, OPEX, ... |
| GET | `/api/promo/kam-by-network` | JWT | KAM for network |
| GET | `/api/promo/last-network-data` | JWT | Pharmacy count for network |
| GET | `/api/promo/network-geo` | JWT | Geo mapping for network |
| GET | `/api/promo/history` | JWT | Top-10 history rows |
| GET | `/api/promo/sku-info` | JWT | Brand for SKU |
| GET | `/api/promo/last-sku-data` | JWT | Latest contract_price, gm, olap_price |

### Promo — Approval
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/promo/approvals` | JWT | Approval list (`?kam=&approval_status=&year=&month=`) |
| GET | `/api/promo/approval-filters` | JWT | Networks/brands/mechanics/kams (cross-filtered) |
| GET | `/api/promo/approval-kams` | JWT | KAMs with pending approval |
| GET | `/api/promo/approval-networks` | JWT | Networks for KAM |
| GET | `/api/promo/approval-brands` | JWT | Brands for KAM+network |
| POST | `/api/promo/approve` | JWT | Approve/reject/comment (`{id, status, comment}`) |

### Promo — Write
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/promo/save` | JWT + Roles: admin,agreement1,agreement2 | Create/Update promo |
| DELETE | `/api/promo/:id` | JWT + Role: admin | Soft-delete promo |

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
| agreement1 / agreement2 | nvarchar | Статус согласования |
| status | nvarchar | Статус промо |
| deleted_at | datetime | Soft delete |
| updated_at | datetime | Обновлён |
| created_at | datetime | Создан |

### Вспомогательные таблицы
- `tbl_EcomSalesNormalized` — интернет-продажи (brandName, productName, networkName, metric_type, metric_value, un_rub, segment, channel)
- `tbl_MechanicsChannelMapping` — механика → канал
- `tbl_ChannelSegmentMapping` — канал ↔ сегмент
- `tbl_KAMNetworkMapping` — KAM ↔ сеть
- `tbl_NetworkGeoMapping` — сеть → key_region, top20_segment
- `tbl_SKUMapping` — SKU → brand, brand_as

---

## Готово (Done)

### Авторизация и безопасность
- ✅ JWT-авторизация с ролями: admin, agreement1, agreement2
- ✅ Middleware AuthRequired + RoleRequired
- ✅ Rate limiter (100 запросов/мин на IP, sync.RWMutex)
- ✅ Structured logging (slog + lumberjack, ротация логов)

### Просмотр и редактирование промо
- ✅ DataGrid с пагинацией, сортировкой, поиском, экспортом CSV
- ✅ Фильтры с Autocomplete + debounce 300ms
- ✅ Сохранение состояния фильтров (sessionStorage, localStorage)
- ✅ CRUD промо: создание, редактирование, удаление (soft delete)
- ✅ Optimistic locking (базовая версия, без WHERE updated_at)
- ✅ Автообновление UI после редактирования/удаления

### Согласование
- ✅ Карточки промо в CSS Grid (React.memo для производительности)
- ✅ Три действия: Комментарий / Согласовано / Отклонено
- ✅ Перекрёстная каскадная фильтрация (все фильтры ограничивают друг друга)
- ✅ Фильтр «Состояние согласования» (На согласовании / С комментариями / Согласовано / Отклонено / Все)
- ✅ Кнопка «Применить» (контроль момента загрузки)
- ✅ Защита от загрузки без фильтров
- ✅ Отображение комментариев обоих согласующих в карточке
- ✅ Автообновление таблицы после согласования
- ✅ SQL: CHARINDEX для Unicode-поиска, TOP 500, фильтр по дате

### Другое
- ✅ Главная страница (6 карточек, CSS Grid)
- ✅ Интернет-продажи (FilterPanel + DataTable + DrilldownModal)
- ✅ Основная документация в project.md
- ✅ Теги snapshot-2026-08-02 и snapshot-2026-08-03

---

## Известные проблемы (Bugs)

### Критические (P0) — исправлены ✅
1. ~~usePromoData: JSON.stringify в зависимостях → лишние HTTP-запросы~~ → **исправлено** (fetchTrigger + useRef)
2. ~~Rate limiter: глобальный sync.Mutex~~ → **исправлено** (sync.RWMutex)
3. ~~Memory leak: commentRefs в PromoApproval~~ → **исправлено** (очистка перед setApprovals)
4. ~~Форма редактирования: 409 Conflict при сохранении~~ → **исправлено** (убран WHERE updated_at)

### Средние (P1) — требуют внимания
5. **GetPromoFilters: 7 последовательных SQL-запросов** — при 10 пользователях создаёт нагрузку. Нужен UNION ALL или параллельные горутины
6. **ApprovalCard отображается даже после согласования** — нужно дождаться ответа API и только потом убирать карточку (сейчас оптимистичный UI)
7. **Фильтр по бренду использует `.includes(sku)`** — неточный, нужен brand_as в модели ApprovalRow

### Косметические (P2)
8. **CORS: хардкод localhost:5173** — не работает на других портах
9. **JWT без refresh token** — пользователь разлогинивается через 8 часов
10. **Нет тестов на фронтенд** — только бэкенд main_test.go

---

## План доработок

### Краткосрочные (ближайшие сессии)
| # | Задача | Оценка |
|---|--------|--------|
| 1 | Исправить фильтр по бренду в approval (добавить brand_as) | 20 мин |
| 2 | Объединить 7 запросов GetPromoFilters в один UNION ALL | 30 мин |
| 3 | Добавить индикацию загрузки в ApprovalCard (skeleton) | 15 мин |
| 4 | Сделать рефакторинг promo.go → promo_crud.go (Save/Delete) | 30 мин |
| 5 | Вынести общий SQL WHERE builder | 20 мин |

### Среднесрочные
| # | Задача | Оценка |
|---|--------|--------|
| 6 | Счётчик обработанных промо в согласовании | 15 мин |
| 7 | Сортировка карточек по ROI / дате / сети | 20 мин |
| 8 | Цветовая индикация убыточных промо | 10 мин |
| 9 | Экспорт в CSV из согласования | 15 мин |
| 10 | CORS из переменной окружения | 10 мин |

### Технический долг
| # | Задача | Оценка |
|---|--------|--------|
| 11 | Вынести сервисный слой (бизнес-логика между handlers и SQL) | 1 час |
| 12 | TypeScript для фронтенда | 2-3 дня |
| 13 | Refresh token для JWT | 1 час |
| 14 | Тесты на фронтенд (Jest + React Testing Library) | 3-4 дня |

### Будущий функционал
| # | Задача | Оценка |
|---|--------|--------|
| 15 | «Более трудная задача» — расчёты, визуал (со слов пользователя) | TBD |
| 16 | Дашборд с графиками (ROI, uplift, план/факт) | TBD |
| 17 | Мобильная версия | TBD |

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

### Требования
- Go 1.21+
- Node.js 18+
- SQL Server (MSSQL)
- Переменные окружения в `backend/.env` (DB_SERVER, DB_USER, DB_PASSWORD, DB_NAME, DB_PORT, JWT_SECRET)