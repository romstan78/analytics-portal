# Карта проекта

Analytics Portal: Go 1.25 + Gin + MSSQL (backend), React 19 + TS + Vite + MUI + React Query
(frontend), Python-скрипты импорта (sync_script). Четыре раздела портала:
**интернет-продажи**, **промо**, **реестр сетей**, **админ-справочники**.

Этот файл — навигация и инварианты. Подробности живут в `README.md` (эксплуатация,
видимость, выгрузки) и `project.md` (журнал изменений). Читай их по ссылке из раздела
ниже, а не целиком — оба большие.

## Слои и направление зависимостей

```
main.go  →  handlers/  →  services/  →  repository/  →  MSSQL
(роуты,     (HTTP, вход-  (формулы,      (SQL)
 CORS,       выход, права) DTO, свод)
 rate limit)
                 models/  — контракт ответов, общий для всех слоёв
                 migrations/ — goose, встроены в бинарник, применяются при старте
```

Снизу вверх зависимостей нет. Бизнес-правила — только в `services/`, SQL — только в
`repository/`, `handlers/` разбирают запрос и проверяют права.

Фронтенд: `pages/` → `components/` → `api/` → бэкенд. Общие хелперы HTTP
(`fetchWithAuth`, `parseJSONResponse`, `buildParams`) лежат в `api/promo.ts`, остальные
клиенты импортируют их оттуда. Чистые расчёты и форматирование — в `utils/` (покрыты
vitest), состояние форм — в `hooks/`.

## Куда идти по задаче

| Задача | Backend | Frontend |
| --- | --- | --- |
| Интернет-продажи: дашборд, сводная, дрилл-даун, выгрузка | `handlers/sales.go`, `sales_pivot.go`, `sales_export_jobs.go`; `services/sales_service.go`, `sales_pivot_service.go`; `repository/sales_repo.go`; `models/sales.go` | `pages/InternetSales.tsx`, `components/InternetSalesDashboard.tsx`, `InternetSalesSummaryTable.tsx`, `DrilldownModal.tsx`, `utils/salesPivot.ts` |
| Промо: таблица, карточка, согласование, формулы | `handlers/promo.go`; `services/promo_service.go` (формулы), `promo_dashboard_service.go`; `repository/promo_repo.go`, `promo_lock.go`, `promo_idempotency.go`; `models/types.go` | `pages/PromoAnalysis.tsx`, `PromoForm.tsx`, `PromoApproval.tsx`; `components/PromoEditDialog.tsx`, `ApprovalCard.tsx`; `hooks/usePromo*.ts` |
| Реестр сетей: план и факт, прогноз, инвестиции, цены | `handlers/network.go`, `network_dashboard.go`, `network_forecast_import.go`; `services/network_service.go` (план, валовый пул, пороги), `network_forecast_service.go`, `network_dashboard_service.go`, `network_investment_columns.go`; `repository/network_repo.go`, `network_monthly_repo.go`, `network_dashboard_repo.go`; `models/network.go` | `pages/NetworkRegistry.tsx`; `components/NetworkDetailView.tsx`, `NetworkForecastTab.tsx`, `NetworkPlanGrid.tsx`, `NetworkDashboardView.tsx`, `NetworkPricesTab.tsx`; `utils/networkPlan.ts`, `networkPrices.ts` |
| Вход, роли, сессии, лимиты попыток | `handlers/auth.go`; `middleware/auth.go`; `config/auth.go`; `repository/user_repo.go`, `session_repo.go`, `login_attempts_repo.go` | `pages/Login.tsx`, `api/auth.ts` |
| Админ-справочники | `handlers/dictionaries.go`; `repository/dictionaries_repo.go` | `pages/AdminDictionaries.tsx`, `api/dictionaries.ts` |
| Роут, CORS, rate limit, graceful shutdown | `main.go` | — |
| Загрузка данных из внешних источников | — | `sync_script/*.py` |

Самые крупные файлы (там же чаще всего и правки): `repository/promo_repo.go`,
`services/network_dashboard_service.go`, `handlers/promo.go`, `handlers/network.go`,
`components/NetworkDetailView.tsx`, `pages/NetworkRegistry.tsx`.

## От экрана к коду

Задача обычно приходит на языке интерфейса: «в промо-календаре не то число». Надпись
с экрана — точный адрес, точнее названия сервиса: подписи вкладок и кнопок лежат в коде
дословно, и поиск по ним попадает в нужный файл с первой попытки.

