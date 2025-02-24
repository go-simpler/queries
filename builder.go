package queries

import (
	"fmt"
	"strings"
)

type Builder struct {
	query       strings.Builder
	args        []any
	counter     int
	placeholder rune
}

func (b *Builder) Appendf(format string, args ...any) {
	a := make([]any, len(args))
	for i, arg := range args {
		a[i] = argument{value: arg, builder: b}
	}
	fmt.Fprintf(&b.query, format, a...)
}

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

func (b *Builder) Args() []any { return b.args }

type argument struct {
	value   any
	builder *Builder
}

// Format implements the [fmt.Formatter] interface.
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
