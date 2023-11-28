package main

import (
	"regexp"
	"strings"
)

// cleanGoRoot is similar to tracetest, but deals with GOROOT paths.
// It replaces paths+line numbers for fixed values so we can use the
// output in an example test.
func cleanGoRoot(s string) string {
	gorootPath := regexp.MustCompile("/.*/src/")
	s = gorootPath.ReplaceAllString(s, "/goroot/src/")

	fileLine := regexp.MustCompile(`/goroot/.*:[0-9]+`)
	return fileLine.ReplaceAllStringFunc(s, func(path string) string {
		file, _, ok := strings.Cut(path, ":")
		if !ok {
			return path
		}

		return file + ":0"
	})
}