| Что на экране | Frontend | Backend |
| --- | --- | --- |
| Главное меню: «Продажи», «Анализ промо», «Реестр сетей», «Справочники» | `pages/Home.tsx`, маршруты в `App.tsx` | — |
| «Анализ продаж», «Оборачиваемость», «Продажи Like For Like» | заглушки `PlaceholderPage` в `App.tsx` — раздела ещё нет | — |
| **Продажи** → вкладки «Аналитика» / «Сводная таблица» / «Исходные данные» | `pages/InternetSales.tsx` | `handlers/sales.go` |
| Продажи → Аналитика → «Рейтинг», «Сезонность», «Детализация» | `components/InternetSalesDashboard.tsx` | `services/sales_service.go` |
| Продажи → «Сводная таблица» | `components/InternetSalesSummaryTable.tsx`, `utils/salesPivot.ts` | `handlers/sales_pivot.go`, `services/sales_pivot_service.go` |
| Продажи → «Исходные данные», выгрузка в Excel | `components/DataTable.tsx` | `handlers/sales_export_jobs.go` |
| Окно дрилл-дауна: «График» / «Таблица», «Уп/Руб» / «По сегментам» / «По каналам» | `components/DrilldownModal.tsx` | `handlers/sales.go` → `GetDrilldown` |
| **Анализ промо** → вкладки «Дашборд» / «Просмотр данных» | `pages/PromoAnalysis.tsx` | `handlers/promo.go` |
| Промо → Дашборд → «План–факт и эффективность», «Календарь и сезонность» | `components/PromoDashboard.tsx` | `services/promo_dashboard_service.go` |
| Промо → карточка, форма, согласование | `pages/PromoForm.tsx`, `PromoApproval.tsx`, `components/PromoEditDialog.tsx` | `services/promo_service.go` (формулы) |
| **Реестр сетей** → карточка сети: «Профиль сети», «Цены и SKU», «План и факт», «Прогноз», «Комментарии», «История» | `pages/NetworkRegistry.tsx`, `components/NetworkDetailView.tsx`, `NetworkPricesTab.tsx`, `NetworkPlanGrid.tsx`, `NetworkForecastTab.tsx` | `handlers/network.go`, `services/network_service.go`, `network_forecast_service.go` |
| **Справочники** (только admin) | `pages/AdminDictionaries.tsx` | `handlers/dictionaries.go` |

## Инварианты

Это то, чего не видно из беглого чтения кода. Нарушение каждого уже стоило починки.

1. **Контракт API описан один раз — в Go.** `frontend/src/types/api.generated.ts`
   собирается генератором `backend/cmd/tsgen` и руками не правится. Новая структура в
   ответе обязана попасть в реестр `exported` в `cmd/tsgen/main.go`, иначе генерация
   падает. После правки моделей — `make types`; `make types-check` внутри `make test`
   валит сборку при расхождении. Ручные типы фронтенда (`types/network.ts`, `sales.ts`,
   `promo.ts`) только реэкспортируют сгенерированные и добавляют то, чего в Go нет:
   сужения значений и тела запросов.

2. **Расчёты живут только на бэкенде.** Черновик карточки промо пересчитывает
   `POST /api/promo/calculate`, черновик плана сети — `POST /api/networks/:id/plan/preview`.
   Оба в БД не пишут. Дублировать формулы на фронте нельзя — разойдутся.

3. **Область видимости.** Промо и реестр сетей сужаются по закреплению КАМа
   (`tbl_Users.kam`) и области согласования; ограничение применяется на сервере во всех
   читающих путях сразу — фильтры, строки, дашборд, Excel, открытие по ссылке — и входит
   в ключ кэша справочников. Роль `kam` без закрепления получает `403`, а не пустую
   выборку (fail-closed). **Интернет-продажи областью не ограничены намеренно** — это
   решение, а не упущение; см. README «Интернет-продажи областью не ограничены».

4. **Миграции goose встроены в бинарник и применяются при старте.** Файлы
   `backend/migrations/0NN_*.sql` пронумерованы до 030; изменение уже применённого файла
   ломает установленные базы — новое изменение = новый файл. Запуск миграций и правка
   схемы — только с явного разрешения.

5. **Оптимистичная блокировка через `updated_at`.** Устаревшая карточка получает `409`.
   Массовое согласование — одна транзакция: конфликт любой карточки откатывает пакет
   целиком; карточки блокируются в порядке ID.

