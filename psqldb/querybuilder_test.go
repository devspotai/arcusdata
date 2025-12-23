package psqldb

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildBasicSelectFrom(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("id", "name")
	q.FromTable("users")

	sql, args, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "SELECT") || !strings.Contains(sql, "FROM") {
		t.Fatalf("unexpected sql: %q", sql)
	}
	if !strings.Contains(sql, "users") {
		t.Fatalf("expected table name in sql: %q", sql)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got: %#v", args)
	}
	assert.Equal(t, "SELECT \"id\", \"name\" FROM \"users\"", sql)
}

func TestWhereEqAddsArgAndClause(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("id")
	q.FromTable("people")
	q.WhereEq("age", 30)

	sql, args, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "WHERE") || !strings.Contains(sql, "age") || !strings.Contains(sql, "=") {
		t.Fatalf("unexpected sql: %q", sql)
	}
	if len(args) != 1 || args[0] != 30 {
		t.Fatalf("expected args [30], got: %#v", args)
	}
	assert.Equal(t, "SELECT \"id\" FROM \"people\" WHERE \"age\" = $1", sql)
}

func TestWhereNulls(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("id")
	q.FromTable("items")
	q.WhereIsNull("deleted_at")
	q.WhereIsNotNull("created_at")

	sql, _, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "IS NULL") {
		t.Fatalf("expected IS NULL in sql: %q", sql)
	}
	if !strings.Contains(sql, "IS NOT NULL") {
		t.Fatalf("expected IS NOT NULL in sql: %q", sql)
	}
	assert.Equal(t, "SELECT \"id\" FROM \"items\" WHERE \"deleted_at\" IS NULL AND \"created_at\" IS NOT NULL", sql)
}

func TestLimitAndOffsetBehavior(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("id")
	q.FromTable("logs")

	// Limit
	q.Limit(5)
	sql, _, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "LIMIT 5") {
		t.Fatalf("expected LIMIT 5 in sql: %q", sql)
	}
	assert.Equal(t, "SELECT \"id\" FROM \"logs\" LIMIT 5", sql)

	// Offset positive should set pointer
	q.SetOffset(10)
	if q.offset == nil || *q.offset != 10 {
		t.Fatalf("expected offset set to 10, got: %#v", q.offset)
	}
	// Build should include OFFSET
	sql, _, err = q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, "SELECT \"id\" FROM \"logs\" LIMIT 5 OFFSET 10", sql)

	// Negative offset clears it
	q.SetOffset(-1)
	if q.offset != nil {
		t.Fatalf("expected offset cleared, got: %#v", q.offset)
	}
}

func TestSafeOrderByValidAndInvalid(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("id", "name", "created_at")
	q.FromTable("logs")
	q.Limit(10)
	q.SetOffset(10)
	// valid empty dir -> ASC
	q.SafeOrderBy("name", "", nil)
	if q.err != nil {
		t.Fatalf("unexpected error for valid orderby: %v", q.err)
	}
	if !strings.Contains(q.orderBy, "ORDER BY") || !strings.Contains(strings.ToUpper(q.orderBy), "ASC") {
		t.Fatalf("unexpected orderBy: %q", q.orderBy)
	}
	sql, args, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, "SELECT \"id\", \"name\", \"created_at\" FROM \"logs\" ORDER BY \"name\" ASC LIMIT 10 OFFSET 10", sql)
	assert.Equal(t, 0, len(args))
	// invalid direction sets error
	q2 := NewQueryBuilder()
	q2.SafeOrderBy("name", "UP", nil)
	if q2.err == nil {
		t.Fatalf("expected error for invalid direction")
	}
	// orderBy should not be populated on error
	if q2.orderBy != "" {
		t.Fatalf("expected empty orderBy on error, got: %q", q2.orderBy)
	}
}

func TestRequireAuthCTEAddsExistsClauseInQueryBuilder(t *testing.T) {
	qb := NewQueryBuilder()
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
		AllowedRoles:    []string{"OWNER"},
		RequireVerified: true,
	}.Apply(qb)
	qb.SelectCols("id")
	qb.FromTable("sessions")
	qb.RequireAuthCTE("auth_cte")
	sql, _, err := qb.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "EXISTS") || !strings.Contains(sql, "SELECT 1 FROM") {
		t.Fatalf("expected EXISTS(SELECT 1 FROM ...) in sql: %q", sql)
	}
	assert.Equal(t, "WITH \"auth_cte\" AS (SELECT 1 FROM \"host_company_user_permissions\" \"p\" WHERE \"p\".\"user_id\" = $1 AND \"p\".\"host_company_id\" = $2 AND \"p\".\"permission_status\" = 'VERIFIED' AND \"p\".\"host_role\" = ANY($3)) SELECT \"id\" FROM \"sessions\" WHERE EXISTS (SELECT 1 FROM \"auth_cte\")", sql)
}

func TestAppendArrayOverlapAtLeastAddsWhereAndArgs(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("stay_id", "stay_name", "labels")
	q.FromTable("stay")

	values := []string{"a", "b"}
	q.AppendArrayOverlapAtLeast("tags", values, 2)

	sql, args, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Param called twice in AppendArrayOverlapAtLeast -> expect two args appended and both equal to provided slice
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got: %#v", args)
	}

	assert.Equal(t, `SELECT "stay_id", "stay_name", "labels" FROM "stay" WHERE (
		SELECT COUNT(*) FROM (
				SELECT UNNEST(COALESCE("tags", '{}'::text[]))
				INTERSECT
				SELECT UNNEST($1::text[])
			) AS inter
		) >= $2`, sql)
}

func TestAppendProximitySearchAddsST_DWithin(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("id")
	q.FromTable("places")

	lat := 12.34
	lon := 56.78
	dist := 5.0
	q.AppendProximitySearch("location", lat, lon, dist)

	sql, args, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "ST_DWithin") {
		t.Fatalf("expected ST_DWithin in sql: %q", sql)
	}
	// Check that coordinates are inlined (formatted with %f in code)
	if !strings.Contains(sql, fmt.Sprintf("%f", lat)) || !strings.Contains(sql, fmt.Sprintf("%f", lon)) {
		t.Fatalf("expected coordinates in sql: %q", sql)
	}
	// Proximity search does not add params
	if len(args) != 0 {
		t.Fatalf("expected no args from proximity search, got: %#v", args)
	}
}

func TestRequireAuthCTEAddsExistsClause(t *testing.T) {
	q := NewQueryBuilder()
	q.SelectCols("id")
	q.FromTable("sessions")
	q.RequireAuthCTE("auth")
	sql, _, err := q.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "EXISTS") || !strings.Contains(sql, "SELECT 1 FROM") {
		t.Fatalf("expected EXISTS(SELECT 1 FROM ...) in sql: %q", sql)
	}
}

func TestBuildErrorsWhenMissingSelectOrFrom(t *testing.T) {
	// Missing selects
	q := NewQueryBuilder()
	_, _, err := q.Build()
	if err == nil {
		t.Fatalf("expected error when no SELECT columns")
	}

	// Missing from
	q2 := NewQueryBuilder()
	q2.SelectCols("id")
	_, _, err2 := q2.Build()
	if err2 == nil {
		t.Fatalf("expected error when no FROM table")
	}
}
