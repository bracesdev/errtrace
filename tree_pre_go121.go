//go:build !go1.21

package errtrace

func sliceReverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[j], s[i] = s[i], s[j]
	}
}
