Ниже представлен детальный и структурированный аудит вашего проекта. Формат подготовлен специально для сохранения в `.md` файл.

В ходе аудита выявлено несколько критических архитектурных недочетов, логических ошибок (включая неработающие комментарии и сломанный фильтр) и проблем с безопасностью (неработающий Optimistic Locking).

---

# Полный аудит проекта «Аналитический портал»

**Дата аудита:** Текущая дата
**Стек:** Go (Gin) + React (Vite, MUI) + MSSQL

---

## 1. Разбор заявленных проблем

### 1.1. Не работает добавление комментариев (Лист согласования)
**Симптомы:** Комментарии при согласовании либо теряются, либо ведут себя непредсказуемо, массовое согласование не оставляет истории.

**Причины (Backend & Frontend):**
1. **Проблема с Refs на фронтенде:** В `PromoApproval.jsx` (вид «Карточки») инпуты комментариев привязываются через `commentRefs.current[id] = el`. При пагинации, фильтрации или скрытии карточек (React может перерендерить список), DOM-элементы уничтожаются, и `ref` указывает на `null` или старый элемент. При отправке комментария отправляется пустая строка.
2. **Перезапись истории при обычном редактировании:** В `handlers/promo.go` (`SavePromo`) есть функция `applyJSONToRow`. Если пользователь открывает диалог редактирования промо и нажимает «Сохранить», поле `Comments` перезаписывается значением из формы (`r.Comments = fmt.Sprint(v)`). Вся история согласований, накопленная функцией `ApprovePromoWithStatus`, **безвозвратно стирается**.
3. **Mass Approve (Массовое согласование) игнорирует историю:** В `repository.BatchApprove` обновляются только поля `agreementN_status` и `agreementN_comment`. В общее поле `comments` (где хранится лог `[дата автор]: текст`) записи **не добавляются**.

**Решение:**
* На фронтенде: Вместо `refs` использовать контролируемое состояние для комментариев: `const [comments, setComments] = useState({})`, обновлять по `onChange`.
* На бэкенде: Запретить прямое редактирование поля `comments` из базовой формы (добавить `"comments"` в список игнорируемых полей в `applyJSONToRow`), либо разделить поля: `manager_comments` (для формы) и `approval_log` (только для записи логов согласования).
* В `BatchApprove` добавить логику дописывания лога в поле `comments`, аналогично одиночному `ApprovePromoWithStatus`.

### 1.2. Криво работает фильтр для карточек (Лист согласования)
**Симптомы:** При первом входе ничего не грузится (пустой экран). Если сбросить фильтры и нажать «Применить», данные не обновляются.

**Причины (Frontend):**
В файле `PromoApproval.jsx` есть жесткая блокировка запросов, если не выбран хотя бы один текстовый фильтр:
```javascript
const handleApply = () => {
  const hasAnyFilter = draftKam || draftNetwork || draftBrand || draftMechanics || draftYear || draftMonth;
  if (!hasAnyFilter) return; // <--- ОШИБКА 1
  // ...
}

const fetchApprovals = useCallback(async () => {
  // ОШИБКА 2: Если текстовые фильтры пустые, запрос не летит, даже если статус "pending"!
  if (!hasApplied || (!appliedKam && !appliedNetwork && !appliedBrand && !appliedMechanics && !appliedYear && !appliedMonth)) return; 
  // ...
```
Пользователь по умолчанию хочет видеть **все промо в статусе "pending"**. Но код заставляет его обязательно выбрать Год или Сеть.

**Решение:**
Убрать эти блокировки. Разрешить отправку запроса только по статусу:
```javascript
const handleApply = () => {
  setHasApplied(true);
  // ... сохранение фильтров
};

const fetchApprovals = useCallback(async () => {
  if (!hasApplied) return; // Достаточно только этого
  // ...
```

### 1.3. Состояние механизма токенов (Refresh Token)
**Текущая реализация:**
Используется хорошая практика: Access Token в памяти/localStorage, Refresh Token в `httpOnly` cookie. При получении 401 ошибки фронтенд вызывает `refreshToken()` и повторяет запрос.

**Найденные проблемы (Race Condition / Шторм запросов):**
Если на странице параллельно отправляются 3 запроса (например, за справочниками, графиками и таблицей) и токен протух, **все 3 запроса одновременно** получат 401 и вызовут `refreshToken()`.
Бэкенд 3 раза сгенерирует новые пары токенов. Из-за асинхронности записи куки браузером, валидным останется только последний, а остальные запросы упадут.

**Решение (Promise Lock на фронтенде):**
В `api/promo.js` нужно внедрить блокировку, чтобы рефреш происходил только один раз, а остальные запросы ждали его завершения:
```javascript
let isRefreshing = false;
let refreshPromise = null;

async function fetchWithAuth(url, options = {}, timeout = 15000) {
  // ... базовый fetch ...
  if (res.status === 401) {
    if (!isRefreshing) {
      isRefreshing = true;
      refreshPromise = refreshToken().finally(() => { isRefreshing = false; });
    }
    const success = await refreshPromise;
    if (success) return doFetch(); // повторяем запрос с новым токеном
  }
  return res;
}
```
Также нужно добавить глобальную очистку сессии (Logout) и редирект на `/login`, если `refreshToken()` вернул `false`.

---

## 2. Критические баги и уязвимости (Backend)

