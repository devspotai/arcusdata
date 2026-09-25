package psqldb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeleteBuilder_RequireWhereEnforced(t *testing.T) {
	b := NewDeleteBuilder("users")
	_, _, err := b.Build()
	if err == nil {
		t.Fatalf("expected error when WHERE is required but none was returned")
	}
	if !strings.Contains(err.Error(), "WHERE") {
		t.Fatalf("expected WHERE-related error, got: %v", err)
	}
}

func TestDeleteBuilder_NoRequireWhere_AllowsDeleteWithoutWhere(t *testing.T) {
	b := NewDeleteBuilder("users").RequireWhere(false)
	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "DELETE FROM ") {
		t.Fatalf("expected SQL to start with DELETE FROM, got: %s", sql)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got: %v", args)
	}
	assert.Equal(t, "DELETE FROM \"users\"", sql)
}

func TestDeleteBuilder_WhereEq_AddsArgAndPlaceholder(t *testing.T) {
	b := NewDeleteBuilder("things")
	b.WhereEq("id", 42)
	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	if args[0] != 42 {
		t.Fatalf("expected arg[0]==42, got %#v", args[0])
	}
	if !strings.Contains(sql, "WHERE") || !strings.Contains(sql, "=") {
		t.Fatalf("expected WHERE ... = ... in SQL, got: %s", sql)
	}
	assert.Equal(t, "DELETE FROM \"things\" WHERE \"id\" = $1", sql)
	assert.Equal(t, []any{42}, args)
}

func TestDeleteBuilder_NullChecks_And_CombinedWhere(t *testing.T) {
	b := NewDeleteBuilder("t")
	b.WhereIsNull("deleted_at")
	b.WhereIsNotNull("active_since")
	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args for IS NULL/IS NOT NULL, got: %v", args)
	}
	if !strings.Contains(sql, "IS NULL") {
		t.Fatalf("expected IS NULL in SQL, got: %s", sql)
	}
	if !strings.Contains(sql, "IS NOT NULL") {
		t.Fatalf("expected IS NOT NULL in SQL, got: %s", sql)
	}
	// ensure the two conditions are joined (AND)
	if !strings.Contains(sql, "AND") {
		t.Fatalf("expected conditions to be joined by AND, got: %s", sql)
	}
	assert.Equal(t, "DELETE FROM \"t\" WHERE \"deleted_at\" IS NULL AND \"active_since\" IS NOT NULL", sql)
}

func TestDeleteBuilder_CTE_And_ReturningCols(t *testing.T) {
	b := NewDeleteBuilder("items")
	AuthPermissionsCTE{
		CTEName:         "auth_cte",
		PermTable:       "host_company_user_permissions",
		PermAlias:       "p",
		UserIDCol:       "user_id",
		CompanyIDCol:    "host_company_id",
		StatusCol:       "permission_status",
		RoleCol:         "host_role",
		UserID:          "user-123",
		CompanyID:       "host-company-456",
		AllowedRoles:    []string{"OWNER", "ADMIN_ALL_STAYS"},
		RequireVerified: true,
	}.Apply(b)
	b.RequireAuthCTE("auth_cte")
	b.WhereEq("owner_id", 99)
	b.ReturningCols("id", "name")
	sql, args, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "WITH ") {
		t.Fatalf("expected SQL to start with WITH when CTEs present, got: %s", sql)
	}
	if !strings.Contains(sql, "EXISTS (SELECT 1 FROM") {
		t.Fatalf("expected EXISTS(...) clause for RequireAuthCTE, got: %s", sql)
	}
	if !strings.Contains(sql, "RETURNING") {
		t.Fatalf("expected RETURNING clause, got: %s", sql)
	}
	if len(args) != 5 {
		t.Fatalf("expected 5 arguments, got: %v", args)
	}
	assert.Equal(t, "WITH \"auth_cte\" AS (SELECT 1 FROM \"host_company_user_permissions\" \"p\" WHERE \"p\".\"user_id\" = $1 AND \"p\".\"host_company_id\" = $2 AND \"p\".\"permission_status\" = $3 AND \"p\".\"host_role\" = ANY($4)) DELETE FROM \"items\" WHERE EXISTS (SELECT 1 FROM \"auth_cte\") AND \"owner_id\" = $5 RETURNING \"id\", \"name\"", sql)
	assert.Equal(t, []any{"user-123", "host-company-456", "VERIFIED", []string{"OWNER", "ADMIN_ALL_STAYS"}, 99}, args)
}
