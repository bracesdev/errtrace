//go:build go1.21

package errtrace

import (
	"bytes"
	"log/slog"
)

// LogValue implements the [slog.LogValuer] interface.
func (e *errTrace) LogValue() slog.Value {
	var outb bytes.Buffer
	err := writeTree(&outb, buildTraceTree(e))
	if err != nil {
		return slog.GroupValue(
			slog.String("message", e.Error()),
			slog.Any("formatErr", err),
		)
	}

	return slog.StringValue(outb.String())
}

// ErrAttr is a helper to convert an error to a slog Attr
// Usage:
//
//	slog.Default().Error("msg here", errtrace.ErrAttr(err))
func ErrAttr(err error) slog.Attr {
	return slog.Any("err", err)
}
