package itoa

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	_ "github.com/pilosa/pilosa/test"
)

func TestHello(t *testing.T) {
	buf := make([]byte, 8)
	Anltoa(buf, 10000000)
	if strings.Compare(string(buf), string("10000000")) != 0 {
		t.Errorf("\n(%s)\n(%s)", len(string(buf)), len("10000000"))
	}
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
