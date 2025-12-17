package psqldb

import (
	"fmt"
	"strings"
)

type DeleteBuilder struct {
	BuilderCore

	table   string
	wheres  []string
	returns []string

	requireWhere bool
}

func NewDeleteBuilder(table string) *DeleteBuilder {
	d := &DeleteBuilder{
		wheres:       make([]string, 0, 4),
		returns:      make([]string, 0, 2),
		requireWhere: true,
	}
	tq, err := d.QuoteDottedIdentifier(table)
	if err != nil {
		d.SetErr(err)
	} else {
		d.table = tq
	}
	return d
}

func (d *DeleteBuilder) RequireWhere(v bool) *DeleteBuilder { d.requireWhere = v; return d }

func (d *DeleteBuilder) WhereEq(col string, val any) *DeleteBuilder {
	if d.err != nil {
		return d
	}
	cq, err := d.QuoteDottedIdentifier(col)
	if err != nil {
		d.SetErr(err)
		return d
	}
	d.wheres = append(d.wheres, fmt.Sprintf("%s = %s", cq, d.Param(val)))
	return d
}

func (u *DeleteBuilder) WhereIsNull(col string) *DeleteBuilder {
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

func (u *DeleteBuilder) WhereIsNotNull(col string) *DeleteBuilder {
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

func (d *DeleteBuilder) RequireAuthCTE(cteName string) *DeleteBuilder {
	if d.err != nil {
		return d
	}
	cteQ, err := d.QuoteIdentifier(cteName)
	if err != nil {
		d.SetErr(err)
		return d
	}
	d.wheres = append(d.wheres, fmt.Sprintf("EXISTS (SELECT 1 FROM %s)", cteQ))
	return d
}

func (d *DeleteBuilder) ReturningCols(cols ...string) *DeleteBuilder {
	if d.err != nil {
		return d
	}
	d.returns = d.returns[:0]
	for _, c := range cols {
		cq, err := d.QuoteDottedIdentifier(c)
		if err != nil {
			d.SetErr(err)
			return d
		}
		d.returns = append(d.returns, cq)
	}
	return d
}

func (d *DeleteBuilder) Build() (string, []any, error) {
	if d.err != nil {
		return "", nil, d.err
	}
	if d.table == "" {
		return "", nil, fmt.Errorf("no table")
	}
	if d.requireWhere && len(d.wheres) == 0 {
		return "", nil, fmt.Errorf("WHERE is required (safety guard)")
	}

	var sb strings.Builder
	if len(d.ctes) > 0 {
		sb.WriteString("WITH ")
		sb.WriteString(strings.Join(d.ctes, ", "))
		sb.WriteByte(' ')
	}

	sb.WriteString("DELETE FROM ")
	sb.WriteString(d.table)

	if len(d.wheres) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(d.wheres, " AND "))
	}

	if len(d.returns) > 0 {
		sb.WriteString(" RETURNING ")
		sb.WriteString(strings.Join(d.returns, ", "))
	}

	return sb.String(), d.args, nil
}
