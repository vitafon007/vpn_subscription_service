// Package migrations содержит SQL-миграции, встраиваемые в бинарник.
package migrations

import "embed"

// FS — файловая система со всеми *.sql миграциями.
//
//go:embed *.sql
var FS embed.FS
