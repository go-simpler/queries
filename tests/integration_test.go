package tests

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	pgx "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
	mssql "github.com/microsoft/go-mssqldb"
	"go-simpler.org/assert"
	. "go-simpler.org/assert/EF"
	"go-simpler.org/queries"
	"modernc.org/sqlite"
)

//	----------------------------------------------------------------------------------------------------------------------
//	| Interface / Driver          | lib/pq | jackc/pgx | go-sql-driver/mysql | modernc.org/sqlite | microsoft/go-mssqldb |
//	|-----------------------------|--------|-----------|---------------------|--------------------|----------------------|
//	| [driver.DriverContext]      |   -    |     +     |          +          |          -         |           -          |
//	| [driver.Pinger]             |   +    |     +     |          +          |          +         |           +          |
//	| [driver.ExecerContext]      |   +    |     +     |          +          |          +         |           -          |
//	| [driver.QueryerContext]     |   +    |     +     |          +          |          +         |           -          |
//	| [driver.ConnPrepareContext] |   +    |     +     |          +          |          +         |           +          |
//	| [driver.ConnBeginTx]        |   +    |     +     |          +          |          +         |           +          |
//	| [driver.SessionResetter]    |   +    |     +     |          +          |          +         |           +          |
//	| [driver.Validator]          |   +    |     -     |          +          |          +         |           +          |
//	| [driver.NamedValueChecker]  |   -    |     +     |          +          |          -         |           +          |
//	----------------------------------------------------------------------------------------------------------------------
//
// See https://go.dev/wiki/SQLDrivers for the full list of drivers.
var databases = map[string]struct {
	dataSourceName      string
	insertFixturesQuery string
	drivers             map[string]driver.Driver
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
			"modernc.org/sqlite": new(sqlite.Driver), // https://gitlab.com/cznic/sqlite ,
		},
	},
	"mssql": {
		"sqlserver://sa:root+1234@localhost:1433/msdb",
		"INSERT INTO users (id, name) VALUES (%@, %@), (%@, %@)",
		map[string]driver.Driver{
			"microsoft/go-mssqldb": new(mssql.Driver), // https://github.com/microsoft/go-mssqldb ,
		},
	},
}

func TestIntegration(t *testing.T) {
	ctx := t.Context()

	type dto struct {
		ID   int    `sql:"id"`
		Name string `sql:"name"`
	}
	table := []dto{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
	}

	for _, params := range databases {
		for driverName, driverIface := range params.drivers {
			var execCalls int
			var queryCalls int
			var prepareCalls int

			interceptor := queries.Interceptor{
				Driver: driverIface,
				ExecContext: func(ctx context.Context, query string, args []driver.NamedValue, execer driver.ExecerContext) (driver.Result, error) {
					execCalls++
					t.Logf("[%s] ExecContext: %s %v", driverName, query, namedToAny(args))
					return execer.ExecContext(ctx, query, args)
				},
				QueryContext: func(ctx context.Context, query string, args []driver.NamedValue, queryer driver.QueryerContext) (driver.Rows, error) {
					queryCalls++
					t.Logf("[%s] QueryContext: %s %v", driverName, query, namedToAny(args))
					return queryer.QueryContext(ctx, query, args)
				},
				PrepareContext: func(ctx context.Context, query string, preparer driver.ConnPrepareContext) (driver.Stmt, error) {
					prepareCalls++
					t.Logf("[%s] PrepareContext: %s", driverName, query)
					return preparer.PrepareContext(ctx, query)
				},
			}

			sql.Register(driverName, interceptor)
			db, err := sql.Open(driverName, params.dataSourceName)
			assert.NoErr[F](t, err)
			defer db.Close()

			// wait until the database is ready.
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

			query, args := queries.Build(params.insertFixturesQuery,
				table[0].ID, table[0].Name,
				table[1].ID, table[1].Name,
			)
			_, err = db.ExecContext(ctx, query, args...)
			assert.NoErr[F](t, err)

			tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
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
				assert.Equal[E](t, user.ID, table[0].ID)
				assert.Equal[E](t, user.Name, table[0].Name)

				var i int
				for user, err := range queries.Query[dto](ctx, queryer, "SELECT id, name FROM users ORDER BY id") {
					assert.NoErr[F](t, err)
					assert.Equal[E](t, user.ID, table[i].ID)
					assert.Equal[E](t, user.Name, table[i].Name)
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
			case *mssql.Driver: // always uses PrepareContext.
				assert.Equal[E](t, execCalls, 0)
				assert.Equal[E](t, queryCalls, 0)
				assert.Equal[E](t, prepareCalls, 3+5*2)
			default:
				assert.Equal[E](t, execCalls, 3)
				assert.Equal[E](t, queryCalls, 5*2)
				assert.Equal[E](t, prepareCalls, 0)
			}
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
