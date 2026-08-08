Вы проделали отличную работу! Переход на `squirrel` для `buildSalesWhere` и `buildPromoWhere`, внедрение `StreamWriter` для Excel и нормализация комментариев — это мощные архитектурные шаги. 

Однако, как вы сами заметили, остались неприятные "хвосты": фантомные удаленные записи и `setTimeout`. Обе эти проблемы имеют **одну общую причину — неправильная работа с кэшем на фронтенде**. Кроме того, при аудите бэкенда я нашел досадную оплошность: вы написали `squirrel`-билдер для страницы согласований, но забыли его применить!

Ниже представлен детальный аудит и готовые куски кода для решения ваших задач. Вы можете скопировать этот ответ в ваш Markdown-отчет.

---

# 📊 Детальный аудит и решения (07.08.2026)

## 🚨 1. Критический баг бэкенда: Забытый `squirrel` в `GetApprovals`

**Проблема:** В файле `backend/repository/promo_repo.go` вы создали отличную функцию `buildApprovalsWhere(params)`, которая использует `squirrel` для безопасной сборки SQL. Но если посмотреть ниже, в саму функцию `GetApprovals`, вы увидите, что она **всё ещё использует старую ручную конкатенацию строк** (`query += " AND p.year = ?"`), полностью игнорируя созданный вами безопасный билдер!

**Решение:** Переписать `GetApprovals`, чтобы она использовала `buildApprovalsWhere`.

**Замените код `GetApprovals` в `backend/repository/promo_repo.go` на этот:**
```go
func GetApprovals(params ApprovalParams) ([]models.ApprovalRow, int, error) {
	whereClause, args := buildApprovalsWhere(params)

	baseSelect := `
		SELECT
			p.id, p.network_name, p.brand_as, p.sku, p.mechanics, p.year, p.month,
			p.baseline_units, p.plan_promo_units, p.actual_promo_sales_units,
			p.plan_investments_rub, p.plan_roi, p.actual_roi,
			p.conditions, p.comments,
			p.agreement1, p.agreement2, p.status,
			p.agreement1_status, p.agreement1_comment,
			p.agreement2_status, p.agreement2_comment,
			0 as historical_count,
			CAST(NULL AS FLOAT) as avg_historical_roi
		FROM dbo.tbl_PromoActivities p
		WHERE ` + whereClause

	// 1. Считаем общее количество
	countQuery := "SELECT COUNT(*) FROM dbo.tbl_PromoActivities p WHERE " + whereClause
	var total int
	if err := config.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		total = 0
	}

	// 2. Пагинация и сортировка
	query := baseSelect + " ORDER BY p.year DESC, p.month DESC, p.network_name"
	if params.PageSize <= 0 {
		params.PageSize = 50
	}
	if params.PageSize > 500 {
		params.PageSize = 500
	}
	offset := params.Page * params.PageSize
	query += fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, params.PageSize)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []models.ApprovalRow
	for rows.Next() {
		var r models.ApprovalRow
		if err := rows.Scan(
			&r.ID, &r.NetworkName, &r.BrandAS, &r.SKU, &r.Mechanics, &r.Year, &r.Month,
			&r.BaselineUnits, &r.PlanPromoUnits, &r.ActualPromoSalesUnits,
			&r.PlanInvestmentsRub, &r.PlanROI, &r.ActualROI,
			&r.Conditions, &r.Comments,
			&r.Agreement1, &r.Agreement2, &r.Status,
			&r.Agreement1Status, &r.Agreement1Comment,
			&r.Agreement2Status, &r.Agreement2Comment,
			&r.HistoricalCount, &r.AvgHistoricalROI,
		); err == nil {
			results = append(results, r)
		}
	}
	if results == nil {
		results = []models.ApprovalRow{}
	}
	return results, total, nil
}
```

---

## 🛠 2. Решение проблемы Soft-Delete (Фантомные записи)

**Почему записи остаются в таблице после удаления?**
Ваш хук `usePromoData.ts` использует `refreshTrigger`. Когда вы инкрементируете `refreshTrigger` после удаления, React Query делает запрос по *новому* ключу кэша `['promoData', filters, 2]`. Но компонент таблицы может всё ещё отрисовывать данные из старого ключа `['promoData', filters, 1]`, пока идет загрузка. Это классический антипаттерн React Query.

**Как правильно:** Нужно использовать фиксированный ключ и метод `invalidateQueries`.

**1. Исправьте `frontend/src/hooks/usePromoData.ts`:**
```typescript
import { useQuery } from '@tanstack/react-query';
import { useRef } from 'react';

export interface PromoDataResult {
  rows: unknown[];
  loading: boolean;
  error: string | null;
}

export function usePromoData(filters: Record<string, unknown>): PromoDataResult {
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  // УБРАЛИ refreshTrigger из ключа. Теперь ключ стабилен!
  const queryKey = ['promoData', filters];

  const { data: rows = [], isLoading, error } = useQuery({
    queryKey,
    queryFn: async ({ signal }) => {
      const currentFilters = filtersRef.current;
      const params = new URLSearchParams();
      Object.entries(currentFilters).forEach(([key, value]) => {
        if (Array.isArray(value)) {
          value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
        } else if (value !== '' && value != null) {
          params.set(key, String(value));
        }
      });

      const response = await fetch(
        `${import.meta.env.VITE_API_BASE || 'http://localhost:8080'}/api/promo/data?all=true&${params}`,
        { signal, headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` } }
      );
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json();
      return json.data || [];
    },
  });

  return {
    rows: rows as unknown[],
    loading: isLoading,
    error: error ? (error as Error).message : null,
  };
}
```

**2. Обновите `PromoAnalysis.tsx`:**
```typescript
import { useQueryClient } from '@tanstack/react-query';
// ... внутри компонента ...
const queryClient = useQueryClient();
const { rows, loading: dataLoading, error: dataError } = usePromoData(appliedFilters);

