package errtrace

import "runtime"

// UnwrapFrame unwraps the outermost frame from the given error,
// returning it and the inner error.
// ok is true if the frame was successfully extracted,
// and false otherwise, or if the error is not an errtrace error.
//
// You can use this for structured access to trace information.
func UnwrapFrame(err error) (frame runtime.Frame, inner error, ok bool) { //nolint:revive // error is intentionally middle return
	e, ok := err.(interface{ TracePC() uintptr })
	if !ok {
		return runtime.Frame{}, err, false
	}

	wrapErr := unwrapOnce(err)
	frames := runtime.CallersFrames([]uintptr{e.TracePC()})
	f, _ := frames.Next()
	if f == (runtime.Frame{}) {
		// Unlikely, but if PC didn't yield a frame,
		// just return the inner error.
		return runtime.Frame{}, wrapErr, false
	}

	return f, wrapErr, true
}

// unwrapOnce accesses the direct cause of the error if any, otherwise
// returns nil.
//
// It supports both errors implementing causer (`Cause()` method, from
// github.com/pkg/errors) and `Wrapper` (`Unwrap()` method, from the
// Go 2 error proposal).
func unwrapOnce(err error) error {
	switch e := err.(type) {
	case interface{ Cause() error }:
		return e.Cause()
	case interface{ Unwrap() error }:
		return e.Unwrap()
	}

	return nil
}
