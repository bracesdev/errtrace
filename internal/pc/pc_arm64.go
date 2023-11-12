//go:build !safe && arm64

package pc

func getcallerpc() uintptr

// GetCaller gets the caller's PC.
//
//go:inline
func GetCaller[T any](firstArgAddr *T) uintptr {
	return getcallerpc()
}
