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
2. **ensureTables()** — ручная инициализация таблиц, нужно заменить на goose/golang-migrate

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
- [ ] **#3:** goose/golang-migrate вместо ensureTables()
- [ ] **#4:** TS: включить noUnusedLocals, noUnusedParameters после чистки
- [ ] **#5:** Покрывающий индекс `IX_PromoActivities_Filters` на tbl_PromoActivities
- [ ] **#6:** MUI Grid item → Grid2 (deprecation)

## Файлы, изменённые в последней сессии (09-10.08.2026)
- `backend/repository/promo_repo.go` — DeletedFilter, RestorePromo, pending=IS NULL OR commented
- `backend/models/types.go` — DeletedAt в PromoRow
- `backend/handlers/promo.go` — DeletedFilter query param, RestorePromo handler, Excel Scan fix
- `backend/main.go` — роут PATCH /api/promo/:id/restore, PATCH в CORS
- `frontend/src/api/promo.ts` — promoAPI.restore()
- `frontend/src/pages/PromoAnalysis.tsx` — селект "Состояние" (admin only), подсветка deleted строк
- `frontend/src/components/PromoEditDialog.tsx` — кнопка восстановления, refetchQueries для комментариев
- `frontend/src/hooks/usePromoForm.ts` — deleted_at в PromoFormValues
