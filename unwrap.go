package errtrace

import "runtime"

// UnwrapFrame unwraps the outermost frame from the given error,
// returning it and the inner error.
// ok is true if the frame was successfully extracted,
// and false otherwise, or if the error is not an errtrace error.
//
// You can use this for structured access to trace information.
func UnwrapFrame(err error) (frame runtime.Frame, inner error, ok bool) { //nolint:revive // error is intentionally middle return
	e, ok := err.(*errTrace)
	if !ok {
		return runtime.Frame{}, err, false
	}

	frames := runtime.CallersFrames([]uintptr{e.pc})
	f, _ := frames.Next()
	if f == (runtime.Frame{}) {
		// Unlikely, but if PC didn't yield a frame,
		// just return the inner error.
		return runtime.Frame{}, e.err, false
	}

	return f, e.err, true
}