package repository

import (
	"context"
	"database/sql"
	"errors"

	"backend/config"
)

// Взаимное исключение записи промо и офлайн-дедупликации.
//
// sync_script/dedupe_promo.py переносит строки между promo_id и переписывает
// ссылки в tbl_AuditLog и tbl_PromoComments. Если в этот момент бэкенд правит
// те же строки, правка уходит в запись, которую скрипт уже пометил удалённой.
//
// Разграничение — на прикладной блокировке SQL Server. Бэкенд берёт Shared:
// такие блокировки совместимы между собой, поэтому обычная запись через API
// не замедляется. Скрипт берёт Exclusive: он дожидается уже начатых запросов
// и не пускает новые, пока не закончит. Владелец блокировки — транзакция,
// поэтому она снимается на commit, rollback и на обрыве соединения:
// зависшей блокировки после падения не остаётся.

// PromoDedupLockResource — имя ресурса блокировки.
// Должно совпадать с DEDUP_LOCK_RESOURCE в sync_script/dedupe_promo.py.
const PromoDedupLockResource = "promo_dedup"

// promoWriteLockTimeoutMS — сколько запрос ждёт своей очереди.
// Дедупликация идёт заметно дольше, поэтому ждать её окончания бессмысленно:
// лучше быстро вернуть 503 и дать клиенту повторить.
const promoWriteLockTimeoutMS = 5000

// ErrPromoDedupInProgress — идёт дедупликация, запись сейчас недоступна.
var ErrPromoDedupInProgress = errors.New("promo dedup in progress")

// acquirePromoSharedLock берёт разделяемую блокировку до конца транзакции.
func acquirePromoSharedLock(tx *sql.Tx) error {
	var code int
	err := tx.QueryRow(`
		DECLARE @result int;
		EXEC @result = sp_getapplock
			@Resource = ?, @LockMode = 'Shared',
			@LockOwner = 'Transaction', @LockTimeout = ?;
		SELECT @result;`,
		PromoDedupLockResource, promoWriteLockTimeoutMS).Scan(&code)
	if err != nil {
		return err
	}
	// 0 и 1 — блокировка получена; отрицательные коды: -1 таймаут,
	// -2 отмена, -3 взаимоблокировка, -999 ошибка вызова.
	if code < 0 {
		config.Logger.Warn("promo_write_lock_denied", "code", code)
		return ErrPromoDedupInProgress
	}
	return nil
}

// WithPromoWrite выполняет fn в транзакции, удерживающей блокировку записи.
// Все правки промо обязаны идти через неё: иначе дедупликация может увести
// строку из-под запроса.
func WithPromoWrite(fn func(tx *sql.Tx) error) error {
	tx, err := config.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := acquirePromoSharedLock(tx); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
