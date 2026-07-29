
---
# 📊 Аналитический портал интернет-продаж

## Стек технологий
- **Frontend:** React 18 + Vite + MUI X Data Grid + react-router-dom + recharts
- **Backend:** Go 1.25 + Gin 1.12 + MSSQL (go-mssqldb)
- **ETL:** Python 3.12 + pyodbc + pandas + openpyxl
- **База данных:** MSSQL `local_project_db` на localhost
- **Логирование:** slog + lumberjack (JSON, ротация)
- **Rate limiting:** Встроенный middleware (100 запросов/мин/IP)

---

## Структура проекта
project/
├── backend/
│ ├── main.go # Точка входа, роуты, rate limiter
│ ├── main_test.go # 15+ тестов
│ ├── config/
│ │ └── db.go # Подключение к БД, логгер
│ ├── models/
│ │ └── types.go # Row, PromoRow, HistoryRow, DrilldownRow
│ ├── handlers/
│ │ ├── sales.go # getData, getFilterOptions, getDrilldown
│ │ └── promo.go # Все promo-эндпоинты + SavePromo + DeletePromo
│ ├── logs/ # Логи (ротация 100MB × 5)
│ └── .env # DB_SERVER, DB_USER, DB_PASSWORD, DB_NAME, DB_PORT
├── frontend/
│ ├── src/
│ │ ├── main.jsx
│ │ ├── App.jsx
│ │ ├── index.css
│ │ ├── api/
│ │ │ └── promo.js # Все API-запросы с AbortController
│ │ ├── components/
│ │ │ ├── DataTable.jsx # Универсальная таблица
│ │ │ ├── DrilldownModal.jsx # Детализация интернет-продаж
│ │ │ ├── FilterPanel.jsx # Панель фильтров с множественным выбором
│ │ │ └── PromoEditDialog.jsx# Редактирование промо
│ │ ├── hooks/
│ │ │ ├── usePromoCalculations.js # Расчёты (превью)
│ │ │ ├── usePromoData.js # Загрузка данных таблицы
│ │ │ ├── usePromoFilters.js # Каскадные фильтры
│ │ │ └── usePromoForm.js # Логика формы
│ │ └── pages/
│ │ ├── Home.jsx # Главная (6 блоков)
│ │ ├── InternetSales.jsx # Интернет-продажи
│ │ ├── PromoAnalysis.jsx # Промо (таблица + форма)
│ │ └── PromoForm.jsx # Форма нового промо
│ ├── package.json
│ └── vite.config.js
├── sync_script/
│ ├── sync_data.py # Синхронизация OLAP → локальная БД
│ ├── import_promo.py # Импорт промо из Excel
│ └── .env
└── upload/
└── сборка 23_26.xlsx # Исходный файл промо

## База данных

### Таблицы

**dbo.tbl_EcomSalesConsolidated**
- Основная таблица продаж (Wide Format)
- Колонки: id, year, month, brandName, productName, networkName, qty, rub, qty_ZC, rub_ZC, ... (22 метрики)
- ~10 715 записей (промо), ~1 млн записей (интернет-продажи)

**dbo.tbl_PromoActivities**
- Промо-активности (68 колонок)
- Основные поля: network_name, kam, sku, brand, brand_as, mechanics, gtn_opex, baseline_units, plan_promo_units, contract_price, gm, ...

**dbo.tbl_ChannelSegmentMapping**
- Справочник metric_type → un_rub, segment, channel

**dbo.tbl_MechanicsChannelMapping**
- Справочник механик и каналов (онлайн/оффлайн)

**dbo.tbl_SKUMapping**
- Справочник SKU ↔ Brand ↔ Бренд для АС

**dbo.tbl_KAMNetworkMapping**
- Справочник KAM ↔ Сеть

---

## API эндпоинты

### Интернет-продажи
| Метод | URL | Описание |
|-------|-----|----------|
| GET | /api/data | Данные продаж с UNPIVOT + JOIN |
| GET | /api/filters | Справочники + segmentChannelMap |
| GET | /api/drilldown | Детализация по бренду и сети |

### Промо
| Метод | URL | Описание |
|-------|-----|----------|
| GET | /api/promo/data | Данные промо с фильтрами |
| GET | /api/promo/filters | Каскадные фильтры |
| GET | /api/promo/sku-by-brand | SKU по бренду |
| GET | /api/promo/sku-info | Бренд по SKU |
| GET | /api/promo/last-sku-data | Последние данные по SKU |
| GET | /api/promo/last-contract-price | Последняя цена контракта |
| GET | /api/promo/last-network-data | Последние данные по сети |
| GET | /api/promo/kam-by-network | KAM по сети |
| GET | /api/promo/investment-types | Типы инвестиций |
| GET | /api/promo/history | История промо |
| POST | /api/promo/save | Создание/обновление |
| DELETE | /api/promo/:id | Удаление |

---

## Формулы расчётов (бэкенд)

### Плановые
- `quarter = CEILING(month / 3)`
- `plan_promo_rub = plan_promo_units × contract_price`
- `plan_promo_uplift_units = plan_promo_units − baseline_units`
- `plan_promo_uplift_rub = plan_promo_uplift_units × contract_price`
- `plan_investments_pct = plan_investments_rub / plan_promo_rub × 100`
- `plan_roi = (plan_promo_uplift_rub / plan_investments_rub) × gm × 100 − 100`
- `baseline_rub = baseline_units × contract_price`

### Фактические
- `actual_promo_rub = actual_promo_sales_units × contract_price`
- `actual_promo_uplift_units = actual_promo_sales_units − baseline_units`
- `actual_promo_uplift_rub = actual_promo_uplift_units × contract_price`
- `actual_roi = (actual_promo_uplift_rub / actual_investments) × gm × 100 − 100`
- `net_promo_uplift_rub = actual_promo_uplift_rub × gm`
- `turnover_per_point = actual_corrected_baseline / promo_pharmacies`

---

## Ключевые функции

### Интернет-продажи
- Фильтры: год, месяц, бренд, сеть, уп/руб, сегмент, канал
- Каскадная фильтрация канал↔сегмент с автосвязыванием
- Drill-down с графиками (уп/руб, по сегментам, по каналам)
- Экспорт CSV

### Промо
- Импорт из Excel с очисткой данных
- Каскадные фильтры (KAM → Бренд → SKU → Сеть → Механика → Канал → Статус)
- Форма нового промо с историей и графиком
- Диалог редактирования с автосчётом показателей
- Сохранение (INSERT/UPDATE), удаление
- Валидация обязательных полей
- Все числа с разделителями разрядов и копейками
- Множественный выбор в фильтрах без чипсов

---

## Запуск

```bash
# Бэкенд
cd backend
go mod tidy
go run main.go

# Фронтенд
cd frontend
npm install
npm run dev

# Тесты
cd backend
go test -v

# Импорт промо
cd sync_script
python import_promo.py

Логи

JSON-формат, ротация каждые 100 МБ (5 файлов)
Логируются: создание, обновление, удаление промо; ошибки; старт сервера
Пример: {"time":"...","level":"INFO","msg":"promo_created","id":123,"sku":"SKU001","plan_rub":15000}

Безопасность

Пароли в .env (в .gitignore)
CORS только для localhost:5173
Rate limiting: 100 запросов/мин с IP
Плейсхолдеры в SQL (защита от инъекций)

Тесты

15+ тестов:

Создание промо (валидные, пустые, нулевые, отрицательные, кварталы)
Удаление (не найдено, некорректный ID)
Математика (ROI, baseline_rub, uplift, quarter, uplift%, investments%)
Rate limiter (лимит, очистка)

