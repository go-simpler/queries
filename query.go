package queries

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"reflect"
	"sync"
	"time"
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

		columns, err := rows.Columns()
		if err != nil {
			yield(zero[T](), err)
			return
		}

		for rows.Next() {
			t, err := scan[T](rows, columns)
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

	columns, err := rows.Columns()
	if err != nil {
		return zero[T](), err
	}

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero[T](), err
		}
		return zero[T](), sql.ErrNoRows
	}

	t, err := scan[T](rows, columns)
	if err != nil {
		return zero[T](), err
	}
	if err := rows.Err(); err != nil {
		return zero[T](), err
	}

	return t, nil
}

func zero[T any]() (t T) { return t }

type scanner interface {
	Scan(...any) error
}

func scan[T any](s scanner, columns []string) (T, error) {
	if len(columns) == 0 {
		panic("queries: no columns specified") // valid in PostgreSQL (for some reason).
	}

	var t T
	v := reflect.ValueOf(&t).Elem()
	args := make([]any, len(columns))

	switch {
	case scannable(v):
		if len(columns) > 1 {
			panic("queries: T must be a struct if len(columns) > 1")
		}
		args[0] = v.Addr().Interface()
	case v.Kind() == reflect.Struct:
		indexes := parseStruct(v.Type())
		for i, column := range columns {
			idx, ok := indexes[column]
			if !ok {
				panic(fmt.Sprintf("queries: no field for column %q", column))
			}
			args[i] = v.Field(idx).Addr().Interface()
		}
	default:
		panic(fmt.Sprintf("queries: unsupported T %T", t))
	}

	if err := s.Scan(args...); err != nil {
		return zero[T](), err
	}

	return t, nil
}

func scannable(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return true
	}
	if v.Type() == reflect.TypeFor[time.Time]() {
		return true
	}
	if v.Addr().Type().Implements(reflect.TypeFor[sql.Scanner]()) {
		return true
	}
	return false
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
