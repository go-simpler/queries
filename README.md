# queries

[![checks](https://github.com/go-simpler/queries/actions/workflows/checks.yml/badge.svg)](https://github.com/go-simpler/queries/actions/workflows/checks.yml)
[![pkg.go.dev](https://pkg.go.dev/badge/go-simpler.org/queries.svg)](https://pkg.go.dev/go-simpler.org/queries)
[![goreportcard](https://goreportcard.com/badge/go-simpler.org/queries)](https://goreportcard.com/report/go-simpler.org/queries)
[![codecov](https://codecov.io/gh/go-simpler/queries/branch/main/graph/badge.svg)](https://codecov.io/gh/go-simpler/queries)

Convenience helpers for working with SQL queries.

## 🚀 Features

- Builder: a `strings.Builder` wrapper with added support for placeholder verbs to easily build raw SQL queries conditionally.
- Scanner: `sql.DB.Query/QueryRow` wrappers that automatically scan `sql.Rows` into the given struct. Inspired by [golang/go#61637][1].
- Interceptor: a `driver.Driver` wrapper to easily add instrumentation (logs, metrics, traces) to the database layer. Similar to [gRPC interceptors][2].

## 📦 Install

Go 1.24+

```shell
go get go-simpler.org/queries
```

## 📋 Usage

### Builder

```go
columns := []string{"id", "name"}

var qb queries.Builder
qb.Appendf("SELECT %s FROM users", strings.Join(columns, ", "))

if role != nil { // "admin"
    qb.Appendf(" WHERE role = %$", *role)
}
if orderBy != nil { // "name"
    qb.Appendf(" ORDER BY %$", *orderBy)
}
if limit != nil { // 10
    qb.Appendf(" LIMIT %$", *limit)
}

query, args := qb.Build()
db.QueryContext(ctx, query, args...)
// Query: "SELECT id, name FROM users WHERE role = $1 ORDER BY $2 LIMIT $3"
// Args: ["admin", "name", 10]
```

The following database placeholders are supported:
- `?` (used by MySQL and SQLite)
- `$1`, `$2`, ..., `$N` (used by PostgreSQL)
- `@p1`, `@p2`, ..., `@pN` (used by MSSQL)

### Scanner

```go
type User struct {
    ID   int    `sql:"id"`
    Name string `sql:"name"`
}

// single column, single row:
name, _ := queries.QueryRow[string](ctx, db, "SELECT name FROM users WHERE id = 1")

// single column, multiple rows:
names, _ := queries.Collect(queries.Query[string](ctx, db, "SELECT name FROM users"))

// multiple columns, single row:
user, _ := queries.QueryRow[User](ctx, db, "SELECT id, name FROM users WHERE id = 1")

// multiple columns, multiple rows:
for user, _ := range queries.Query[User](ctx, db, "SELECT id, name FROM users") {
    // ...
}
```

### Interceptor

```go
interceptor := queries.Interceptor{
    Driver: // database driver of your choice.
    ExecContext: func(ctx context.Context, query string, args []driver.NamedValue, execer driver.ExecerContext) (driver.Result, error) {
        slog.InfoContext(ctx, "ExecContext", "query", query)
        return execer.ExecContext(ctx, query, args)
    },
    QueryContext: func(ctx context.Context, query string, args []driver.NamedValue, queryer driver.QueryerContext) (driver.Rows, error) {
        slog.InfoContext(ctx, "QueryContext", "query", query)
        return queryer.QueryContext(ctx, query, args)
    },
}

sql.Register("interceptor", interceptor)
db, _ := sql.Open("interceptor", "dsn")

db.ExecContext(ctx, "INSERT INTO users VALUES (1, 'John Doe')")
// stderr: INFO ExecContext query="INSERT INTO users VALUES (1, 'John Doe')"

db.QueryContext(ctx, "SELECT id, name FROM users")
// stderr: INFO QueryContext query="SELECT id, name FROM users"
```

Integration tests cover the following databases and drivers:
- PostgreSQL with [lib/pq][3], [jackx/pgx][4]
- MySQL with [go-sql-driver/mysql][5]
- SQLite with [modernc.org/sqlite][6]
- Microsoft SQL Server with [microsoft/go-mssqldb][7]

See [integration_test.go](tests/integration_test.go) for details.

## 🚧 TODOs

- Add examples for tested databases and drivers.
- Add benchmarks.

[1]: https://github.com/golang/go/issues/61637
[2]: https://grpc.io/docs/guides/interceptors
[3]: https://github.com/lib/pq
[4]: https://github.com/jackc/pgx
[5]: https://github.com/go-sql-driver/mysql
[6]: https://gitlab.com/cznic/sqlite
[7]: https://github.com/microsoft/go-mssqldb
