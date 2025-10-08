package queries_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"go-simpler.org/queries"
	"go-simpler.org/queries/internal/assert"
	. "go-simpler.org/queries/internal/assert/EF"
)

func TestInterceptor(t *testing.T) {
	ctx := t.Context()

	var execCalled bool
	var queryCalled bool
	var prepareCalled bool

	interceptor := queries.Interceptor{
		Driver: mockDriver{conn: spyConn{}},
		ExecContext: func(ctx context.Context, query string, args []driver.NamedValue, execer driver.ExecerContext) (driver.Result, error) {
			execCalled = true
			return execer.ExecContext(ctx, query, args)
		},
		QueryContext: func(ctx context.Context, query string, args []driver.NamedValue, queryer driver.QueryerContext) (driver.Rows, error) {
			queryCalled = true
			return queryer.QueryContext(ctx, query, args)
		},
		PrepareContext: func(ctx context.Context, query string, preparer driver.ConnPrepareContext) (driver.Stmt, error) {
			prepareCalled = true
			return preparer.PrepareContext(ctx, query)
		},
	}

	driverName := t.Name() + "_interceptor"
	sql.Register(driverName, interceptor)

	db, err := sql.Open(driverName, "")
	assert.NoErr[F](t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, "")
	assert.IsErr[E](t, err, errCalled)
	assert.Equal[E](t, execCalled, true)

	_, err = db.QueryContext(ctx, "") //nolint:gocritic // sqlQuery: unused result is fine here.
	assert.IsErr[E](t, err, errCalled)
	assert.Equal[E](t, queryCalled, true)

	_, err = db.PrepareContext(ctx, "")
	assert.IsErr[E](t, err, errCalled)
	assert.Equal[E](t, prepareCalled, true)
}

func TestInterceptor_passthrough(t *testing.T) {
	ctx := t.Context()

	interceptor := queries.Interceptor{
		Driver: mockDriver{conn: spyConn{}},
	}

	driverName := t.Name() + "_interceptor"
	sql.Register(driverName, interceptor)

	db, err := sql.Open(driverName, "")
	assert.NoErr[F](t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, "")
	assert.IsErr[E](t, err, errCalled)

	_, err = db.QueryContext(ctx, "") //nolint:gocritic // sqlQuery: unused result is fine here.
	assert.IsErr[E](t, err, errCalled)

	_, err = db.PrepareContext(ctx, "")
	assert.IsErr[E](t, err, errCalled)
}

func TestInterceptor_unimplemented(t *testing.T) {
	ctx := t.Context()

	interceptor := queries.Interceptor{
		Driver: mockDriver{conn: unimplementedConn{}},
	}

	driverName := t.Name() + "_interceptor"
	sql.Register(driverName, interceptor)

	db, err := sql.Open(driverName, "")
	assert.NoErr[F](t, err)
	defer db.Close()

	pingFn := func() { _ = db.PingContext(ctx) }
	assert.Panics[E](t, pingFn, "queries: driver does not implement driver.Pinger")

	execFn := func() { _, _ = db.ExecContext(ctx, "") }
	assert.Panics[E](t, execFn, "queries: driver does not implement driver.ConnPrepareContext")

	queryFn := func() { _, _ = db.QueryContext(ctx, "") } //nolint:gocritic // sqlQuery: unused result is fine here.
	assert.Panics[E](t, queryFn, "queries: driver does not implement driver.ConnPrepareContext")

	prepareFn := func() { _, _ = db.PrepareContext(ctx, "") }
	assert.Panics[E](t, prepareFn, "queries: driver does not implement driver.ConnPrepareContext")

	beginFn := func() { _, _ = db.BeginTx(ctx, nil) }
	assert.Panics[E](t, beginFn, "queries: driver does not implement driver.ConnBeginTx")
}

func TestInterceptor_driver(t *testing.T) {
	mdriver := mockDriver{}
	interceptor := queries.Interceptor{Driver: mdriver}

	driverName := t.Name() + "_interceptor"
	sql.Register(driverName, interceptor)

	db, err := sql.Open(driverName, "")
	assert.NoErr[F](t, err)
	defer db.Close()

	assert.Equal[E](t, db.Driver(), driver.Driver(mdriver))
}

type mockDriver struct{ conn driver.Conn }

func (d mockDriver) Open(string) (driver.Conn, error) { return d.conn, nil }

type unimplementedConn struct{}

func (unimplementedConn) Prepare(string) (driver.Stmt, error) { panic("unimplemented") }
func (unimplementedConn) Close() error                        { return nil }
func (unimplementedConn) Begin() (driver.Tx, error)           { panic("unimplemented") }

var errCalled = errors.New("called")

type spyConn struct{ unimplementedConn }

func (spyConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return nil, errCalled
}

func (spyConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, errCalled
}

func (spyConn) PrepareContext(context.Context, string) (driver.Stmt, error) {
	return nil, errCalled
}
