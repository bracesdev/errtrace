//go:build safe || !(amd64 || 386)

package pc

import "runtime"

// GetCaller gets the caller's caller's PC.
//
// Note: firstArgAddr is used with unsafe, it's ignored for safe.
func GetCaller[T any](firstArgAddr *T) uintptr {
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
