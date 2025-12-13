package psqldb

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devspotai/arcusdata/util"
)

type InsertBuilder struct {
	table               string
	offsetColumnStart   int
	columns             []string
	valueArgs           []interface{}
	selectTable         string
	parameterizedSelect bool
	valueRows           int
	returning           string
}

// NewInsertBuilder starts an INSERT INTO <table> ...
func NewInsertBuilder(table string) *InsertBuilder {
	return &InsertBuilder{
		table:     table,
		columns:   make([]string, 0, 8),
		valueArgs: make([]any, 0, 8),
		valueRows: 0,
	}
}

// Columns sets the columns to insert into
func (ib *InsertBuilder) Columns(cols ...string) *InsertBuilder {
	for _, c := range cols {
		if _, err := util.ValidateIdentifier(c); err != nil {
			panic(err) // or return error via a different API
		}
		ib.columns = append(ib.columns, c)
	}
	return ib
}

// Values adds a row of values to insert
func (ib *InsertBuilder) Values(vals ...any) *InsertBuilder {
	ib.valueArgs = append(ib.valueArgs, vals...)
	ib.valueRows++
	return ib
}

// SelectFrom sets the SELECT table for INSERT ... SELECT ...
func (ib *InsertBuilder) SelectFrom(table string) *InsertBuilder {
	if _, err := util.ValidateIdentifier(table); err != nil {
		panic(err)
	}
	ib.selectTable = table
	return ib
}

// Returning sets the RETURNING clause
func (ib *InsertBuilder) ReturningColumns(cols ...string) *InsertBuilder {
	for _, c := range cols {
		if _, err := util.ValidateIdentifier(c); err != nil {
			panic(err)
		}
	}
	ib.returning = strings.Join(cols, ", ")
	return ib
}

// Build produces the SQL with $-placeholders and the args slice.
func (ib *InsertBuilder) Build() (string, []interface{}, error) {
	if len(ib.columns) == 0 {
		return "", nil, errors.New("InsertBuilder: no columns specified")
	}
	if ib.selectTable == "" && ib.valueRows == 0 {
		return "", nil, errors.New("InsertBuilder: no values or select table specified")
	}
	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(ib.table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(ib.columns, ", "))
	sb.WriteString(") ")

	paramStart := 1 + ib.offsetColumnStart
	if ib.selectTable != "" {
		sb.WriteString("SELECT ")
		for i, col := range ib.columns {
			if i > 0 {
				sb.WriteString(", ")
			}
			if ib.parameterizedSelect {
				sb.WriteString(fmt.Sprintf("$%d", paramStart+i))
			} else {
				sb.WriteString(col)
			}
		}

		sb.WriteString(" FROM ")
		sb.WriteString(ib.selectTable)
	} else {
		sb.WriteString("VALUES ")

		valueCount := len(ib.columns)
		for r := 0; r < ib.valueRows; r++ {
			if r > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("(")
			for c := 0; c < valueCount; c++ {
				if c > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("$%d", paramStart+(r*valueCount)+c))
			}
			sb.WriteString(")")
		}
	}

	if ib.returning != "" {
		sb.WriteString(" RETURNING ")
		sb.WriteString(ib.returning)
	}

	return sb.String(), ib.valueArgs, nil
}