const handleDataChanged = useCallback(() => {
  // МАГИЯ ЗДЕСЬ: React Query мгновенно пометит данные как устаревшие
  // и сделает фоновый запрос. Таблица обновится автоматически без морганий.
  queryClient.invalidateQueries({ queryKey: ['promoData'] });
  queryClient.invalidateQueries({ queryKey: ['approvals'] });
}, [queryClient]);
```

---

## ⏱ 3. Избавляемся от `setTimeout` в комментариях

В файлах `ApprovalCard.tsx` и `PromoEditDialog.tsx` вы используете `setTimeout(() => setCommentsVersion(v => v + 1), 800)`. Это вызывает состояние гонки. 

**Решение: React Query для комментариев.**

**1. В `ApprovalCard.tsx` (и аналогично в `PromoEditDialog.tsx`) замените `useEffect` на `useQuery`:**
```typescript
import { useQuery } from '@tanstack/react-query';

// Внутри ApprovalCard:
const { data: comments = [], isLoading: commentsLoading } = useQuery({
  queryKey: ['comments', item.id],
  queryFn: async () => {
    const res = await promoAPI.getComments(item.id);
    return res.data || [];
  },
  enabled: !!item.id, // Запрашиваем только если есть ID
});

// УБЕРИТЕ wrappedCommentOnly и wrappedConfirm. 
// Кнопки должны напрямую вызывать onOpenConfirm и onCommentOnly.
```

**2. В `PromoApproval.tsx` после успешного API-вызова инвалидируйте кэш:**
```typescript
import { useQueryClient } from '@tanstack/react-query';

export default function PromoApproval({ role, onDataChanged }) {
  const queryClient = useQueryClient();
  
  const handleConfirmedAction = async () => {
    // ...
    try {
      await promoAPI.approve(id, status, comment);
      
      // СКАЖИТЕ REACT QUERY ОБНОВИТЬ КОММЕНТАРИИ ДЛЯ ЭТОЙ КАРТОЧКИ!
      queryClient.invalidateQueries({ queryKey: ['comments', id] });
      
      if (status !== 'comment') {
         // Удаляем карточку только если это финальное согласование
         setApprovals(prev => prev.filter(a => a.id !== id));
      }
    // ...
  };
```
*Теперь при сохранении комментария спиннер не появится, а новый комментарий мгновенно отрисуется, как только API ответит `200 OK`.*

---

## 🗂 4. Типизация `PromoEditDialog.tsx` (Пункт #6 из вашего плана)

Чтобы избавиться от `any` и ошибок TS, добавьте интерфейс в `frontend/src/components/PromoEditDialog.tsx` (и переименуйте файл в `.tsx`):

```typescript
import type { PromoFormValues } from '../hooks/usePromoForm';
import type { FilterMeta } from '../hooks/usePromoFilters';

interface PromoEditDialogProps {
  open: boolean;
  onClose: () => void;
  form: PromoFormValues | null;
  setForm: React.Dispatch<React.SetStateAction<PromoFormValues>>;
  recalcPlan: (updates: Partial<PromoFormValues>) => Record<string, string>;
  recalcActual: (updates: Partial<PromoFormValues>) => Record<string, string>;
  onSave: (commentOverride?: string | null) => Promise<void> | void;
  onDelete: () => void;
  saving: boolean;
  deleting: boolean;
  meta: FilterMeta;
  allSkuOptions: string[];
  allNetworkOptions: string[];
  investmentTypes: string[];
  role: string | null;
}

export default function PromoEditDialog({
  open, onClose, form, setForm, recalcPlan, recalcActual,
  onSave, onDelete, saving, deleting,
  meta, allSkuOptions, allNetworkOptions, investmentTypes,
  role,
}: PromoEditDialogProps) { ... }
```

---

## 🗑 5. Как реализовать восстановление Soft-Deleted записей (Пункт #1 из плана)

Для реализации восстановления удаленных промо-акций (Корзина) вам нужно:

**1. Бэкенд (`repository/promo_repo.go`):**
Добавить метод восстановления:
```go
func RestorePromo(id int) (int64, error) {
	result, err := config.DB.Exec("UPDATE dbo.tbl_PromoActivities SET deleted_at = NULL, updated_at = GETDATE() WHERE id = ?", id)
	return result.RowsAffected(), err
}
```
**2. Бэкенд (`handlers/promo.go`):**
Создать хендлер `POST /api/promo/restore/:id`. В нем вызывать `RestorePromo` и писать аудит-лог `InsertAuditLog(id, user, "RESTORE", "")`.

**3. Фронтенд:**
* Добавить в интерфейс `FilterPanel` чекбокс "Показывать удаленные" (передавать `include_deleted=true` в API).
* В таблице `DataTable` добавить кнопку-иконку ♻️ (Restore), которая будет вызывать API и делать `queryClient.invalidateQueries(['promoData'])`.

---

## 🚀 Резюме: Что делать прямо сейчас?

1. Скопируйте исправленный метод `GetApprovals` в `promo_repo.go`. Это устранит несоответствие логики.
2. Переведите `usePromoData` и `PromoAnalysis` на чистый React Query (удалите `refreshTrigger`, используйте `invalidateQueries`). Это починит баг с фантомными удаленными записях в DataGrid.
3. Уберите `setTimeout` из карточек и замените пропс `comments` на локальный `useQuery` внутри `ApprovalCard.tsx`.
4. Переименуйте `PromoEditDialog.jsx` в `.tsx` и вставьте интерфейс `PromoEditDialogProps`.