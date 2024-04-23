package queries

import (
	"fmt"
	"slices"
	"strings"
)

type Builder struct {
	query        strings.Builder
	Args         []any
	counter      int
	placeholders []rune
}

func (b *Builder) Appendf(format string, args ...any) {
	a := make([]any, len(args))
	for i, arg := range args {
		a[i] = argument{value: arg, builder: b}
	}
	fmt.Fprintf(&b.query, format, a...)
}

func (b *Builder) String() string {
	slices.Sort(b.placeholders)
	if len(slices.Compact(b.placeholders)) > 1 {
		panic(fmt.Sprintf("queries.Builder: bad query: %s placeholders used", string(b.placeholders)))
	}

	s := b.query.String()
	if strings.Contains(s, "%!") {
		// fmt silently recovers panics and writes them to the output.
		// we want panics to be loud, so we find and rethrow them.
		// see also https://github.com/golang/go/issues/28150.
		panic(fmt.Sprintf("queries.Builder: bad query: %s", s))
	}

	return s
}

type argument struct {
	value   any
	builder *Builder
}

// Format implements the [fmt.Formatter] interface.
func (a argument) Format(s fmt.State, verb rune) {
	switch verb {
	case '?', '$', '@':
		a.builder.Args = append(a.builder.Args, a.value)
		a.builder.placeholders = append(a.builder.placeholders, verb)
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
