package errtrace

import "braces.dev/errtrace/internal/pc"

// Caller represents a single caller frame, and is intended for error helpers
// to capture caller information for wrapping. See [GetCaller] for details.
type Caller struct {
	callerPC uintptr
}

// GetCaller captures the program counter of a caller, primarily intended for
// error helpers so caller information captures the helper's caller.
//
// Callers of this function should be marked '//go:noinline' to avoid inlining,
// as GetCaller expects to skip the caller's stack frame.
//
//	//go:noinline
//	func Wrapf(err error, msg string, args ...any) {
//		caller := errtrace.GetCaller()
//		err := ...
//		return caller.Wrap(err)
//	}
//
//go:noinline
func GetCaller() Caller {
	return Caller{pc.GetCallerSkip1()}
}

// Wrap adds the program counter captured in Caller to the error,
// similar to [Wrap], but relying on previously captured caller inforamtion.
func (c Caller) Wrap(err error) error {
	return wrap(err, c.callerPC)
}
