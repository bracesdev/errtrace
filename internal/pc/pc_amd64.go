//go:build !safe && amd64

package pc

import "unsafe"

// GetCaller gets the caller's PC.
//
//go:inline
func GetCaller[T any](firstArgAddr *T) uintptr {
	// PC is stored 8 bytes below first parameter, see:
	// https://github.com/golang/go/blob/d72f4542fea6c2724a253a8322bc8aeed637021e/src/cmd/compile/internal/amd64/ssa.go#L1088
	return derefAddr(unsafe.Pointer(firstArgAddr), 8)
}
