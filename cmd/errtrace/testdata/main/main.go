package main

import (
	"fmt"

	"braces.dev/errtrace/cmd/errtrace/testdata/main/foo"
)

func main() {
	if err := foo.Foo(); err != nil {
		fmt.Printf("%+v\n", err)
	}
}
