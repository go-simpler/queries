package tests

import (
	"cmp"
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	pgx "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
	mssqldb "github.com/microsoft/go-mssqldb"
	ora "github.com/sijms/go-ora/v2"
	"go-simpler.org/assert"
	. "go-simpler.org/assert/EF"
	"go-simpler.org/queries"
	"modernc.org/sqlite"
)

//	-------------------------------------------------------------------------------------------------------------------------------------
//	| Interface / Driver          | lib/pq | jackc/pgx | go-sql-driver/mysql | modernc.org/sqlite | microsoft/go-mssqldb | sijms/go-ora |
//	|-----------------------------|--------|-----------|---------------------|--------------------|----------------------|--------------|
//	| [driver.DriverContext]      |   -    |     +     |          +          |          -         |           -          |      +       |
//	| [driver.Pinger]             |   +    |     +     |          +          |          +         |           +          |      +       |
//	| [driver.ExecerContext]      |   +    |     +     |          +          |          +         |           -          |      +       |
//	| [driver.QueryerContext]     |   +    |     +     |          +          |          +         |           -          |      +       |
//	| [driver.ConnPrepareContext] |   +    |     +     |          +          |          +         |           +          |      +       |
//	| [driver.ConnBeginTx]        |   +    |     +     |          +          |          +         |           +          |      +       |
//	| [driver.SessionResetter]    |   +    |     +     |          +          |          +         |           +          |      +       |
//	| [driver.Validator]          |   +    |     -     |          +          |          +         |           +          |      -       |
//	| [driver.NamedValueChecker]  |   -    |     +     |          +          |          -         |           +          |      +       |
//	-------------------------------------------------------------------------------------------------------------------------------------
//
// See https://go.dev/wiki/SQLDrivers for the full list of drivers.
var databases = map[string]struct {
	dataSourceName            string
	insertFixturesQueryFormat string
	drivers                   map[string]driver.Driver
}{
	"postgres": {
		"postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
		"INSERT INTO users (id, name) VALUES (%$, %$), (%$, %$)",
		map[string]driver.Driver{
			"lib/pq":    new(pq.Driver),         // https://github.com/lib/pq
			"jackc/pgx": pgx.GetDefaultDriver(), // https://github.com/jackc/pgx
		},
	},
	"mysql": {
		"root:root@tcp(localhost:3306)/mysql?parseTime=true",
		"INSERT INTO users (id, name) VALUES (%?, %?), (%?, %?)",
		map[string]driver.Driver{
			"go-sql-driver/mysql": new(mysql.MySQLDriver), // https://github.com/go-sql-driver/mysql
		},
	},
	"sqlite": {
		"test.sqlite",
		"INSERT INTO users (id, name) VALUES (%?, %?), (%?, %?)",
		map[string]driver.Driver{
			"modernc.org/sqlite": new(sqlite.Driver), // https://gitlab.com/cznic/sqlite
		},
	},
	"mssql": {
		"sqlserver://sa:root+1234@localhost:1433/msdb",
		"INSERT INTO users (id, name) VALUES (%@, %@), (%@, %@)",
		map[string]driver.Driver{
			"microsoft/go-mssqldb": new(mssqldb.Driver), // https://github.com/microsoft/go-mssqldb
		},
	},
	"oracle": {
		"oracle://sys:root@localhost:1521/freepdb1",
		"INSERT INTO users (id, name) VALUES (%:, %:), (%:, %:)",
		map[string]driver.Driver{
			"sijms/go-ora": new(ora.OracleDriver), // https://github.com/sijms/go-ora
		},
	},
}

