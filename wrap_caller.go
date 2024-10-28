package errtrace

import "braces.dev/errtrace/internal/pc"

// Caller represents a single caller frame, intended for ...
type Caller struct {
	callerPC uintptr
}

// GetCaller captures the program counter of a caller, primarily intended for
// error helpers so caller information captures the helper's caller.
//
// Note: Callers of this function should be marked using `//go:noinline`
// to avoid inlining, as GetCaller expects to skip the caller's stack frame.
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
