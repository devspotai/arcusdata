package psqldb

import (
	"strings"
)

// QueryBuilder helps build dynamic queries safely
type QueryBuilder struct {
	query        string
	filters      []string
	modifier     string
	args         []interface{}
	modifierArgs []interface{}
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(baseQuery string) *QueryBuilder {
	return &QueryBuilder{
		query:        baseQuery,
		filters:      make([]string, 0),
		modifier:     "",
		args:         make([]interface{}, 0),
		modifierArgs: make([]interface{}, 0),
	}
}

// Append adds a clause to the query
func (qb *QueryBuilder) Append(clause string, args ...interface{}) *QueryBuilder {
	qb.filters = append(qb.filters, clause)
	if len(args) > 0 {
		qb.args = append(qb.args, args...)
	}
	return qb
}

// AppendIf conditionally adds a clause to the query
func (qb *QueryBuilder) AppendIf(condition bool, clause string, args ...interface{}) *QueryBuilder {
	if condition {
		return qb.Append(clause, args...)
	}
	return qb
}

func (qb *QueryBuilder) SetModifierAndModifierArgs(modifier string, args ...interface{}) *QueryBuilder {
	qb.modifier = modifier
	qb.modifierArgs = append(make([]interface{}, 0, len(args)), args...)
	return qb
}

// Build returns the final query and arguments
func (qb *QueryBuilder) Build() (string, []interface{}) {
	if len(qb.filters) > 0 {
		qb.query += " WHERE " + strings.Join(qb.filters, " AND ")
	}
	if qb.modifier != "" {
		qb.query += " " + qb.modifier
		if len(qb.modifierArgs) > 0 {
			qb.args = append(qb.args, qb.modifierArgs...)
		}
	}
	qb.query = strings.TrimSpace(qb.query)
	if !strings.HasSuffix(qb.query, ";") {
		qb.query += ";"
	}
	// Convert question marks to dollar placeholders
	return convertQuestionMarksToDollarPlaceholders(qb.query), qb.args
}

func (qb *QueryBuilder) GetFilters() []string {
	return qb.filters
}

func (qb *QueryBuilder) SetFilters(filters []string) {
	qb.filters = append(make([]string, 0, len(filters)), filters...)
}

func (qb *QueryBuilder) GetArgs() []interface{} {
	return qb.args
}

func (qb *QueryBuilder) SetArgs(args []interface{}) {
	qb.args = append(make([]interface{}, 0, len(args)), args...)
}

func (qb *QueryBuilder) GetModifier() string {
	return qb.modifier
}

func (qb *QueryBuilder) GetModifierArgs() []interface{} {
	return qb.modifierArgs
}
