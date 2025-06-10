// Package queries implements convenience helpers for working with SQL queries.
package queries

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Builder is a raw SQL query builder.
// The zero value is ready to use.
// Do not copy a non-zero Builder.
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
//
// IMPORTANT: to avoid SQL injections, make sure to pass arguments from user input with placeholder verbs.
//
// Placeholder verbs map to the following database placeholders:
//   - MySQL, SQLite: %? -> ?
//   - PostgreSQL:    %$ -> $N
//   - MSSQL:         %@ -> @pN
//
// TODO: document slice arguments usage.
func (b *Builder) Appendf(format string, args ...any) {
	a := make([]any, len(args))
	for i, arg := range args {
		a[i] = argument{value: arg, builder: b}
	}
	fmt.Fprintf(&b.query, format, a...)
}

// Query returns the query string.
func (b *Builder) Query() string { return b.query.String() }

// Args returns the query arguments.
func (b *Builder) Args() []any { return b.args }

type argument struct {
	value   any
	builder *Builder
}

// Format implements [fmt.Formatter].
func (a argument) Format(s fmt.State, verb rune) {
	switch verb {
	case '?', '$', '@':
		if a.builder.placeholder == 0 {
			a.builder.placeholder = verb
		}
		if a.builder.placeholder != verb {
			panic("unexpected placeholder")
		}
	default:
		format := fmt.FormatString(s, verb)
		fmt.Fprintf(s, format, a.value)
		return
	}

	if s.Flag('+') {
		a.writeSlice(s, verb)
	} else {
		a.writePlaceholder(s, verb)
		a.builder.args = append(a.builder.args, a.value)
	}
}

func (a argument) writePlaceholder(w io.Writer, verb rune) {
	switch verb {
	case '?': // MySQL, SQLite
		fmt.Fprint(w, "?")
	case '$': // PostgreSQL
		a.builder.counter++
		fmt.Fprintf(w, "$%d", a.builder.counter)
	case '@': // MSSQL
		a.builder.counter++
		fmt.Fprintf(w, "@p%d", a.builder.counter)
	}
}

func (a argument) writeSlice(w io.Writer, verb rune) {
	slice := reflect.ValueOf(a.value)
	if slice.Kind() != reflect.Slice {
		panic("non-slice argument")
	}

	if slice.Len() == 0 {
		// TODO: revisit.
		// "WHERE IN (NULL)" will always result in an empty result set,
		// which may be undesirable in some situations.
		fmt.Fprint(w, "NULL")
		return
	}

	for i := range slice.Len() {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}
		a.writePlaceholder(w, verb)
		a.builder.args = append(a.builder.args, slice.Index(i).Interface())
	}
}
