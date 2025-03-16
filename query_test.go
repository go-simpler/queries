package queries

import (
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

	t.Run("ok", func(t *testing.T) {
		rows := mockRows{
			columns: []string{"foo", "bar"},
			values:  []any{1, "A"},
		}

		type row struct {
			Foo int    `sql:"foo"`
			Bar string `sql:"bar"`
		}
		r, err := scan[row](&rows)
		assert.NoErr[F](t, err)
		assert.Equal[E](t, r.Foo, 1)
		assert.Equal[E](t, r.Bar, "A")
	})
}

type mockRows struct {
	columns []string
	values  []any
}

func (r *mockRows) Columns() ([]string, error) { return r.columns, nil }

func (r *mockRows) Scan(dst ...any) error {
	for i := 0; i < len(dst); i++ {
		v := reflect.ValueOf(r.values[i])
		reflect.ValueOf(dst[i]).Elem().Set(v)
	}
	return nil
}
