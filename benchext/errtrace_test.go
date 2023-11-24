package main

import (
	"strings"
	"testing"

	"braces.dev/errtrace"
)

func recurseErrtrace(n int) error {
	if n == 0 {
		return errtrace.New("f5 failed")
	}
	return errtrace.Wrap(recurseErrtrace(n - 1))
}

func BenchmarkErrtrace(b *testing.B) {
	var err error
	for i := 0; i < b.N; i++ {
		err = recurseErrtrace(10)
	}

	if wantMin, got := 10, strings.Count(errtrace.FormatString(err), "errtrace_test.go"); got < wantMin {
		b.Fatalf("missing expected stack frames, expected >%v, got %v", wantMin, got)
	}
}
