package queries

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"sync"
	"time"
)

// Queryer is an interface implemented by [sql.DB] and [sql.Tx].
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Query executes a query that returns rows, scans each row into a T, and returns an iterator over the Ts.
// If an error occurs, the iterator yields it as the second value, and the caller should then stop the iteration.
// [Queryer] can be either [sql.DB] or [sql.Tx], the rest of the arguments are passed directly to [Queryer.QueryContext].
// Query fully manages the lifecycle of the [sql.Rows] returned by [Queryer.QueryContext], so the caller does not have to.
//
// The following Ts are supported:
//   - int (any kind)
//   - uint (any kind)
//   - float (any kind)
//   - bool
//   - string
//   - time.Time
//   - [sql.Scanner] (implemented by [sql.Null] types)
//   - any struct
//
// See the [sql.Rows.Scan] documentation for the scanning rules.
// If the query has multiple columns, T must be a struct, other types can only be used for single-column queries.
// The fields of a struct T must have the `sql:"COLUMN"` tag, where COLUMN is the name of the corresponding column in the query.
// Untagged and unexported and fields are ignored.
//
// If the caller prefers the result to be a slice rather than an iterator, Query can be combined with [Collect].
func Query[T any](ctx context.Context, q Queryer, query string, args ...any) iter.Seq2[T, error] {
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

// QueryRow is a [Query] variant for queries that are expected to return at most one row,
// so instead of an iterator, it returns a single T.
// Like [sql.DB.QueryRow], QueryRow returns [sql.ErrNoRows] if the query selects no rows,
// otherwise it scans the first row and discards the rest.
// See the [Query] documentation for details on supported Ts.
func QueryRow[T any](ctx context.Context, q Queryer, query string, args ...any) (T, error) {
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

// Collect is a [slices.Collect] variant that collects values from an iter.Seq2[T, error].
// If an error occurs during the collection, Collect stops the iteration and returns the error.
func Collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var ts []T
	for t, err := range seq {
		if err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, nil
}

func zero[T any]() (t T) { return t }

type scanner interface {
	Scan(...any) error
}

var (
	errNoColumns     = errors.New("queries: no columns in the query")
	errNonStructT    = errors.New("queries: T must be a struct if len(columns) > 1")
	errNoStructField = errors.New("queries: no struct field for the column")
	errUnsupportedT  = errors.New("queries: unsupported T")
)

func scan[T any](s scanner, columns []string) (T, error) {
	if len(columns) == 0 {
		return zero[T](), errNoColumns
	}

	var t T
	v := reflect.ValueOf(&t).Elem()
	args := make([]any, len(columns))

	switch {
	case scannable(v):
		if len(columns) > 1 {
			return zero[T](), errNonStructT
		}
		args[0] = v.Addr().Interface()
	case v.Kind() == reflect.Struct:
		indexes := parseStruct(v.Type())
		for i, column := range columns {
			idx, ok := indexes[column]
			if !ok {
				return zero[T](), fmt.Errorf("%w %q", errNoStructField, column)
			}
			args[i] = v.Field(idx).Addr().Interface()
		}
	default:
		return zero[T](), fmt.Errorf("%w %T", errUnsupportedT, t)
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

var (
	useCache = true
	cache    sync.Map // map[reflect.Type]map[string]int
)

// parseStruct parses the given struct type and returns a map of column names to field indexes.
// The result is cached, so each struct type is parsed only once.
func parseStruct(t reflect.Type) map[string]int {
	if useCache {
		if m, ok := cache.Load(t); ok {
			return m.(map[string]int)
		}
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
			continue
		}
		indexes[column] = i
	}

	if useCache {
		cache.Store(t, indexes)
	}
	return indexes
}
