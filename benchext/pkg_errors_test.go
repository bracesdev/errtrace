package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func recurseErrPkgErrors(n int) error {
	if n == 0 {
		return errors.New("error")
	}

	return recurseErrPkgErrors(n - 1)
}

func BenchmarkPkgErrors(b *testing.B) {
	var err error
	for i := 0; i < b.N; i++ {
		err = recurseErrPkgErrors(10)
	}

	if wantMin, got := 10, strings.Count(fmt.Sprintf("%+v", err), "pkg_errors_test.go"); got < wantMin {
		b.Fatalf("missing expected stack frames, expected >%v, got %v", wantMin, got)
	}
}
