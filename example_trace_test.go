package errtrace_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"braces.dev/errtrace"
	"braces.dev/errtrace/internal/tracetest"
)

func f1() error {
	return errtrace.Wrap(f2())
}

func f2() error {
	return errtrace.Wrap(f3())
}

func f3() error {
	return errtrace.New("failed")
}

func Example_trace() {
	got := errtrace.FormatString(f1())

	// make trace agnostic to environment-specific location
	// and less sensitive to line number changes.
	fmt.Println(tracetest.MustClean(got))

	// Output:
	//failed
	//
	//braces.dev/errtrace_test.f3
	//	/path/to/errtrace/example_trace_test.go:3
	//braces.dev/errtrace_test.f2
	//	/path/to/errtrace/example_trace_test.go:2
	//braces.dev/errtrace_test.f1
	//	/path/to/errtrace/example_trace_test.go:1
}

func f4() error {
	return errtrace.Wrap(fmt.Errorf("wrapped: %w", f1()))
}

func ExampleUnwrapFrame() {
	var frames []runtime.Frame
	current := f4()
	for current != nil {
		frame, inner, ok := errtrace.UnwrapFrame(current)
		if !ok {
			// If the error is not wrapped with errtrace,
			// unwrap it directly with errors.Unwrap.
			current = errors.Unwrap(current)
			continue
			// Note that this example does not handle multi-errors,
			// for example those returned by errors.Join.
			// To handle those, this loop would need to also check
			// for the 'Unwrap() []error' method on the error.
		}
		frames = append(frames, frame)
		current = inner
	}

	var trace strings.Builder
	for _, frame := range frames {
		fmt.Fprintf(&trace, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
	}
	fmt.Println(tracetest.MustClean(trace.String()))

	// Output:
	//
	//braces.dev/errtrace_test.f4
	//	/path/to/errtrace/example_trace_test.go:4
	//braces.dev/errtrace_test.f1
	//	/path/to/errtrace/example_trace_test.go:1
	//braces.dev/errtrace_test.f2
	//	/path/to/errtrace/example_trace_test.go:2
	//braces.dev/errtrace_test.f3
	//	/path/to/errtrace/example_trace_test.go:3
}
