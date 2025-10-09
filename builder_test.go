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

	query, args := qb.Build()
	assert.Equal[E](t, query, "SELECT * FROM tbl WHERE 1=1 AND foo = $1 AND bar = $2 AND baz = $3")
	assert.Equal[E](t, args, []any{42, "test", false})
}

func TestBuilder_dialects(t *testing.T) {
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

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			query, args := queries.Build(test.format, 1, 2, 3)
			assert.Equal[E](t, query, test.query)
			assert.Equal[E](t, args, []any{1, 2, 3})
		})
	}
}

func TestBuilder_sliceArgument(t *testing.T) {
	query, args := queries.Build("SELECT * FROM tbl WHERE foo IN (%+$)", []int{1, 2, 3})
	assert.Equal[E](t, query, "SELECT * FROM tbl WHERE foo IN ($1, $2, $3)")
	assert.Equal[E](t, args, []any{1, 2, 3})
}

func TestBuilder_badQuery(t *testing.T) {
	tests := map[string]struct {
		format string
		args   []any
		query  string
	}{
		"wrong verb": {
			format: "SELECT %d FROM tbl",
			args:   []any{"foo"},
			query:  "SELECT %!d(string=foo) FROM tbl",
		},
		"too few arguments": {
			format: "SELECT %s FROM tbl",
			args:   []any{},
			query:  "SELECT %!s(MISSING) FROM tbl",
		},
		"too many arguments": {
			format: "SELECT %s FROM tbl",
			args:   []any{"foo", "bar"},
			query:  "SELECT foo FROM tbl%!(EXTRA queries.formatter=bar)",
		},
		"unexpected placeholder": {
			format: "SELECT * FROM tbl WHERE foo = %? AND bar = %$",
			args:   []any{1, 2},
			query:  "SELECT * FROM tbl WHERE foo = ? AND bar = %!$(PANIC=Format method: unexpected placeholder)",
		},
		"non-slice argument": {
			format: "SELECT * FROM tbl WHERE foo IN (%+$)",
			args:   []any{1},
			query:  "SELECT * FROM tbl WHERE foo IN (%!$(PANIC=Format method: non-slice argument))",
		},
		"zero-length slice argument": {
			format: "SELECT * FROM tbl WHERE foo IN (%+$)",
			args:   []any{[]int{}},
			query:  "SELECT * FROM tbl WHERE foo IN (%!$(PANIC=Format method: zero-length slice argument))",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			query, _ := queries.Build(test.format, test.args...)
			assert.Equal[E](t, query, test.query)
		})
	}
}
