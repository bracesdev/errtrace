//go:build !safe && (arm64 || amd64)

package pc

func GetCaller() uintptr
