package psqldb

import (
	"fmt"
	"strings"
)

type InsertBuilder struct {
	BuilderCore

	table string
	cols  []string
	// values will be placeholders only: $1, $2...
	vals []string

	returns []string

	// if authCTEName set, we emit INSERT..SELECT..WHERE EXISTS(auth)
	authCTEName string
}

func NewInsertBuilder(table string) *InsertBuilder {
	i := &InsertBuilder{
		cols:    make([]string, 0, 8),
		vals:    make([]string, 0, 8),
		returns: make([]string, 0, 2),
	}
	tq, err := i.QuoteDottedIdentifier(table)
	if err != nil {
		i.SetErr(err)
	} else {
		i.table = tq
	}
	return i
}

func (i *InsertBuilder) Columns(cols ...string) *InsertBuilder {
	if i.err != nil {
		return i
	}
	if len(cols) == 0 {
		i.SetErr(fmt.Errorf("no columns specified"))
		return i
	}
	i.cols = i.cols[:0]
	for _, c := range cols {
		cq, err := i.QuoteIdentifier(c)
		if err != nil {
			i.SetErr(err)
			return i
		}
		i.cols = append(i.cols, cq)
	}
	return i
}

func (i *InsertBuilder) Values(vals ...any) *InsertBuilder {
	if i.err != nil {
		return i
	}
	if len(i.cols) == 0 {
		i.SetErr(fmt.Errorf("call Columns() before Values()"))
		return i
	}
	if len(vals) != len(i.cols) {
		i.SetErr(fmt.Errorf("values count %d does not match columns %d", len(vals), len(i.cols)))
		return i
	}

	i.vals = i.vals[:0]
	for _, v := range vals {
		i.vals = append(i.vals, i.Param(v))
	}
	return i
}

// RequireAuthCTE makes the INSERT conditional: INSERT .. SELECT .. WHERE EXISTS(auth)
func (i *InsertBuilder) RequireAuthCTE(cteName string) *InsertBuilder {
	if i.err != nil {
		return i
	}
	i.authCTEName = cteName
	return i
}

func (i *InsertBuilder) ReturningCols(cols ...string) *InsertBuilder {
	if i.err != nil {
		return i
	}
	i.returns = i.returns[:0]
	for _, c := range cols {
		cq, err := i.QuoteIdentifier(c)
		if err != nil {
			i.SetErr(err)
			return i
		}
		i.returns = append(i.returns, cq)
	}
	return i
}

func (i *InsertBuilder) Build() (string, []any, error) {
	if i.err != nil {
		return "", nil, i.err
	}
	if i.table == "" {
		return "", nil, fmt.Errorf("no table")
	}
	if len(i.cols) == 0 {
		return "", nil, fmt.Errorf("no columns")
	}
	if len(i.vals) == 0 {
		return "", nil, fmt.Errorf("no values")
	}

	var sb strings.Builder
	if len(i.ctes) > 0 {
		sb.WriteString("WITH ")
		sb.WriteString(strings.Join(i.ctes, ", "))
		sb.WriteByte(' ')
	}

	sb.WriteString("INSERT INTO ")
	sb.WriteString(i.table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(i.cols, ", "))
	sb.WriteString(") ")

	if strings.TrimSpace(i.authCTEName) == "" {
		// regular VALUES insert
		sb.WriteString("VALUES (")
		sb.WriteString(strings.Join(i.vals, ", "))
		sb.WriteString(")")
	} else {
		// gated insert
		cteQ, err := i.QuoteIdentifier(i.authCTEName)
		if err != nil {
			return "", nil, err
		}
		sb.WriteString("SELECT ")
		sb.WriteString(strings.Join(i.vals, ", "))
		sb.WriteString(" WHERE EXISTS (SELECT 1 FROM ")
		sb.WriteString(cteQ)
		sb.WriteString(")")
	}

	if len(i.returns) > 0 {
		sb.WriteString(" RETURNING ")
		sb.WriteString(strings.Join(i.returns, ", "))
	}

	return sb.String(), i.args, nil
}

// ApplyAuthGuard satisfies CTEContext.
func (i *InsertBuilder) ApplyAuthGuard(cteName string) { i.RequireAuthCTE(cteName) }
