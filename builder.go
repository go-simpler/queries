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
	// TODO: prealloc?
	query       strings.Builder
	args        []any
	counter     int
	placeholder rune
}

// Appendf formats according to the given format and appends the result to the query.
// It works like [fmt.Appendf], meaning all the rules from the [fmt] package are applied.
// In addition, Appendf supports special verbs that automatically expand to database placeholders.
//
//	-----------------------------------------------
//	| Database               | Verb | Placeholder |
//	|------------------------|------|-------------|
//	| MySQL, MariaDB, SQLite | %?   | ?           |
//	| PostgreSQL             | %$   | $N          |
//	| Microsoft SQL Server   | %@   | @pN         |
//	| Oracle Database        | %:   | :N          |
//	-----------------------------------------------
//
// Here, N is an auto-incrementing counter.
// For example, "%$, %$, %$" expands to "$1, $2, $3".
//
// If a special verb includes the "+" flag, it automatically expands to multiple placeholders.
// For example, given the verb "%+?" and the argument []int{1, 2, 3},
// Appendf writes "?, ?, ?" to the query and appends 1, 2, and 3 to the arguments.
// You may want to use this flag to build "WHERE IN (...)" clauses.
//
// Make sure to always pass arguments from user input with placeholder verbs to avoid SQL injections.
func (b *Builder) Appendf(format string, a ...any) {
	fs := make([]any, len(a))
	for i := range a {
		fs[i] = formatter{arg: a[i], builder: b}
	}
	fmt.Fprintf(&b.query, format, fs...)
}

// Build returns the query and its arguments.
func (b *Builder) Build() (query string, args []any) {
	return b.query.String(), b.args
}

// Build is a shorthand for a new [Builder] + [Builder.Appendf] + [Builder.Build].
func Build(format string, a ...any) (query string, args []any) {
	var b Builder
	b.Appendf(format, a...)
	return b.Build()
}

type formatter struct {
	arg     any
	builder *Builder
}

// Format implements [fmt.Formatter].
func (f formatter) Format(s fmt.State, verb rune) {
	switch verb {
	case '?', '$', '@', ':':
		if f.builder.placeholder == 0 {
			f.builder.placeholder = verb
		}
		if f.builder.placeholder != verb {
			panic("unexpected placeholder")
		}
		if s.Flag('+') {
			appendAll(s, f.builder, verb, f.arg)
		} else {
			appendOne(s, f.builder, verb, f.arg)
		}
	default:
		format := fmt.FormatString(s, verb)
		fmt.Fprintf(s, format, f.arg)
	}
}

func appendOne(w io.Writer, b *Builder, verb rune, arg any) {
	switch verb {
	case '?':
		fmt.Fprint(w, "?")
	case '$':
		b.counter++
		fmt.Fprintf(w, "$%d", b.counter)
	case '@':
		b.counter++
		fmt.Fprintf(w, "@p%d", b.counter)
	case ':':
		b.counter++
		fmt.Fprintf(w, ":%d", b.counter)
	}
	b.args = append(b.args, arg)
}

func appendAll(w io.Writer, b *Builder, verb rune, arg any) {
	slice := reflect.ValueOf(arg)
	if slice.Kind() != reflect.Slice {
		panic("non-slice argument")
	}
	if slice.Len() == 0 {
		panic("zero-length slice argument")
	}
	for i := range slice.Len() {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}
		appendOne(w, b, verb, slice.Index(i).Interface())
	}
}
