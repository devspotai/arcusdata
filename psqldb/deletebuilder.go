package psqldb

import (
	"fmt"
	"strings"
)

// ===== Selective UPDATE builder =====

type DeleteBuilder struct {
	query        string
	filters      []string
	whereArgs    []interface{}
	requireWhere bool // safety guard to avoid accidental full-table updates
}

// NewDeleteBuilder starts a DELETE FROM <table> ...
func NewDeleteBuilder(baseQuery string) *DeleteBuilder {
	return &DeleteBuilder{
		query:        baseQuery,
		filters:      make([]string, 0, 4),
		whereArgs:    make([]interface{}, 0, 4),
		requireWhere: true,
	}
}

func (db *DeleteBuilder) Append(additionalQuery string) *DeleteBuilder {
	db.query += " " + additionalQuery
	return db
}

// Where appends a WHERE clause fragment joined with AND (use ? placeholders)
func (db *DeleteBuilder) Where(clause string, args ...interface{}) *DeleteBuilder {
	db.filters = append(db.filters, clause)
	if len(args) > 0 {
		db.whereArgs = append(db.whereArgs, args...)
	}
	return db
}

// WhereIf conditionally appends a WHERE clause
func (db *DeleteBuilder) WhereIf(condition bool, clause string, args ...interface{}) *DeleteBuilder {
	if condition {
		return db.Where(clause, args...)
	}
	return db
}

// AllowFullTableUpdate disables the WHERE-required safety check (use sparingly)
func (db *DeleteBuilder) AllowFullTableUpdate() *DeleteBuilder {
	db.requireWhere = false
	return db
}

// Build produces the SQL with $-placeholders and the args slice.
func (db *DeleteBuilder) Build() (string, []interface{}, error) {
	if len(db.filters) == 0 && db.requireWhere {
		return "", nil, fmt.Errorf("DeleteBuilder: missing WHERE (safety guard)")
	}

	var sb strings.Builder
	if len(db.filters) > 0 {
		if strings.Contains(strings.ToUpper(db.query), "WHERE") {
			sb.WriteString(" AND ")
		} else {
			sb.WriteString(" WHERE ")
		}
		sb.WriteString(strings.Join(db.filters, " AND "))
	}

	sb.WriteString(";")

	// args = setArgs followed by whereArgs to match placeholder order
	args := append([]interface{}{}, db.whereArgs...)

	sql := convertQuestionMarksToDollarPlaceholders(sb.String())
	return sql, args, nil
}
