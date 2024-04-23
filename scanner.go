package queries

import (
	"errors"
	"fmt"
	"reflect"
)

// TODO: consider merging ScanOne() + ScanAll() -> Scan().

type Rows interface {
	Scan(...any) error
	Columns() ([]string, error)
	Next() bool
	Err() error
}

func ScanOne(dst any, rows Rows) error {
	v := reflect.ValueOf(dst)
	if !v.IsValid() || v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct || v.IsNil() {
		panic("queries: dst must be a non-nil struct pointer")
	}

	fields := parseStruct(v.Elem())

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("getting column names: %w", err)
	}

	target := make([]any, len(columns))
	for i, column := range columns {
		field, ok := fields[column]
		if !ok {
			panic(fmt.Sprintf("queries: no field for the %#q column", column))
		}
		target[i] = field
	}

	if !rows.Next() {
		return errors.New("queries: no rows to scan")
	}
	if err := rows.Scan(target...); err != nil {
		return fmt.Errorf("scanning rows: %w", err)
	}

	return rows.Err()
}

func ScanAll(dst any, rows Rows) error {
	v := reflect.ValueOf(dst)
	if !v.IsValid() || v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Slice || v.Elem().Type().Elem().Kind() != reflect.Struct {
		panic("queries: dst must be a pointer to a slice of structs")
	}

	slice := v.Elem()
	typ := slice.Type().Elem()
	elem := reflect.New(typ).Elem()
	fields := parseStruct(elem)

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("getting column names: %w", err)
	}

	target := make([]any, len(columns))
	for i, column := range columns {
		field, ok := fields[column]
		if !ok {
			panic(fmt.Sprintf("queries: no field for the %#q column", column))
		}
		target[i] = field
	}

	for rows.Next() {
		if err := rows.Scan(target...); err != nil {
			return fmt.Errorf("scanning rows: %w", err)
		}
		slice.Set(reflect.Append(slice, elem))
	}

	return rows.Err()
}

// TODO: support nested structs.
func parseStruct(v reflect.Value) map[string]any {
	fields := make(map[string]any, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		sf := v.Type().Field(i)
		name, ok := sf.Tag.Lookup("sql")
		if !ok {
			continue
		}
		if name == "" {
			panic(fmt.Sprintf("queries: %s field has an empty `sql` tag", sf.Name))
		}

		fields[name] = field.Addr().Interface()
	}

	return fields
}
