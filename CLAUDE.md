# arcusdata — Developer Guide

## Overview

Private PostgreSQL client library (`github.com/devspotai/arcusdata`) consumed by all devspotai Go backend services. Provides a fluent builder API for parameterized SQL, a lazy-initialized connection pool, and pgx/v5 error helpers.

## Module

```
Module: github.com/devspotai/arcusdata
Go version: 1.24.0
```

## Key Dependencies

| Package | Version |
|---------|---------|
| github.com/jackc/pgx/v5 | v5.7.6 |

## Directory Structure

```
psqldb/
  builder.go                  # Op constants, CTEContext interface, BuilderCore
  querybuilder.go             # SELECT builder
  updatebuilder.go            # UPDATE builder
  insertbuilder.go            # INSERT builder
  deletebuilder.go            # DELETE builder
  auth_permissions_cte.go     # AuthPermissionsCTE helper
  trusted_db_expressions.go   # Expr type + ExprNow()
  database.go                 # PgxDatabase, error helpers, transaction helpers
  lazydatabase.go             # LazyDatabaseInterface
  config.go                   # Config, ConnectionPoolConfig
  dbinterface/
    database_interface.go     # Database, Transaction, Rows, ... interfaces
util/
  property.go                 # GetEnv, GetEnvAsInt, LoadEnvFile
```

## Config

### `ConnectionPoolConfig` (`psqldb/config.go`)

```go
type ConnectionPoolConfig struct {
    MaxOpenConns              int   // default 25
    MaxIdleConns              int   // default 25
    ConnMaxLifetimeInHours    int   // default 1
    ConnMaxIdleTimeInMin      int   // default 15
    HealthCheckPeriodInSec    int   // default 30
}
```

- `SetDefaults()` — fills zeros with the defaults listed above
- `IsValid() bool` — returns false if any field is ≤ 0

### `Config` (`psqldb/config.go`)

```go
type Config struct {
    ApplicationRWDatabaseURL               string
    ApplicationSchemaMigrationDatabaseUrl  string
    ApplicationServerPort                  string
    ConnectionPoolConfig
}
```

## Database Provider

```go
// Construct a DB provider (lazy, validates config only on first Get)
provider := psqldb.NewPgxDatabaseProvider(databaseURL string, cfg ConnectionPoolConfig)

// Wrap in lazy loader — Get() calls provider on first access, caches thereafter
db := psqldb.NewLazyDatabase(provider)
conn, err := db.Get()   // returns (Database, error)
db.Reset()              // clears cached connection (force re-connect)
db.Close()              // close the pool
```

### `Database` interface (key methods)

```go
Exec(ctx, sql, args...) (CommandTag, error)
Query(ctx, sql, args...) (Rows, error)
QueryRow(ctx, sql, args...) Row
BeginFunc(ctx, fn func(Transaction) error) error    // auto commit/rollback
BeginTxFunc(ctx, opts pgx.TxOptions, fn) error
NewBatchOperation() BatchOperation
CopyFrom(ctx, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
ExecMigration(ctx, sql) error
```

### Pre-built `pgx.TxOptions`

```go
psqldb.TxReadCommitted    // pgx.TxOptions{IsoLevel: pgx.ReadCommitted}
psqldb.TxRepeatableRead   // pgx.TxOptions{IsoLevel: pgx.RepeatableRead}
psqldb.TxSerializable     // pgx.TxOptions{IsoLevel: pgx.Serializable}
```

## Error Helpers (`psqldb/database.go`)

```go
psqldb.IsNoRows(err error) bool              // err == pgx.ErrNoRows
psqldb.IsUniqueViolation(err error) bool     // PgError code "23505"
psqldb.IsForeignKeyViolation(err error) bool // PgError code "23503"
psqldb.IsCheckViolation(err error) bool      // PgError code "23514"
psqldb.GetConstraintName(err error) string   // extracts ConstraintName from *pgconn.PgError
```

## SQL Builders

All builders share these traits:

- **Fluent chaining**: each method returns the builder (except `Build`)
- **`Build() (string, []any, error)`** — always 3 return values; never ignore the error
- **`$n` placeholders**: params are numbered automatically via `BuilderCore.Param(v)`
- **CTEContext**: all builders implement it, so `AuthPermissionsCTE.Apply(b)` works on any builder

### `Op` constants (`psqldb/builder.go`)

```go
OpEqual              // "="
OpNotEqual           // "!="
OpGreaterThan        // ">"
OpLessThan           // "<"
OpIn                 // "IN"
OpLike               // "LIKE"
OpILike              // "ILIKE"
OpGreaterThanOrEqual // ">="
OpLessThanOrEqual    // "<="
```

### QueryBuilder

