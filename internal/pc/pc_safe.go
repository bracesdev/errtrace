//go:build safe || !(amd64 || arm64)

package pc

import "runtime"

// GetCaller returns the program counter of the caller's caller.
func GetCaller() uintptr {
	return getCaller(0)
}

// GetCallerSkip1 is similar to GetCaller, but skips an additional caller.
func GetCallerSkip1() uintptr {
	return getCaller(1)
}

func getCaller(skip int) uintptr {
	const baseSkip = 1 + // runtime.Callers
		1 + // getCaller
		1 + // GetCaller or GetCallerSkip1
		1 // errtrace.Wrap, or errtrace.GetCaller

	var callers [1]uintptr
	n := runtime.Callers(baseSkip+skip, callers[:]) // skip getcallerpc + caller
	if n == 0 {
		return 0
	}
	return callers[0]
}
