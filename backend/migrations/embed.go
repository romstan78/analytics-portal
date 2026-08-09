// Package migrations встраивает SQL-миграции goose в бинарник.
package migrations

import "embed"

// FS содержит все goose-миграции (только с числовым префиксом).
// seed_users.sql намеренно исключён: его содержимое дублируется
// в 001_create_tbl_users.sql (seed пользователей).
//
//go:embed 0*.sql
var FS embed.FS
