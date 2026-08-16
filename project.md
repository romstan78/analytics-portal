# Analytics Portal

## Стек
- **Backend:** Go 1.22 + Gin + MSSQL (go-mssqldb) + squirrel + excelize + golang-jwt
- **Frontend:** React 18 + TypeScript + Vite + MUI v5/v6 + Recharts + React Query + Vite
- **Infra:** MSSQL (dbo.tbl_PromoActivities), Docker (docker-compose.yml), локальный сервер localhost:8080/5173

## Архитектура
```
frontend/           backend/
├─ src/
│  ├─ api/          ├─ main.go            (роутинг, rate limiter)
│  ├─ components/   ├─ config/            (DB, cache, auth, logger)
│  ├─ hooks/        ├─ handlers/          (Gin, HTTP-хендлеры)
│  ├─ pages/        ├─ middleware/        (JWT + RoleRequired)
│  ├─ types/        ├─ models/            (типы + хелперы PtrFloat/PtrInt)
│  └─ utils/        ├─ repository/        (SQL-запросы)
│                   ├─ services/          (DTO, расчёты, MergeDTO)
│                   └─ migrations/        (SQL-файлы + ensureTables())
└─ sync_script/     (импорт данных из внешних источников)
```

**Ключевые точки:**
- `tbl_PromoActivities` — основная таблица промо-акций
- `tbl_PromoComments` — новая таблица комментариев (запись в неё + дублирование в comments)
- `tbl_AuditLog` — аудит изменений
- Комментарии: старый формат `[DD.MM.YYYY роль|автор]: текст` в поле comments, новый — в tbl_PromoComments
- `applyJSONToRow` — switch для UPDATE (пропускает comments, согласования)
- `services/MapToDTO` → `DTOToDBRow` — для INSERT
- `MergeCalculatedIntoDBRow` — запись вычисляемых полей
- `DBRowToDTO` → `CalculateFields` — пересчёт при UPDATE
- Optimistic Locking через `updated_at`

## Выполненные изменения (09.08.2026)

### Backend
1. ✅ `GetApprovals` — переписан на `buildApprovalsWhere` (была ошибка `Invalid usage of the option NEXT in the FETCH statement`)
2. ✅ `buildPromoWhere` — переписан без squirrel (был пустой WHERE → фантомные удалённые записи при soft-delete)
3. ✅ `GetPromoRows` / `GetPromoRowsStream` — фильтрация `deleted_at IS NULL` теперь работает корректно
4. ✅ Исправлен soft-delete: удалённые записи больше не отображаются в таблице

### Frontend
5. ✅ `usePromoData` — стабильный queryKey `['promoData', filters]` без `refreshTrigger`
6. ✅ `PromoAnalysis` — `removeQueries` + `refetchQueries` при изменении данных
7. ✅ `ApprovalCard` / `PromoEditDialog` — комментарии через `useQuery` вместо `setTimeout`
8. ✅ `PromoApproval` — инвалидация кэша `['comments', id]` после сохранения
9. ✅ `PromoEditDialog` — типизация пропсов `PromoEditDialogProps`

## Известные проблемы
1. **`npx tsc --noEmit`** — предсуществующие TS-ошибки (implicit any, MUI Grid deprecated props)

## Выполненные изменения (10.08.2026)

### #1: Кнопка восстановления soft-deleted записей
- `DeletedFilter` в `PromoFilterParams` ("" / "deleted" / "all") — позволяет admin видеть удалённые
- `RestorePromo(id)` — очищает `deleted_at`, пишет аудит-лог с action_type "RESTORE"
- Роут `PATCH /api/promo/:id/restore` (admin only), PATCH в CORS
- `deleted_at` в модели `PromoRow` (для подсветки и кнопки)
- Селект "Состояние" (Актуальные/Все/Удалённые) — виден только admin
- Подсветка удалённых строк серым фоном (`getRowClassName`)
- Кнопка "Восстановить (отмена удаления)" в диалоге редактирования

### #2: Обновление комментариев КАМ
- `invalidateQueries` → `refetchQueries` для `['comments', id]` — гарантирует немедленный refetch

### Внеплановая правка: фильтр "на согласовании"
- `approval_status=pending` теперь включает `IS NULL OR = 'commented'` — промо с комментариями, но не согласованные, отображаются
- Та же правка в `buildApprovalWhere` (фильтры согласования) и хелперах KAM/Networks/Brands

## План действий (приоритет)
- [x] **#1:** Кнопка восстановления soft-deleted записей в таблице
- [x] **#2:** Проверить обновление комментариев КАМ после сохранения в PromoEditDialog
- [x] **#3:** goose/golang-migrate вместо ensureTables()
  - установлен `github.com/pressly/goose/v3` (go 1.25.0 → 1.25.7 в go.mod)
  - заменён драйвер: `denisenkom/go-mssqldb` → `microsoft/go-mssqldb` v1.10.0
  - два соединения: `"sqlserver"` (goose, @pN-плейсхолдеры) и `"mssql"` (squirrel, ?-плейсхолдеры)
  - миграции 001-004: goose-аннотации `-- +goose Up/Down`, `;` на внешних стейтментах, без `;` внутри `BEGIN...END`
  - `backend/migrations/embed.go` — embedded FS (`//go:embed 0*.sql`, seed_users.sql исключён)
  - `ensureTables()` удалён, миграции через `goose.NewProvider(...).Up(...)` на временном соединении
  - проверки: `go build ./...` ✅, `go vet ./...` ✅, `go test` ✅ (27/30 PASS, 3 FAIL — предсуществующие ожидания 500→404)
