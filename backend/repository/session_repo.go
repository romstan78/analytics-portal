package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"backend/config"
)

// ErrSessionNotActive возвращается, когда предъявленный refresh-токен не найден
// среди активных сессий: он уже использован, отозван или истёк.
var ErrSessionNotActive = errors.New("refresh-сессия не активна")

// Причины отзыва. Значения совпадают с CHECK-ограничением миграции 009.
const (
	RevokeCauseLogout        = "logout"
	RevokeCauseRotated       = "rotated"
	RevokeCauseReuseDetected = "reuse_detected"
	RevokeCauseUserRevoked   = "user_revoked"
)

// HashRefreshToken возвращает SHA-256 от токена в hex. В базе хранится только
// хеш, поэтому её содержимое нельзя предъявить как валидный токен.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateRefreshSession регистрирует выданный refresh-токен.
func CreateRefreshSession(username, token string, expiresAt time.Time) error {
	_, err := config.DB.Exec(
		"INSERT INTO dbo.tbl_RefreshSessions (username, token_hash, expires_at) VALUES (?, ?, ?)",
		username, HashRefreshToken(token), expiresAt.UTC(),
	)
	return err
}

// ConsumeRefreshSession атомарно гасит активную сессию по токену: строка
// помечается отозванной только если она ещё активна. Это делает повторное
// использование одного refresh-токена невозможным даже при гонке запросов.
func ConsumeRefreshSession(token string) (username string, err error) {
	row := config.DB.QueryRow(`
		UPDATE dbo.tbl_RefreshSessions
		SET revoked_at = SYSUTCDATETIME(), revoke_cause = ?
		OUTPUT deleted.username
		WHERE token_hash = ? AND revoked_at IS NULL AND expires_at > SYSUTCDATETIME()`,
		RevokeCauseRotated, HashRefreshToken(token),
	)
	if scanErr := row.Scan(&username); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return "", ErrSessionNotActive
		}
		return "", scanErr
	}
	return username, nil
}

// RevokeRefreshSession отзывает конкретную сессию, например при выходе.
// Отсутствие строки ошибкой не считается: выход должен завершаться успешно.
func RevokeRefreshSession(token, cause string) error {
	_, err := config.DB.Exec(`
		UPDATE dbo.tbl_RefreshSessions
		SET revoked_at = SYSUTCDATETIME(), revoke_cause = ?
		WHERE token_hash = ? AND revoked_at IS NULL`,
		cause, HashRefreshToken(token),
	)
	return err
}

// RevokeAllUserSessions гасит все активные сессии пользователя. Вызывается при
// признаках кражи токена: повторное предъявление уже использованного refresh.
func RevokeAllUserSessions(username, cause string) (int64, error) {
	res, err := config.DB.Exec(`
		UPDATE dbo.tbl_RefreshSessions
		SET revoked_at = SYSUTCDATETIME(), revoke_cause = ?
		WHERE username = ? AND revoked_at IS NULL`,
		cause, username,
	)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return affected, nil
}

// DeleteExpiredRefreshSessions удаляет давно истёкшие записи, чтобы таблица не
// росла бесконечно. Ошибка не критична: это обслуживание, а не бизнес-операция.
func DeleteExpiredRefreshSessions(olderThan time.Duration) error {
	_, err := config.DB.Exec(
		"DELETE FROM dbo.tbl_RefreshSessions WHERE expires_at < DATEADD(second, ?, SYSUTCDATETIME())",
		-int(olderThan.Seconds()),
	)
	return err
}
