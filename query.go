package queries

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"reflect"
	"sync"
)

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// TODO: document me.
func Query[T any](ctx context.Context, q queryer, query string, args ...any) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		rows, err := q.QueryContext(ctx, query, args...)
		if err != nil {
			yield(zero[T](), err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			t, err := scan[T](rows)
			if err != nil {
				yield(zero[T](), err)
				return
			}
			if !yield(t, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(zero[T](), err)
			return
		}
	}
}

// TODO: document me.
func QueryRow[T any](ctx context.Context, q queryer, query string, args ...any) (T, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return zero[T](), err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero[T](), err
		}
		return zero[T](), sql.ErrNoRows
	}

	t, err := scan[T](rows)
	if err != nil {
		return zero[T](), err
	}
	if err := rows.Err(); err != nil {
		return zero[T](), err
	}

	return t, nil
}

func zero[T any]() (t T) { return t }

type rows interface {
	Columns() ([]string, error)
	Scan(...any) error
}

func scan[T any](rows rows) (T, error) {
	var t T
	v := reflect.ValueOf(&t).Elem()
	if v.Kind() != reflect.Struct {
		panic("queries: T must be a struct")
	}

	columns, err := rows.Columns()
	if err != nil {
		return zero[T](), fmt.Errorf("getting column names: %w", err)
	}

	indexes := parseStruct(v.Type())
	args := make([]any, len(columns))

	for i, column := range columns {
		idx, ok := indexes[column]
		if !ok {
			panic(fmt.Sprintf("queries: no field for column %q", column))
		}
		args[i] = v.Field(idx).Addr().Interface()
	}
	if err := rows.Scan(args...); err != nil {
		return zero[T](), err
	}

	return t, nil
}

var cache sync.Map // map[reflect.Type]map[string]int

// parseStruct parses the given struct type and returns a map of column names to field indexes.
// The result is cached, so each struct type is parsed only once.
func parseStruct(t reflect.Type) map[string]int {
	if m, ok := cache.Load(t); ok {
		return m.(map[string]int)
	}

	indexes := make(map[string]int, t.NumField())

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		column, ok := field.Tag.Lookup("sql")
		if !ok {
			continue
		}
		if column == "" {
			panic(fmt.Sprintf("queries: field %s has an empty `sql` tag", field.Name))
		}

		indexes[column] = i
	}

	cache.Store(t, indexes)
	return indexes
}
