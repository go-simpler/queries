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
		fn := func() { _, _ = scan[int](nil, nil) }
		assert.Panics[E](t, fn, "queries: T must be a struct")
	})

	t.Run("empty tag", func(t *testing.T) {
		type row struct {
			Foo int `sql:""`
		}
		fn := func() { _, _ = scan[row](nil, nil) }
		assert.Panics[E](t, fn, "queries: field Foo has an empty `sql` tag")
	})

	t.Run("missing field", func(t *testing.T) {
		type row struct {
			Foo int `sql:"foo"`
			Bar string
		}
		fn := func() { _, _ = scan[row](nil, []string{"foo", "bar"}) }
		assert.Panics[E](t, fn, `queries: no field for column "bar"`)
	})

	t.Run("scan error", func(t *testing.T) {
		columns := []string{"foo"}
		s := mockScanner{err: errors.New("an error")}

		type row struct {
			Foo int `sql:"foo"`
		}
		_, err := scan[row](&s, columns)
		assert.IsErr[E](t, err, s.err)
	})

	t.Run("ok", func(t *testing.T) {
		columns := []string{"foo", "bar"}
		s := mockScanner{values: []any{1, "A"}}

		type row struct {
			Foo        int    `sql:"foo"`
			Bar        string `sql:"bar"`
			unexported bool
		}
		r, err := scan[row](&s, columns)
		assert.NoErr[F](t, err)
		assert.Equal[E](t, r.Foo, 1)
		assert.Equal[E](t, r.Bar, "A")
		assert.Equal[E](t, r.unexported, false)
	})
}

type mockScanner struct {
	values []any
	err    error
}

func (s *mockScanner) Scan(dst ...any) error {
	if s.err != nil {
		return s.err
	}
	for i := range dst {
		v := reflect.ValueOf(s.values[i])
		reflect.ValueOf(dst[i]).Elem().Set(v)
	}
	return nil
}
