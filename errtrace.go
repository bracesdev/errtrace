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
//	for {
//		frame, err, ok := errtrace.UnwrapFrame(err)
//		if !ok {
//			break // end of trace
//		}
//		printFrame(frame)
//	}
//
// # See also
//
// https://github.com/bracesdev/errtrace.
package errtrace

import (
	"fmt"
	"io"
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
// The output takes a fromat similar to the following:
//
//	<error message>
//
//	<function>
//		<file>:<line>
//	<caller of function>
//		<file>:<line>
//	[...]
//
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
// See [Format] for details of the output format.
func FormatString(target error) string {
	var s strings.Builder
	_ = Format(&s, target)
	return s.String()
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
