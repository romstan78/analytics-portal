Я внимательно изучил предоставленный код. Вы отлично потрудились! У вас уже есть крепкая архитектура. 

В коде для `ensureTables/goose` вы корректно закрываете соединение через `migrateDB.Close()`, так что **утечки соединений с БД нет** (хотя для стиля Go лучше использовать `defer`, но это мелочи). 

Ниже я подготовил готовые куски кода для исправления **всех 4-х оставшихся архитектурных рисков**, включая тесты, утечку памяти в кэше и безопасность массового согласования.

---

### 🛠 1. Устраняем утечку памяти (Memory Leak) в Кэше

Нужно добавить Garbage Collector (очистку по таймеру) в ваш `InMemoryCache`.

**Файл: `backend/config/cache.go`**
Замените содержимое на это:

```go
package config

import (
	"sync"
	"time"
)

type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]CacheEntry
}

// NewInMemoryCache создаёт новый кэш и запускает фоновую очистку
func NewInMemoryCache(cleanupInterval time.Duration) *InMemoryCache {
	c := &InMemoryCache{
		items: make(map[string]CacheEntry),
	}
	go c.startCleanupTimer(cleanupInterval)
	return c
}

func (c *InMemoryCache) startCleanupTimer(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.ExpiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Data, true
}

func (c *InMemoryCache) Set(key string, data interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}
}

var FiltersCache *InMemoryCache

func init() {
	// Очищаем протухшие ключи каждые 10 минут
	FiltersCache = NewInMemoryCache(10 * time.Minute)
}

const FilterCacheTTL = 5 * time.Minute
```

---

### ✅ 2. Чиним упавшие тесты (Ожидание 404 вместо 500)

Так как вы улучшили логику (теперь при обращении к несуществующей/удаленной записи бэкенд отдает правильный `404 Not Found`, а не падает с `500`), тесты нужно просто обновить под новые реалии.

**Файл: `backend/main_test.go`**
Найдите эти два теста и исправьте ожидаемый HTTP статус:

```go
func TestUpdatePromo_UpdateNonExistent(t *testing.T) {
	// ... (начало теста без изменений)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// ИСПРАВЛЕНИЕ: Теперь мы ожидаем 404 Not Found
	if w.Code != http.StatusNotFound {
		t.Errorf("Ожидался статус 404 для несуществующего ID, получен %d", w.Code)
	}
}
```

```go
func TestDeletePromo_ThenVerifyGone(t *testing.T) {
	// ... (код создания и удаления без изменений)

	// Пробуем обновить удалённое → 404
	payload := map[string]interface{}{"id": id, "status": "Обновлено"}
	body, _ := json.Marshal(payload)
	req2, _ := http.NewRequest("POST", "/api/promo/save", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	
	// ИСПРАВЛЕНИЕ: Теперь мы ожидаем 404 Not Found для удаленной записи
	if w2.Code != http.StatusNotFound {
		t.Errorf("Ожидался статус 404 при обновлении удалённой записи, получен %d", w2.Code)
	}
}
```

---

### 🔒 3. Optimistic Locking для Массового Согласования (Batch Approve)

Нам нужно передавать на бэкенд не только массив `[1, 2, 3]`, но и их `updated_at`.

#### Шаг 3.1: Фронтенд API (`frontend/src/api/promo.ts`)
Измените метод `batchApprove`:
```typescript
  // Массовое согласование
  batchApprove: (items: { id: number, updated_at: string }[], status: string, comment = ''): Promise<unknown> =>
    fetchWithAuth(`${API_BASE}/api/promo/approve/batch`, {
      method: 'POST',
      body: JSON.stringify({ items, status, comment }),
    }).then(async r => {
      const json = await r.json();
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка' } as ApiError;
      return json;
    }),
```

#### Шаг 3.2: Вызов API (`frontend/src/pages/PromoApproval.tsx`)
В функции `handleBatchAction` сформируйте массив объектов:
```typescript
  const handleBatchAction = async () => {
    // ИСПРАВЛЕНИЕ: Собираем id и updated_at для каждой выбранной карточки
    const itemsToApprove = Array.from(selectedIds).map(id => {
      const promo = approvals.find(a => a.id === id);
      return { id: id as number, updated_at: promo?.updated_at || '' };
    });
    
    const status = batchDialog.status;
    setBatchDialog({ open: false, status: '' });
    // ...
    try {
      // Передаем itemsToApprove вместо ids
      await promoAPI.batchApprove(itemsToApprove, status, '');
      // ...
```

