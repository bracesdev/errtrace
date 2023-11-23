package errtrace

import (
	"encoding/json"
	"strings"
	"testing"

	"braces.dev/errtrace/internal/diff"
)

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
				"+",
				"|",
				"+- foo",
				"|  	foo.go:42",
				"|  bar",
				"|  	bar.go:24",
				"|",
				"+- baz",
				"   	baz.go:24",
				"   qux",
				"   	qux.go:48",
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
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
				"|",
				"+- baz",
				"   	baz.go:24",
				"   qux",
				"   	qux.go:48",
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
				),
				tree(
					frames{
						{"corge", "corge.go", 24},
						{"grault", "grault.go", 48},
					},
				),
			),
			want: []string{
				"foo",
				"	foo.go:42",
				"bar",
				"	bar.go:24",
				"|",
				"+- baz",
				"|  	baz.go:24",
				"|  qux",
				"|  	qux.go:48",
				"|  |",
				"|  +- quux",
				"|     	quux.go:24",
				"|     quuz",
				"|     	quuz.go:48",
				"|",
				"+- corge",
				"   	corge.go:24",
				"   grault",
				"   	grault.go:48",
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
