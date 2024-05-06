package errtrace

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnwrapFrame(t *testing.T) {
	giveErr := errors.New("great sadness")

	t.Run("not wrapped", func(t *testing.T) {
		_, inner, ok := UnwrapFrame(giveErr)
		if got, want := ok, false; got != want {
			t.Errorf("ok: got %v, want %v", got, want)
		}

		if got, want := inner, giveErr; got != want {
			t.Errorf("inner: got %v, want %v", inner, giveErr)
		}
	})

	t.Run("wrapped", func(t *testing.T) {
		wrapped := Wrap(giveErr)
		frame, inner, ok := UnwrapFrame(wrapped)
		if got, want := ok, true; got != want {
			t.Errorf("ok: got %v, want %v", got, want)
		}

		if got, want := inner, giveErr; got != want {
			t.Errorf("inner: got %v, want %v", inner, giveErr)
		}

		if got, want := frame.Function, ".TestUnwrapFrame.func2"; !strings.HasSuffix(got, want) {
			t.Errorf("frame.Func: got %q, does not contain %q", got, want)
		}

		if got, want := filepath.Base(frame.File), "unwrap_test.go"; got != want {
			t.Errorf("frame.File: got %v, want %v", got, want)
		}
	})
}

func TestUnwrapFrame_badPC(t *testing.T) {
	giveErr := errors.New("great sadness")
	_, inner, ok := UnwrapFrame(wrap(giveErr, 0))
	if got, want := ok, false; got != want {
		t.Errorf("ok: got %v, want %v", got, want)
	}

	if got, want := inner, giveErr; got != want {
		t.Errorf("inner: got %v, want %v", inner, giveErr)
	}
}
