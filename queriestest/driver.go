// Package queriestest implements utilities for testing SQL queries.
package queriestest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"slices"
	"testing"
)

// Driver is a convenience helper to easily create an implementation of [driver.Driver] for use in tests.
type Driver struct {
	// ExecContext is a test implementation of [driver.ExecerContext].
	// If the code being tested uses [sql.Result],
	// ExecContext should return a [driver.Result] created with [NewResult].
	// Optional.
	ExecContext func(t *testing.T, query string, args []any) (driver.Result, error)

	// QueryContext is a test implementation of [driver.QueryerContext].
	// If the code being tested uses [sql.Rows],
	// QueryContext should return [Rows] created with [NewRows].
	// Optional.
	QueryContext func(t *testing.T, query string, args []any) (driver.Rows, error)
}

// NewDB creates a test [sql.DB] backed by the given [Driver].
func NewDB(t *testing.T, d Driver) *sql.DB {
	name := t.Name()
	sql.Register(name, testDriver{t, d})
	db, _ := sql.Open(name, "")
	return db
}

var (
	_ driver.Driver         = testDriver{}
	_ driver.Conn           = testDriver{}
	_ driver.ConnBeginTx    = testDriver{}
	_ driver.Tx             = testDriver{}
	_ driver.ExecerContext  = testDriver{}
	_ driver.QueryerContext = testDriver{}
)

type testDriver struct {
	t      *testing.T
	driver Driver
}

// Open implements [driver.Driver].
func (d testDriver) Open(string) (driver.Conn, error) { return d, nil }

// Prepare implements [driver.Conn].
func (testDriver) Prepare(string) (driver.Stmt, error) {
	panic("unimplemented") // TODO: implement [driver.ConnPrepareContext]
}

// Close implements [driver.Conn].
func (testDriver) Close() error { return nil }

// Begin implements [driver.Conn].
func (testDriver) Begin() (driver.Tx, error) {
	panic("unreachable") // BeginTx always takes precedence over Begin.
}

// BeginTx implements [driver.ConnBeginTx].
func (d testDriver) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) { return d, nil }

// Commit implements [driver.Tx].
func (testDriver) Commit() error { return nil }

// Rollback implements [driver.Tx].
func (testDriver) Rollback() error { return nil }

// ExecContext implements [driver.ExecerContext].
func (d testDriver) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if d.driver.ExecContext == nil {
		panic("queriestest: Driver.ExecContext is not set")
	}
	return d.driver.ExecContext(d.t, query, namedToAny(args))
}

// QueryContext implements [driver.QueryerContext].
func (d testDriver) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if d.driver.QueryContext == nil {
		panic("queriestest: Driver.QueryContext is not set")
	}
	return d.driver.QueryContext(d.t, query, namedToAny(args))
}

func namedToAny(values []driver.NamedValue) []any {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value.Value
	}
	return args
}

var _ driver.Result = testResult{}

type testResult struct {
	lastInsertId int64
	rowsAffected int64
}

// NewResult creates a test [driver.Result] from the given values.
func NewResult(lastInsertId, rowsAffected int64) driver.Result {
	return testResult{lastInsertId, rowsAffected}
}

// LastInsertId implements [driver.Result].
func (r testResult) LastInsertId() (int64, error) { return r.lastInsertId, nil }

// RowsAffected implements [driver.Result].
func (r testResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

var _ driver.Rows = new(Rows)

// Rows is a test implementation of [driver.Rows].
type Rows struct {
	columns []string
	values  [][]any
}

// NewRows creates [Rows] from the given columns.
func NewRows(columns ...string) *Rows {
	return &Rows{columns: columns}
}

// Add adds another row to the [Rows].
func (r *Rows) Add(values ...any) *Rows {
	r.values = append(r.values, values)
	return r
}

// Columns implements [driver.Rows].
func (r *Rows) Columns() []string { return r.columns }

// Close implements [driver.Rows].
func (r *Rows) Close() error { return nil }

// Next implements [driver.Rows].
func (r *Rows) Next(values []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	for i := range values {
		values[i] = r.values[0][i]
	}
	r.values = slices.Delete(r.values, 0, 1)
	return nil
}
