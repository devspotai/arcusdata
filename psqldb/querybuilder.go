package psqldb

import (
	"fmt"
	"strings"
)

// QueryBuilder helps build dynamic queries safely
type QueryBuilder struct {
	BuilderCore

	selects []string
	from    string
	wheres  []string
	limit   *int
	offset  *int
	orderBy string
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		selects: make([]string, 0, 4),
		wheres:  make([]string, 0, 4),
		limit:   nil,
		offset:  nil,
	}
}

func (q *QueryBuilder) SelectCols(cols ...string) *QueryBuilder {
	if q.err != nil {
		return q
	}
	for _, c := range cols {
		cq, err := q.QuoteDottedIdentifier(c)
		if err != nil {
			q.SetErr(err)
			return q
		}
		q.selects = append(q.selects, cq)
	}
	return q
}

func (q *QueryBuilder) FromTable(table string) *QueryBuilder {
	if q.err != nil {
		return q
	}
	tq, err := q.QuoteDottedIdentifier(table)
	if err != nil {
		q.SetErr(err)
		return q
	}
	q.from = tq
	return q
}

func (q *QueryBuilder) WhereEq(col string, val any) *QueryBuilder {
	if q.err != nil {
		return q
	}
	cq, err := q.QuoteDottedIdentifier(col)
	if err != nil {
		q.SetErr(err)
		return q
	}
	q.wheres = append(q.wheres, fmt.Sprintf("%s = %s", cq, q.Param(val)))
	return q
}

func (u *QueryBuilder) WhereIsNull(col string) *QueryBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s IS NULL", cq))
	return u
}

func (u *QueryBuilder) WhereIsNotNull(col string) *QueryBuilder {
	if u.err != nil {
		return u
	}
	cq, err := u.QuoteDottedIdentifier(col)
	if err != nil {
		u.SetErr(err)
		return u
	}
	u.wheres = append(u.wheres, fmt.Sprintf("%s IS NOT NULL", cq))
	return u
}

// RequireAuthCTE adds EXISTS(SELECT 1 FROM "auth") to WHERE.
func (q *QueryBuilder) RequireAuthCTE(cteName string) *QueryBuilder {
	if q.err != nil {
		return q
	}
	cteQ, err := q.QuoteIdentifier(cteName)
	if err != nil {
		q.SetErr(err)
		return q
	}
	q.wheres = append(q.wheres, fmt.Sprintf("EXISTS (SELECT 1 FROM %s)", cteQ))
	return q
}

func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = &n
	return q
}

// SafeOrderBy whitelists column + direction and prevents double ORDER BY.
func (q *QueryBuilder) SafeOrderBy(col string, dir string, allowedCols map[string]struct{}) *QueryBuilder {
	if q.err != nil {
		return q
	}
	verifiedCol, err := q.QuoteIdentifier(col)
	if err != nil {
		q.SetErr(err)
		return q
	}
	ud := strings.ToUpper(strings.TrimSpace(dir))
	if ud == "" {
		ud = "ASC"
	}
	if ud != "ASC" && ud != "DESC" {
		err := fmt.Errorf("unsafe ORDER BY direction: %q", dir)
		q.SetErr(err)
		return q
	}
	q.orderBy = fmt.Sprintf("ORDER BY %s %s", verifiedCol, ud)
	return q
}

func (qb *QueryBuilder) SetOffset(offset int) *QueryBuilder {
	if offset > 0 {
		qb.offset = &offset
	}
	if offset < 0 {
		qb.offset = nil
	}
	return qb
}

func (q *QueryBuilder) Build() (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	if len(q.selects) == 0 {
		return "", nil, fmt.Errorf("no SELECT columns")
	}
	if q.from == "" {
		return "", nil, fmt.Errorf("no FROM table")
	}

	var sb strings.Builder
	if len(q.ctes) > 0 {
		sb.WriteString("WITH ")
		sb.WriteString(strings.Join(q.ctes, ", "))
		sb.WriteByte(' ')
	}

	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(q.selects, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(q.from)

	if len(q.wheres) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(q.wheres, " AND "))
	}

	if q.limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *q.limit))
	}

	return sb.String(), q.args, nil
}

func (q *QueryBuilder) AppendArrayOverlapAtLeast(col, elemType string, values any, n int) *QueryBuilder {
	if q.err != nil {
		return q
	}
	quotedColumn, err := q.QuoteDottedIdentifier(col)
	if err != nil {
		q.SetErr(err)
		return q
	}
	q.wheres = append(q.wheres, fmt.Sprintf("%s && %s::%s[]", quotedColumn, q.Param(values), elemType))
	q.wheres = append(q.wheres, fmt.Sprintf(`
		cardinality(ARRAY(
				SELECT UNNEST(%s)
				INTERSECT
				SELECT UNNEST(%s::%s[])
		)) >= %s`, quotedColumn, q.Param(values), elemType), fmt.Sprintf("%d", n))
	return q
}

func (q *QueryBuilder) AppendProximitySearch(col string, latitude float64, longitude float64, distance float64) *QueryBuilder {
	if q.err != nil {
		return q
	}
	quotedColumn, err := q.QuoteDottedIdentifier(col)
	if err != nil {
		q.SetErr(err)
		return q
	}
	q.wheres = append(q.wheres, fmt.Sprintf(`
			ST_DWithin(
				%s,
				ST_SetSRID(ST_MakePoint(%f, %f), 4326)::geography,
				%f * 1000
			)`,
		quotedColumn, latitude, longitude, distance,
	))
	return q
}
