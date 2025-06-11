package queries

import (
	"context"
	"database/sql/driver"
)

var (
	_ driver.Driver        = Interceptor{}
	_ driver.DriverContext = Interceptor{}
)

// TODO: document that database/sql falls back to Prepare if the driver returns ErrSkip for Exec/Query.

// Interceptor is a [driver.Driver] wrapper that allows to register callbacks for database queries.
// It must first be registered with [sql.Register] with the same name that is then passed to [sql.Open]:
//
//	interceptor := queries.Interceptor{...}
//	sql.Register("interceptor", interceptor)
//	db, err := sql.Open("interceptor", "dsn")
type Interceptor struct {
	// Driver is a database driver.
	// It must implement [driver.Pinger], [driver.ExecerContext], [driver.QueryerContext],
	// [driver.ConnPrepareContext], and [driver.ConnBeginTx] (most drivers do).
	// Required.
	Driver driver.Driver

	// ExecContext is a callback for both [sql.DB.ExecContext] and [sql.Tx.ExecContext].
	// The implementation must call execer.ExecerContext(ctx, query, args) and return the result.
	// Optional.
	ExecContext func(ctx context.Context, query string, args []driver.NamedValue, execer driver.ExecerContext) (driver.Result, error)

	// QueryContext is a callback for both [sql.DB.QueryContext] and [sql.Tx.QueryContext].
	// The implementation must call queryer.QueryContext(ctx, query, args) and return the result.
	// Optional.
	QueryContext func(ctx context.Context, query string, args []driver.NamedValue, queryer driver.QueryerContext) (driver.Rows, error)

	// PrepareContext is a callback for [sql.DB.PrepareContext].
	// The implementation must call preparer.ConnPrepareContext(ctx, query) and return the result.
	// Optional.
	PrepareContext func(ctx context.Context, query string, preparer driver.ConnPrepareContext) (driver.Stmt, error)
}

// Open implements [driver.Driver].
func (i Interceptor) Open(name string) (driver.Conn, error) {
	panic("unreachable") // driver.DriverContext always takes precedence over driver.Driver.
}

// OpenConnector implements [driver.DriverContext].
func (i Interceptor) OpenConnector(name string) (driver.Connector, error) {
	if d, ok := i.Driver.(driver.DriverContext); ok {
		connector, err := d.OpenConnector(name)
		if err != nil {
			return nil, err
		}
		return wrappedConnector{connector, i}, nil
	}
	connector := dsnConnector{name, i.Driver}
	return wrappedConnector{connector, i}, nil
}

var (
	_ driver.Conn               = wrappedConn{}
	_ driver.Pinger             = wrappedConn{}
	_ driver.ExecerContext      = wrappedConn{}
	_ driver.QueryerContext     = wrappedConn{}
	_ driver.ConnPrepareContext = wrappedConn{}
	_ driver.ConnBeginTx        = wrappedConn{}
)

type wrappedConn struct {
	driver.Conn
	interceptor Interceptor
}

// Ping implements [driver.Pinger].
func (c wrappedConn) Ping(ctx context.Context) error {
	pinger, ok := c.Conn.(driver.Pinger)
	if !ok {
		panic("queries: driver does not implement driver.Pinger")
	}
	return pinger.Ping(ctx)
}

// ExecContext implements [driver.ExecerContext].
func (c wrappedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		panic("queries: driver does not implement driver.ExecerContext")
	}
	if c.interceptor.ExecContext != nil {
		return c.interceptor.ExecContext(ctx, query, args, execer)
	}
	return execer.ExecContext(ctx, query, args)
}

// QueryContext implements [driver.QueryContext].
func (c wrappedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		panic("queries: driver does not implement driver.QueryerContext")
	}
	if c.interceptor.QueryContext != nil {
		return c.interceptor.QueryContext(ctx, query, args, queryer)
	}
	return queryer.QueryContext(ctx, query, args)
}

