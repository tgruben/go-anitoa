package itoa

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
)

const (
	printerIterations = 10000
)

func testPrinter(t *testing.T, fn func(out []byte) (value interface{}, result string)) {
	rand.Seed(0)

	buf := make([]byte, 20)
	for i := 0; i < printerIterations; i++ {
		value, actual := fn(buf)
		expected := fmt.Sprintf("%d", value)

		if string(expected) != string(actual) {
			t.Errorf("Expected %q, got %q", expected, actual)
		}
	}
}

func TestItoaHundred(t *testing.T) {
	testPrinter(t, func(out []byte) (value interface{}, result string) {
		v := uint64(rand.Intn(100))
		return v, FormatUint(v)
	})
}

func TestItoaTenThousand(t *testing.T) {
	testPrinter(t, func(out []byte) (value interface{}, result string) {
		v := uint64(rand.Intn(10000))
		return v, FormatUint(v)
	})
}

func TestUint(t *testing.T) {
	testPrinter(t, func(out []byte) (value interface{}, result string) {
		v := rand.Uint64() >> uint(rand.Intn(64))
		return v, FormatUint(v)
	})
}
func TestInt(t *testing.T) {
	testPrinter(t, func(out []byte) (value interface{}, result string) {
		v := rand.Int63() >> uint(rand.Intn(64))
		if rand.Intn(1) == 1 {
			v = -v
		}

		return v, FormatInt(v)
	})
}

var smallInt = 35
var bigInt = 999999999999999

func BenchmarkItoa(b *testing.B) {
	for i := 0; i < b.N; i++ {
		val := strconv.Itoa(smallInt)
		_ = val
	}
}

func BenchmarkItoaBig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		val := strconv.Itoa(bigInt)
		_ = val
	}
}

func BenchmarkAnItoa(b *testing.B) {
	buf := make([]byte, 80)
	for i := 0; i < b.N; i++ {
		val := Anltoa(buf, uint64(smallInt))
		_ = val
	}
}

func BenchmarkAnItoaBig(b *testing.B) {
	buf := make([]byte, 80)
	for i := 0; i < b.N; i++ {
		val := Anltoa(buf, uint64(bigInt))
		_ = val
	}
}

func BenchmarkFmt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		val := fmt.Sprintf("%d", smallInt)
		_ = val
	}
}

func BenchmarkFmtBig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		val := fmt.Sprintf("%d", bigInt)
		_ = val
	}
}
