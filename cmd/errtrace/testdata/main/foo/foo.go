package foo

import "braces.dev/errtrace"

import "errors"

var e = errtrace.Wrap

func Foo() error {
	return errors.New("test")
}
