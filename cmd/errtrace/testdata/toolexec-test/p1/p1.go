package p1

import (
	"fmt"

	"braces.dev/errtrace/cmd/errtrace/testdata/toolexec-test/p2"
)

// WrapP2 wraps an error return from p2.
func WrapP2() error {
	return fmt.Errorf("test2: %w", p2.ReturnErr())
}
