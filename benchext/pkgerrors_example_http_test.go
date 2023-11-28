package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"braces.dev/errtrace/internal/tracetest"
	"github.com/pkg/errors"
)

func Example_http() {
	tp := &http.Transport{Dial: rateLimitDialer}
	client := &http.Client{Transport: tp}
	ps := &PackageStore{
		client: client,
	}

	_, err := ps.Get()

	// Unwrap the HTTP-wrapped error, so we can print a proper stacktrace.
	var stErr interface {
		error
		StackTrace() errors.StackTrace
	}
	if errors.As(err, &stErr) {
		err = stErr
	}

	fmt.Printf("Error fetching packages: %s\n", cleanGoRoot(tracetest.MustClean(fmt.Sprintf("%+v", err))))
	// Output:
	// Error fetching packages: connect rate limited
	// braces.dev/errtrace/benchext.rateLimitDialer
	// 	/path/to/errtrace/benchext/pkgerrors_example_http_test.go:1
	// net/http.(*Transport).dial
	// 	/goroot/src/net/http/transport.go:0
	// net/http.(*Transport).dialConn
	// 	/goroot/src/net/http/transport.go:0
	// net/http.(*Transport).dialConnFor
	// 	/goroot/src/net/http/transport.go:0
	// runtime.goexit
	// 	/goroot/src/runtime/asm_amd64.s:0
}

type PackageStore struct {
	client         *http.Client
	packagesCached []string
}

func (ps *PackageStore) Get() ([]string, error) {
	if ps.packagesCached != nil {
		return ps.packagesCached, nil
	}

	packages, err := ps.updateIndex()
	if err != nil {
		return nil, err
	}

	ps.packagesCached = packages
	return packages, nil
}

func (ps *PackageStore) updateIndex() ([]string, error) {
	resp, err := ps.client.Get("http://example.com/packages.index")
	if err != nil {
		return nil, err
	}

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return strings.Split(string(contents), ","), nil
}

func rateLimitDialer(network, addr string) (net.Conn, error) {
	// for testing, always return an error.
	return nil, errors.Errorf("connect rate limited")
}
