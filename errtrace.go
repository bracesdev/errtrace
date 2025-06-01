// Package errtrace provides the ability to track a return trace for errors.
// This differs from a stack trace in that
// it is not a snapshot of the call stack at the time of the error,
// but rather a trace of the path taken by the error as it was returned
// until it was finally handled.
//
// # Wrapping errors
//
// Use the [Wrap] function at a return site
// to annotate it with the position of the return.
//
//	// Before
//	if err != nil {
//		return err
//	}
//
//	// After
//	if err != nil {
//		return errtrace.Wrap(err)
//	}
//
// # Formatting return traces
//
// errtrace provides the [Format] and [FormatString] functions
// to obtain the return trace of an error.
//
//	errtrace.Format(os.Stderr, err)
//
// See [Format] for details of the output format.
//
// Additionally, errors returned by errtrace will also print a trace
// if formatted with the %+v verb when used with a Printf-style function.
//
//	log.Printf("error: %+v", err)
//
// # Unwrapping errors
//
// Use the [UnwrapFrame] function to unwrap a single frame from an error.
//
//	for err != nil {
//		frame, inner, ok := errtrace.UnwrapFrame(err)
//		if !ok {
//			break // end of trace
//		}
//		printFrame(frame)
//		err = inner
//	}
//
// See the [UnwrapFrame] example test for a more complete example.
//
// # See also
//
// https://github.com/bracesdev/errtrace.
package errtrace

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

var _arena = newArena[errTrace](1024)

func wrap(err error, callerPC uintptr) error {
	et := _arena.Take()
	et.err = err
	et.pc = callerPC
	return et
}

// Format writes the return trace for given error to the writer.
// The output takes a format similar to the following:
//
//	<error message>
//
//	<function>
//		<file>:<line>
//	<caller of function>
//		<file>:<line>
//	[...]
//
// Any error that has a method `TracePC() uintptr` will
// contribute to the trace.
// If the error doesn't have a return trace attached to it,
// only the error message is reported.
// If the error is comprised of multiple errors (e.g. with [errors.Join]),
// the return trace of each error is reported as a tree.
//
// Returns an error if the writer fails.
func Format(w io.Writer, target error) (err error) {
	return writeTree(w, buildTraceTree(target))
}

// FormatString writes the return trace for err to a string.
// Any error that has a method `TracePC() uintptr` will
// contribute to the trace.
// See [Format] for details of the output format.
func FormatString(target error) string {
	var s strings.Builder
	_ = Format(&s, target)
	return s.String()
}

// LogAttr builds a slog attribute for an error with the key "error".
//
// When serialized with a slog-based logger,
// this will report an error return trace if the error has one,
// otherwise the original error message will be logged as-is.
func LogAttr(err error) slog.Attr {
	return slog.Any("error", err)
}

type errTrace struct {
	err error
	pc  uintptr
}

func (e *errTrace) Error() string {
	return e.err.Error()
}

func (e *errTrace) Unwrap() error {
	return e.err
}

func (e *errTrace) Format(s fmt.State, verb rune) {
	if verb == 'v' && s.Flag('+') {
		_ = Format(s, e)
		return
	}

	fmt.Fprintf(s, fmt.FormatString(s, verb), e.err)
}

// LogValue implements the [slog.LogValuer] interface.
func (e *errTrace) LogValue() slog.Value {
	return slog.StringValue(FormatString(e))
}

// TracePC returns the program counter for the location
// in the frame that the error originated with.
//
// The returned PC is intended to be used with
// runtime.CallersFrames or runtime.FuncForPC
// to aid in generating the error return trace
func (e *errTrace) TracePC() uintptr {
	return e.pc
}

// compile time tracePCprovider interface check
var _ interface{ TracePC() uintptr } = &errTrace{}
