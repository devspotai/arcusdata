package psqldb

import (
	"fmt"
	"strings"
)

// ===== Selective UPDATE builder =====

type UpdateBuilder struct {
	table        string
	sets         []string
	args         []interface{}
	filters      []string
	whereArgs    []interface{}
	returning    string
	requireWhere bool // safety guard to avoid accidental full-table updates
}

// NewUpdateBuilder starts an UPDATE <table> ...
func NewUpdateBuilder(table string) *UpdateBuilder {
	return &UpdateBuilder{
		table:        table,
		sets:         make([]string, 0, 8),
		args:         make([]interface{}, 0, 8),
		filters:      make([]string, 0, 4),
		whereArgs:    make([]interface{}, 0, 4),
		requireWhere: true,
	}
}

// Set adds "col = ?" with a value
func (ub *UpdateBuilder) Set(column string, value interface{}) *UpdateBuilder {
	ub.sets = append(ub.sets, column+" = ?")
	ub.args = append(ub.args, value)
	return ub
}

// SetIf adds "col = ?" only if condition is true
func (ub *UpdateBuilder) SetIf(condition bool, column string, value interface{}) *UpdateBuilder {
	if condition {
		return ub.Set(column, value)
	}
	return ub
}

// SetExpr sets column to a raw SQL expression (e.g., now(), ST_GeomFromText(?))
// You can pass args used by the expression in order.
func (ub *UpdateBuilder) SetExpr(column string, expr string, args ...interface{}) *UpdateBuilder {
	ub.sets = append(ub.sets, column+" = "+expr)
	if len(args) > 0 {
		ub.args = append(ub.args, args...)
	}
	return ub
}

// Where appends a WHERE clause fragment joined with AND (use ? placeholders)
func (ub *UpdateBuilder) Where(clause string, args ...interface{}) *UpdateBuilder {
	ub.filters = append(ub.filters, clause)
	if len(args) > 0 {
		ub.whereArgs = append(ub.whereArgs, args...)
	}
	return ub
}

// WhereIf conditionally appends a WHERE clause
func (ub *UpdateBuilder) WhereIf(condition bool, clause string, args ...interface{}) *UpdateBuilder {
	if condition {
		return ub.Where(clause, args...)
	}
	return ub
}

// Returning sets RETURNING <cols>
func (ub *UpdateBuilder) Returning(cols string) *UpdateBuilder {
	ub.returning = cols
	return ub
}

// AllowFullTableUpdate disables the WHERE-required safety check (use sparingly)
func (ub *UpdateBuilder) AllowFullTableUpdate() *UpdateBuilder {
	ub.requireWhere = false
	return ub
}

// Build produces the SQL with $-placeholders and the args slice.
func (ub *UpdateBuilder) Build() (string, []interface{}, error) {
	if len(ub.sets) == 0 {
		return "", nil, fmt.Errorf("UpdateBuilder: no columns to update")
	}
	if ub.requireWhere && len(ub.filters) == 0 {
		return "", nil, fmt.Errorf("UpdateBuilder: missing WHERE (safety guard)")
	}

	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(ub.table)
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(ub.sets, ", "))

	if len(ub.filters) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(ub.filters, " AND "))
	}

	if ub.returning != "" {
		sb.WriteString(" RETURNING ")
		sb.WriteString(ub.returning)
	}
	sb.WriteString(";")

	// args = setArgs followed by whereArgs to match placeholder order
	args := append(append([]interface{}{}, ub.args...), ub.whereArgs...)

	sql := convertQuestionMarksToDollarPlaceholders(sb.String())
	return sql, args, nil
}

// SetPtr adds "col = ?" only if ptr != nil (generic helper)
func SetPtr[T any](ub *UpdateBuilder, column string, ptr *T) *UpdateBuilder {
	if ptr != nil {
		ub.Set(column, *ptr)
	}
	return ub
}
