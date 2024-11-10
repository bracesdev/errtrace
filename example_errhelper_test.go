package errtrace_test

import (
	"errors"
	"fmt"

	"braces.dev/errtrace"
	"braces.dev/errtrace/internal/tracetest"
)

func f1Wrap() error {
	return wrap(f2Wrap(), "u=1")
}

func f2Wrap() error {
	return wrap(f3Wrap(), "method=order")
}

func f3Wrap() error {
	return wrap(errors.New("failed"), "foo")
}

func wrap(err error, fields ...string) error {
	return errtrace.GetCaller().
		Wrap(fmt.Errorf("%w %v", err, fields))
}

func Example_getCaller() {
	got := errtrace.FormatString(f1Wrap())

	// make trace agnostic to environment-specific location
	// and less sensitive to line number changes.
	fmt.Println(tracetest.MustClean(got))

	// Output:
	//failed [foo] [method=order] [u=1]
	//
	//braces.dev/errtrace_test.f3Wrap
	//	/path/to/errtrace/example_errhelper_test.go:3
	//braces.dev/errtrace_test.f2Wrap
	//	/path/to/errtrace/example_errhelper_test.go:2
	//braces.dev/errtrace_test.f1Wrap
	//	/path/to/errtrace/example_errhelper_test.go:1
}
