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

// caused follows the pkg/errors Cause interface
type caused struct {
	err error
}

func (e *caused) Error() string {
	return e.err.Error()
}

func (e *caused) Cause() error {
	return e.err
}

func TestUnwrapOnce(t *testing.T) {
	rootErr := New("root")
	var errTrace *errTrace
	errors.As(rootErr, &errTrace)
	unwrapped := errTrace.err
	wrapper := Wrap(rootErr)

	causedErr := &caused{rootErr}

	type want struct {
		wantErr     bool
		matchErr    error
		matchString string
	}
	tests := []struct {
		name string
		arg  error
		want want
	}{
		{
			name: "unwrap wrapped provides root",
			arg:  wrapper,
			want: want{
				wantErr:     true,
				matchErr:    rootErr,
				matchString: "root",
			},
		},
		{
			name: "unwrap root provides unwrapped",
			arg:  rootErr,
			want: want{
				wantErr:     true,
				matchErr:    unwrapped,
				matchString: "root",
			},
		},
		{name: "unwrap nil provides nil"},
		{
			name: "unwrap caused provides root",
			arg:  causedErr,
			want: want{
				wantErr:     true,
				matchErr:    rootErr,
				matchString: "root",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnwrapOnce(tt.arg)
			if (err != nil) != tt.want.wantErr {
				t.Errorf("UnwrapOnce() error = %v, but wantErr %v", err, tt.want.wantErr)
			} else if !errors.Is(err, tt.want.matchErr) {
				t.Errorf("UnwrapOnce() error = %v, does not match matchErr %v", err, tt.want.matchErr)
			}
		})
	}
}
