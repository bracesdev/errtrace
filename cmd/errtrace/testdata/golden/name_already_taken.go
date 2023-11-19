//go:build ignore

package foo

import "strconv"

func Unwrapped(errtrace string) (int, error) {
	// For some reason, the string is named errtrace.
	// Don't think about it too hard.
	i, err := strconv.Atoi(errtrace)
	return i + 42, err
}
