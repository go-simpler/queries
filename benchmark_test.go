package queries

import "testing"

func Benchmark_scanSmallWithCache(b *testing.B)     { benchmarkScan[small](b, true) }
func Benchmark_scanSmallWithoutCache(b *testing.B)  { benchmarkScan[small](b, false) }
func Benchmark_scanMediumWithCache(b *testing.B)    { benchmarkScan[medium](b, true) }
func Benchmark_scanMediumWithoutCache(b *testing.B) { benchmarkScan[medium](b, false) }
func Benchmark_scanLargeWithCache(b *testing.B)     { benchmarkScan[large](b, true) }
func Benchmark_scanLargeWithoutCache(b *testing.B)  { benchmarkScan[large](b, false) }

func benchmarkScan[T dst](b *testing.B, cache bool) {
	useCache = cache

	var t T
	columns := t.columns()
	s := mockScanner{values: t.values()}

	b.ResetTimer()
	b.ReportAllocs()
	// TODO: use b.Loop() instead when Go 1.24 becomes oldstable.
	for range b.N {
		_, _ = scan[T](&s, columns)
	}
}

type dst interface {
	columns() []string
	values() []any
}

type small struct {
	A int `sql:"a"`
	B int `sql:"b"`
}

func (small) columns() []string { return []string{"a", "b"} }
func (small) values() []any     { return []any{1, 2} }

type medium struct {
	A int `sql:"a"`
	B int `sql:"b"`
	C int `sql:"c"`
	D int `sql:"d"`
	E int `sql:"e"`
	F int `sql:"f"`
	G int `sql:"g"`
	H int `sql:"h"`
}

func (medium) columns() []string { return []string{"a", "b", "c", "d", "e", "f", "g", "h"} }
func (medium) values() []any     { return []any{1, 2, 3, 4, 5, 6, 7, 8} }

type large struct {
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

func (large) columns() []string {
	return []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}
}

func (large) values() []any {
	return []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
}
