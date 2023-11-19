// Package errtrace provides the ability to track a Return Trace for errors.
// This differs from a stack trace in that
// it is not a snapshot of the call stack at the time of the error,
// but rather a trace of the path taken by the error as it was returned
// until it was finally handled.
package errtrace

import (
	"errors"
	"fmt"
	"io"
	"runtime"
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
	// Same format as tracebacks:
	//
	// functionName
	// 	file:line
	Frames(target)(func(f Frame) bool {
		_, err = fmt.Fprintf(w, "%s\n\t%s:%d\n", f.Func, f.File, f.Line)
		return err == nil
	})
	return err
}

func FormatString(target error) string {
	var s strings.Builder
	_ = Format(&s, target)
	return s.String()
}

type Frame struct {
	File string
	Line int
	Func string // fully qualified function name
}

func Frames(target error) func(yield func(Frame) bool) bool {
	return func(yield func(Frame) bool) bool {
		var tr *errTrace
		for ; errors.As(target, &tr); target = tr.err {
			frames := runtime.CallersFrames([]uintptr{tr.pc})

			for {
				f, more := frames.Next()
				if f == (runtime.Frame{}) {
					break
				}

				frame := Frame{
					File: f.File,
					Line: f.Line,
					Func: f.Function,
				}
				if !yield(frame) {
					return false
				}

				if !more {
					break
				}
			}
		}
		return true
	}
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
