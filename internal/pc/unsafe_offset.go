//go:build !safe

package pc

import "unsafe"

func derefAddr(addr unsafe.Pointer, subOffset uintptr) uintptr {
	addrOffset := unsafe.Pointer((uintptr)(addr) - subOffset)
	addrPtr := (*uintptr)(addrOffset)
	return *addrPtr
}
