//go:build !go1.21

package diff

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
