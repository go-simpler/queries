package queries_test

import (
	"testing"

	"go-simpler.org/queries"
	"go-simpler.org/queries/internal/assert"
	. "go-simpler.org/queries/internal/assert/EF"
)

//go:generate go run -tags=cp go-simpler.org/assert/cmd/cp@v0.9.0 -dir=internal

func TestBuilder(t *testing.T) {
	var qb queries.Builder
	qb.Appendf("SELECT %s FROM tbl WHERE 1=1", "*")
	qb.Appendf(" AND foo = %$", 42)
	qb.Appendf(" AND bar = %$", "test")
	qb.Appendf(" AND baz = %$", false)

	assert.Equal[E](t, qb.Query(), "SELECT * FROM tbl WHERE 1=1 AND foo = $1 AND bar = $2 AND baz = $3")
	assert.Equal[E](t, qb.Args(), []any{42, "test", false})
}

func TestBuilder_placeholders(t *testing.T) {
	tests := map[string]struct {
		format string
		query  string
	}{
		"?": {
			format: "SELECT * FROM tbl WHERE foo = %? AND bar = %? AND baz = %?",
			query:  "SELECT * FROM tbl WHERE foo = ? AND bar = ? AND baz = ?",
		},
		"$": {
			format: "SELECT * FROM tbl WHERE foo = %$ AND bar = %$ AND baz = %$",
			query:  "SELECT * FROM tbl WHERE foo = $1 AND bar = $2 AND baz = $3",
		},
		"@": {
			format: "SELECT * FROM tbl WHERE foo = %@ AND bar = %@ AND baz = %@",
			query:  "SELECT * FROM tbl WHERE foo = @p1 AND bar = @p2 AND baz = @p3",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var qb queries.Builder
			qb.Appendf(tt.format, 1, 2, 3)
			assert.Equal[E](t, qb.Query(), tt.query)
			assert.Equal[E](t, qb.Args(), []any{1, 2, 3})
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
				qb.Appendf("SELECT %d FROM tbl", "foo")
			},
			panicMsg: "queries: bad query: SELECT %!d(string=foo) FROM tbl",
		},
		"too few arguments": {
			appends: func(qb *queries.Builder) {
				qb.Appendf("SELECT %s FROM tbl")
			},
			panicMsg: "queries: bad query: SELECT %!s(MISSING) FROM tbl",
		},
		"too many arguments": {
			appends: func(qb *queries.Builder) {
				qb.Appendf("SELECT %s FROM tbl", "foo", "bar")
			},
			panicMsg: "queries: bad query: SELECT foo FROM tbl%!(EXTRA queries.argument=bar)",
		},
		"different placeholders": {
			appends: func(qb *queries.Builder) {
				qb.Appendf("SELECT * FROM tbl WHERE foo = %? AND bar = %$ AND baz = %@", 1, 2, 3)
			},
			panicMsg: "queries: different placeholders used",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var qb queries.Builder
			tt.appends(&qb)
			assert.Panics[E](t, func() { _ = qb.Query() }, tt.panicMsg)
		})
	}
}
