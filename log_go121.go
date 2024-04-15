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

// LogAttr builds a slog attribute for an error.
// It will log the error with an error trace
// if the error has been wrapped with this package.
// Otherwise, the error message will be logged as is.
//
// Usage:
//
//	slog.Default().Error("msg here", errtrace.LogAttr(err))
func LogAttr(err error) slog.Attr {
	return slog.Any("err", err)
}