### 2.1. Optimistic Locking полностью сломан
В файле `handlers/promo.go` функция `SavePromo` пытается реализовать Optimistic Locking (защиту от одновременного редактирования), но логика реализована неверно:
```go
// 1. Бэкенд достает текущую строку из БД
row, err := repository.FetchExistingRow(idInt)
updatedAt := row.UpdatedAt // <--- Это время из БД!

// 2. Функция применяет JSON к строке (при этом поле updated_at игнорируется!)
applyJSONToRow(row, input) 

// 3. Бэкенд делает UPDATE, сравнивая updated_at из БД с updated_at из БД
rowsAffected, err := repository.UpdatePromo(idInt, row, updatedAt) 
```
**Суть бага:** Бэкенд игнорирует время `updated_at`, которое прислал фронтенд. Он берет время из базы и сравнивает его с базой. Это условие *всегда* выполняется. Защита от конкурентной записи не работает.

**Решение:**
```go
clientUpdatedAt := fmt.Sprint(input["updated_at"])
// Передаем clientUpdatedAt в UpdatePromo
rowsAffected, err := repository.UpdatePromo(idInt, row, clientUpdatedAt)
```

### 2.2. Игнорирование ошибок в горутинах (Silent Fails)
В функциях `GetPromoFilters` и `GetApprovalFilters` используется `errgroup.WithContext`:
```go
g, _ := errgroup.WithContext(context.Background())
g.Go(func() error { ... return nil })
_ = g.Wait() // <--- ОШИБКА
```
Если запрос к БД упадет из-за таймаута или ошибки, `g.Wait()` вернет ошибку, но она игнорируется `_ = g.Wait()`. В результате фронтенд получит пустые массивы для фильтров, и пользователь не поймет, что произошло. Необходимо обрабатывать ошибку и отдавать `500 Internal Server Error`.

---

## 3. Архитектура и Фронтенд (React)

### 3.1. Антипаттерн использования React Query
В `hooks/usePromoData.js` вы используете `@tanstack/react-query`, но пытаетесь управлять его кэшем вручную через `setRows`:
```javascript
const handleEditSuccess = useCallback((editedId, updatedData) => {
  setRows(prev => prev.map(row => row.id === editedId ? { ...row, ...updatedData } : row));
}, [setRows]);
```
Если React Query решит сделать автоматический refetch (например, при перефокусировке окна или по таймауту), все ваши ручные `setRows` будут перезаписаны старыми данными, пока сервер не ответит.
**Правильный подход:** Использовать `queryClient.invalidateQueries({ queryKey: ['promoData'] })` после успешного `SAVE`, чтобы React Query сам скачал актуальные данные.

### 3.2. Экспорт тяжелых Excel-файлов на клиенте
В `DataTable.jsx` экспорт работает так: скачивается *весь* объем данных JSON (`all=true`), и браузер с помощью `ExcelJS` пытается построить файл:
```javascript
const data = await fetchExportData(); // может быть 50,000+ строк
const workbook = new ExcelJS.Workbook();
// ...
```
Для MSSQL баз с десятками тысяч записей это приведет к зависанию вкладки (Out of Memory).
**Рекомендация:** Перенести генерацию Excel на бэкенд (библиотека `github.com/xuri/excelize/v2`). Бэкенд должен отдавать готовый `.xlsx` файл как бинарный поток (Stream).

### 3.3. Отсутствие пагинации на странице согласования
Эндпоинт `/api/promo/approvals` использует хардкод `SELECT TOP 500`. На фронтенде стоит жесткий срез `approvals.slice(0, 50)`.
Если у КАМа висит 100 промо на согласование, он увидит только первые 50. Чтобы увидеть остальные, ему придется согласовать первые 50 (чтобы они ушли из фильтра). Это плохой UX. Нужна полноценная пагинация (как в `DataTable.jsx`).

---

## 4. Рекомендации по чистоте кода

1. **Типизация:** В проекте есть микс `.js` и `.ts`. Файл `usePromoCalculations.ts` типизирован, а `usePromoData.js` — нет (хотя рядом лежит старый `usePromoData.ts`). Рекомендуется полностью перевести фронтенд на TypeScript.
2. **Безопасность SQL:** Вы используете параметризованные запросы (`?`), что отлично защищает от SQL Injection. Вспомогательная логика сборки `WHERE` (например `AddFilter`) реализована безопасно.
3. **Хранение секретов:** В `docker-compose.yml` засвечен пароль от MSSQL (`SA_PASSWORD: $#Pfchfytw_0378`). Если репозиторий публичный, пароль скомпрометирован. Уберите его в `.env`.
4. **Удаление тестовых данных:** В `main_test.go` есть `cleanupTestData()`, удаляющая по паттерну `LIKE 'TEST-%'`. Это опасно запускать на боевой БД. Для тестов следует использовать отдельную тестовую базу (Testcontainers или SQLite in-memory).

---

## 5. Пошаговый план исправления (Roadmap)

**Этап 1: Исправление критических багов (Backend)**
1. Починить Optimistic Locking в `handlers/promo.go`.
2. Изменить логику `SavePromo`, чтобы она не перезаписывала поле `comments` (убрать `"comments"` из `applyJSONToRow`).
3. В `promo_repo.go` функцию `BatchApprove` переписать так, чтобы она генерировала и аппендила строку лога в `comments` для каждого ID, как это делает одиночный Approve.

**Этап 2: Исправление логики интерфейса (Frontend)**
1. В `PromoApproval.jsx` убрать жесткую блокировку `hasAnyFilter` — позволить запрашивать данные только по фильтру `status`.
2. Заменить `commentRefs` в `PromoApproval.jsx` на стейт (или хотя бы проверять существование DOM узла и не терять его при рендере).
3. Внедрить Promise-lock для `refreshToken()` в `api/promo.js`, чтобы избежать шторма 401 ошибок.

**Этап 3: Оптимизация и рефакторинг**
1. Внедрить серверный рендеринг XLSX через Go.
2. Заменить ручной стейт `setRows` на `queryClient.invalidateQueries` (React Query).
3. Настроить обработку ошибок `g.Wait()` в errgroup на бэкенде.