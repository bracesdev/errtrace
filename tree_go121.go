//go:build go1.21

package errtrace

import "slices"

func sliceReverse[T any](s []T) {
	slices.Reverse(s)
}
