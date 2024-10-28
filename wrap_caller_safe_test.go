//go:build safe || !(amd64 || arm64)

//
// Build tag must match pc_safe.go

package errtrace_test

func init() {
	safe = true
}
