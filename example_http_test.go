package errtrace_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"braces.dev/errtrace"
	"braces.dev/errtrace/internal/tracetest"
)

func Example_http() {
	tp := &http.Transport{Dial: rateLimitDialer}
	client := &http.Client{Transport: tp}
	ps := &PackageStore{
		client: client,
	}

	_, err := ps.Get()
	fmt.Printf("Error fetching packages: %+v\n", tracetest.MustClean(errtrace.FormatString(err)))
	// Output:
	// Error fetching packages: Get "http://example.com/packages.index": connect rate limited
	//
	// braces.dev/errtrace_test.rateLimitDialer
	// 	/path/to/errtrace/example_http_test.go:3
	// braces.dev/errtrace_test.(*PackageStore).updateIndex
	// 	/path/to/errtrace/example_http_test.go:2
	// braces.dev/errtrace_test.(*PackageStore).Get
	// 	/path/to/errtrace/example_http_test.go:1
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
		return nil, errtrace.Wrap(err)
	}

	ps.packagesCached = packages
	return packages, nil
}

func (ps *PackageStore) updateIndex() ([]string, error) {
	resp, err := ps.client.Get("http://example.com/packages.index")
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	return strings.Split(string(contents), ","), nil
}

func rateLimitDialer(network, addr string) (net.Conn, error) {
	// for testing, always return an error.
	return nil, errtrace.New("connect rate limited")
}
