//go:build !safe && (arm64 || amd64)

// Package pc provides access to the program counter
// to determine the caller of a function.
package pc

// GetCaller returns the program counter of the caller's caller.
func GetCaller() uintptr

// GetCallerSkip1 is similar to GetCaller, but skips an additional caller.
func GetCallerSkip1() uintptr
