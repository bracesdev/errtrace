package errtrace

import (
	"fmt"
	"runtime"
)

// Frame is a single frame in an error trace
// identifying a site where an error was wrapped.
type Frame struct {
	// Func is the fully qualified function name
	// inside which the error was wrapped.
	Func string

	// File is the file inside which the function is defined.
	File string

	// Line is the line number inside the file
	// where the error was wrapped.
	Line int
}

func (f Frame) String() string {
	return fmt.Sprintf("%s (%s:%d)", f.Func, f.File, f.Line)
}

// UnwrapFrame unwraps the outermost frame from the given error,
// returning it and the inner error.
// If the outermost error is not an errtrace-wrapped error,
// UnwrapFrame returns (Frame{}, err, false).
//
// You can use this for structured access to trace information.
// For example:
//
//	err := // ..
//	var frames []Frame
//	for {
//		frame, err, ok := UnwrapFrame(err)
//		if !ok {
//			break
//		}
//		frames = append(frames, frame)
//	}
//
// Note that the loop like the above will stop
// when it encounters an error that wasn't wrapped
// with errtrace.Wrap or its friends.
// A fully complete version will also want to handle
// errors that wrap other errors in a different way,
// and multi-errors where each error has its own trace.
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
		Func: f.Function,
		File: f.File,
		Line: f.Line,
	}, e.err, true
}
