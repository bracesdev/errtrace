package errtrace

import (
	"fmt"
	"runtime"
)

// Frame is a single frame in an error trace
// identifying a site where an error was wrapped.
type Frame struct {
	// Function is the fully qualified function name
	// inside which the error was wrapped.
	Function string

	// File is the file inside which the function is defined.
	File string

	// Line is the line number inside the file
	// where the error was wrapped.
	Line int
}

func (f Frame) String() string {
	return fmt.Sprintf("%s (%s:%d)", f.Function, f.File, f.Line)
}

// UnwrapFrame unwraps the outermost frame from the given error,
// returning it and the inner error.
// ok is true if the frame was successfully extracted,
// and false otherwise, or if the error is not an errtrace error.
//
// You can use this for structured access to trace information.
func UnwrapFrame(err error) (frame Frame, inner error, ok bool) { //nolint:revive // error is intentionally middle return
	e, ok := err.(*errTrace)
	if !ok {
		return Frame{}, err, false
	}

	frames := runtime.CallersFrames([]uintptr{e.pc})
	f, _ := frames.Next()
	if f == (runtime.Frame{}) {
		// Unlikely, but if PC didn't yield a frame,
		// just return the inner error.
		return Frame{}, e.err, false
	}

	return Frame{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}, e.err, true
}
