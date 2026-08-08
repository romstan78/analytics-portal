# Analytics Portal

## Стек
- **Backend:** Go 1.22 + Gin + MSSQL (go-mssqldb) + squirrel + excelize + golang-jwt
- **Frontend:** React 18 + TypeScript + Vite + MUI v5/v6 + Recharts + React Query (useEffect) + Vite
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

## Выполненные изменения (из project_audit.md)
1. ✅ `getComments` API + `FetchPromoCommentsFallback` (слияние legacy + dbComments)
2. ✅ JWT_SECRET: panic при GO_ENV=production без ключа
3. ✅ TS-интерфейсы для DrilldownModal, FilterPanel
4. ✅ `buildPromoWhere` на squirrel (GetPromoRows, GetPromoRowsStream)
5. ✅ RateLimiter → golang.org/x/time/rate (Token Bucket)
6. ✅ `applyJSONToRow` сохранён для UPDATE, DTO — для INSERT

## Известные проблемы
1. **Soft-deleted записи отображаются в таблице** — страница не обновляется после удаления. API возвращает 404.
2. **История комментариев не обновляется мгновенно** — `setTimeout(800ms)` в ApprovalCard + PromoEditDialog. Нет WebSocket/SSE.
3. **500 при FetchExistingRow на старых записях** — исправлено через `NULL as plan_promo_cip_olap`
4. **Фронтенд не перезапрашивает историю после комментария** — частично исправлено через `commentsVersion`, но 800ms задержка ненадёжна

## План действий (приоритет)
- [ ] **#1:** Кнопка восстановления soft-deleted записей в таблице
- [ ] **#2:** Заменить `setTimeout(800)` на перезапрос после успешного ответа API (onSuccess callback)
- [ ] **#3:** goose/golang-migrate вместо ensureTables()
- [ ] **#4:** TS: включить noUnusedLocals, noUnusedParameters после чистки
- [ ] **#5:** Покрывающий индекс `IX_PromoActivities_Filters` на tbl_PromoActivities
- [ ] **#6:** Типизация пропсов PromoEditDialog (implicit any)
- [ ] **#7:** MUI Grid item → Grid2 (deprecation)

## Файлы, изменённые в последней сессии
- `backend/config/auth.go` — JWT_SECRET requireSecret()
- `backend/main.go` — IPRateLimiter, удалён DebugPromo
- `backend/config/db.go` — GetDBInfo()
- `backend/handlers/promo.go` — GetPromoCommentsHandler (слияние), SavePromo (диагностика)
- `backend/repository/promo_repo.go` — buildPromoWhere, FetchPromoCommentsFallback (ISO-даты), NULL-колонки для старых записей
- `backend/services/promo_service.go` — MergeDTOIntoRow
- `backend/models/types.go` — PtrString
- `frontend/src/components/ApprovalCard.tsx` — getComments API, разбор {data: [...]}, commentsVersion
- `frontend/src/components/PromoEditDialog.tsx` — аналогично + handleSaveClick async
- `frontend/src/components/DrilldownModal.tsx` — TS-интерфейсы
- `frontend/src/components/FilterPanel.tsx` — TS-интерфейсы
- `frontend/src/api/promo.ts` — getComments метод
- `frontend/src/types/promo.ts` — CommentRow тип