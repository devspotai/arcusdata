package psqldb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsertBuilder_SelectFrom_NonParameterized(t *testing.T) {
	ib := NewInsertBuilder("dest_table")
	ib.Columns("col1", "col2")
	ib.SelectFrom("src_table")

	sql, args, err := ib.Build()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO dest_table (col1, col2) SELECT col1, col2 FROM src_table", sql)
	assert.Empty(t, args)
}

func TestInsertBuilder_SelectFrom_Parameterized_WithReturningAndArgs(t *testing.T) {
	ib := NewInsertBuilder("dest")
	ib.Columns("a", "b")
	ib.SelectFrom("src")
	ib.parameterizedSelect = true
	ib.offsetColumnStart = 2
	ib.valueArgs = []interface{}{"x", "y"}
	ib.returning = "id"

	sql, args, err := ib.Build()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO dest (a, b) SELECT $3, $4 FROM src RETURNING id", sql)
	assert.Equal(t, []interface{}{"x", "y"}, args)
}
