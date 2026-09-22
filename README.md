# arcusdata

[![Go Reference](https://pkg.go.dev/badge/github.com/devspotai/arcusdata.svg)](https://pkg.go.dev/github.com/devspotai/arcusdata)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A small, safe SQL toolkit for PostgreSQL, built on [pgx v5](https://github.com/jackc/pgx).

It gives you parameterized query builders that never concatenate user input, a pooled
connection wrapper with lazy initialization, and helpers for the pgx errors you actually
branch on.

## Why

Hand-rolled SQL string building is where injection bugs come from. arcusdata makes the
safe path the default:

- **Values are always parameters.** Every `Values`/`Set`/`Where*` call emits a `$n`
  placeholder and appends to the arg list. There is no API that interpolates a value.
- **Identifiers are validated and quoted.** Table and column names must match
  `^[A-Za-z_][A-Za-z0-9_]*$` before being double-quoted. An invalid identifier is
  recorded on the builder and surfaces from `Build()`.
- **Dangerous operations need opt-out, not opt-in.** `UPDATE` and `DELETE` builders
  refuse to build without a `WHERE` clause unless you explicitly call
  `RequireWhere(false)`.
- **`ORDER BY` is allow-list guarded**, because it is the one clause that cannot be
  parameterized.

## Requirements

- Go **1.27** or later
- PostgreSQL
- PostGIS, only if you use `AppendProximitySearch`

## Install

```bash
go get github.com/devspotai/arcusdata
```

## Quick start

Wire the pool once, in `main`:

```go
import "github.com/devspotai/arcusdata/psqldb"

provider := psqldb.NewPgxDatabaseProvider(databaseURL, psqldb.ConnectionPoolConfig{
    MaxOpenConns:           25,
    MaxIdleConns:           25,
    ConnMaxLifetimeInHours: 1,
    ConnMaxIdleTimeInMin:   15,
    HealthCheckPeriodInSec: 30,
})

lazyDB := psqldb.NewLazyDatabase(provider) // does not connect yet
defer lazyDB.Close()

database, err := lazyDB.Get() // connects on first call, cached thereafter
if err != nil {
    return err
}
if err := database.Ping(ctx); err != nil {
    return err
}
```

`ConnectionPoolConfig.SetDefaults()` fills any zero field with the values shown above.

Then pass `database` (a `dbinterface.Database`) into your repositories:

```go
type repo struct{ db dbinterface.Database }

func (r *repo) Create(ctx context.Context, id uuid.UUID, name string) (time.Time, error) {
    sql, args, err := psqldb.NewInsertBuilder("widget").
        Columns("widget_id", "name").
        Values(id, name).
        ReturningCols("created_at").
        Build()
    if err != nil {
        return time.Time{}, err
    }

    var createdAt time.Time
    if err := r.db.QueryRow(ctx, sql, args...).Scan(&createdAt); err != nil {
        if psqldb.IsNoRows(err) {
            return time.Time{}, ErrNotFound
        }
        return time.Time{}, err
    }
    return createdAt, nil
}
```

## Builders

Every builder embeds `BuilderCore`, which carries the args, any CTEs, and the first
error encountered. Call `Build()` once at the end and check its error — intermediate
calls never panic and never return an error of their own.

```go
Build() (sql string, args []any, err error)
```

Table names may be schema-qualified (`"myschema"."widget"`); both halves are validated
and quoted separately.

### QueryBuilder

```go
sql, args, err := psqldb.NewQueryBuilder().
    SelectCols("id", "name", "created_at").
    FromTable("myschema.widget").
    WhereEq("owner_id", ownerID).
    WhereIsNull("deleted_at").
    WhereWithCondition("name", "%boxes%", psqldb.OpILike).
    SafeOrderBy("created_at", "DESC", allowedSortCols).
    Limit(20).
    SetOffset(40).
    Build()
```

Comparison operators: `OpEqual`, `OpNotEqual`, `OpGreaterThan`, `OpLessThan`,
`OpGreaterThanOrEqual`, `OpLessThanOrEqual`, `OpIn`, `OpLike`, `OpILike`.

`GenerateCountSql()` reuses the accumulated `WHERE` clauses to produce the matching
`SELECT COUNT(*)` for pagination.

Two specialised filters are available: `AppendArrayOverlapAtLeast(col, values, n)` for
`text[]` intersection, and `AppendProximitySearch(col, lat, lon, distanceKm)`, which
emits a PostGIS `ST_DWithin` predicate and therefore requires the PostGIS extension.

### InsertBuilder, UpdateBuilder, DeleteBuilder

```go
psqldb.NewInsertBuilder("widget").
    Columns("widget_id", "name").   // call before Values
    Values(id, name).               // count must match Columns
    ReturningCols("created_at")

psqldb.NewUpdateBuilder("widget").
    Set("name", name).
    SetExpr("updated_at", psqldb.ExprNow()).
    WhereEq("widget_id", id).
    ReturningCols("updated_at")

psqldb.NewDeleteBuilder("widget").
    WhereEq("widget_id", id).
    ReturningCols("widget_id")
```

`UPDATE` and `DELETE` default to `RequireWhere(true)`; `Build()` returns an error rather
than emitting a statement that would touch every row.

`Expr` is the only way to inject non-parameterized SQL, and `ExprNow()` (`NOW()`) is the
only constructor. Reserve it for constants you control — never user input.

## `ORDER BY` safety

`ORDER BY` cannot use a placeholder, so `SafeOrderBy` takes an explicit allow-list:

```go
allowed := map[string]struct{}{"created_at": {}, "name": {}}
qb.SafeOrderBy(userSuppliedCol, userSuppliedDir, allowed)
```

A column outside the set records `unsafe ORDER BY column: ...` on the builder. An empty
column string is treated as "no ordering".

> **Passing `nil` as `allowedCols` skips the allow-list check entirely**, leaving only
> identifier validation. Always pass an explicit set when the column comes from user
> input.

## Row-level authorization

`AuthPermissionsCTE` attaches a permission-check CTE to any builder and guards the
statement with `WHERE EXISTS (SELECT 1 FROM <cte>)`, so authorization and mutation happen
in a single statement:

```go
spec := psqldb.AuthPermissionsCTE{
    CTEName:         "auth_check",
    PermTable:       "membership",
    PermAlias:       "p",
    UserIDCol:       "user_id",
    CompanyIDCol:    "company_id",
    StatusCol:       "status",
    RoleCol:         "role",
    UserID:          userID,
    CompanyID:       companyID,
    AllowedRoles:    []string{"OWNER", "MANAGER"},
    RequireVerified: true,
}
spec.Apply(builder) // works on any of the four builders
```

## Transactions

```go
err := database.BeginFunc(ctx, func(tx dbinterface.Transaction) error {
    if _, err := tx.Exec(ctx, sql1, args1...); err != nil {
        return err
    }
    _, err := tx.Exec(ctx, sql2, args2...)
    return err // non-nil rolls back, nil commits
})
```

`BeginTxFunc` takes `pgx.TxOptions`; `TxReadCommitted`, `TxRepeatableRead` and
`TxSerializable` are provided for the common isolation levels.

## Error helpers

```go
psqldb.IsNoRows(err)               // no rows returned
psqldb.IsUniqueViolation(err)      // SQLSTATE 23505
psqldb.IsForeignKeyViolation(err)  // SQLSTATE 23503
psqldb.IsCheckViolation(err)       // SQLSTATE 23514
psqldb.GetConstraintName(err)      // constraint name, or "" if not a PgError
```

## Testing

Generated mocks for every interface in `psqldb/dbinterface` ship in
`psqldb/dbinterface/mocks`, built with [go.uber.org/mock](https://github.com/uber-go/mock):

```go
ctrl := gomock.NewController(t)
db := mocks.NewMockDatabase(ctrl)
db.EXPECT().QueryRow(gomock.Any(), gomock.Any(), gomock.Any()).Return(row)
```

Regenerate them after changing an interface:

```bash
make generate
```

## Development

```bash
make build     # compile all packages
make test      # tests with -race and coverage
make cover     # coverage summary
make fmt vet   # format and vet
make lint      # golangci-lint
make tidy      # go mod tidy + verify
```

## License

[MIT](LICENSE)
