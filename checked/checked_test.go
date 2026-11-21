package checked

import (
	"math"
	"math/big"
	"testing"
)

type bounds struct {
	minimum int64
	maximum int64
}

func (bounds bounds) fits(value int64) bool {
	return value >= bounds.minimum && value <= bounds.maximum
}

func TestOperationsMatchWideArithmeticForEveryInt8Pair(t *testing.T) {
	operationsExhaustiveCheck[int8](t, bounds{math.MinInt8, math.MaxInt8})
}

func TestOperationsMatchWideArithmeticForEveryUint8Pair(t *testing.T) {
	operationsExhaustiveCheck[uint8](t, bounds{0, math.MaxUint8})
}

func operationsExhaustiveCheck[T Integer](t *testing.T, limits bounds) {
	t.Helper()
	for left := limits.minimum; left <= limits.maximum; left++ {
		for right := limits.minimum; right <= limits.maximum; right++ {
			ringCheck[T](t, limits, left, right)
		}
	}
}

func ringCheck[T Integer](t *testing.T, limits bounds, left int64, right int64) {
	t.Helper()
	sum, ok := Add(T(left), T(right))
	if sum != T(left+right) || ok != limits.fits(left+right) {
		t.Fatalf("Add(%d, %d) = %d, %v", left, right, sum, ok)
	}
	difference, ok := Subtract(T(left), T(right))
	if difference != T(left-right) || ok != limits.fits(left-right) {
		t.Fatalf("Subtract(%d, %d) = %d, %v", left, right, difference, ok)
	}
	product, ok := Multiply(T(left), T(right))
	if product != T(left*right) || ok != limits.fits(left*right) {
		t.Fatalf("Multiply(%d, %d) = %d, %v", left, right, product, ok)
	}
}

func TestConvertKeepsExactlyTheValuesThatFitForEvery16BitValue(t *testing.T) {
	for value := int64(math.MinInt16); value <= math.MaxUint16; value++ {
		if value <= math.MaxInt16 {
			convertCheck[int8](t, int16(value), bounds{math.MinInt8, math.MaxInt8})
			convertCheck[uint8](t, int16(value), bounds{0, math.MaxUint8})
			convertCheck[uint16](t, int16(value), bounds{0, math.MaxUint16})
		}
		if value >= 0 {
			convertCheck[int8](t, uint16(value), bounds{math.MinInt8, math.MaxInt8})
			convertCheck[uint8](t, uint16(value), bounds{0, math.MaxUint8})
			convertCheck[int16](t, uint16(value), bounds{math.MinInt16, math.MaxInt16})
		}
	}
}

func convertCheck[To Integer, From Integer](t *testing.T, value From, limits bounds) {
	t.Helper()
	converted, ok := Convert[To](value)
	wide := int64(value)
	if ok != limits.fits(wide) {
		t.Fatalf("Convert(%d) to %T reports %v", value, converted, ok)
	}
	if ok && int64(converted) != wide {
		t.Fatalf("Convert(%d) to %T = %d", value, converted, converted)
	}
}

func TestConvertRejectsTheEdgesOf64Bits(t *testing.T) {
	if _, ok := Convert[int64](uint64(math.MaxUint64)); ok {
		t.Error("the largest uint64 fits in an int64")
	}
	if _, ok := Convert[int64](uint64(math.MaxInt64) + 1); ok {
		t.Error("2^63 fits in an int64")
	}
	if _, ok := Convert[uint64](int64(-1)); ok {
		t.Error("-1 fits in a uint64")
	}
	if _, ok := Convert[uint](int(math.MinInt)); ok {
		t.Error("the smallest int fits in a uint")
	}
	if value, ok := Convert[uint64](int64(math.MaxInt64)); !ok || value != math.MaxInt64 {
		t.Errorf("the largest int64 does not survive in a uint64: %d, %v", value, ok)
	}
}

func FuzzInt64Operations(f *testing.F) {
	for _, seed := range [][2]int64{{math.MinInt64, -1}, {math.MaxInt64, 1}, {math.MinInt64, 1}, {-7, 2}, {7, -2}} {
		f.Add(seed[0], seed[1])
	}
	limits := [2]*big.Int{big.NewInt(math.MinInt64), big.NewInt(math.MaxInt64)}
	f.Fuzz(func(t *testing.T, left int64, right int64) {
		operationsBigCheck(t, left, right, limits)
	})
}

func FuzzUint64Operations(f *testing.F) {
	for _, seed := range [][2]uint64{{math.MaxUint64, 1}, {0, 1}, {1 << 32, 1 << 32}, {7, 2}} {
		f.Add(seed[0], seed[1])
	}
	limits := [2]*big.Int{big.NewInt(0), new(big.Int).SetUint64(math.MaxUint64)}
	f.Fuzz(func(t *testing.T, left uint64, right uint64) {
		operationsBigCheck(t, left, right, limits)
	})
}

func operationsBigCheck[T Integer](t *testing.T, left T, right T, limits [2]*big.Int) {
	t.Helper()
	wideLeft, wideRight := bigOf(left), bigOf(right)
	sum, sumOK := Add(left, right)
	difference, differenceOK := Subtract(left, right)
	product, productOK := Multiply(left, right)
	results := []bigResult[T]{
		{"Add", sum, sumOK, new(big.Int).Add(wideLeft, wideRight)},
		{"Subtract", difference, differenceOK, new(big.Int).Sub(wideLeft, wideRight)},
		{"Multiply", product, productOK, new(big.Int).Mul(wideLeft, wideRight)},
	}
	for _, result := range results {
		fits := result.want.Cmp(limits[0]) >= 0 && result.want.Cmp(limits[1]) <= 0
		if result.ok != fits || (result.ok && bigOf(result.value).Cmp(result.want) != 0) {
			t.Fatalf("%s(%d, %d) = %d, %v, want %s", result.name, left, right, result.value, result.ok, result.want)
		}
	}
}

type bigResult[T Integer] struct {
	name  string
	value T
	ok    bool
	want  *big.Int
}

func bigOf[T Integer](value T) *big.Int {
	if value < 0 {
		return big.NewInt(int64(value))
	}
	return new(big.Int).SetUint64(uint64(value))
}
