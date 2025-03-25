package queries

import (
	"errors"
	"reflect"
	"testing"

	"go-simpler.org/queries/internal/assert"
	. "go-simpler.org/queries/internal/assert/EF"
)

func Test_scan(t *testing.T) {
	t.Run("non-struct T", func(t *testing.T) {
		fn := func() { _, _ = scan[int](new(mockRows)) }
		assert.Panics[E](t, fn, "queries: T must be a struct")
	})

	t.Run("empty tag", func(t *testing.T) {
		type row struct {
			Foo int `sql:""`
		}
		fn := func() { _, _ = scan[row](new(mockRows)) }
		assert.Panics[E](t, fn, "queries: field Foo has an empty `sql` tag")
	})

	t.Run("missing field", func(t *testing.T) {
		rows := mockRows{
			columns: []string{"foo", "bar"},
		}

		type row struct {
			Foo int `sql:"foo"`
			Bar string
		}
		fn := func() { _, _ = scan[row](&rows) }
		assert.Panics[E](t, fn, `queries: no field for column "bar"`)
	})

	t.Run("columns error", func(t *testing.T) {
		someErr := errors.New("")

		rows := mockRows{
			columnsErr: someErr,
		}

		type row struct {
			Foo int `sql:"foo"`
		}
		_, err := scan[row](&rows)
		assert.IsErr[E](t, err, someErr)
	})

	t.Run("scan error", func(t *testing.T) {
		someErr := errors.New("")

		rows := mockRows{
			columns: []string{"foo"},
			scanErr: someErr,
		}

		type row struct {
			Foo int `sql:"foo"`
		}
		_, err := scan[row](&rows)
		assert.IsErr[E](t, err, someErr)
	})

	t.Run("ok", func(t *testing.T) {
		rows := mockRows{
			columns: []string{"foo", "bar"},
			values:  []any{1, "A"},
		}

		type row struct {
			Foo        int    `sql:"foo"`
			Bar        string `sql:"bar"`
			unexported bool
		}
		r, err := scan[row](&rows)
		assert.NoErr[F](t, err)
		assert.Equal[E](t, r.Foo, 1)
		assert.Equal[E](t, r.Bar, "A")
		assert.Equal[E](t, r.unexported, false)
	})
}

type mockRows struct {
	columns    []string
	values     []any
	columnsErr error
	scanErr    error
}

func (r *mockRows) Columns() ([]string, error) {
	if r.columnsErr != nil {
		return nil, r.columnsErr
	}
	return r.columns, nil
}

func (r *mockRows) Scan(dst ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i := range dst {
		v := reflect.ValueOf(r.values[i])
		reflect.ValueOf(dst[i]).Elem().Set(v)
	}
	return nil
}
