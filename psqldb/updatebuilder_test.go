package psqldb

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateBuilder_BasicBuildAndArgsOrder(t *testing.T) {
	b := NewUpdateBuilder("users")
	b.Set("name", "Bob")
	b.WhereEq("id", 1)
	b.ReturningCols("id", "name")

	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "UPDATE") {
		t.Fatalf("sql missing UPDATE: %s", sql)
	}
	if !strings.Contains(sql, " SET ") {
		t.Fatalf("sql missing SET: %s", sql)
	}
	if !strings.Contains(sql, " WHERE ") {
		t.Fatalf("sql missing WHERE: %s", sql)
	}
	if !strings.Contains(sql, "RETURNING") {
		t.Fatalf("sql missing RETURNING: %s", sql)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %#v", len(args), args)
	}
	if args[0] != "Bob" || args[1] != 1 {
		t.Fatalf("unexpected args order or values: %#v", args)
	}
	assert.Equal(t, "UPDATE \"users\" SET \"name\" = $1 WHERE \"id\" = $2 RETURNING \"id\", \"name\"", sql)
}

func TestUpdateBuilder_RequireWhereGuardAndDisable(t *testing.T) {
	// With requireWhere=true (default), missing WHERE should error
	b := NewUpdateBuilder("users")
	b.Set("name", "Bob")
	_, _, err := b.Build()
	if err == nil || !strings.Contains(err.Error(), "WHERE is required") {
		t.Fatalf("expected WHERE guard error, got: %v", err)
	}

	// Disable requireWhere and build should succeed without WHERE
	b2 := NewUpdateBuilder("users")
	b2.Set("name", "Bob")
	b2.RequireWhere(false)
	sql, args, err := b2.Build()
	if err != nil {
		t.Fatalf("unexpected error when requireWhere disabled: %v", err)
	}
	if !strings.Contains(sql, "UPDATE") || !strings.Contains(sql, " SET ") {
		t.Fatalf("unexpected sql when requireWhere disabled: %s", sql)
	}
	if len(args) != 1 || args[0] != "Bob" {
		t.Fatalf("unexpected args when requireWhere disabled: %#v", args)
	}
	assert.Equal(t, "UPDATE \"users\" SET \"name\" = $1", sql)
}

func TestUpdateBuilder_NullChecks(t *testing.T) {
	b := NewUpdateBuilder("users")
	b.Set("deleted", true)
	b.WhereIsNull("deleted_at")
	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "IS NULL") {
		t.Fatalf("expected IS NULL in sql, got: %s", sql)
	}
	if len(args) != 1 || args[0] != true {
		t.Fatalf("unexpected args for IS NULL test: %#v", args)
	}
	assert.Equal(t, "UPDATE \"users\" SET \"deleted\" = $1 WHERE \"deleted_at\" IS NULL", sql)

	b2 := NewUpdateBuilder("users")
	b2.Set("active", false)
	b2.WhereIsNotNull("last_seen")
	sql2, args2, err := b2.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql2, "IS NOT NULL") {
		t.Fatalf("expected IS NOT NULL in sql, got: %s", sql2)
	}
	if len(args2) != 1 || args2[0] != false {
		t.Fatalf("unexpected args for IS NOT NULL test: %#v", args2)
	}
	assert.Equal(t, "UPDATE \"users\" SET \"active\" = $1 WHERE \"last_seen\" IS NOT NULL", sql2)
}

func TestUpdateBuilder_RequireAuthCTEAndMultipleClauses(t *testing.T) {
	b := NewUpdateBuilder("users")
	b.Set("count", 10)
	b.RequireAuthCTE("auth_allowed")
	b.WhereEq("id", 3)

	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "EXISTS (SELECT 1 FROM") {
		t.Fatalf("expected EXISTS CTE check in sql, got: %s", sql)
	}
	// args should be [10, 3] in that order
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %#v", len(args), args)
	}
	if args[0] != 10 || args[1] != 3 {
		t.Fatalf("unexpected args order or values: %#v", args)
	}

	// Multiple sets and wheres mixing param and non-param where
	b2 := NewUpdateBuilder("items")
	b2.Set("a", 1).Set("b", 2)
	b2.WhereEq("x", 3).WhereIsNotNull("y")
	sql2, args2, err := b2.Build()
	if err != nil {
		t.Fatalf("unexpected error building multiple clauses: %v", err)
	}
	if !strings.Contains(sql2, " AND ") {
		t.Fatalf("expected multiple WHERE clauses joined by AND, got: %s", sql2)
	}
	// args should be [1,2,3] since WhereIsNotNull adds no arg
	if !reflect.DeepEqual(args2, []any{1, 2, 3}) {
		t.Fatalf("unexpected args for multiple clauses: %#v", args2)
	}
}
func TestUpdateBuilder_WhereIn(t *testing.T) {
	b := NewUpdateBuilder("users")
	b.Set("status", "active")
	b.WhereIn("id", 1, 2, 3)

	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, " IN (") {
		t.Fatalf("expected IN clause in sql, got: %s", sql)
	}
	// args should be ["active", 1, 2, 3]
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %#v", len(args), args)
	}
	if !reflect.DeepEqual(args, []any{"active", 1, 2, 3}) {
		t.Fatalf("unexpected args order or values: %#v", args)
	}
	assert.Equal(t, "UPDATE \"users\" SET \"status\" = $1 WHERE \"id\" IN ($2, $3, $4)", sql)
}

func TestUpdateBuilder_WhereNotIn(t *testing.T) {
	b := NewUpdateBuilder("products")
	b.Set("available", true)
	b.WhereNotIn("sku", "a", "b")

	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, " NOT IN (") {
		t.Fatalf("expected NOT IN clause in sql, got: %s", sql)
	}
	// args should be [true, "a", "b"]
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d: %#v", len(args), args)
	}
	if !reflect.DeepEqual(args, []any{true, "a", "b"}) {
		t.Fatalf("unexpected args for NOT IN test: %#v", args)
	}
	assert.Equal(t, "UPDATE \"products\" SET \"available\" = $1 WHERE \"sku\" NOT IN ($2, $3)", sql)
}

func TestUpdateBuilder_WhereNotEq(t *testing.T) {
	b := NewUpdateBuilder("sessions")
	b.Set("active", false)
	b.WhereNotEq("user_id", 42)

	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "!=") {
		t.Fatalf("expected != in sql, got: %s", sql)
	}
	// args should be [false, 42]
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %#v", len(args), args)
	}
	if args[0] != false || args[1] != 42 {
		t.Fatalf("unexpected args for NOT EQ test: %#v", args)
	}
	assert.Equal(t, "UPDATE \"sessions\" SET \"active\" = $1 WHERE \"user_id\" != $2", sql)
}
