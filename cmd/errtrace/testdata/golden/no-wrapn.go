//go:build ignore

// @runIf options=no-wrapn
package foo

import "example.com/bar"

func hasTwo() (int, error) {
	// Same names as used by rewriting, with different types to verify scoping.
	r1 := true
	r2 := false
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
	return bar.Seven()
}

func nonFinalError() (error, bool) {
	return bar.NonFinalError() // want:"skipping function with non-final error return"
}

func multipleErrors() (x int, err1, err2 error) {
	return bar.MultipleErrors() // want:"skipping function with multiple error returns"
}

func invalid() (x int, err error) {
	return 42 // want:"skipping function with incorrect number of return values: got 1, want 2"
}

func nestedExpressions() (int, error) {
	return func() (int, error) {
		r1 := true
		r2 := false
		return bar.Two()
	}()
}
