// Package queries implements convenience helpers for working with SQL queries.
package queries

import (
	"fmt"
	"strings"
)

// Builder is a raw SQL query builder.
// The zero value is ready to use.
// Do not copy a non-zero Builder.
// Do not reuse a single Builder for multiple queries.
type Builder struct {
	query       strings.Builder
	args        []any
	counter     int
	placeholder rune
}

// Appendf formats according to the given format and appends the result to the query.
// It works like [fmt.Appendf], i.e. all rules from the [fmt] package are applied.
// In addition, Appendf supports %?, %$, and %@ verbs, which are automatically expanded to the query placeholders ?, $N, and @pN,
// where N is the auto-incrementing counter.
// The corresponding arguments can then be accessed with the [Builder.Args] method.
// IMPORTANT: to avoid SQL injections, make sure to pass arguments from user input with placeholder verbs.
// Always test your queries.
//
// Placeholder verbs map to the following database placeholders:
//   - MySQL, SQLite: %? -> ?
//   - PostgreSQL:    %$ -> $N
//   - MSSQL:         %@ -> @pN
func (b *Builder) Appendf(format string, args ...any) {
	a := make([]any, len(args))
	for i, arg := range args {
		a[i] = argument{value: arg, builder: b}
	}
	fmt.Fprintf(&b.query, format, a...)
}

// Query returns the query string.
// If the query is invalid, e.g. too few/many arguments are given or different placeholders are used, Query panics.
func (b *Builder) Query() string {
	query := b.query.String()
	if strings.Contains(query, "%!") {
		// fmt silently recovers panics and writes them to the output.
		// We want panics to be loud, so we find and rethrow them.
		// See also https://github.com/golang/go/issues/28150.
		panic(fmt.Sprintf("queries: bad query: %s", query))
	}
	if b.placeholder == -1 {
		panic("queries: different placeholders used")
	}
	return query
}

// Args returns the argument slice.
func (b *Builder) Args() []any { return b.args }

type argument struct {
	value   any
	builder *Builder
}

// Format implements [fmt.Formatter].
func (a argument) Format(s fmt.State, verb rune) {
	switch verb {
	case '?', '$', '@':
		a.builder.args = append(a.builder.args, a.value)
		if a.builder.placeholder == 0 {
			a.builder.placeholder = verb
		}
		if a.builder.placeholder != verb {
			a.builder.placeholder = -1
		}
	}

	switch verb {
	case '?': // MySQL, SQLite
		fmt.Fprint(s, "?")
	case '$': // PostgreSQL
		a.builder.counter++
		fmt.Fprintf(s, "$%d", a.builder.counter)
	case '@': // MSSQL
		a.builder.counter++
		fmt.Fprintf(s, "@p%d", a.builder.counter)
	default:
		format := fmt.FormatString(s, verb)
		fmt.Fprintf(s, format, a.value)
	}
}
