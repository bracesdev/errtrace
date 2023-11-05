//go:build safe || !(amd64 || 386)

package pc

import "runtime"

// GetCaller gets the caller's PC.
func GetCaller(unused int) uintptr {
	var callers [1]uintptr
	n := runtime.Callers(2, callers[:]) // skip getcallerpc + caller
	if n == 0 {
		return 0
	}
	return callers[0]
}
