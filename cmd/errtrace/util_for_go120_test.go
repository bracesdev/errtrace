//go:build go1.20 && !go1.21

package main

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
