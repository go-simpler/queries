package queries

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"go-simpler.org/queries/queriestest"
)

func BenchmarkQuery_withScanner(b *testing.B) {
	db := newDB(b)
	b.ReportAllocs()
	for b.Loop() {
		for range Query[mediumRow](b.Context(), db, "") {
		}
	}
}

func BenchmarkQuery_withoutScanner(b *testing.B) {
	db := newDB(b)
	b.ReportAllocs()
	for b.Loop() {
		rows, _ := db.QueryContext(b.Context(), "")
		for rows.Next() {
			var row mediumRow
			_ = rows.Scan(&row.A, &row.B, &row.C, &row.D, &row.E, &row.F, &row.G, &row.H)
		}
		_ = rows.Close()
	}
}

func BenchmarkQueryRow_withScanner(b *testing.B) {
	db := newDB(b)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = QueryRow[mediumRow](b.Context(), db, "")
	}
}

func BenchmarkQueryRow_withoutScanner(b *testing.B) {
	db := newDB(b)
	b.ReportAllocs()
	for b.Loop() {
		var row mediumRow
		_ = db.QueryRowContext(b.Context(), "").
			Scan(&row.A, &row.B, &row.C, &row.D, &row.E, &row.F, &row.G, &row.H)
	}
}

func newDB(tb testing.TB) *sql.DB {
	return queriestest.NewDB(tb, queriestest.Driver{
		QueryContext: func(testing.TB, string, []any) (driver.Rows, error) {
			return queriestest.NewRows("a", "b", "c", "d", "e", "f", "g", "h").
				Add(1, 2, 3, 4, 5, 6, 7, 8).
				Add(1, 2, 3, 4, 5, 6, 7, 8).
				Add(1, 2, 3, 4, 5, 6, 7, 8).
				Add(1, 2, 3, 4, 5, 6, 7, 8).
				Add(1, 2, 3, 4, 5, 6, 7, 8).
				Add(1, 2, 3, 4, 5, 6, 7, 8).
				Add(1, 2, 3, 4, 5, 6, 7, 8).
				Add(1, 2, 3, 4, 5, 6, 7, 8), nil
		},
	})
}

func Benchmark_scan_smallRowWithCache(b *testing.B)     { benchmarkScan[smallRow](b, true) }
func Benchmark_scan_smallRowWithoutCache(b *testing.B)  { benchmarkScan[smallRow](b, false) }
func Benchmark_scan_mediumRowWithCache(b *testing.B)    { benchmarkScan[mediumRow](b, true) }
func Benchmark_scan_mediumRowWithoutCache(b *testing.B) { benchmarkScan[mediumRow](b, false) }
func Benchmark_scan_largeRowWithCache(b *testing.B)     { benchmarkScan[largeRow](b, true) }
func Benchmark_scan_largeRowWithoutCache(b *testing.B)  { benchmarkScan[largeRow](b, false) }

func benchmarkScan[T dst](b *testing.B, cache bool) {
	useCache = cache

	var t T
	columns := t.columns()
	s := mockScanner{values: t.values()}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = scan[T](&s, columns)
	}
}

type dst interface {
	columns() []string
	values() []any
}

type smallRow struct {
	A int `sql:"a"`
	B int `sql:"b"`
	C int `sql:"c"`
	D int `sql:"d"`
}

func (smallRow) columns() []string { return []string{"a", "b", "c", "d"} }
func (smallRow) values() []any     { return []any{1, 2, 3, 4} }

type mediumRow struct {
	A int `sql:"a"`
	B int `sql:"b"`
	C int `sql:"c"`
	D int `sql:"d"`
	E int `sql:"e"`
	F int `sql:"f"`
	G int `sql:"g"`
	H int `sql:"h"`
}

func (mediumRow) columns() []string { return []string{"a", "b", "c", "d", "e", "f", "g", "h"} }
func (mediumRow) values() []any     { return []any{1, 2, 3, 4, 5, 6, 7, 8} }

type largeRow struct {
	A int `sql:"a"`
	B int `sql:"b"`
	C int `sql:"c"`
	D int `sql:"d"`
	E int `sql:"e"`
	F int `sql:"f"`
	G int `sql:"g"`
	H int `sql:"h"`
	I int `sql:"i"`
	J int `sql:"j"`
	K int `sql:"k"`
	L int `sql:"l"`
	M int `sql:"m"`
	N int `sql:"n"`
	O int `sql:"o"`
	P int `sql:"p"`
}

func (largeRow) columns() []string {
	return []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}
}

func (largeRow) values() []any {
	return []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
}
