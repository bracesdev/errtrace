//go:build !safe

package pc

import "unsafe"

func derefAddr(addr unsafe.Pointer, subOffset uintptr) uintptr {
	addrOffset := (uintptr)(addr) - subOffset
	addrPtr := (*uintptr)(unsafe.Pointer(addrOffset))
	return *addrPtr
}
