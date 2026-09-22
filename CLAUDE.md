# CLAUDE.md — arcusdata

Guidance for Claude Code (and humans) working **on** this library. For usage docs, read
[README.md](README.md) first — this file covers what the README deliberately leaves out:
internal invariants, maintenance workflow, and known sharp edges.

## What this is

`github.com/devspotai/arcusdata` is a PostgreSQL toolkit built on pgx v5, consumed by tag
across the devspotai Go backends. It is a **library**: it has no `main`, and `make build`
is a compile-only sanity check.

```
Module: github.com/devspotai/arcusdata
Go:     1.27.0  (toolchain go1.27.1)
```

| Dependency | Version | Notes |
|---|---|---|
| `github.com/jackc/pgx/v5` | v5.11.0 | Only direct runtime dependency |
| `github.com/joho/godotenv` | v1.5.1 | Used by `util` only; upstream is feature-complete |
| `github.com/stretchr/testify` | v1.12.1 | Tests only |
| `go.uber.org/mock` | v0.6.0 | Mock generation; replaced the archived `github.com/golang/mock` |

## Layout

```
psqldb/
  builder.go                  # Op constants, CTEContext, BuilderCore, ValidateIdentifier
  querybuilder.go             # SELECT
  insertBuilder.go            # INSERT   (note the capital B in the filename)
  updatebuilder.go            # UPDATE
  deletebuilder.go            # DELETE
  auth_permissions_cte.go     # AuthPermissionsCTE
  trusted_db_expressions.go   # Expr, ExprNow
  database.go                 # PgxDatabase + wrappers, error helpers, Tx options
  lazydatabase.go             # LazyDatabase (sync.Once)
  config.go                   # Config, ConnectionPoolConfig
  dbinterface/
    database_interface.go     # Database, Transaction, Rows, Row, CommandTag, Batch...
    mocks/mock_database.go    # generated — never edit by hand
util/
  property.go                 # GetEnv, GetEnvAsInt, GetEnvName, LoadEnvFile
```

## Architecture invariants

Break these and the safety guarantees in the README stop holding.

**Values go through `Param`, identifiers go through the quoters.** `BuilderCore.Param(v)`
appends to `args` and returns the `$n` placeholder. Any new builder method that accepts a
value must use it. Any method accepting a column or table name must go through
`QuoteIdentifier` (bare) or `QuoteDottedIdentifier` (`a.b` → `"a"."b"`). Both validate
against `^[A-Za-z_][A-Za-z0-9_]*$`.

**Errors accumulate; they do not propagate early.** Builder methods check `c.err` first
and return the receiver unchanged if it is set, so a failure part-way through a chain is
sticky and surfaces once at `Build()`. Never return an error from a chaining method — it
would break the fluent API.

**`dbinterface` is a deliberate indirection, not a thin alias.** `Rows`, `Row`,
`CommandTag` etc. are this library's *own* minimal interfaces, and `PgxRows`/`PgxRow`
wrap the pgx equivalents. This is what keeps consumers off the pgx surface — and it is
why pgx 5.11's new `Rows.TypeMap` requirement for custom `pgx.Rows` implementations did
not affect us. Preserve that boundary: do not widen these interfaces to match pgx.

**`Expr` is the single escape hatch.** It is the only type that renders unparameterized
into SQL. `ExprNow()` is the only constructor, and it should stay that way unless there
is a strong reason.

## Mocks

`psqldb/dbinterface/mocks/mock_database.go` is generated. After changing any interface in
`database_interface.go`:

```bash
make generate    # runs the go:generate directive in database_interface.go
```

That pins mockgen to `go.uber.org/mock/mockgen@v0.6.0`. Bump the version in the directive
and in `go.mod` together.

## Testing

```bash
make test     # -race -cover across ./...
make cover    # function-level coverage summary
```

Tests are pure unit tests against generated SQL — there is no live database anywhere in
the suite, and it should stay that way. Builder tests assert on the exact SQL string and
the arg slice, so formatting changes to generated SQL will fail tests loudly. That is
intended.

## Releasing

Consumed by tag, so a release is a tag:

```bash
git checkout main && git pull
make release-minor    # test + tidy, then tag and push
```

`release-patch` / `release-minor` / `release-major` all run `test` and `tidy` first.

Two things to watch:

- **`tidy` runs before tagging, and its edits are not committed.** If `go mod tidy`
  changes anything, that change will not be in the tag. Commit first, then release.
- **`git describe --tags --abbrev=0` currently returns `v0.1.18`, not `v0.1.19`** —
  both tags point at the same commit (`17979b5`). So `make tag-patch` computes `v0.1.19`,
  which already exists, and fails. `tag-minor` and `tag-major` are unaffected.

Breaking changes go in the **minor** slot while the module is `v0.x`. No `/v2` module
path suffix is needed until `v2.0.0`.

## Known issues

- `AppendProximitySearch` formats its coordinates into the SQL with `%f` instead of using
  `Param`. The values are `float64`, so this is not injectable, but it is inconsistent
  with every other builder method. Changing it would alter the generated SQL string and
  break the builder tests that assert on it, so it needs its own change.

The latitude/longitude argument order in `AppendProximitySearch` was wrong until it was
corrected; `TestAppendProximitySearchAddsST_DWithin` now pins the `ST_MakePoint(lon, lat)`
ordering. Keep that assertion — checking only that both values appear in the SQL passes
even when the arguments are swapped.

## Don't

- Don't concatenate values into SQL — use `Values`/`Set`/`Where*`.
- Don't edit `mocks/mock_database.go` by hand.
- Don't add a `replace` directive in a consumer; pin a tag.
- Don't widen `dbinterface` to expose pgx types.
- Don't re-point an existing tag. Consumers resolve by checksum, and a moved tag breaks
  every build that has already fetched it.
