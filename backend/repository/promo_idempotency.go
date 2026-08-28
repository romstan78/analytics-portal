package repository

import (
	"database/sql"
	"errors"
	"strings"

	"backend/config"
)

// Ключ идемпотентности на создание промо.
//
// Клиент выдаёт UUID на открытие формы. Если ответ на «Сохранить» не дошёл
// из-за сети и пользователь нажал ещё раз, повтор приходит с тем же ключом —
// и вместо второй вставки возвращается уже созданная запись. Только для
// вставки: при обновлении от повторов защищает optimistic locking по updated_at.

// ErrPromoIdempotencyKeyTaken — ключ уже занят другой записью.
//
// Так выглядит проигранная гонка двух одновременных повторов: обе вставки
// дошли до базы, вторая упёрлась в уникальность ключа и откатилась вместе со
// своим промо. Нужный результат при этом уже есть — его создал победитель.
var ErrPromoIdempotencyKeyTaken = errors.New("promo idempotency key taken")

// NormalizePromoIdempotencyKey проверяет ключ, пришедший от клиента.
//
// Формат — UUID: он приходит из чужого браузера, и в ключ первичного ключа
// таблицы годится только то, что мы сами и признаём ключом. Регистр приводится
// к нижнему, иначе один и тот же ключ в разном написании считался бы двумя.
// Пустой и не-UUID отбрасываются молча: сохранение без ключа остаётся
// обычной вставкой, как было до идемпотентности.
func NormalizePromoIdempotencyKey(raw string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if len(key) != 36 {
		return "", false
	}
	for i, r := range key {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
			if !isHex {
				return "", false
			}
		}
	}
	return key, true
}

// FindPromoByIdempotencyKey возвращает промо, созданное по этому ключу.
//
// Ключ ищется вместе с автором: чужой ключ — не наш повтор, и отдавать по нему
// чужую запись нельзя.
func FindPromoByIdempotencyKey(key, username string) (int, bool, error) {
	var promoID int
	err := config.DB.QueryRow(
		`SELECT promo_id FROM dbo.tbl_PromoIdempotency
		  WHERE idempotency_key = ? AND username = ?`,
		key, username,
	).Scan(&promoID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return promoID, true, nil
}

// DeleteExpiredPromoIdempotencyKeys убирает отработавшие ключи.
//
// Ключ нужен ровно до подтверждения записи: повтор через сутки — это уже
// осознанное создание второй записи, а не потерянный ответ.
func DeleteExpiredPromoIdempotencyKeys(olderThanSeconds int) error {
	_, err := config.DB.Exec(
		`DELETE FROM dbo.tbl_PromoIdempotency
		  WHERE created_at < DATEADD(SECOND, ?, SYSUTCDATETIME())`,
		-olderThanSeconds,
	)
	return err
}
