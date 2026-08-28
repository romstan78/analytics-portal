package repository

import (
	"backend/config"
	"database/sql"
	"errors"
)

// ErrKAMNotLinked — у учётной записи с ролью kam нет закрепления за КАМом.
//
// Для администратора и унаследованных согласующих пустая область по-прежнему
// означает «ограничений нет»: они заводились до появления этой связи. У роли
// kam закрепление обязательно, и его отсутствие — пропущенный шаг заведения
// учётной записи, а не разрешение видеть весь портфель компании. Поэтому режим
// отказа здесь fail-closed: доступа нет, а причина называется прямо.
var ErrKAMNotLinked = errors.New("учётная запись не привязана к КАМу")

// kamLinkRequired отказывает роли kam без закрепления.
func kamLinkRequired(role string, linked bool) error {
	if role == "kam" && !linked {
		return ErrKAMNotLinked
	}
	return nil
}

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
// отсутствие ограничения, а у роли kam незакреплённая учётная запись получает
// ErrKAMNotLinked вместо доступа ко всему реестру.
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
	if errors.Is(err, sql.ErrNoRows) {
		kam, err = "", nil
	}
	if err != nil {
		return "", err
	}
	if err := kamLinkRequired(role, kam != ""); err != nil {
		return "", err
	}
	return kam, nil
}