6. **Панели фильтров.** Список никогда не сужается собственным фильтром (иначе из списка
   брендов после выбора бренда некуда переключиться). Справочники кэшируются целиком в
   `config.FiltersCache` при любом наборе фильтров; веер запросов одной панели ограничен
   тремя одновременными (`FilterQueryConcurrency`) — пул соединений один на приложение.

7. **Потолки выгрузок различаются осознанно.** Промо `all=true` — `PROMO_MAX_ROWS`
   (50 000, сверх — `413`), потому что строки уходят в память вкладки. Продажи —
   `SALES_EXPORT_MAX_ROWS` (200 000), потому что идут потоком из курсора; большие
   выгрузки продаж работают через фоновые задания (`/api/data/export-jobs`).

8. **Черновики форм в `localStorage`.** Ключ включает пользователя и идентификатор записи,
   срок 7 дней, каждое обращение в `try/catch`. Расчётные поля в черновик не попадают —
   их пересчитывает сервер. Восстановление всегда спрашивают, молча не подставляют.

9. **Денормализованные колонки в `tbl_NetworkPlans`.** `fact_rub` / `forecast_rub` и
   расчётные колонки инвестиций читают внешние потребители напрямую, поэтому после
   ежедневной заливки факта нужен `make recalc-investments YEAR=…`. Прогноз хранит пару
   «рубли / упаковки»; обработчики поддерживают её сами, `make backfill-forecast-pairs` —
   разовый прогон для строк, заведённых до перехода.

## Команды

```bash
make up                 # весь стек (SQL Server на томе mssql_data_volume)
make test               # types-check + go vet/test + lint/vitest/build + python unittest
make types              # пересобрать api.generated.ts после правки Go-моделей
make test-e2e           # playwright (frontend/e2e)
```

Демо-контур (`docker-compose.demo.yml`, порты 8081/5174, отдельный том) — команды
`make demo-*`. Он не трогает основную базу.

Точечно, без полного `make test`:

```bash
cd backend && go test ./services -run TestNetworkForecast
cd frontend && npm run test:unit -- src/utils/salesPivot.test.ts
```

## Куда не смотреть

`frontend/dist`, `.vite`, `node_modules`, `outputs/` (рабочие выгрузки, вне Git),
`.claude/worktrees/` (копия репозитория — поиск по ней даёт дубли каждого файла),
`backend/logs`, `__pycache__`.

## Ведение задачи

Задача ставится на языке интерфейса и результата, а не на языке кода. Точное место
правки и способ проверки определяются здесь, а не спрашиваются у постановщика.

1. **Надпись с экрана — это адрес.** Есть в запросе текст из интерфейса — ищи по нему
   дословно (`grep "Календарь и сезонность"`), это короче и надёжнее догадок о слоях.
   Таблица выше разрешает случаи, где подписи нет или она неоднозначна.

2. **Сначала диагноз, потом код.** Запрос описывает желаемое поведение — сперва проверь,
   не реализовано ли оно уже, и скажи об этом, прежде чем писать. Расчёты здесь копятся
   годами и часто уже учитывают правило, которое просят добавить: у метрик промо-дашборда
   для «факт, если есть, иначе план» есть готовое `EffectiveInvestmentsRub`. Вторая копия
   такой логики со временем разойдётся с первой, и расхождение заметят по числам.

3. **Границу правки держи сам.** Меняй только названный экран или блок. Если правка
   неизбежно задевает соседний — скажи об этом до того, как трогать.

4. **Проверку выбирай сам.** Команду проверки называть не обязаны: подбери самую узкую
   релевантную, запусти и укажи в ответе, что запускал и с каким результатом. Критерий
   приёмки может прийти на человеческом языке («в календаре за март должно стоять 1,2
   млн») — этого достаточно, перевод в тест твоя работа. Полный `make test` — перед
   коммитом или по просьбе, не после каждой правки.

5. **Незаданное — предполагай и называй.** Не хватает мелочи, которую можно разумно
   выбрать (формат числа, поведение при пустом значении) — выбери и скажи, что выбрал.
   Останавливайся и спрашивай только там, где неверное предположение обесценит работу.

## Правила работы

Минимальные изменения, scope не расширять, попутный рефакторинг не делать — несвязанные
проблемы называть в отчёте. Без явного разрешения не менять `.env`, `go.mod`/`go.sum`,
`package.json`, `requirements.txt`, Docker-конфигурацию, схему БД и миграции; не запускать
серверы и миграции; не делать `git commit`/`push`. Полная версия — в `.clinerules`.