```go
qb := psqldb.NewQueryBuilder()
qb.SelectCols("col1", "col2")                                // validates identifiers
qb.FromTable("schema.table")
qb.WhereEq("col", value)
qb.WhereWithCondition("col", value, psqldb.OpILike)
qb.WhereIsNull("col")
qb.WhereIsNotNull("col")
qb.RequireAuthCTE("cte_name")                                // adds EXISTS(SELECT 1 FROM "cte_name")
qb.Limit(20)
qb.SetOffset(40)
qb.SafeOrderBy("col", "ASC", map[string]struct{}{"col": {}}) // only allows listed columns
qb.AppendArrayOverlapAtLeast("tags_col", []string{"a","b"}, 1) // array intersection filter
qb.AppendProximitySearch("location_col", lat, lon, distanceKm) // ST_DWithin (distance × 1000 internally)

sql, args, err := qb.Build()

// Count variant — same WHERE clauses, generates SELECT COUNT(*)
countSQL, args, err := qb.GenerateCountSql()
```

### UpdateBuilder

```go
ub := psqldb.NewUpdateBuilder("schema.table")
ub.RequireWhere(true)              // default true — Build() errors if no WHERE clause
ub.Set("col", value)
ub.SetExpr("updated_at", psqldb.ExprNow()) // trusted SQL expression (no parameterization)
ub.WhereEq("id", id)
ub.WhereIn("status", "active", "pending")
ub.WhereNotIn("status", "deleted")
ub.WhereNotEq("col", value)
ub.WhereIsNull("deleted_at")
ub.WhereIsNotNull("col")
ub.RequireAuthCTE("cte_name")
ub.ReturningCols("id", "updated_at")

sql, args, err := ub.Build()       // errors if no SET or no WHERE (when requireWhere=true)
```

### InsertBuilder

```go
ib := psqldb.NewInsertBuilder("schema.table")
ib.Columns("col1", "col2", "col3")  // must call before Values
ib.Values(v1, v2, v3)               // count must match Columns
ib.RequireAuthCTE("cte_name")       // INSERT…SELECT…WHERE EXISTS(auth) pattern
ib.ReturningCols("id", "created_at")

sql, args, err := ib.Build()
```

### DeleteBuilder

```go
db := psqldb.NewDeleteBuilder("schema.table")
db.RequireWhere(true)               // default true
db.WhereEq("id", id)
db.WhereIn("col", val1, val2)
db.WhereNotIn("col", val1)
db.WhereNotEq("col", value)
db.WhereIsNull("deleted_at")
db.WhereIsNotNull("col")
db.RequireAuthCTE("cte_name")
db.ReturningCols("id")

sql, args, err := db.Build()
```

### Trusted SQL Expressions

```go
psqldb.ExprNow() psqldb.Expr    // renders as literal NOW() in SQL — not parameterized
// Used with UpdateBuilder.SetExpr() for timestamp columns
```

## AuthPermissionsCTE

Attaches a row-level permission guard CTE to any builder:

```go
spec := psqldb.AuthPermissionsCTE{
    CTEName:         "auth_check",
    PermTable:       "schema.permission_table",
    PermAlias:       "p",
    UserIDCol:       "user_id",
    CompanyIDCol:    "company_id",
    StatusCol:       "status",
    RoleCol:         "role",
    UserID:          userID,
    CompanyID:       companyID,
    AllowedRoles:    []string{"OWNER", "MANAGER"},
    RequireVerified: true,
    VerifiedValue:   "VERIFIED",   // default if empty
}
spec.Apply(builder)  // works on QueryBuilder, UpdateBuilder, InsertBuilder, DeleteBuilder
```

The `Apply` call:
1. Renders the CTE SQL and calls `builder.AddCTE(cteDef)`
2. Calls `builder.RequireAuthCTE(spec.CTEName)` to add the EXISTS guard

## Util (`util/property.go`)

```go
util.LoadEnvFile()                        // loads .env.<APP_ENV>
util.GetEnvName() string                  // returns APP_ENV value
util.GetEnv(key, fallback string) string
util.GetEnvAsInt(name string, default int) int
// NOTE: GetEnvAsBool does NOT exist in arcusdata/util
// Use sharedkit/config.GetEnvAsBool or: util.GetEnv("KEY","false") == "true"
```

## Identifier Safety

`ValidateIdentifier(name string) (string, error)` validates against `^[A-Za-z_][A-Za-z0-9_]*$`. All builder column/table methods call this internally — they will record an error on the builder if an identifier is invalid, and `Build()` will return that error.

## Common Patterns

```go
// Typical repository SELECT
sql, args, err := psqldb.NewQueryBuilder().
    SelectCols("id", "name", "created_at").
    FromTable("myschema.mytable").
    WhereEq("id", id).
    WhereIsNull("deleted_at").
    Limit(1).
    Build()
if err != nil { return nil, err }

row := conn.QueryRow(ctx, sql, args...)
// scan...

// Soft delete UPDATE
sql, args, err := psqldb.NewUpdateBuilder("myschema.mytable").
    SetExpr("deleted_at", psqldb.ExprNow()).
    SetExpr("updated_at", psqldb.ExprNow()).
    WhereEq("id", id).
    WhereIsNull("deleted_at").
    ReturningCols("id").
    Build()

// Transaction
err = conn.BeginFunc(ctx, func(tx psqldb.Transaction) error {
    _, err := tx.Exec(ctx, sql1, args1...)
    if err != nil { return err }
    _, err = tx.Exec(ctx, sql2, args2...)
    return err
})
```
