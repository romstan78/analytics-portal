package repository

import (
	"database/sql"
	"errors"
	"time"

	"backend/config"
)

// LoginAttemptState — накопленные неудачные попытки одного логина.
type LoginAttemptState struct {
	FailedCount  int
	LastFailedAt time.Time
	LockedUntil  time.Time
}

// LoginLockoutPolicy — правила временной блокировки.
//
// Threshold — сколько неудач подряд закрывают вход, Window — за какое время они
// должны накопиться (редкие опечатки не должны складываться месяцами), Lockout —
// на сколько закрывается вход. Нулевой Lockout выключает блокировку целиком:
// защита от подбора остаётся на лимите по адресу.
type LoginLockoutPolicy struct {
	Threshold int
	Window    time.Duration
	Lockout   time.Duration
}

// Enabled — включена ли блокировка.
func (p LoginLockoutPolicy) Enabled() bool {
	return p.Lockout > 0 && p.Threshold > 0
}

// Locked сообщает, закрыт ли вход на момент now.
func (s LoginAttemptState) Locked(now time.Time) bool {
	return !s.LockedUntil.IsZero() && s.LockedUntil.After(now)
}

// NextAttemptState считает новое состояние после неудачной попытки.
//
// Вынесено из запроса к БД, потому что это и есть само правило блокировки:
// его нужно уметь проверить без базы. Счётчик начинается заново, если прошлая
// неудача была давно, — иначе две опечатки в разные месяцы накапливались бы до
// блокировки. Достигнув порога, вход закрывается на Lockout, а счётчик
// обнуляется: следующая серия считается с нуля.
func NextAttemptState(current LoginAttemptState, now time.Time, policy LoginLockoutPolicy) LoginAttemptState {
	next := LoginAttemptState{FailedCount: 1, LastFailedAt: now, LockedUntil: current.LockedUntil}
	if !current.LastFailedAt.IsZero() && now.Sub(current.LastFailedAt) <= policy.Window {
		next.FailedCount = current.FailedCount + 1
	}
	if policy.Enabled() && next.FailedCount >= policy.Threshold {
		next.LockedUntil = now.Add(policy.Lockout)
		next.FailedCount = 0
	}
	return next
}

// GetLoginAttemptState читает состояние по логину.
// Логина может не быть ни в этой таблице, ни в tbl_Users — это нормально:
// пустое состояние означает «неудач не было».
func GetLoginAttemptState(username string) (LoginAttemptState, error) {
	var (
		state        LoginAttemptState
		lastFailedAt sql.NullTime
		lockedUntil  sql.NullTime
	)
	err := config.DB.QueryRow(
		`SELECT failed_count, last_failed_at, locked_until
		   FROM dbo.tbl_LoginAttempts WHERE username = ?`,
		username,
	).Scan(&state.FailedCount, &lastFailedAt, &lockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginAttemptState{}, nil
	}
	if err != nil {
		return LoginAttemptState{}, err
	}
	if lastFailedAt.Valid {
		state.LastFailedAt = lastFailedAt.Time
	}
	if lockedUntil.Valid {
		state.LockedUntil = lockedUntil.Time
	}
	return state, nil
}

// SaveLoginAttemptState сохраняет состояние попыток (upsert по логину).
func SaveLoginAttemptState(username string, state LoginAttemptState) error {
	var lockedUntil interface{}
	if !state.LockedUntil.IsZero() {
		lockedUntil = state.LockedUntil.UTC()
	}
	_, err := config.DB.Exec(
		`MERGE dbo.tbl_LoginAttempts AS target
		 USING (SELECT ? AS username) AS source ON target.username = source.username
		 WHEN MATCHED THEN UPDATE SET
		     failed_count = ?, last_failed_at = ?, locked_until = ?, updated_at = SYSUTCDATETIME()
		 WHEN NOT MATCHED THEN INSERT (username, failed_count, last_failed_at, locked_until)
		     VALUES (?, ?, ?, ?);`,
		username,
		state.FailedCount, state.LastFailedAt.UTC(), lockedUntil,
		username, state.FailedCount, state.LastFailedAt.UTC(), lockedUntil,
	)
	return err
}

// ResetLoginAttempts убирает счётчик после успешного входа.
func ResetLoginAttempts(username string) error {
	_, err := config.DB.Exec("DELETE FROM dbo.tbl_LoginAttempts WHERE username = ?", username)
	return err
}

// DeleteExpiredLoginAttempts убирает давно не обновлявшиеся строки.
//
// Счётчик заводится на каждый введённый логин, в том числе выдуманный, поэтому
// подбор по словарю оставил бы в таблице по строке на каждую попытку. Живыми
// нужны только свежие: истёкшая блокировка и забытая серия ничего не решают.
func DeleteExpiredLoginAttempts(olderThan time.Duration) error {
	_, err := config.DB.Exec(
		`DELETE FROM dbo.tbl_LoginAttempts
		 WHERE updated_at < DATEADD(SECOND, ?, SYSUTCDATETIME())
		   AND (locked_until IS NULL OR locked_until < SYSUTCDATETIME())`,
		-int(olderThan.Seconds()),
	)
	return err
}
