package psqldb

import (
	"fmt"
	"strings"
)

// AuthPermissionsCTE defines a common table expression (CTE) for filtering rows
// based on user permissions.
type AuthPermissionsCTE struct {
	CTEName string

	PermTable string
	PermAlias string

	UserIDCol    string
	CompanyIDCol string
	StatusCol    string
	RoleCol      string

	UserID          any
	CompanyID       any
	AllowedRoles    []string
	RequireVerified bool
	VerifiedValue   string // default "VERIFIED"
}

// Apply renders a safe, structured auth CTE and attaches it to any builder.
func (spec AuthPermissionsCTE) Apply(b CTEContext) {
	if b == nil || b.Err() != nil {
		return
	}

	cteName := strings.TrimSpace(spec.CTEName)
	if cteName == "" {
		cteName = "auth"
	}
	alias := strings.TrimSpace(spec.PermAlias)
	if alias == "" {
		alias = "p"
	}
	verified := spec.VerifiedValue
	if verified == "" {
		verified = "VERIFIED"
	}

	cteQ, err := b.QuoteIdentifier(cteName)
	if err != nil {
		b.SetErr(err)
		return
	}
	tableQ, err := b.QuoteDottedIdentifier(spec.PermTable)
	if err != nil {
		b.SetErr(err)
		return
	}
	aliasQ, err := b.QuoteIdentifier(alias)
	if err != nil {
		b.SetErr(err)
		return
	}

	// Qualify columns as: "p"."user_id"
	userCol, err := qualify(alias, spec.UserIDCol, b)
	if err != nil {
		b.SetErr(err)
		return
	}
	coCol, err := qualify(alias, spec.CompanyIDCol, b)
	if err != nil {
		b.SetErr(err)
		return
	}
	roleCol, err := qualify(alias, spec.RoleCol, b)
	if err != nil {
		b.SetErr(err)
		return
	}
	statusCol, err := qualify(alias, spec.StatusCol, b)
	if err != nil {
		b.SetErr(err)
		return
	}

	pUser := b.Param(spec.UserID)
	pCo := b.Param(spec.CompanyID)

	var sb strings.Builder
	sb.WriteString(cteQ)
	sb.WriteString(" AS (SELECT 1 FROM ")
	sb.WriteString(tableQ)
	sb.WriteByte(' ')
	sb.WriteString(aliasQ)
	sb.WriteString(" WHERE ")
	sb.WriteString(userCol)
	sb.WriteString(" = ")
	sb.WriteString(pUser)
	sb.WriteString(" AND ")
	sb.WriteString(coCol)
	sb.WriteString(" = ")
	sb.WriteString(pCo)

	if spec.RequireVerified {
		// Parameterized rather than hand-escaped: doubling quotes is only
		// correct while standard_conforming_strings is on, and every other
		// value in this library is bound as $n.
		sb.WriteString(" AND ")
		sb.WriteString(statusCol)
		sb.WriteString(" = ")
		sb.WriteString(b.Param(verified))
	}

	sb.WriteString(" AND ")
	sb.WriteString(roleCol)
	sb.WriteString(" = ANY(")
	// Allocated here so the placeholders read in the same order as the SQL.
	// pgx binds []string to text[]; wrap upstream for other drivers.
	sb.WriteString(b.Param(spec.AllowedRoles))
	sb.WriteString("))")

	b.AddCTE(sb.String())

	// Attach the EXISTS guard as well. Adding the CTE alone defines a
	// permission check that nothing references, so the statement would run
	// completely unfiltered while reading at the call site as if it were
	// guarded. The guard is idempotent, so an explicit RequireAuthCTE call
	// by the caller remains harmless.
	b.ApplyAuthGuard(cteName)
}

func qualify(alias, col string, b CTEContext) (string, error) {
	// Force single identifiers for columns (recommended).
	col = strings.TrimSpace(col)
	if col == "" {
		return "", fmt.Errorf("column cannot be empty")
	}
	if !identifierRegex.MatchString(col) {
		return "", fmt.Errorf("unsafe column %q", col)
	}
	aliasQ, err := b.QuoteIdentifier(alias)
	if err != nil {
		return "", err
	}
	colQ, err := b.QuoteIdentifier(col)
	if err != nil {
		return "", err
	}
	return aliasQ + "." + colQ, nil
}
