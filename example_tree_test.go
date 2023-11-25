package errtrace_test

import (
	"errors"
	"fmt"
	"strings"

	"braces.dev/errtrace"
	"braces.dev/errtrace/internal/tracetest"
)

func normalErr() error {
	return errors.New("std err")
}

func wrapNormalErr() error {
	return errtrace.Wrap(normalErr())
}

func nestedErrorList() error {
	return errors.Join(
		normalErr(),
		wrapNormalErr(),
	)
}

func Example_tree() {
	errs := errtrace.Wrap(errors.Join(
		normalErr(),
		wrapNormalErr(),
		nestedErrorList(),
	))
	got := errtrace.FormatString(errs)

	// make trace agnostic to environment-specific location
	// and less sensitive to line number changes.
	fmt.Println(trimTrailingSpaces(tracetest.MustClean(got)))

	// Output:
	// +- std err
	// |
	// +- std err
	// |
	// |  braces.dev/errtrace_test.wrapNormalErr
	// |  	/path/to/errtrace/example_tree_test.go:1
	// |
	// |  +- std err
	// |  |
	// |  +- std err
	// |  |
	// |  |  braces.dev/errtrace_test.wrapNormalErr
	// |  |  	/path/to/errtrace/example_tree_test.go:1
	// |  |
	// +- std err
	// |  std err
	// |
	// std err
	// std err
	// std err
	// std err
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