#### Шаг 3.3: Бэкенд Хендлер (`backend/handlers/promo.go`)
В функции `BatchApprovePromo` измените структуру `req`:
```go
func BatchApprovePromo(c *gin.Context) {
	// ...
	var req struct {
		Items []struct {
			ID        int    `json:"id"`
			UpdatedAt string `json:"updated_at"`
		} `json:"items"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный запрос"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items не может быть пустым"})
		return
	}
	// ... (switch req.Status оставляем без изменений)

    // Конвертируем в формат репозитория
	var repoItems []repository.BatchApproveItem
	for _, item := range req.Items {
		repoItems = append(repoItems, repository.BatchApproveItem{
			ID:        item.ID,
			UpdatedAt: item.UpdatedAt,
		})
	}

	rowsAffected, err := repository.BatchApprove(agreementNum, repoItems, status, comment, legacyValue)
    // ...
```

#### Шаг 3.4: Бэкенд Репозиторий (`backend/repository/promo_repo.go`)
Обновите функцию `BatchApprove` для работы с объектами и проверкой версии:
```go
type BatchApproveItem struct {
	ID        int
	UpdatedAt string
}

func BatchApprove(agreementNum int, items []BatchApproveItem, status string, comment string, legacyValue string) (int64, error) {
	if len(items) == 0 {
		return 0, nil
	}

	statusField := fmt.Sprintf("agreement%d_status", agreementNum)
	commentField := fmt.Sprintf("agreement%d_comment", agreementNum)
	agreementField := fmt.Sprintf("agreement%d", agreementNum)
	commentRole := fmt.Sprintf("согласование%d", agreementNum)
	timestamp := time.Now().Format("02.01.2006 15:04")

	var totalAffected int64

	for _, item := range items {
		// 1. Читаем текущие comments для конкретного ID
		var currentComments sql.NullString
		err := config.DB.QueryRow("SELECT comments FROM dbo.tbl_PromoActivities WHERE id = ? AND deleted_at IS NULL", item.ID).Scan(&currentComments)
		if err != nil {
			continue // Пропускаем, если удалено или не найдено
		}

		newComments := currentComments.String
		if comment != "" {
			if len(newComments) > 0 && !strings.HasSuffix(newComments, "\n") {
				newComments += "\n"
			}
			newComments += fmt.Sprintf("[%s %s|batch]: %s\n", timestamp, commentRole, comment)
		}

		// 2. Обновляем строку С УЧЕТОМ updated_at (Optimistic Locking)
		query := fmt.Sprintf(
			"UPDATE dbo.tbl_PromoActivities SET %s = ?, %s = ?, %s = ?, comments = ?, updated_at = GETDATE() WHERE id = ? AND deleted_at IS NULL AND updated_at = ?",
			agreementField, statusField, commentField,
		)
		result, err := config.DB.Exec(query, legacyValue, status, comment, newComments, item.ID, item.UpdatedAt)
		if err != nil {
			return totalAffected, err
		}

		affected, _ := result.RowsAffected()
		totalAffected += affected

		// Запись в новую таблицу комментариев только если строка была реально обновлена
		if affected > 0 && comment != "" {
			_ = InsertComment(item.ID, "batch", commentRole, comment)
		}
	}

	return totalAffected, nil
}
```

---

### 👁‍🗨 4. Блокировка полей для удаленных (Soft-Deleted) записей

Чтобы пользователи не пытались редактировать удаленные промо-акции, заблокируем форму.

**Файл: `frontend/src/components/PromoEditDialog.tsx`**

1. Определите флаг удаленности в начале компонента (сразу после `if (!form) return null;`):
```typescript
  const isDeleted = !!form.deleted_at;
  const isApprover = role === 'agreement1' || role === 'agreement2';
```

2. Запретите ввод комментариев для удаленных:
```tsx
  {/* Поле для нового комментария КАМ */}
  <TextField label="Новый комментарий" size="small" fullWidth multiline minRows={1} maxRows={3}
    value={newComment} onChange={(e) => setNewComment(e.target.value)} disabled={isDeleted} />
```

3. Отключите кнопку сохранения:
```tsx
  <Button variant="contained" startIcon={<SaveIcon />} onClick={handleSaveClick} disabled={saving || isDeleted}>
    {saving ? 'Сохранение...' : 'Сохранить'}
  </Button>
```

4. Защитите динамические поля `TextField` в блоках "Плановые" и "Фактические" показатели (найдите функцию `map` и обновите проверку на `editable`):
```tsx
  {[
    { label: 'Baseline (уп)', field: 'baseline_units', editable: true },
    // ...
  ].map(({ label, field, editable }) => {
    // Поле редактируемо, только если оно editable по конфигурации И запись не удалена
    const canEdit = editable && !isDeleted;
    
    return (
      <TextField key={field} label={label} type="text" size="small" fullWidth
        value={getDisplayValue(field, canEdit)}
        onChange={canEdit ? handleFieldChange(field) : undefined}
        onFocus={canEdit ? handleFocus(field) : undefined}
        onBlur={canEdit ? handleBlur(field) : undefined}
        slotProps={{ input: canEdit ? {} : { readOnly: true }, htmlInput: { inputMode: 'text' } }} 
        sx={{ bgcolor: canEdit ? '#ffffff' : '#f0f0f0' }} 
      />
    );
  })}
```
