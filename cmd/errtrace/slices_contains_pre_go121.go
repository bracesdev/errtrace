//go:build !go1.21

package main

func slicesContains[T comparable](s []T, find T) bool {
	for _, v := range s {
		if v == find {
			return true
		}
	}
	return false
}
