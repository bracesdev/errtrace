package errtrace

import (
	"errors"
	"fmt"
	"io"
	"runtime"
)

// traceFrame is a single frame in a stack trace.
type traceFrame struct {
	Name string // function name
	File string // file name
	Line int    // line number
}

// traceTree represents an error and its traces
// as a tree structure.
//
// The root of the tree is the trace for the error itself.
// Children, if any, are the traces for each of the errors
// inside the multi-error (if the error was a multi-error).
type traceTree struct {
	// Trace is the trace for the error down until
	// the first multi-error was encountered.
	Trace []traceFrame

	// Children are the traces for each of the errors
	// inside the multi-error.
	Children []traceTree
}

// buildTraceTree builds a trace tree from an error.
//
// All errors connected to the given error
// are considered part of its trace except:
// if a multi-error is found,
// a separate trace is built from each of its errors
// and they're all considered children of this error.
func buildTraceTree(err error) traceTree {
	var current traceTree
	for {
		switch x := err.(type) {
		case *errTrace:
			frames := runtime.CallersFrames([]uintptr{x.pc})
			for {
				f, more := frames.Next()
				if f == (runtime.Frame{}) {
					break
				}

				current.Trace = append(current.Trace, traceFrame{
					Name: f.Function,
					File: f.File,
					Line: f.Line,
				})

				if !more {
					break
				}
			}

			err = x.err

		// We unwrap errors manually instead of using errors.As
		// because we don't want to accidentally skip over multi-errors
		// or interpret them as part of a single error chain.

		case interface{ Unwrap() error }:
			err = x.Unwrap()

		case interface{ Unwrap() []error }:
			// Encountered a multi-error.
			// Everything else is a child of current.
			errs := x.Unwrap()
			current.Children = make([]traceTree, 0, len(errs))
			for _, err := range errs {
				current.Children = append(current.Children, buildTraceTree(err))
			}
			return current

		default:
			// Reached a terminal error.
			return current
		}
	}
}

func writeTree(w io.Writer, tree traceTree) error {
	return (&treeWriter{W: w}).WriteTree(tree)
}

type treeWriter struct {
	W io.Writer
	e error
}

func (p *treeWriter) WriteTree(t traceTree) error {
	p.writeTree(t, nil /* path */, nil /* counts */)
	return p.e
}

// Records the error if non-nil.
// Will be returned from WriteTree, ultimately.
func (p *treeWriter) err(err error) {
	p.e = errors.Join(p.e, err)
}

// writeTree writes the tree to the writer.
//
// path is a slice of indexes leading to the current node
// in the tree.
// counts is a slice of the number of children
// in each layer leading to the current node.
//
// Invariant: len(path) == len(counts)
func (p *treeWriter) writeTree(t traceTree, path, counts []int) {
	p.writeTrace(t.Trace, path, counts)

	counts = append(counts, len(t.Children))
	for i, child := range t.Children {
		p.writeTree(child, append(path, i), counts)
	}
}

func (p *treeWriter) writeTrace(trace []traceFrame, path, counts []int) {
	// A trace for a single error takes
	// the same form as a stack trace:
	//
	// func1
	// 	path/to/file.go:12
	// func2
	// 	path/to/file.go:34
	//
	// However, when depth > 1, we're part of a tree,
	// so we need to add prefixes containers around the trace
	// to indicate the tree structure:
	//
	// func1
	// 	path/to/file.go:12
	// func2
	// 	path/to/file.go:34
	// |
	// +- func3
	//    	path/to/file.go:57
	//    func4
	//    	path/to/file.go:78
	//    |
	//    +- func5
	//    |  	path/to/file.go:90
	//    |  func6
	//    |  	path/to/file.go:12
	//    |
	//    +- func7
	//       	path/to/file.go:34
	//       func8
	//       	path/to/file.go:56

	// Connecting "|" lines when starting a new trace.
	// This is the "empty" line between traces.
	// This doesn't use p.pipes because it doesn't care
	// whether we're the last element in the path,
	// and it doesn't want to leave a trailing space on this line.
	if len(path) > 0 {
		for i := 0; i < len(path); i++ {
			if i > 0 {
				p.writeString("  ")
			}
			p.writeString("|")
		}
		p.writeString("\n")
	}

	// This node doesn't have any trace information.
	// It's likely a multi-error that wasn't wrapped with errtrace.
	// Print something simple to mark its presence.
	if len(trace) == 0 {
		p.connectToParent(path, counts)
		p.writeString("+\n")
		return
	}

	for i, frame := range trace {
		if i == 0 {
			// First frame of the trace
			// needs to connect to the "|" from above.
			p.connectToParent(path, counts)
		} else {
			p.pipes(path, counts) // | |
		}

		p.writeString(frame.Name)
		p.writeString("\n")

		p.pipes(path, counts) // | |
		p.printf("\t%s:%d\n", frame.File, frame.Line)
	}
}

func (p *treeWriter) connectToParent(path, counts []int) {
	if len(path) == 0 {
		return
	}

	p.pipes(path[:len(path)-1], counts[:len(counts)-1])
	p.writeString("+- ")
}

func (p *treeWriter) pipes(path, counts []int) {
	// We don't draw pipe for last element in each layer.
	// For example, if paths is [3, 5, 2],
	// and counts is [5, 6, 4],
	// then this node is the last element in the second layer,
	// so we don't draw a pipe for it.
	for depth, idx := range path {
		if idx == counts[depth]-1 {
			//
			p.writeString("   ")
		} else {
			p.writeString("|  ")
		}
	}
}

func (p *treeWriter) writeString(s string) {
	_, err := io.WriteString(p.W, s)
	p.err(err)
}

func (p *treeWriter) printf(format string, args ...interface{}) {
	_, err := fmt.Fprintf(p.W, format, args...)
	p.err(err)
}