func TestIntegration(t *testing.T) {
	ctx := t.Context()

	type dto struct {
		ID   int    `sql:"id"`
		Name string `sql:"name"`

		// In Oracle, all columns are uppercase.
		ID2   int    `sql:"ID"`
		Name2 string `sql:"NAME"`
	}
	table := []dto{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
	}

	for dbName, dbParams := range databases {
		for driverName, driverIface := range dbParams.drivers {
			t.Run(dbName+"+"+driverName, func(t *testing.T) {
				var execCalls int
				var queryCalls int
				var prepareCalls int
				var beginTxCalls int

				interceptor := queries.Interceptor{
					Driver: driverIface,
					ExecContext: func(ctx context.Context, query string, args []driver.NamedValue, execer driver.ExecerContext) (driver.Result, error) {
						execCalls++
						t.Logf("ExecContext: %s %v", query, namedToAny(args))
						return execer.ExecContext(ctx, query, args)
					},
					QueryContext: func(ctx context.Context, query string, args []driver.NamedValue, queryer driver.QueryerContext) (driver.Rows, error) {
						queryCalls++
						t.Logf("QueryContext: %s %v", query, namedToAny(args))
						return queryer.QueryContext(ctx, query, args)
					},
					PrepareContext: func(ctx context.Context, query string, preparer driver.ConnPrepareContext) (driver.Stmt, error) {
						prepareCalls++
						t.Logf("PrepareContext: %s", query)
						return preparer.PrepareContext(ctx, query)
					},
					BeginTx: func(ctx context.Context, opts driver.TxOptions, beginner driver.ConnBeginTx) (driver.Tx, error) {
						beginTxCalls++
						t.Log("BeginTx")
						return beginner.BeginTx(ctx, opts)
					},
				}

				driverName += "+interceptor"
				sql.Register(driverName, interceptor)

				db, err := sql.Open(driverName, dbParams.dataSourceName)
				assert.NoErr[F](t, err)
				defer db.Close()

				for attempt := 0; ; attempt++ {
					err := db.PingContext(ctx)
					if err == nil {
						break
					}
					if attempt == 10 {
						t.Fatal(err)
					}
					time.Sleep(time.Second)
				}

				_, err = db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name VARCHAR(8))")
				assert.NoErr[F](t, err)

				query, args := queries.Build(dbParams.insertFixturesQueryFormat,
					table[0].ID, table[0].Name,
					table[1].ID, table[1].Name,
				)
				_, err = db.ExecContext(ctx, query, args...)
				assert.NoErr[F](t, err)

				tx, err := db.BeginTx(ctx, nil)
				assert.NoErr[F](t, err)
				defer tx.Rollback()

				for _, queryer := range []queries.Queryer{db, tx} {
					_, err := queries.QueryRow[string](ctx, queryer, "SELECT name FROM users WHERE id = 0")
					assert.IsErr[E](t, err, sql.ErrNoRows)

					name, err := queries.QueryRow[string](ctx, queryer, "SELECT name FROM users WHERE id = 1")
					assert.NoErr[F](t, err)
					assert.Equal[E](t, name, table[0].Name)

					names, err := queries.Collect(queries.Query[string](ctx, queryer, "SELECT name FROM users"))
					assert.NoErr[F](t, err)
					assert.Equal[E](t, names, []string{table[0].Name, table[1].Name})

					user, err := queries.QueryRow[dto](ctx, queryer, "SELECT id, name FROM users WHERE id = 1")
					assert.NoErr[F](t, err)
					assert.Equal[E](t, cmp.Or(user.ID, user.ID2), table[0].ID)
					assert.Equal[E](t, cmp.Or(user.Name, user.Name2), table[0].Name)

					var i int
					for user, err := range queries.Query[dto](ctx, queryer, "SELECT id, name FROM users ORDER BY id") {
						assert.NoErr[F](t, err)
						assert.Equal[E](t, cmp.Or(user.ID, user.ID2), table[i].ID)
						assert.Equal[E](t, cmp.Or(user.Name, user.Name2), table[i].Name)
						i++
					}
				}

				assert.NoErr[F](t, tx.Commit())

				_, err = db.ExecContext(ctx, "DROP TABLE users")
				assert.NoErr[F](t, err)

				switch db.Driver().(type) {
				case *mysql.MySQLDriver: // falls back to PrepareContext for queries with arguments.
					assert.Equal[E](t, execCalls, 3)
					assert.Equal[E](t, queryCalls, 5*2)
					assert.Equal[E](t, prepareCalls, 1)
					assert.Equal[E](t, beginTxCalls, 1)
				case *mssqldb.Driver: // always uses PrepareContext.
					assert.Equal[E](t, execCalls, 0)
					assert.Equal[E](t, queryCalls, 0)
					assert.Equal[E](t, prepareCalls, 3+5*2)
					assert.Equal[E](t, beginTxCalls, 1)
				default:
					assert.Equal[E](t, execCalls, 3)
					assert.Equal[E](t, queryCalls, 5*2)
					assert.Equal[E](t, prepareCalls, 0)
					assert.Equal[E](t, beginTxCalls, 1)
				}
			})
		}
	}
}

func namedToAny(values []driver.NamedValue) []any {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value.Value
	}
	return args
}
