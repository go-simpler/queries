package queries

import (
	"context"
	"database/sql/driver"
)

var (
	_ driver.Driver        = Interceptor{}
	_ driver.DriverContext = Interceptor{}
)

type Interceptor struct {
	Driver       driver.Driver
	ExecContext  func(ctx context.Context, query string, args []driver.NamedValue, execer driver.ExecerContext) (driver.Result, error)
	QueryContext func(ctx context.Context, query string, args []driver.NamedValue, queryer driver.QueryerContext) (driver.Rows, error)
}

// Open implements [driver.Driver].
func (i Interceptor) Open(name string) (driver.Conn, error) {
	panic("unreachable") // driver.DriverContext always takes precedence over driver.Driver.
}

// OpenConnector implements [driver.DriverContext].
func (i Interceptor) OpenConnector(name string) (driver.Connector, error) {
	if driver, ok := i.Driver.(driver.DriverContext); ok {
		connector, err := driver.OpenConnector(name)
		if err != nil {
			return nil, err
		}
		return wrappedConnector{connector, i}, nil
	}
	connector := dsnConnector{name, i.Driver}
	return wrappedConnector{connector, i}, nil
}

var (
	_ driver.Conn           = wrappedConn{}
	_ driver.ExecerContext  = wrappedConn{}
	_ driver.QueryerContext = wrappedConn{}
)

type wrappedConn struct {
	driver.Conn
	interceptor Interceptor
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
	return wrappedConn{conn, c.interceptor}, nil
}

// copied from https://go.dev/src/database/sql/sql.go
type dsnConnector struct {
	dsn    string
	driver driver.Driver
}

func (t dsnConnector) Connect(_ context.Context) (driver.Conn, error) { return t.driver.Open(t.dsn) }
func (t dsnConnector) Driver() driver.Driver                          { return t.driver }
