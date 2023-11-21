//go:build ignore

package foo

import "example.com/bar"

func hasTwo() (int, error) {
	return bar.Two()
}

func hasThree() (string, int, error) {
	return bar.Three()
}

func hasFour() (string, int, bool, error) {
	return bar.Four()
}

func hasFive() (a int, b bool, c string, d int, e error) {
	return bar.Five()
}

func hasSix() (a int, b bool, c string, d int, e bool, f error) {
	return bar.Six()
}

func hasSeven() (a int, b bool, c string, d int, e bool, f string, g error) {
	return bar.Seven() // want:"skipping function with too many return values"
}

func multipleErrors() (x int, err1, err2 error) {
	return bar.MultipleErrors() // want:"skipping function with multiple error returns"
}
