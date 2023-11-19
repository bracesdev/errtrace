package errtrace

import (
	"fmt"
	"runtime"

	"braces.dev/errtrace/internal/pc"
)

// Wrap adds information about the program counter of the caller to the error.
// This is intended to be used at all return points in a function.
// If err is nil, Wrap returns nil.
//
//go:noinline since unsafe GetCaller requires a stack and the address of the first arg.
func Wrap(err error) error {
	if err == nil {
		return nil
	}

	return wrap(err, pc.GetCaller())
}

func GetCallerSafe() uintptr {
	const skip = 1 + // frame for Callers
		1 + // frame for GetCaller
		1 // frame for our caller, which should be errtrace.Wrap

	var callers [1]uintptr
	n := runtime.Callers(3, callers[:]) // skip getcallerpc + caller
	if n == 0 {
		return 0
	}
	return callers[0]
}

func printFrame(pc uintptr) {
	frames := runtime.CallersFrames([]uintptr{pc})
	f, _ := frames.Next()
	fmt.Printf("  FILELINE  %v\n    %v:%v\n", f.Function, f.File, f.Line)

}

// Wrap2 is used to `Wrap` the last error return when returning 2 values.
//
//go:inline
func Wrap2[T any](t T, err error) (T, error) {
	if err == nil {
		return t, nil
	}

	return t, wrap(err, pc.GetCaller())
}

// Wrap3 is used to `Wrap` the last error return when returning 3 values.
//
//go:noinline for unsafe GetCaller (see `Wrap` for details).
func Wrap3[T1, T2 any](t1 T1, t2 T2, err error) (T1, T2, error) {
	if err == nil {
		return t1, t2, nil
	}

	return t1, t2, wrap(err, pc.GetCaller())
}

// Wrap4 is used to `Wrap` the last error return when returning 4 values.
//
//go:noinline for unsafe GetCaller (see `Wrap` for details).
func Wrap4[T1, T2, T3 any](t1 T1, t2 T2, t3 T3, err error) (T1, T2, T3, error) {
	if err == nil {
		return t1, t2, t3, nil
	}

	return t1, t2, t3, wrap(err, pc.GetCaller())
}

// Wrap5 is used to `Wrap` the last error return when returning 5 values.
//
//go:noinline for unsafe GetCaller (see `Wrap` for details).
func Wrap5[T1, T2, T3, T4 any](t1 T1, t2 T2, t3 T3, t4 T4, err error) (T1, T2, T3, T4, error) {
	if err == nil {
		return t1, t2, t3, t4, nil
	}

	return t1, t2, t3, t4, wrap(err, pc.GetCaller())
}

// Wrap6 is used to `Wrap` the last error return when returning 6 values.
//
//go:noinline for unsafe GetCaller (see `Wrap` for details).
func Wrap6[T1, T2, T3, T4, T5 any](t1 T1, t2 T2, t3 T3, t4 T4, t5 T5, err error) (T1, T2, T3, T4, T5, error) {
	if err == nil {
		return t1, t2, t3, t4, t5, nil
	}

	return t1, t2, t3, t4, t5, wrap(err, pc.GetCaller())
}
