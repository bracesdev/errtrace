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
