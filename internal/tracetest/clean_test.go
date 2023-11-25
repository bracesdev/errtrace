package tracetest

import (
	"strings"
	"testing"

	"braces.dev/errtrace"
)

func TestClean_RealTrace(t *testing.T) {
	e1 := errtrace.Wrap(f1())
	// dummy comments to offset line numbers by > 1
	//
	// dummy line to make line numbers offset by > 1
	e2 := errtrace.Wrap(e1)
	//
	e3 := errtrace.Wrap(e2)

	want := strings.Join([]string{
		"braces.dev/errtrace/internal/tracetest.TestClean_RealTrace",
		"	/path/to/errtrace/internal/tracetest/clean_test.go:3",
		"braces.dev/errtrace/internal/tracetest.TestClean_RealTrace",
		"	/path/to/errtrace/internal/tracetest/clean_test.go:2",
		"braces.dev/errtrace/internal/tracetest.TestClean_RealTrace",
		"	/path/to/errtrace/internal/tracetest/clean_test.go:1",
		"braces.dev/errtrace/internal/tracetest.f1",
		"	/path/to/errtrace/internal/tracetest/clean_2_test.go:1",
		"braces.dev/errtrace/internal/tracetest.f2",
		"	/path/to/errtrace/internal/tracetest/clean_2_test.go:2",
		"braces.dev/errtrace/internal/tracetest.f3",
		"	/path/to/errtrace/internal/tracetest/clean_2_test.go:3",
		"",
	}, "\n")

	if got := MustClean(errtrace.FormatString(e3)); want != got {
		t.Errorf("want:\n%v\ngot:\n%v\n", want, got)
	}
}
