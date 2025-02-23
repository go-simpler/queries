package queries_test

import (
	"database/sql"
	"reflect"
	"testing"

	"go-simpler.org/queries"
	"go-simpler.org/queries/internal/assert"
	. "go-simpler.org/queries/internal/assert/EF"
)

func Test_misuse(t *testing.T) {
	t.Run("non-struct T", func(t *testing.T) {
		const panicMsg = "queries: T must be a struct"

		assert.Panics[E](t, func() { _ = queries.Scan(new([]int), nil) }, panicMsg)
		assert.Panics[E](t, func() { _ = queries.ScanRow(new(int), nil) }, panicMsg)
	})

	t.Run("empty tag", func(t *testing.T) {
		const panicMsg = "queries: field Foo has an empty `sql` tag"

		type dst struct {
			Foo int `sql:""`
		}
		assert.Panics[E](t, func() { _ = queries.Scan(new([]dst), nil) }, panicMsg)
		assert.Panics[E](t, func() { _ = queries.ScanRow(new(dst), nil) }, panicMsg)
	})

	t.Run("missing field", func(t *testing.T) {
		const panicMsg = `queries: no field for column "foo"`

		rows := mockRows{columns: []string{"foo"}}

		type dst struct {
			Foo int
		}
		assert.Panics[E](t, func() { _ = queries.Scan(new([]dst), &rows) }, panicMsg)
		assert.Panics[E](t, func() { _ = queries.ScanRow(new(dst), &rows) }, panicMsg)
	})
}

func TestScan(t *testing.T) {
	rows := mockRows{
		columns: []string{"foo", "bar"},
		values:  [][]any{{1, "A"}, {2, "B"}},
	}

	var dst []struct {
		Foo int    `sql:"foo"`
		Bar string `sql:"bar"`
	}
	err := queries.Scan(&dst, &rows)
	assert.NoErr[F](t, err)
	assert.Equal[E](t, len(dst), 2)
	assert.Equal[E](t, dst[0].Foo, 1)
	assert.Equal[E](t, dst[1].Foo, 2)
	assert.Equal[E](t, dst[0].Bar, "A")
	assert.Equal[E](t, dst[1].Bar, "B")
}

func TestScanRow(t *testing.T) {
	rows := mockRows{
		columns: []string{"foo", "bar"},
		values:  [][]any{{1, "A"}},
	}

	var dst struct {
		Foo int    `sql:"foo"`
		Bar string `sql:"bar"`
	}
	err := queries.ScanRow(&dst, &rows)
	assert.NoErr[F](t, err)
	assert.Equal[E](t, dst.Foo, 1)
	assert.Equal[E](t, dst.Bar, "A")

	t.Run("no rows", func(t *testing.T) {
		rows := mockRows{columns: []string{"foo"}}

		var dst struct {
			Foo int `sql:"foo"`
		}
		err := queries.ScanRow(&dst, &rows)
		assert.IsErr[E](t, err, sql.ErrNoRows)
	})
}

type mockRows struct {
	columns []string
	values  [][]any
	idx     int
}

func (r *mockRows) Columns() ([]string, error) { return r.columns, nil }
func (r *mockRows) Next() bool                 { return r.idx < len(r.values) }
func (r *mockRows) Err() error                 { return nil }

func (r *mockRows) Scan(dst ...any) error {
	for i := 0; i < len(dst); i++ {
		v := reflect.ValueOf(r.values[r.idx][i])
		reflect.ValueOf(dst[i]).Elem().Set(v)
	}
	r.idx++
	return nil
}
