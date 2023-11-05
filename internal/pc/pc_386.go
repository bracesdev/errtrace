//go:build !safe && 386

package pc

import "unsafe"

// GetCaller gets the caller's PC.
//
//go:noinline
func GetCaller[T any](firstArgAddr *T) uintptr {
	// PC is stored 4 bytes below first parameter, see:
	// https://github.com/golang/go/blob/d72f4542fea6c2724a253a8322bc8aeed637021e/src/cmd/compile/internal/x86/ssa.go#L722
	return derefAddr(unsafe.Pointer(firstArgAddr), 4)
}
