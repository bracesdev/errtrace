package pc

import (
	"errors"
	"testing"
)

//go:noinline
func wrap(err error) uintptr {
	return GetCaller(&err)
}

func BenchmarkGetCaller(b *testing.B) {
	err := errors.New("test")

	var last uintptr
	for i := 0; i < b.N; i++ {
		cur := wrap(err)
		if cur == 0 {
			panic("invalid PC")
		}
		if last != 0 && cur != last {
			panic("inconsistent results")
		}
		last = cur
	}
}
