package repository

import (
	"backend/config"
)

// UserRecord — запись из tbl_Users.
type UserRecord struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
}

// GetUserByUsername возвращает пользователя из БД по логину.
// Если пользователь не найден, возвращает nil, nil.
func GetUserByUsername(username string) (*UserRecord, error) {
	var u UserRecord
	err := config.DB.QueryRow(
		"SELECT id, username, password_hash, role FROM dbo.tbl_Users WHERE username = ? AND deleted_at IS NULL",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)
	if err != nil {
		// sql.ErrNoRows — не ошибка, просто пользователь не найден
		return nil, nil
	}
	return &u, nil
}
