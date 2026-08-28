import { useCallback, useEffect } from 'react';
import { getUsername } from '../api/auth';
import { writeDraft } from '../utils/formDraft';

// Пауза перед записью черновика: набранное успевает дописаться, а хранилище не
// переписывается на каждое нажатие.
const DRAFT_SAVE_DELAY_MS = 500;

interface FormDraftOptions<T> {
  // null выключает черновик: форма закрыта, открыта на просмотр или правка запрещена.
  storageKey: string | null;
  values: T;
  // Есть ли что сохранять: значения формы разошлись с тем, что пришло с сервера.
  dirty: boolean;
  delayMs?: number;
}

/**
 * Сохранение черновика формы и предупреждение о несохранённом.
 *
 * Восстановления здесь намеренно нет: подставлять значения молча нельзя —
 * пользователь должен понимать, откуда они взялись, — а предложить их можно
 * только в обработчике открытия формы. Эффект с прямым setState для этого не
 * годится и запрещён правилом react-hooks/set-state-in-effect.
 */
export function useFormDraft<T>({
  storageKey, values, dirty, delayMs = DRAFT_SAVE_DELAY_MS,
}: FormDraftOptions<T>) {
  // Немедленная запись — для случаев, когда паузы уже не будет: закрытие формы
  // и выгрузка вкладки. Иначе последние набранные полсекунды пропали бы.
  const saveNow = useCallback(() => {
    if (!storageKey || !dirty) return;
    writeDraft(storageKey, values, getUsername());
  }, [storageKey, values, dirty]);

  useEffect(() => {
    if (!storageKey || !dirty) return;
    const timer = setTimeout(() => writeDraft(storageKey, values, getUsername()), delayMs);
    return () => clearTimeout(timer);
  }, [storageKey, values, dirty, delayMs]);

  useEffect(() => {
    if (!dirty) return;
    const handleUnload = (event: BeforeUnloadEvent) => {
      saveNow();
      // Текст диалога задаёт браузер; от страницы требуется только отмена
      // события. returnValue оставлен для браузеров, где одного preventDefault
      // ещё недостаточно.
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', handleUnload);
    return () => window.removeEventListener('beforeunload', handleUnload);
  }, [dirty, saveNow]);

  return { saveNow };
}