// PrepareContext implements [driver.ConnPrepareContext].
func (c wrappedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := c.Conn.(driver.ConnPrepareContext)
	if !ok {
		panic("queries: driver does not implement driver.ConnPrepareContext")
	}
	if c.interceptor.PrepareContext != nil {
		return c.interceptor.PrepareContext(ctx, query, preparer)
	}
	return preparer.PrepareContext(ctx, query)
}

// BeginTx implements [driver.ConnBeginTx].
func (c wrappedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	beginner, ok := c.Conn.(driver.ConnBeginTx)
	if !ok {
		panic("queries: driver does not implement driver.ConnBeginTx")
	}
	return beginner.BeginTx(ctx, opts)
}

var _ driver.SessionResetter = wrappedConnSessionResetter{}

type wrappedConnSessionResetter struct{ wrappedConn }

// ResetSession implements [driver.SessionResetter].
func (c wrappedConnSessionResetter) ResetSession(ctx context.Context) error {
	return c.Conn.(driver.SessionResetter).ResetSession(ctx)
}

var _ driver.Validator = wrappedConnValidator{}

type wrappedConnValidator struct{ wrappedConn }

// IsValid implements [driver.Validator].
func (c wrappedConnValidator) IsValid() bool {
	return c.Conn.(driver.Validator).IsValid()
}

var _ driver.NamedValueChecker = wrappedConnNamedValueChecker{}

type wrappedConnNamedValueChecker struct{ wrappedConn }

// CheckNamedValue implements [driver.NamedValueChecker].
func (c wrappedConnNamedValueChecker) CheckNamedValue(nv *driver.NamedValue) error {
	return c.Conn.(driver.NamedValueChecker).CheckNamedValue(nv)
}

var _ driver.Connector = wrappedConnector{}

type wrappedConnector struct {
	driver.Connector
	interceptor Interceptor
}

// Connect implements [driver.Connector].
func (c wrappedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	wconn := wrappedConn{conn, c.interceptor}
	_, isSessionResetter := conn.(driver.SessionResetter)
	_, isValidator := conn.(driver.Validator)
	_, isNamedValueChecker := conn.(driver.NamedValueChecker)

	switch {
	case isSessionResetter && isValidator && isNamedValueChecker:
		return struct {
			wrappedConn
			wrappedConnSessionResetter
			wrappedConnValidator
			wrappedConnNamedValueChecker
		}{
			wconn,
			wrappedConnSessionResetter{wconn},
			wrappedConnValidator{wconn},
			wrappedConnNamedValueChecker{wconn},
		}, nil
	case isSessionResetter && isValidator:
		return struct {
			wrappedConn
			wrappedConnSessionResetter
			wrappedConnValidator
		}{
			wconn,
			wrappedConnSessionResetter{wconn},
			wrappedConnValidator{wconn},
		}, nil
	case isSessionResetter && isNamedValueChecker:
		return struct {
			wrappedConn
			wrappedConnSessionResetter
			wrappedConnNamedValueChecker
		}{
			wconn,
			wrappedConnSessionResetter{wconn},
			wrappedConnNamedValueChecker{wconn},
		}, nil
	case isValidator && isNamedValueChecker:
		return struct {
			wrappedConn
			wrappedConnValidator
			wrappedConnNamedValueChecker
		}{
			wconn,
			wrappedConnValidator{wconn},
			wrappedConnNamedValueChecker{wconn},
		}, nil
	case isSessionResetter:
		return wrappedConnSessionResetter{wconn}, nil
	case isValidator:
		return wrappedConnValidator{wconn}, nil
	case isNamedValueChecker:
		return wrappedConnNamedValueChecker{wconn}, nil
	default:
		return wconn, nil
	}
}

// copied from https://go.dev/src/database/sql/sql.go
type dsnConnector struct {
	dsn    string
	driver driver.Driver
}

func (t dsnConnector) Connect(context.Context) (driver.Conn, error) { return t.driver.Open(t.dsn) }
func (t dsnConnector) Driver() driver.Driver                        { return t.driver }
