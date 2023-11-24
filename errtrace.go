// Package errtrace provides the ability to track a Return Trace for errors.
// This differs from a stack trace in that
// it is not a snapshot of the call stack at the time of the error,
// but rather a trace of the path taken by the error as it was returned
// until it was finally handled.
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

// Format writes the return trace for err to the writer.
//
// An error is returned if the writer returns an error.
func Format(w io.Writer, target error) (err error) {
	return writeTree(w, buildTraceTree(target))
}

// FormatString writes the return trace for err to a string.
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
		io.WriteString(s, e.Error())
		io.WriteString(s, "\n")
		_ = Format(s, e)
		return
	}

	fmt.Fprintf(s, fmt.FormatString(s, verb), e.err)
}
