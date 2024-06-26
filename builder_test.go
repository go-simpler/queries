package queries_test

import (
	"context"
	"testing"

	"go-simpler.org/queries"
	"go-simpler.org/queries/internal/assert"
	. "go-simpler.org/queries/internal/assert/EF"
)

//go:generate go run -tags=cp go-simpler.org/assert/cmd/cp@v0.9.0 -dir=internal

func TestBuilder(t *testing.T) {
	var qb queries.Builder
	qb.Appendf("select %s from tbl where 1=1", "*")
	qb.Appendf(" and foo = %$", 1)
	qb.Appendf(" and bar = %$", 2)
	qb.Appendf(" and baz = %$", 3)

	assert.Equal[E](t, qb.String(), "select * from tbl where 1=1 and foo = $1 and bar = $2 and baz = $3")
	assert.Equal[E](t, qb.Args, []any{1, 2, 3})
}

func TestBuilder_placeholders(t *testing.T) {
	tests := map[string]struct {
		format string
		query  string
		debug  string
	}{
		"?": {
			format: "select * from tbl where foo = %? and bar = %? and baz = %?",
			query:  "select * from tbl where foo = ? and bar = ? and baz = ?",
			debug:  "select * from tbl where foo = 42 and bar = 'test' and baz = 'context.Background'",
		},
		"$": {
			format: "select * from tbl where foo = %$ and bar = %$ and baz = %$",
			query:  "select * from tbl where foo = $1 and bar = $2 and baz = $3",
			debug:  "select * from tbl where foo = 42 and bar = 'test' and baz = 'context.Background'",
		},
		"@": {
			format: "select * from tbl where foo = %@ and bar = %@ and baz = %@",
			query:  "select * from tbl where foo = @p1 and bar = @p2 and baz = @p3",
			debug:  "select * from tbl where foo = 42 and bar = 'test' and baz = 'context.Background'",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var qb queries.Builder
			qb.Appendf(tt.format, 42, "test", context.Background())
			assert.Equal[E](t, qb.String(), tt.query)
			assert.Equal[E](t, qb.Args, []any{42, "test", context.Background()})
			assert.Equal[E](t, qb.DebugString(), tt.debug)
		})
	}
}

func TestBuilder_badQuery(t *testing.T) {
	tests := map[string]struct {
		appends  func(*queries.Builder)
		panicMsg string
	}{
		"bad verb": {
			appends: func(qb *queries.Builder) {
				qb.Appendf("select %d from tbl", "foo")
			},
			panicMsg: "queries: bad query: select %!d(string=foo) from tbl",
		},
		"too few arguments": {
			appends: func(qb *queries.Builder) {
				qb.Appendf("select %s from tbl")
			},
			panicMsg: "queries: bad query: select %!s(MISSING) from tbl",
		},
		"too many arguments": {
			appends: func(qb *queries.Builder) {
				qb.Appendf("select %s from tbl", "foo", "bar")
			},
			panicMsg: "queries: bad query: select foo from tbl%!(EXTRA queries.argument=bar)",
		},
		"different placeholders": {
			appends: func(qb *queries.Builder) {
				qb.Appendf("select * from tbl where foo = %? and bar = %$ and baz = %@", 1, 2, 3)
			},
			panicMsg: "queries: bad query: different placeholders used",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var qb queries.Builder
			tt.appends(&qb)
			assert.Panics[E](t, func() { _ = qb.String() }, tt.panicMsg)
		})
	}
}
