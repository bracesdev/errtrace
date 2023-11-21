//go:build ignore

package foo

import "strconv"

func Unwrapped(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return i + 42, nil
}

func Parse(s string) (int, error) {
	return strconv.Atoi(s)
}
