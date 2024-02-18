package p2

import (
	"braces.dev/errtrace"

	"braces.dev/errtrace/cmd/errtrace/testdata/toolexec-test/p3"
)

// CallP3 calls p3, and wraps the error.
func CallP3() error {
	return errtrace.Wrap(p3.ReturnErr()) // @trace
}
