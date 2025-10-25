package psqldb

import (
	"errors"
	"fmt"
	"strings"
)

// QueryBuilder helps build dynamic queries safely
type QueryBuilder struct {
	query      string
	filters    []string
	args       []interface{}
	paramCount int

	modifier             string
	hasOrderByInModifier bool // detect accidental ORDER BY leakage in modifier
	orderBy              string

	limitSet, offsetSet bool
	limit, offset       int
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(baseQuery string) *QueryBuilder {
	return &QueryBuilder{
		query:   strings.TrimSpace(baseQuery),
		filters: make([]string, 0),
		args:    make([]interface{}, 0),
	}
}

func (qb *QueryBuilder) nextParam() string {
	qb.paramCount++
	return fmt.Sprintf("$%d", qb.paramCount)
}

// replaceQMarksInFragment replaces each "?" in this fragment with the next $n,
// affecting ONLY this fragment (so JSON operators in other fragments are safe).
func (qb *QueryBuilder) replaceQMarksInFragment(fragment string) string {
	var builder strings.Builder
	builder.Grow(len(fragment) + 8)
	for i := 0; i < len(fragment); i++ {
		if fragment[i] == '?' {
			builder.WriteString(qb.nextParam())
		} else {
			builder.WriteByte(fragment[i])
		}
	}
	return builder.String()
}

// Append adds a WHERE fragment that contains '?' placeholders and its args.
// Example: qb.Append("host_company_id = ?", hostCompanyID)
func (qb *QueryBuilder) Append(fragment string, args ...interface{}) *QueryBuilder {
	qb.filters = append(qb.filters, qb.replaceQMarksInFragment(fragment))
	qb.args = append(qb.args, args...)
	return qb
}

// AppendRaw adds a WHERE fragment *without* placeholder substitution.
// Use when the fragment legitimately contains '?' operators (JSON ?/ ?|/ ?&)
// or has no parameters at all.
func (qb *QueryBuilder) AppendRaw(fragment string) *QueryBuilder {
	qb.filters = append(qb.filters, fragment)
	return qb
}

// AppendIf conditionally adds a clause to the query
func (qb *QueryBuilder) AppendIf(condition bool, clause string, args ...interface{}) *QueryBuilder {
	if condition {
		return qb.Append(clause, args...)
	}
	return qb
}

// AppendRawIf conditionally adds a raw clause (no placeholder substitution)
func (qb *QueryBuilder) AppendRawIf(condition bool, clause string) *QueryBuilder {
	if condition {
		return qb.AppendRaw(clause)
	}
	return qb
}

// SetModifierAndArgs sets a trailing fragment (no ORDER BY/LIMIT/OFFSET here).
// It can include '?' placeholders which will be $n-ized.
func (qb *QueryBuilder) SetModifierAndArgs(mod string, args ...interface{}) *QueryBuilder {
	upper := strings.ToUpper(mod)
	if strings.Contains(upper, "ORDER BY") {
		qb.hasOrderByInModifier = true
	}
	if strings.Contains(upper, " LIMIT ") || strings.HasSuffix(upper, " LIMIT") ||
		strings.Contains(upper, " OFFSET ") || strings.HasSuffix(upper, " OFFSET") {
		// You may choose to error here; we just note it and rely on Build to guard.
	}
	var builder strings.Builder
	for i := 0; i < len(mod); i++ {
		if mod[i] == '?' {
			builder.WriteString(qb.nextParam())
		} else {
			builder.WriteByte(mod[i])
		}
	}
	qb.modifier = strings.TrimSpace(builder.String())
	qb.args = append(qb.args, args...)
	return qb
}

// SafeOrderBy whitelists column + direction and prevents double ORDER BY.
func (qb *QueryBuilder) SafeOrderBy(col string, dir string, allowedCols map[string]struct{}) error {
	if qb.hasOrderByInModifier {
		return errors.New("ORDER BY already present in modifier; remove duplicate")
	}
	if _, ok := allowedCols[col]; !ok {
		return fmt.Errorf("unsafe ORDER BY column: %q", col)
	}
	ud := strings.ToUpper(strings.TrimSpace(dir))
	if ud == "" {
		ud = "ASC"
	}
	if ud != "ASC" && ud != "DESC" {
		return fmt.Errorf("unsafe ORDER BY direction: %q", dir)
	}
	qb.orderBy = fmt.Sprintf("ORDER BY %s %s", col, ud)
	return nil
}

// Limit/Offset (parameterized)
func (qb *QueryBuilder) SetLimit(limit int) *QueryBuilder {
	if limit > 0 {
		qb.limitSet = true
		qb.limit = limit
	}
	return qb
}
func (qb *QueryBuilder) SetOffset(offset int) *QueryBuilder {
	if offset > 0 {
		qb.offsetSet = true
		qb.offset = offset
	}
	return qb
}

// Build returns the final query and arguments (no trailing semicolon).
func (qb *QueryBuilder) Build() (string, []interface{}) {
	var sb strings.Builder
	sb.Grow(len(qb.query) + 64)
	sb.WriteString(qb.query)

	if len(qb.filters) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(qb.filters, " AND "))
	}

	if qb.modifier != "" {
		upper := strings.ToUpper(qb.modifier)
		// Optional strict guard: forbid LIMIT/OFFSET inside modifier
		if strings.Contains(upper, " LIMIT ") || strings.HasSuffix(upper, " LIMIT") ||
			strings.Contains(upper, " OFFSET ") || strings.HasSuffix(upper, " OFFSET") {
			// Prefer to fail fast instead of silently producing invalid SQL:
			// return "", nil
		}
		sb.WriteByte(' ')
		sb.WriteString(qb.modifier)
	}

	if qb.orderBy != "" {
		sb.WriteByte(' ')
		sb.WriteString(qb.orderBy)
	}

	// Parameterize LIMIT/OFFSET for better plan reuse
	if qb.limitSet {
		sb.WriteString(" LIMIT ")
		sb.WriteString(qb.nextParam())
		qb.args = append(qb.args, qb.limit)
	}
	if qb.offsetSet {
		sb.WriteString(" OFFSET ")
		sb.WriteString(qb.nextParam())
		qb.args = append(qb.args, qb.offset)
	}

	return sb.String(), qb.args
}

func (qb *QueryBuilder) AppendArrayOverlapAtLeast(col, elemType string, values any, n int) *QueryBuilder {
	qb.Append(fmt.Sprintf("%s && ?::%s[]", col, elemType), values)
	qb.Append(fmt.Sprintf(`
		cardinality(ARRAY(
				SELECT UNNEST(%s)
				INTERSECT
				SELECT UNNEST(?::%s[])
		)) >= ?`, col, elemType), values, n)
	return qb
}

func (qb *QueryBuilder) AppendProximitySearch(col string, latitude float64, longitude float64, distance float64) *QueryBuilder {
	qb.Append(`
			ST_DWithin(
				location_geography,
				ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
				? * 1000
			)`,
		latitude, longitude, distance,
	)
	return qb
}
