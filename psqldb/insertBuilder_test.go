package psqldb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsertBuilder_SelectFrom_NonParameterized(t *testing.T) {
	ib := NewInsertBuilder("dest_table")
	ib.Columns("col1", "col2")
	ib.Values("val1", "val2")
	ib.ReturningCols("col1", "col2", "col3")

	sql, args, err := ib.Build()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO \"dest_table\" (\"col1\", \"col2\") VALUES ($1, $2) RETURNING \"col1\", \"col2\", \"col3\"", sql)
	assert.Equal(t, []any{"val1", "val2"}, args)
}

func TestInsertBuilder_WithAuthCTE_Parameterized_WithReturningAndArgs(t *testing.T) {
	ib := NewInsertBuilder("dest")
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
	}.Apply(ib)
	ib.Columns("a", "b")
	ib.Values("x", "y")
	ib.ReturningCols("id", "a", "b")
	ib.RequireAuthCTE("auth_cte")

	sql, args, err := ib.Build()

	assert.NoError(t, err)
	assert.Equal(t, "WITH \"auth_cte\" AS (SELECT 1 FROM \"host_company_user_permissions\" \"p\" WHERE \"p\".\"user_id\" = $1 AND \"p\".\"host_company_id\" = $2 AND \"p\".\"permission_status\" = 'VERIFIED' AND \"p\".\"host_role\" = ANY($3)) INSERT INTO \"dest\" (\"a\", \"b\") SELECT $4, $5 WHERE EXISTS (SELECT 1 FROM \"auth_cte\") RETURNING \"id\", \"a\", \"b\"", sql)
	assert.Equal(t, []interface{}{"user-123", "host-company-456", []string{"OWNER", "ADMIN_ALL_STAYS"}, "x", "y"}, args)
}
