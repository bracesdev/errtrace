package errtrace

import (
	"encoding/json"
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

	if want, got := "braces.dev/errtrace.errorCallee", trace[0].Name; want != got {
		t.Errorf("innermost function should be first, want %q, got %q", want, got)
	}

	if want, got := "braces.dev/errtrace.errorCaller", trace[1].Name; want != got {
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

		if want, got := "braces.dev/errtrace.errorCallee", child.Trace[0].Name; want != got {
			t.Errorf("innermost function should be first, want %q, got %q", want, got)
		}

		if want, got := "braces.dev/errtrace.errorCaller", child.Trace[1].Name; want != got {
			t.Errorf("outermost function should be last, want %q, got %q", want, got)
		}
	}
}

func TestWriteTree(t *testing.T) {
	// Helpers to make tests more readable.
	type frames = []traceFrame
	tree := func(trace frames, children ...traceTree) traceTree {
		return traceTree{
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
				frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				},
			),
			want: []string{
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
			},
		},
		{
			name: "top level multi error",
			give: tree(
				frames{},
				tree(frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				}),
				tree(frames{
					{"baz", "baz.go", 24},
					{"qux", "qux.go", 48},
				}),
			),
			want: []string{
				"+- foo",
				"|  	foo.go:42",
				"|  bar",
				"|  	bar.go:24",
				"|  ",
				"+- baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  ",
				"+",
			},
		},
		{
			name: "wrapped multi error",
			give: tree(
				frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				},
				tree(
					frames{
						{"baz", "baz.go", 24},
						{"qux", "qux.go", 48},
					},
				),
			),
			want: []string{
				"+- baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  ",
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
			},
		},
		{
			name: "wrapped multi error with siblings",
			give: tree(
				frames{
					{"foo", "foo.go", 42},
					{"bar", "bar.go", 24},
				},
				tree(
					frames{
						{"baz", "baz.go", 24},
						{"qux", "qux.go", 48},
					},
					tree(
						frames{
							{"quux", "quux.go", 24},
							{"quuz", "quuz.go", 48},
						},
					),
					tree(
						frames{
							{"abc", "abc.go", 24},
							{"def", "def.go", 48},
						},
					),
				),
				tree(
					frames{
						{"corge", "corge.go", 24},
						{"grault", "grault.go", 48},
					},
				),
			),
			want: []string{
				"   +- quux",
				"   |  	quux.go:24",
				"   |  quuz",
				"   |  	quuz.go:48",
				"   |  ",
				"   +- abc",
				"   |  	abc.go:24",
				"   |  def",
				"   |  	def.go:48",
				"   |  ",
				"+- baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  ",
				"+- corge",
				"|  	corge.go:24",
				"|  grault",
				"|  	grault.go:48",
				"|  ",
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
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

// FuzzWriteTree generates a bunch of random trace trees
// and ensures that writeTree doesn't panic on any of them.
func FuzzWriteTree(f *testing.F) {
	f.Add([]byte(`
	{
		"Trace": [
			{"Name": "foo", "File": "foo.go", "Line": 42},
			{"Name": "bar", "File": "bar.go", "Line": 24}
		]
	}`))
	f.Add([]byte(`{
		"Children": [
			{
				"Trace": [
					{"Name": "foo", "File": "foo.go", "Line": 42},
					{"Name": "bar", "File": "bar.go", "Line": 24}
				]
			}
		]
	}`))
	f.Add([]byte(`{
		"Trace": [
			{"Name": "foo", "File": "foo.go", "Line": 42},
			{"Name": "bar", "File": "bar.go", "Line": 24}
		],
		"Children": [
			{
				"Trace": [
					{"Name": "baz", "File": "baz.go", "Line": 24},
					{"Name": "qux", "File": "qux.go", "Line": 48}
				],
				"Children": [
					{
						"Trace": [
							{"Name": "quux", "File": "quux.go", "Line": 24},
							{"Name": "quuz", "File": "quuz.go", "Line": 48}
						]
					}
				]
			}
		]
	}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var tree traceTree
		if err := json.Unmarshal(data, &tree); err != nil {
			t.Skip(err)
		}

		var s strings.Builder
		if err := writeTree(&s, tree); err != nil {
			t.Fatal(err)
		}
	})
}
