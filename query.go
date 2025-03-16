package queries

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"reflect"
)

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

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

	fields := parseStruct(v)
	args := make([]any, len(columns))

	for i, column := range columns {
		field, ok := fields[column]
		if !ok {
			panic(fmt.Sprintf("queries: no field for column %q", column))
		}
		args[i] = field
	}
	if err := rows.Scan(args...); err != nil {
		return zero[T](), err
	}

	return t, nil
}

// TODO: add sync.Map cache.
func parseStruct(v reflect.Value) map[string]any {
	fields := make(map[string]any, v.NumField())

	for i := range v.NumField() {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		tag, ok := v.Type().Field(i).Tag.Lookup("sql")
		if !ok {
			continue
		}
		if tag == "" {
			panic(fmt.Sprintf("queries: field %s has an empty `sql` tag", v.Type().Field(i).Name))
		}

		fields[tag] = field.Addr().Interface()
	}

	return fields
}
