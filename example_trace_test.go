package errtrace_test

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
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

func ExampleLogAttr() {
	// This example demonstrates use of the LogAttr function.
	// The LogAttr function always uses the "error" key.
	logger := newExampleLogger()

	if err := f1(); err != nil {
		logger.Error("f1 failed", errtrace.LogAttr(err))
	}

	// Output:
	// {"level":"ERROR","msg":"f1 failed","error":"failed\n\nbraces.dev/errtrace_test.f3\n\t/Users/abg/src/braces.dev/errtrace/example_trace_test.go:24\nbraces.dev/errtrace_test.f2\n\t/Users/abg/src/braces.dev/errtrace/example_trace_test.go:20\nbraces.dev/errtrace_test.f1\n\t/Users/abg/src/braces.dev/errtrace/example_trace_test.go:16\n"}
}

func ExampleLogAttr_noTrace() {
	// LogAttr reports the original error message
	// if the error does not have a trace attached to it.
	logger := newExampleLogger()

	if err := errors.New("no trace"); err != nil {
		logger.Error("something broke", errtrace.LogAttr(err))
	}

	// Output:
	// {"level":"ERROR","msg":"something broke","error":"no trace"}
}

func Example_logWithSlog() {
	// This example demonstrates how to log an errtrace-wrapped error
	// with the slog package.
	// Unlike LogAttr, we're able to use any key name here.
	logger := newExampleLogger()

	if err := f1(); err != nil {
		logger.Error("f1 failed", "my-error", err)
	}

	// Output:
	// {"level":"ERROR","msg":"f1 failed","my-error":"failed\n\nbraces.dev/errtrace_test.f3\n\t/Users/abg/src/braces.dev/errtrace/example_trace_test.go:24\nbraces.dev/errtrace_test.f2\n\t/Users/abg/src/braces.dev/errtrace/example_trace_test.go:20\nbraces.dev/errtrace_test.f1\n\t/Users/abg/src/braces.dev/errtrace/example_trace_test.go:16\n"}
}

// newExampleLogger creates a new slog.Logger for use in examples.
// It omits timestamps from the output to allow for output matching.
func newExampleLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
}
