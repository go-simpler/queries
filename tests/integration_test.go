package tests

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	pgx "github.com/jackc/pgx/v5/stdlib"
	"go-simpler.org/assert"
	. "go-simpler.org/assert/EF"
	"go-simpler.org/queries"
)

var DBs = map[string]struct {
	driver driver.Driver
	dsn    string
}{
	"postgres": {pgx.GetDefaultDriver(), "postgres://postgres:postgres@localhost:5432/postgres"},
	"mysql":    {new(mysql.MySQLDriver), "root:root@tcp(localhost:3306)/mysql?parseTime=true"},
}

type User struct {
	ID        int       `sql:"id"`
	Name      string    `sql:"name"`
	CreatedAt time.Time `sql:"created_at"`
}

var TableUsers = []User{
	{ID: 1, Name: "Alice"},
	{ID: 2, Name: "Bob"},
	{ID: 3, Name: "Carol"},
}

func TestIntegration(t *testing.T) {
	ctx := context.Background()

	for name, database := range DBs {
		var execCalls int
		var queryCalls int
		var prepareCalls int

		interceptor := queries.Interceptor{
			Driver: database.driver,
			ExecContext: func(ctx context.Context, query string, args []driver.NamedValue, execer driver.ExecerContext) (driver.Result, error) {
				execCalls++
				t.Logf("[%s] ExecContext: %s %v", name, query, namedToAny(args))
				return execer.ExecContext(ctx, query, args)
			},
			QueryContext: func(ctx context.Context, query string, args []driver.NamedValue, queryer driver.QueryerContext) (driver.Rows, error) {
				queryCalls++
				t.Logf("[%s] QueryContext: %s %v", name, query, namedToAny(args))
				return queryer.QueryContext(ctx, query, args)
			},
			PrepareContext: func(ctx context.Context, query string, preparer driver.ConnPrepareContext) (driver.Stmt, error) {
				prepareCalls++
				t.Logf("[%s] PrepareContext: %s", name, query)
				return preparer.PrepareContext(ctx, query)
			},
		}

		driverName := name + "_interceptor"
		sql.Register(driverName, interceptor)

		db, err := sql.Open(driverName, database.dsn)
		assert.NoErr[F](t, err)
		defer db.Close()

		// wait until db is ready.
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

		assert.NoErr[F](t, migrate(ctx, db))

		tx, err := db.BeginTx(ctx, nil)
		assert.NoErr[F](t, err)
		defer tx.Rollback()

		for _, queryer := range []interface {
			QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
		}{db, tx} {
			_, err := queries.QueryRow[string](ctx, queryer, "SELECT name FROM users WHERE id = 0")
			assert.IsErr[E](t, err, sql.ErrNoRows)

			name, err := queries.QueryRow[string](ctx, queryer, "SELECT name FROM users WHERE id = 1")
			assert.NoErr[F](t, err)
			assert.Equal[E](t, name, TableUsers[0].Name)

			names, err := queries.Collect(queries.Query[string](ctx, queryer, "SELECT name FROM users"))
			assert.NoErr[F](t, err)
			assert.Equal[E](t, names, []string{TableUsers[0].Name, TableUsers[1].Name, TableUsers[2].Name})

			user, err := queries.QueryRow[User](ctx, queryer, "SELECT id, name, created_at FROM users WHERE id = 1")
			assert.NoErr[F](t, err)
			assert.Equal[E](t, user.ID, TableUsers[0].ID)
			assert.Equal[E](t, user.Name, TableUsers[0].Name)

			var i int
			for user, err := range queries.Query[User](ctx, queryer, "SELECT id, name, created_at FROM users") {
				assert.NoErr[F](t, err)
				assert.Equal[E](t, user.ID, TableUsers[i].ID)
				assert.Equal[E](t, user.Name, TableUsers[i].Name)
				i++
			}
		}

		assert.NoErr[F](t, tx.Commit())
		assert.Equal[E](t, execCalls, 2)
		assert.Equal[E](t, queryCalls, 5*2)
		if name == "mysql" {
			// github.com/go-sql-driver/mysql falls back to PrepareContext for queries with arguments.
			assert.Equal[E](t, prepareCalls, 1)
		} else {
			assert.Equal[E](t, prepareCalls, 0)
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

func migrate(ctx context.Context, db *sql.DB) error {
	type migration struct {
		query string
		args  []any
	}
	migrations := []migration{
		{"CREATE TABLE IF NOT EXISTS users (id INT PRIMARY KEY, name TEXT, created_at TIMESTAMP DEFAULT NOW())", nil},
	}

	var args []any
	for _, user := range TableUsers {
		args = append(args, user.ID, user.Name)
	}

	switch db.Driver().(type) {
	case *pgx.Driver:
		migrations = append(migrations, migration{"INSERT INTO users (id, name) VALUES (%$, %$), (%$, %$), (%$, %$) ON CONFLICT DO NOTHING", args})
	case *mysql.MySQLDriver:
		migrations = append(migrations, migration{"INSERT IGNORE INTO users (id, name) VALUES (%?, %?), (%?, %?), (%?, %?)", args})
	default:
		panic("unreachable")
	}

	for _, m := range migrations {
		var qb queries.Builder
		qb.Appendf(m.query, m.args...)
		if _, err := db.ExecContext(ctx, qb.Query(), qb.Args()...); err != nil {
			return err
		}
	}

	return nil
}
