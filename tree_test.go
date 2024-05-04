package errtrace

import (
	"errors"
	"strings"
	"testing"

	"braces.dev/errtrace/internal/diff"
)

func errorCaller() error {
	return Wrap(errorCallee())
}

func errorCallee() error {
	return New("test error")
}

func errorMultiCaller() error {
	return errors.Join(
		errorCaller(),
		errorCaller(),
	)
}

func TestBuildTreeSingle(t *testing.T) {
	tree := buildTraceTree(errorCaller())
	trace := tree.Trace

	if want, got := 2, len(trace); want != got {
		t.Fatalf("trace length mismatch, want %d, got %d", want, got)
	}

	if want, got := "braces.dev/errtrace.errorCallee", trace[0].Function; want != got {
		t.Errorf("innermost function should be first, want %q, got %q", want, got)
	}

	if want, got := "braces.dev/errtrace.errorCaller", trace[1].Function; want != got {
		t.Errorf("outermost function should be last, want %q, got %q", want, got)
	}
}

func TestBuildTreeMulti(t *testing.T) {
	tree := buildTraceTree(errorMultiCaller())

	if want, got := 0, len(tree.Trace); want != got {
		t.Fatalf("unexpected trace: %v", tree.Trace)
	}

	if want, got := 2, len(tree.Children); want != got {
		t.Fatalf("children length mismatch, want %d, got %d", want, got)
	}

	for _, child := range tree.Children {
		if want, got := 2, len(child.Trace); want != got {
			t.Fatalf("trace length mismatch, want %d, got %d", want, got)
		}

		if want, got := "braces.dev/errtrace.errorCallee", child.Trace[0].Function; want != got {
			t.Errorf("innermost function should be first, want %q, got %q", want, got)
		}

		if want, got := "braces.dev/errtrace.errorCaller", child.Trace[1].Function; want != got {
			t.Errorf("outermost function should be last, want %q, got %q", want, got)
		}
	}
}

func TestWriteTree(t *testing.T) {
	// Helpers to make tests more readable.
	type frames = []Frame
	tree := func(err error, trace frames, children ...traceTree) traceTree {
		return traceTree{
			Err:      err,
			Trace:    trace,
			Children: children,
		}
	}

	tests := []struct {
		name string
		give traceTree
		want []string // lines minus trailing newline
	}{
		{
			name: "top level single error",
			give: tree(
				errors.New("test error"),
				frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				},
			),
			want: []string{
				"test error",
				"",
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
			},
		},
		{
			name: "multi error without trace",
			give: tree(
				errors.Join(
					errors.New("err a"),
					errors.New("err b"),
				),
				frames{},
				tree(errors.New("err a"), frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				}),
				tree(errors.New("err b"), frames{
					{"baz", "baz.go", 24},
					{"qux", "qux.go", 48},
				}),
			),
			want: []string{
				"+- err a",
				"|  ",
				"|  foo",
				"|  	foo.go:42",
				"|  bar",
				"|  	bar.go:24",
				"|  ",
				"+- err b",
				"|  ",
				"|  baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  ",
				"err a",
				"err b",
			},
		},
		{
			name: "multi error with trace",
			give: tree(
				errors.Join(
					errors.New("err a"),
					errors.New("err b"),
				),
				frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				},
				tree(
					errors.New("err a"),
					frames{
						{"baz", "baz.go", 24},
						{"qux", "qux.go", 48},
					},
				),
				tree(
					errors.New("err b"),
					frames{
						{"corge", "corge.go", 24},
						{"grault", "grault.go", 48},
					},
				),
			),
			want: []string{
				"+- err a",
				"|  ",
				"|  baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  ",
				"+- err b",
				"|  ",
				"|  corge",
				"|  	corge.go:24",
				"|  grault",
				"|  	grault.go:48",
				"|  ",
				"err a",
				"err b",
				"",
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
			},
		},
		{
			name: "wrapped multi error with siblings",
			give: tree(
				errors.Join(
					errors.Join(
						errors.New("err a"),
						errors.New("err b"),
					),
					errors.New("err c"),
				),
				frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				},
				tree(
					errors.Join(
						errors.New("err a"),
						errors.New("err b"),
					),
					frames{
						{"baz", "baz.go", 24},
						{"qux", "qux.go", 48},
					},
					tree(
						errors.New("err a"),
						frames{
							{"quux", "quux.go", 24},
							{"quuz", "quuz.go", 48},
						},
					),
					tree(
						errors.New("err b"),
						frames{
							{"abc", "abc.go", 24},
							{"def", "def.go", 48},
						},
					),
				),
				tree(
					errors.New("err c"),
					frames{
						{"corge", "corge.go", 24},
						{"grault", "grault.go", 48},
					},
				),
			),
			want: []string{
				"   +- err a",
				"   |  ",
				"   |  quux",
				"   |  	quux.go:24",
				"   |  quuz",
				"   |  	quuz.go:48",
				"   |  ",
				"   +- err b",
				"   |  ",
				"   |  abc",
				"   |  	abc.go:24",
				"   |  def",
				"   |  	def.go:48",
				"   |  ",
				"+- err a",
				"|  err b",
				"|  ",
				"|  baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  ",
				"+- err c",
				"|  ",
				"|  corge",
				"|  	corge.go:24",
				"|  grault",
				"|  	grault.go:48",
				"|  ",
				"err a",
				"err b",
				"err c",
				"",
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
			},
		},
		{
			name: "multi error with one non-traced error",
			give: tree(
				errors.Join(
					errors.New("err a"),
					errors.New("err b"),
					errors.New("err c"),
				),
				frames{},
				tree(
					errors.New("err a"),
					frames{
						{"foo", "foo.go", 42},
						{"bar", "bar.go", 24},
					},
				),
				tree(
					errors.New("err b"),
					frames{},
				),
				tree(
					errors.New("err c"),
					frames{
						{"baz", "baz.go", 24},
						{"qux", "qux.go", 48},
					},
				),
			),
			want: []string{
				"+- err a",
				"|  ",
				"|  foo",
				"|  	foo.go:42",
				"|  bar",
				"|  	bar.go:24",
				"|  ",
				"+- err b",
				"|  ",
				"+- err c",
				"|  ",
				"|  baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  ",
				"err a",
				"err b",
				"err c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s strings.Builder
			if err := writeTree(&s, tt.give); err != nil {
				t.Fatal(err)
			}

			if want, got := strings.Join(tt.want, "\n")+"\n", s.String(); want != got {
				t.Errorf("output mismatch:\n"+
					"want:\n%s\n"+
					"got:\n%s\n"+
					"diff:\n%s", want, got, diff.Lines(want, got))
			}
		})
	}
}
