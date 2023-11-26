package errtrace_test

import (
	"errors"
	"fmt"
	"strings"

	"braces.dev/errtrace"
	"braces.dev/errtrace/internal/tracetest"
)

func normalErr(i int) error {
	return fmt.Errorf("std err %v", i)
}

func wrapNormalErr(i int) error {
	return errtrace.Wrap(normalErr(i))
}

func nestedErrorList(i int) error {
	return errors.Join(
		normalErr(i),
		wrapNormalErr(i+1),
	)
}

func Example_tree() {
	errs := errtrace.Wrap(errors.Join(
		normalErr(1),
		wrapNormalErr(2),
		nestedErrorList(3),
	))
	got := errtrace.FormatString(errs)

	// make trace agnostic to environment-specific location
	// and less sensitive to line number changes.
	fmt.Println(trimTrailingSpaces(tracetest.MustClean(got)))

	// Output:
	// +- std err 1
	// |
	// +- std err 2
	// |
	// |  braces.dev/errtrace_test.wrapNormalErr
	// |  	/path/to/errtrace/example_tree_test.go:1
	// |
	// |  +- std err 3
	// |  |
	// |  +- std err 4
	// |  |
	// |  |  braces.dev/errtrace_test.wrapNormalErr
	// |  |  	/path/to/errtrace/example_tree_test.go:1
	// |  |
	// +- std err 3
	// |  std err 4
	// |
	// std err 1
	// std err 2
	// std err 3
	// std err 4
	//
	// braces.dev/errtrace_test.Example_tree
	// 	/path/to/errtrace/example_tree_test.go:2
}

func trimTrailingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}