- [x] **#4:** TS: включить noUnusedLocals, noUnusedParameters после чистки
  - исправлено 15 TS6133-ошибок (неиспользуемые импорты, переменные, параметры) в 7 файлах
  - `tsconfig.json`: `noUnusedLocals: true`, `noUnusedParameters: true`
- [x] **#5:** Покрывающий индекс `IX_PromoActivities_Filters` на tbl_PromoActivities
  - Миграция 005: индекс на (deleted_at, year, month, kam, network_name, brand_as) + INCLUDE
- [x] **#6:** MUI Grid item → Grid2 (deprecation)
  - `ApprovalCard.tsx`, `PromoForm.tsx`: `<Grid item xs={N}>` → `<Grid size={N}>` / `<Grid size={{ xs: N, md: M }}>`

## Файлы, изменённые в последней сессии (09-10.08.2026)
- `backend/repository/promo_repo.go` — DeletedFilter, RestorePromo, pending=IS NULL OR commented
- `backend/models/types.go` — DeletedAt в PromoRow
- `backend/handlers/promo.go` — DeletedFilter query param, RestorePromo handler, Excel Scan fix
- `backend/main.go` — роут PATCH /api/promo/:id/restore, PATCH в CORS
- `frontend/src/api/promo.ts` — promoAPI.restore()
- `frontend/src/pages/PromoAnalysis.tsx` — селект "Состояние" (admin only), подсветка deleted строк
- `frontend/src/components/PromoEditDialog.tsx` — кнопка восстановления, refetchQueries для комментариев
- `frontend/src/hooks/usePromoForm.ts` — deleted_at в PromoFormValues

### Файлы за #3 (goose-миграции)
- `backend/go.mod` / `backend/go.sum` — добавлен `github.com/pressly/goose/v3`, go 1.25.0 → 1.25.7
- `backend/migrations/embed.go` — embedded FS (`//go:embed 0*.sql`)
- `backend/migrations/001-004_*.sql` — goose-аннотации Up/Down, идемпотентность 003/004, убраны `GO`
- `backend/config/db.go` — `ensureTables()` удалён → `goose.NewProvider(...).Up(...)`

### Файлы за #4 (TS cleanup) + fix кэша фильтров
- `frontend/tsconfig.json` — `noUnusedLocals: true`, `noUnusedParameters: true`
- `frontend/src/*` — убраны неиспользуемые импорты/переменные (FilterPanel, PromoEditDialog, main, Home, PromoAnalysis, PromoApproval, calcUtils.test)
- `backend/handlers/promo.go` — кэш фильтров только для дефолтной страницы

### Fix: Internet Sales фильтры + #5 индекс + #6 Grid2
- `backend/handlers/sales.go` — `sq.Select("1")` вместо `sq.Select()` (фильтры не применялись)
- `backend/migrations/005_add_filter_index.sql` — индекс IX_PromoActivities_Filters
- `frontend/src/components/ApprovalCard.tsx` — Grid item → size
- `frontend/src/pages/PromoForm.tsx` — Grid item → size

## Выполненные изменения (16.08.2026)

### Этап 4: целостность согласований

- Одиночное и массовое согласование требуют актуальный `updated_at`; устаревшая карточка получает `409 Conflict`.
- Массовое согласование выполняется одной транзакцией: при конфликте любой карточки пакет полностью откатывается.
- Карточки блокируются в стабильном порядке ID, чтобы снизить риск взаимных блокировок параллельных пакетов.
- Комментарий и статус согласования сохраняются в одной транзакции.
- Удалённые промо доступны в форме только для чтения до восстановления.
- Просроченные записи кэша фильтров удаляются автоматически.
- Добавлены модульные, race- и интеграционные тесты конфликтов и атомарности пакета.

### Исправление Excel-выгрузки промо

- API и Excel используют единый порядок сканирования колонок.
- Ошибки чтения строк больше не скрываются.
- Реальная проверка выгрузки: заголовок и строки данных формируются корректно.

### Этап 5: подготовка реестра конфликтующих промо

- Выполнен свежий read-only аудит базы: 228 групп, 508 строк-участников и 280 лишних строк.
- Классификация: 226 конфликтов данных, 1 конфликт согласований и 1 точный дубль.
- Подготовлен локальный Excel-реестр для бизнес-решений; каталог `outputs/` исключён из Git, чтобы не публиковать рабочие данные.
- Решение по очистке изменено: конфликтующие бизнес-записи пока сохраняются, поскольку данные будут перезагружены.
- 16.08.2026 мягко удалены 15 активных тестовых промо с маркером `тест`/`test` в описательных полях; совпадения только в `conditions` и `comments` исключены.
- Перед операцией сохранён локальный JSON-снимок строк и аудита; операция проведена одной транзакцией, активных тестовых промо после проверки — 0.

### Этап 6: готовность к эксплуатации

- Unit, lint, build, Python и браузерные smoke-проверки разделены и запускаются воспроизводимо.
- Подготовлен шаблон GitHub Actions с проверкой чистой SQL Server базы, всех Goose-миграций и интеграционных тестов; автоматическая активация отложена из-за ограничений GitHub-токена.
- Backend корректно завершает запросы при `SIGTERM`, использует HTTP-таймауты и пишет JSON-логи в stdout и файл.
- Production-конфигурация запрещает `DB_AUTO_CREATE`, требует явные HTTPS origins и не публикует SQL Server на хосте.
- Python-зависимости закреплены; уязвимый `quic-go` обновлён до исправленной версии.
- Добавлен эксплуатационный чек-лист для health/readiness, backup/restore и production-запуска.
