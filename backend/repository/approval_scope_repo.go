package repository

import (
	"backend/config"
)

// Область согласования: какие КАМы закреплены за пользователем и на какой
// ступени. Пустой результат означает отсутствие ограничения — так работают
// согласующие, заведённые до появления этой таблицы.

// GetApprovalScope возвращает КАМов, промо которых пользователь согласует на
// указанной ступени. Пустой срез — ограничения нет.
func GetApprovalScope(username string, agreementNum int) ([]string, error) {
	rows, err := config.DB.Query(
		`SELECT kam FROM dbo.tbl_ApprovalScope
		 WHERE username = ? AND agreement_num = ?
		 ORDER BY kam`,
		username, agreementNum,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	kams := []string{}
	for rows.Next() {
		var kam string
		if err := rows.Scan(&kam); err != nil {
			return nil, err
		}
		kams = append(kams, kam)
	}
	return kams, rows.Err()
}

// GetApprovalScopeStages возвращает ступени, на которых у пользователя есть
// закреплённые КАМы. Нужна там, где ступень не следует из роли: у роли kam её
// неоткуда взять, кроме самой области.
func GetApprovalScopeStages(username string) ([]int, error) {
	rows, err := config.DB.Query(
		`SELECT DISTINCT agreement_num FROM dbo.tbl_ApprovalScope
		 WHERE username = ? ORDER BY agreement_num`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stages := []int{}
	for rows.Next() {
		var stage int
		if err := rows.Scan(&stage); err != nil {
			return nil, err
		}
		stages = append(stages, stage)
	}
	return stages, rows.Err()
}

// KAMAllowedByScope проверяет, входит ли КАМ в область. Пустая область
// пропускает всех: ограничения нет.
func KAMAllowedByScope(scope []string, kam string) bool {
	if len(scope) == 0 {
		return true
	}
	for _, allowed := range scope {
		if allowed == kam {
			return true
		}
	}
	return false
}

// GetPromoKAMs возвращает КАМов указанных промо. Используется перед записью:
// решение о согласовании нельзя принимать по данным, присланным клиентом.
func GetPromoKAMs(ids []int) ([]string, error) {
	if len(ids) == 0 {
		return []string{}, nil
	}
	placeholders := ""
	args := make([]interface{}, 0, len(ids))
	for index, id := range ids {
		if index > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	rows, err := config.DB.Query(
		"SELECT DISTINCT ISNULL(kam, '') FROM dbo.tbl_PromoActivities WHERE id IN ("+placeholders+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	kams := []string{}
	for rows.Next() {
		var kam string
		if err := rows.Scan(&kam); err != nil {
			return nil, err
		}
		kams = append(kams, kam)
	}
	return kams, rows.Err()
}

// GetPromoVisibilityScope возвращает КАМов, чьи промо пользователь вправе
// видеть. Пустой срез означает отсутствие ограничения.
//
// Область складывается из двух источников: собственное закрепление учётной
// записи за КАМом (tbl_Users.kam) и КАМы, которых пользователь согласует
// (tbl_ApprovalScope). Поэтому руководитель видит и свой портфель, и портфели
// подчинённых, а рядовой КАМ — только свой.
//
// Администратор ограничению не подлежит. Унаследованный согласующий без
// закрепления и без области тоже: до появления этой связи промо видели все, и
// молча спрятать их у неучтённой учётной записи хуже, чем оставить прежнее
// поведение. У роли kam пустая область — не «ограничений нет», а незаполненное
// закрепление: такая учётная запись получает ErrKAMNotLinked, иначе один
// пропущенный шаг при заведении открывал бы весь портфель компании.
func GetPromoVisibilityScope(username, role string) ([]string, error) {
	if role == "admin" {
		return nil, nil
	}
	rows, err := config.DB.Query(
		`SELECT kam FROM (
		     SELECT LTRIM(RTRIM(kam)) AS kam FROM dbo.tbl_Users
		     WHERE username = ? AND kam IS NOT NULL AND LTRIM(RTRIM(kam)) <> ''
		     UNION
		     SELECT LTRIM(RTRIM(kam)) AS kam FROM dbo.tbl_ApprovalScope
		     WHERE username = ?
		 ) scope ORDER BY kam`,
		username, username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	kams := []string{}
	for rows.Next() {
		var kam string
		if err := rows.Scan(&kam); err != nil {
			return nil, err
		}
		kams = append(kams, kam)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := kamLinkRequired(role, len(kams) > 0); err != nil {
		return nil, err
	}
	return kams, nil
}
