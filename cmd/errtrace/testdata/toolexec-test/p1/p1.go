package p1

import (
	"fmt"

	"braces.dev/errtrace/cmd/errtrace/testdata/toolexec-test/p2"
)

// WrapP2OnlyErr only returns the error from WrapP2.
func WrapP2OnlyErr() error {
	if _, err := WrapP2(); err != nil {
		return fmt.Errorf("test2: %w", err) // @unsafe-trace
	}
	return nil
}

// WrapRet2 calls WrapP2, but has a multi-return.
func WrapP2() (string, error) {
	return p2.CallP3() // @unsafe-trace
}
