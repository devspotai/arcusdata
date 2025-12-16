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
	pRoles := b.Param(spec.AllowedRoles) // ensure your driver binds []string to text[] (pgx) or wrap upstream if using lib/pq

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
		sb.WriteString(" AND ")
		sb.WriteString(statusCol)
		sb.WriteString(" = '")
		sb.WriteString(strings.ReplaceAll(verified, `'`, `''`))
		sb.WriteString("'")
	}

	sb.WriteString(" AND ")
	sb.WriteString(roleCol)
	sb.WriteString(" = ANY(")
	sb.WriteString(pRoles)
	sb.WriteString("))")

	b.AddCTE(sb.String())
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
