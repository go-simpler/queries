package queries

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"go-simpler.org/queries/internal/assert"
	. "go-simpler.org/queries/internal/assert/EF"
)

func Test_scan(t *testing.T) {
	t.Run("no columns", func(t *testing.T) {
		fn := func() { _, _ = scan[int](nil, nil) }
		assert.Panics[E](t, fn, "queries: no columns specified")
	})

	t.Run("unsupported T", func(t *testing.T) {
		columns := []string{"foo", "bar"}

		fn := func() { _, _ = scan[complex64](nil, columns) }
		assert.Panics[E](t, fn, "queries: unsupported T complex64")
	})

	t.Run("non-struct T with len(columns) > 1", func(t *testing.T) {
		columns := []string{"foo", "bar"}

		fn := func() { _, _ = scan[int](nil, columns) }
		assert.Panics[E](t, fn, "queries: T must be a struct if len(columns) > 1")
	})

	t.Run("empty tag", func(t *testing.T) {
		columns := []string{"foo", "bar"}

		type row struct {
			Foo int    `sql:"foo"`
			Bar string `sql:""`
		}
		fn := func() { _, _ = scan[row](nil, columns) }
		assert.Panics[E](t, fn, "queries: field Bar has an empty `sql` tag")
	})

	t.Run("missing field", func(t *testing.T) {
		columns := []string{"foo", "bar"}

		type row struct {
			Foo int `sql:"foo"`
			Bar string
		}
		fn := func() { _, _ = scan[row](nil, columns) }
		assert.Panics[E](t, fn, `queries: no field for column "bar"`)
	})

	t.Run("scan error", func(t *testing.T) {
		columns := []string{"foo", "bar"}
		s := mockScanner{err: errors.New("an error")}

		type row struct {
			Foo int    `sql:"foo"`
			Bar string `sql:"bar"`
		}
		_, err := scan[row](&s, columns)
		assert.IsErr[E](t, err, s.err)
	})

	t.Run("struct T", func(t *testing.T) {
		columns := []string{"foo", "bar"}
		s := mockScanner{values: []any{1, "test"}}

		type row struct {
			Foo        int    `sql:"foo"`
			Bar        string `sql:"bar"`
			unexported bool
		}
		v, err := scan[row](&s, columns)
		assert.NoErr[F](t, err)
		assert.Equal[E](t, v.Foo, 1)
		assert.Equal[E](t, v.Bar, "test")
		assert.Equal[E](t, v.unexported, false)
	})

	t.Run("struct T with len(columns) == 1", func(t *testing.T) {
		columns := []string{"foo"}
		s := mockScanner{values: []any{1}}

		type row struct {
			Foo int `sql:"foo"`
		}
		v, err := scan[row](&s, columns)
		assert.NoErr[F](t, err)
		assert.Equal[E](t, v.Foo, 1)
	})

	t.Run("non-struct T with len(columns) == 1", func(t *testing.T) {
		columns := []string{"foo"}

		tests := []struct {
			scan  func(scanner) (any, error)
			value any
		}{
			{func(s scanner) (any, error) { return scan[bool](s, columns) }, true},
			{func(s scanner) (any, error) { return scan[int](s, columns) }, int(-1)},
			{func(s scanner) (any, error) { return scan[int8](s, columns) }, int8(-8)},
			{func(s scanner) (any, error) { return scan[int16](s, columns) }, int16(-16)},
			{func(s scanner) (any, error) { return scan[int32](s, columns) }, int32(-32)},
			{func(s scanner) (any, error) { return scan[int64](s, columns) }, int64(-64)},
			{func(s scanner) (any, error) { return scan[uint](s, columns) }, uint(1)},
			{func(s scanner) (any, error) { return scan[uint8](s, columns) }, uint8(8)},
			{func(s scanner) (any, error) { return scan[uint16](s, columns) }, uint16(16)},
			{func(s scanner) (any, error) { return scan[uint32](s, columns) }, uint32(32)},
			{func(s scanner) (any, error) { return scan[uint64](s, columns) }, uint64(64)},
			{func(s scanner) (any, error) { return scan[float32](s, columns) }, float32(0.32)},
			{func(s scanner) (any, error) { return scan[float64](s, columns) }, float64(0.64)},
			{func(s scanner) (any, error) { return scan[string](s, columns) }, "test"},
			{func(s scanner) (any, error) { return scan[time.Time](s, columns) }, time.Now()},
		}
		for _, tt := range tests {
			s := mockScanner{values: []any{tt.value}}
			v, err := tt.scan(&s)
			assert.NoErr[F](t, err)
			assert.Equal[E](t, v, tt.value)
		}

		// sql.Scanner implementation:
		s := mockScanner{values: []any{"test"}}
		v, err := scan[sql.Null[string]](&s, columns)
		assert.NoErr[F](t, err)
		assert.Equal[E](t, v, sql.Null[string]{V: "test", Valid: true})
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
		if sc, ok := dst[i].(sql.Scanner); ok {
			if err := sc.Scan(s.values[i]); err != nil {
				return err
			}
		} else {
			v := reflect.ValueOf(s.values[i])
			reflect.ValueOf(dst[i]).Elem().Set(v)
		}
	}
	return nil
}
