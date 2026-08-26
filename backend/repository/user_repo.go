package repository

import (
	"backend/config"
	"database/sql"
)

// UserRecord — запись из tbl_Users.
type UserRecord struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	// KAM — имя КАМа справочника, за которым закреплена учётная запись.
	// Пусто у администратора и согласующих.
	KAM string `json:"kam"`
}

// GetUserByUsername возвращает пользователя из БД по логину.
// Если пользователь не найден, возвращает nil, nil.
func GetUserByUsername(username string) (*UserRecord, error) {
	var u UserRecord
	err := config.DB.QueryRow(
		"SELECT id, username, password_hash, role, ISNULL(kam, '') FROM dbo.tbl_Users WHERE username = ? AND deleted_at IS NULL",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.KAM)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetOwnKAM возвращает КАМа, за которым закреплена учётная запись.
//
// В отличие от области согласования сюда не входят подчинённые: реестр сетей
// ведёт каждый КАМ сам за себя, и руководитель работает в нём только со своим
// портфелем, хотя промо подчинённых он согласует. Пустая строка означает
// отсутствие ограничения.
func GetOwnKAM(username, role string) (string, error) {
	if role == "admin" {
		return "", nil
	}
	var kam string
	err := config.DB.QueryRow(
		`SELECT ISNULL(LTRIM(RTRIM(kam)), '') FROM dbo.tbl_Users
		 WHERE username = ? AND deleted_at IS NULL`,
		username,
	).Scan(&kam)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return kam, err
}
