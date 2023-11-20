//go:build !safe && (arm64 || amd64)

package pc

func getcallerpc() uintptr

// GetCaller gets the caller's PC.
//
//go:inline
func GetCaller() uintptr {
	return getcallerpc()
}
