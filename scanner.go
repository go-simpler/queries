package queries

import (
	"database/sql"
	"fmt"
	"reflect"
)

type Rows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(...any) error
	Err() error
}

func Scan[T any](dst *[]T, rows Rows) error {
	return scan[T](reflect.ValueOf(dst).Elem(), rows)
}

func ScanRow[T any](dst *T, rows Rows) error {
	return scan[T](reflect.ValueOf(dst).Elem(), rows)
}

func scan[T any](v reflect.Value, rows Rows) error {
	typ := reflect.TypeFor[T]()
	if typ.Kind() != reflect.Struct {
		panic("queries: T must be a struct")
	}

	strct := reflect.New(typ).Elem()
	fields := parseStruct(strct)

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("getting column names: %w", err)
	}

	into := make([]any, len(columns))
	for i, column := range columns {
		field, ok := fields[column]
		if !ok {
			panic(fmt.Sprintf("queries: no field for column %q", column))
		}
		into[i] = field
	}

	slice := reflect.New(reflect.SliceOf(typ)).Elem()
	for rows.Next() {
		if err := rows.Scan(into...); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}
		slice = reflect.Append(slice, strct)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	switch v.Kind() {
	case reflect.Slice:
		v.Set(slice)
	case reflect.Struct:
		if slice.Len() == 0 {
			return sql.ErrNoRows
		}
		v.Set(slice.Index(0))
	default:
		panic("unreachable")
	}

	return nil
}

func parseStruct(v reflect.Value) map[string]any {
	fields := make(map[string]any, v.NumField())

	for i := 0; i < v.NumField(); i++ {
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
