package psqldb

import (
	"fmt"
	"strings"
)

type UpdateBuilder struct {
	BuilderCore

	table   string
	sets    []string
	wheres  []string
	returns []string

	requireWhere bool
	authGuards   map[string]struct{}
}

func NewUpdateBuilder(table string) *UpdateBuilder {
	u := &UpdateBuilder{
		sets:         make([]string, 0, 8),
		wheres:       make([]string, 0, 4),
		returns:      make([]string, 0, 2),
		requireWhere: true,
	}
	tq, err := u.QuoteDottedIdentifier(table)
	if err != nil {
		u.SetErr(err)
	} else {
		u.table = tq
	}
	return u
}

func (u *UpdateBuilder) RequireWhere(v bool) *UpdateBuilder { u.requireWhere = v; return u }

func (u *UpdateBuilder) Set(col string, val any) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.sets = append(u.sets, fmt.Sprintf("%s = %s", cq, u.Param(val)))
	return u
}

func (u *UpdateBuilder) SetExpr(col string, expr Expr) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	if err := safeExpr(expr); err != nil {
		u.SetErr(err)
		return u
	}

	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.sets = append(u.sets, fmt.Sprintf("%s = %s", cq, expr.String()))
	return u
}

func (u *UpdateBuilder) WhereEq(col string, val any) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s = %s", cq, u.Param(val)))
	return u
}

func (u *UpdateBuilder) WhereIsNull(col string) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s IS NULL", cq))
	return u
}

func (u *UpdateBuilder) WhereIsNotNull(col string) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s IS NOT NULL", cq))
	return u
}

func (u *UpdateBuilder) WhereIn(col string, vals ...any) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	var placeholders []string
	for i := range vals {
		placeholders = append(placeholders, u.Param(vals[i]))
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s IN (%s)", cq, strings.Join(placeholders, ", ")))
	return u
}

func (u *UpdateBuilder) WhereNotIn(col string, vals ...any) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	var placeholders []string
	for i := range vals {
		placeholders = append(placeholders, u.Param(vals[i]))
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s NOT IN (%s)", cq, strings.Join(placeholders, ", ")))
	return u
}

func (u *UpdateBuilder) WhereNotEq(col string, val any) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s != %s", cq, u.Param(val)))
	return u
}

func (u *UpdateBuilder) RequireAuthCTE(cteName string) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	cteQ, err := u.QuoteIdentifier(cteName)
	if err != nil {
		u.SetErr(err)
		return u
	}
	if u.authGuards == nil {
		u.authGuards = make(map[string]struct{}, 1)
	}
	if _, done := u.authGuards[cteQ]; done {
		return u
	}
	u.authGuards[cteQ] = struct{}{}
	u.wheres = append(u.wheres, fmt.Sprintf("EXISTS (SELECT 1 FROM %s)", cteQ))
	return u
}

func (u *UpdateBuilder) ReturningCols(cols ...string) *UpdateBuilder {
	if u.err != nil {
		return u
	}
	u.returns = u.returns[:0]
	for _, c := range cols {
		cq, err := u.QuoteDottedIdentifier(c)
		if err != nil {
			u.SetErr(err)
			return u
		}
		u.returns = append(u.returns, cq)
	}
	return u
}

func (u *UpdateBuilder) Build() (string, []any, error) {
	if u.err != nil {
		return "", nil, u.err
	}
	if u.table == "" {
		return "", nil, fmt.Errorf("no table")
	}
	if len(u.sets) == 0 {
		return "", nil, fmt.Errorf("no SET clauses")
	}
	if u.requireWhere && len(u.wheres) == 0 {
		return "", nil, fmt.Errorf("WHERE is required (safety guard)")
	}

	var sb strings.Builder
	if len(u.ctes) > 0 {
		sb.WriteString("WITH ")
		sb.WriteString(strings.Join(u.ctes, ", "))
		sb.WriteByte(' ')
	}

	sb.WriteString("UPDATE ")
	sb.WriteString(u.table)
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(u.sets, ", "))

	if len(u.wheres) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(u.wheres, " AND "))
	}

	if len(u.returns) > 0 {
		sb.WriteString(" RETURNING ")
		sb.WriteString(strings.Join(u.returns, ", "))
	}

	return sb.String(), u.args, nil
}

// ApplyAuthGuard satisfies CTEContext.
func (u *UpdateBuilder) ApplyAuthGuard(cteName string) { u.RequireAuthCTE(cteName) }
