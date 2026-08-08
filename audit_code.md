Проблема со сломанной историей комментариев вызвана **тремя связанными багами**, которые возникли при внедрении новой таблицы `tbl_PromoComments`.

### В чем именно суть проблемы:
1. **Исчезновение старой истории (Backend):** Когда вы открываете старое промо, API использует `FetchPromoCommentsFallback` и показывает историю. Но как только вы добавляете *один новый комментарий*, он записывается в `tbl_PromoComments`. При следующем запросе бэкенд видит, что новая таблица не пуста (`len(comments) > 0`), и возвращает **только этот один новый комментарий**, игнорируя всю старую историю из поля `comments`!
2. **Баг с датами `Invalid Date` (Backend/Frontend):** Старый парсер регулярками (`FetchPromoCommentsFallback`) извлекает дату в формате `DD.MM.YYYY`. Функция `new Date()` в JavaScript (которая используется в компонентах) не умеет парсить такой формат, из-за чего дата не отображается. Новые комментарии приходят в формате `ISO` (YYYY-MM-DD), поэтому с ними всё работает.
3. **Отсутствие автообновления (Frontend):** В карточках `ApprovalCard` вы сделали `refreshComments`, а в `PromoEditDialog.tsx` забыли добавить триггер перезапроса комментариев после сохранения.

Ниже готовые решения для исправления этих багов.

---

### Шаг 1. Исправляем бэкенд (слияние истории и фикс формата дат)

Откройте файл `backend/handlers/promo.go` и замените функцию `GetPromoCommentsHandler`. 
Мы сделаем умное склеивание: так как старое текстовое поле `comments` по-прежнему хранит ВСЮ историю, мы возьмём из него старые записи и приклеим к ним новые записи из таблицы.

```go
// В файле backend/handlers/promo.go

func GetPromoCommentsHandler(c *gin.Context) {
	id := c.Param("id")
	promoID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}
	
	dbComments, _ := repository.GetPromoComments(promoID)
	legacyComments := repository.FetchPromoCommentsFallback(promoID)

	if len(dbComments) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": legacyComments})
		return
	}

	// Склеиваем историю: legacyComments содержит ВСЮ историю.
	// dbComments содержит только новые. Мы берем разницу из старого массива и добавляем новые.
	diff := len(legacyComments) - len(dbComments)
	if diff > 0 {
		combined := append(legacyComments[:diff], dbComments...)
		c.JSON(http.StatusOK, gin.H{"data": combined})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dbComments})
}
```

Теперь откройте файл `backend/repository/promo_repo.go` и обновите `FetchPromoCommentsFallback`, чтобы он отдавал дату в стандарте ISO, понятном для фронтенда:

```go
// В файле backend/repository/promo_repo.go

func FetchPromoCommentsFallback(promoID int) []models.CommentRow {
	var raw sql.NullString
	if err := config.DB.QueryRow(
		"SELECT comments FROM dbo.tbl_PromoActivities WHERE id = ? AND deleted_at IS NULL", promoID,
	).Scan(&raw); err != nil || !raw.Valid || raw.String == "" {
		return []models.CommentRow{}
	}

	re := regexp.MustCompile(`^\[(\d{2}\.\d{2}\.\d{4})\s+([^|]+)\|([^\]]+)\]:\s*(.*)$`)
	lines := strings.Split(raw.String, "\n")
	result := make([]models.CommentRow, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := re.FindStringSubmatch(line)
		if m != nil {
			// ИСПРАВЛЕНИЕ ЗДЕСЬ: Парсим DD.MM.YYYY в формат ISO (RFC3339) для фронтенда
			isoDate := m[1]
			if t, err := time.Parse("02.01.2006", m[1]); err == nil {
				isoDate = t.Format(time.RFC3339)
			}

			result = append(result, models.CommentRow{
				PromoID:     promoID,
				UserName:    m[3],
				Role:        m[2],
				CommentText: m[4],
				CreatedAt:   models.PtrString(isoDate),
			})
		} else if len(result) > 0 {
			result[len(result)-1].CommentText += "\n" + line
		}
	}
	return result
}
```

---

### Шаг 2. Исправляем автообновление в `PromoEditDialog.tsx`

Откройте `frontend/src/components/PromoEditDialog.tsx`. Нам нужно добавить стейт версии, который будет заставлять `useEffect` запрашивать комментарии заново после сохранения.

**1. Добавьте стейт для триггера (где-то около 105 строки):**
```tsx
const [newComment, setNewComment] = useState('');
const [comments, setComments] = useState<CommentRow[]>([]);
const [commentsLoading, setCommentsLoading] = useState(false);
const [commentsVersion, setCommentsVersion] = useState(0); // НОВАЯ СТРОКА
```

**2. Обновите `useEffect` загрузки комментариев (добавив зависимость `commentsVersion`):**
```tsx
useEffect(() => {
  if (!open || !form?.id) return;
  let cancelled = false;
  setCommentsLoading(true);
  promoAPI.getComments(form.id)
    .then((data: unknown) => {
      if (!cancelled) setComments(Array.isArray(data) ? data as CommentRow[] : []);
    })
    .catch(() => {
      if (!cancelled) setComments([]);
    })
    .finally(() => {
      if (!cancelled) setCommentsLoading(false);
    });
  return () => { cancelled = true; };
}, [open, form?.id, commentsVersion]); // ИСПРАВЛЕНО: добавлена commentsVersion
```

**3. Обновите функции сохранения (чтобы они обновляли версию с небольшой задержкой):**
```tsx
const handleSaveClick = async () => {
  if (isApprover) {
    setConfirmOpen(true);
  } else {
    // ИСПРАВЛЕНО: делаем функцию async и добавляем timeout
    await onSave(newComment.trim() || null);
    setNewComment('');
    setTimeout(() => setCommentsVersion(v => v + 1), 500); 
  }
};

const handleConfirmSave = async () => {
  setConfirmOpen(false);
  // ИСПРАВЛЕНО: делаем функцию async и добавляем timeout
  await onSave(newComment.trim() || null);
  setNewComment('');
  setTimeout(() => setCommentsVersion(v => v + 1), 500);
};
```

*(Не забудьте в кнопке подтверждения поменять `onClick={handleConfirmSave}` если потребуется, но React сам обработает async-функцию).*

### Что изменится после правок:
1. Старые комментарии перестанут пропадать при добавлении новых. Они будут плавно объединяться.
2. Даты из старых комментариев будут корректно парситься и отображаться в формате `ДД.ММ.ГГГГ` без ошибки "Invalid Date".
3. В диалоге редактирования, как только КАМ нажмёт "Сохранить" с новым комментарием, он через полсекунды появится в списке "📝 История переписки" без перезагрузки всего окна.