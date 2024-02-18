//go:build go1.21

package main

import "slices"

func slicesContains[T comparable](s []T, find T) bool {
	return slices.Contains[[]T](s, find)
}
