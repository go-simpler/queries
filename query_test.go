package queries

import (
	"database/sql"
	"errors"
	"iter"
	"reflect"
	"slices"
	"testing"
	"time"

	"go-simpler.org/queries/internal/assert"
	. "go-simpler.org/queries/internal/assert/EF"
)

func TestCollect(t *testing.T) {
	anErr := errors.New("an error")

	tests := map[string]struct {
		seq     iter.Seq2[int, error]
		want    []int
		wantErr error
	}{
		"no error": {slices.All([]error{nil, nil}), []int{0, 1}, nil},
		"an error": {slices.All([]error{nil, anErr}), nil, anErr},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Collect(tt.seq)
			assert.IsErr[F](t, err, tt.wantErr)
			assert.Equal[E](t, got, tt.want)
		})
	}
}

func Test_scan(t *testing.T) {
	t.Run("no columns", func(t *testing.T) {
		_, err := scan[int](nil, []string{})
		assert.IsErr[E](t, err, errNoColumns)
	})

	t.Run("non-struct T with len(columns) > 1", func(t *testing.T) {
		_, err := scan[int](nil, []string{"foo", "bar"})
		assert.IsErr[E](t, err, errNonStructT)
	})

	t.Run("no struct field", func(t *testing.T) {
		_, err := scan[struct{}](nil, []string{"foo", "bar"})
		assert.IsErr[E](t, err, errNoStructField)
	})

	t.Run("unsupported T", func(t *testing.T) {
		_, err := scan[complex64](nil, []string{"foo", "bar"})
		assert.IsErr[E](t, err, errUnsupportedT)
	})

	t.Run("scan error", func(t *testing.T) {
		s := mockScanner{err: errors.New("an error")}

		type row struct {
			Foo int    `sql:"foo"`
			Bar string `sql:"bar"`
		}
		_, err := scan[row](&s, []string{"foo", "bar"})
		assert.IsErr[E](t, err, s.err)
	})

	t.Run("struct T", func(t *testing.T) {
		s := mockScanner{values: []any{1, "test", true}}

		type embedded struct {
			Baz bool `sql:"baz"`
		}
		type row struct {
			embedded
			Foo        int    `sql:"foo"`
			Bar        string `sql:"bar"`
			EmptyTag   string `sql:""`
			Untagged   string
			unexported string
		}
		v, err := scan[row](&s, []string{"foo", "bar", "baz"})
		assert.NoErr[F](t, err)
		assert.Equal[E](t, v.Foo, 1)
		assert.Equal[E](t, v.Bar, "test")
		assert.Equal[E](t, v.Baz, true)
		assert.Equal[E](t, v.EmptyTag, "")
		assert.Equal[E](t, v.Untagged, "")
		assert.Equal[E](t, v.unexported, "")
	})

	t.Run("non-struct T", func(t *testing.T) {
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

// Scan implements [sql.Scanner].
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
