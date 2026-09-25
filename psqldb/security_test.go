package psqldb

import (
	"strings"
	"testing"
)

func secAuthSpec() AuthPermissionsCTE {
	return AuthPermissionsCTE{
		CTEName: "auth_cte", PermTable: "membership", PermAlias: "p",
		UserIDCol: "user_id", CompanyIDCol: "company_id",
		StatusCol: "status", RoleCol: "role",
		UserID: 42, CompanyID: 7, AllowedRoles: []string{"OWNER"},
	}
}

// Apply used to add the CTE without referencing it, so the statement ran
// completely unfiltered while the call site read as if it were guarded.
func TestApplyEnforcesGuardOnItsOwn(t *testing.T) {
	qb := NewQueryBuilder()
	qb.SelectCols("id").FromTable("widget")
	secAuthSpec().Apply(qb) // no explicit RequireAuthCTE

	sql, _, err := qb.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `EXISTS (SELECT 1 FROM "auth_cte")`) {
		t.Fatalf("Apply must attach the EXISTS guard, got: %s", sql)
	}
}

func TestApplyThenExplicitGuardDoesNotDuplicate(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func() (string, error)
	}{
		{"query", func() (string, error) {
			b := NewQueryBuilder()
			b.SelectCols("id").FromTable("t")
			secAuthSpec().Apply(b)
			b.RequireAuthCTE("auth_cte")
			s, _, err := b.Build()
			return s, err
		}},
		{"update", func() (string, error) {
			b := NewUpdateBuilder("t")
			b.Set("a", 1).WhereEq("id", 1)
			secAuthSpec().Apply(b)
			b.RequireAuthCTE("auth_cte")
			s, _, err := b.Build()
			return s, err
		}},
		{"delete", func() (string, error) {
			b := NewDeleteBuilder("t")
			b.WhereEq("id", 1)
			secAuthSpec().Apply(b)
			b.RequireAuthCTE("auth_cte")
			s, _, err := b.Build()
			return s, err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sql, err := tc.run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n := strings.Count(sql, "EXISTS (SELECT 1 FROM"); n != 1 {
				t.Fatalf("expected exactly 1 guard, got %d: %s", n, sql)
			}
		})
	}
}

// Op is a string type, so Op(userInput) compiles. Anything outside the
// defined constants must be rejected rather than interpolated.
func TestWhereWithConditionRejectsForgedOperator(t *testing.T) {
	for _, evil := range []Op{
		Op("= '' OR 1=1 --"),
		Op(";DROP TABLE users;--"),
		Op("="), // note: OpEqual is "=", so this one is legitimate
	} {
		qb := NewQueryBuilder()
		qb.SelectCols("id").FromTable("users")
		qb.WhereWithCondition("name", "nobody", evil)
		sql, _, err := qb.Build()

		if evil == OpEqual {
			if err != nil {
				t.Fatalf("legitimate operator rejected: %v", err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("forged operator %q was accepted, sql: %s", string(evil), sql)
		}
		if !strings.Contains(err.Error(), "unsafe SQL operator") {
			t.Fatalf("unexpected error for %q: %v", string(evil), err)
		}
	}
}

func TestAllDefinedOpsAreAccepted(t *testing.T) {
	for _, op := range []Op{
		OpEqual, OpNotEqual, OpGreaterThan, OpLessThan, OpIn,
		OpLike, OpILike, OpGreaterThanOrEqual, OpLessThanOrEqual,
	} {
		if err := ValidateOp(op); err != nil {
			t.Fatalf("defined operator %q rejected: %v", string(op), err)
		}
	}
}

// A nil allow-list used to disable the check that gives SafeOrderBy its name.
func TestSafeOrderByRejectsNilAllowList(t *testing.T) {
	qb := NewQueryBuilder()
	qb.SelectCols("id").FromTable("t")
	qb.SafeOrderBy("anything", "ASC", nil)
	if _, _, err := qb.Build(); err == nil {
		t.Fatal("nil allow-list must be an error")
	}
}

// GenerateCountSql copied the WHERE clauses but dropped the WITH clause they
// referenced, producing SQL that always failed with "relation does not exist".
func TestGenerateCountSqlKeepsCTEs(t *testing.T) {
	qb := NewQueryBuilder()
	qb.SelectCols("id").FromTable("widget")
	secAuthSpec().Apply(qb)

	cnt, _, err := qb.GenerateCountSql()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(cnt, `WITH "auth_cte" AS (`) {
		t.Fatalf("count SQL must define the CTEs it references, got: %s", cnt)
	}
	if !strings.Contains(cnt, `EXISTS (SELECT 1 FROM "auth_cte")`) {
		t.Fatalf("count SQL lost the guard: %s", cnt)
	}
}

func TestGenerateCountSqlRequiresFrom(t *testing.T) {
	qb := NewQueryBuilder()
	qb.SelectCols("id")
	if _, _, err := qb.GenerateCountSql(); err == nil {
		t.Fatal("expected an error when FROM is missing")
	}
}

// The verified status is a value and must be bound, not inlined as a literal.
func TestVerifiedValueIsParameterized(t *testing.T) {
	spec := secAuthSpec()
	spec.RequireVerified = true
	spec.VerifiedValue = "O'Brien" // would need hand-escaping if inlined

	qb := NewQueryBuilder()
	qb.SelectCols("id").FromTable("t")
	spec.Apply(qb)

	sql, args, err := qb.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sql, "O'Brien") {
		t.Fatalf("verified value was inlined into SQL: %s", sql)
	}
	found := false
	for _, a := range args {
		if s, ok := a.(string); ok && s == "O'Brien" {
			found = true
		}
	}
	if !found {
		t.Fatalf("verified value was not bound as an argument: %v", args)
	}
}
