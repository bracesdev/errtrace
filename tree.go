package errtrace

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
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
	// Err is the error at the root of this tree.
	Err error

	// Trace is the trace for the error down until
	// the first multi-error was encountered.
	//
	// The trace is in the reverse order of the call stack.
	// The first element is the deepest call in the stack,
	// and the last element is the shallowest call in the stack.
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
	current := traceTree{Err: err}
loop:
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

			break loop

		default:
			// Reached a terminal error.
			break loop
		}
	}

	sliceReverse(current.Trace)
	return current
}

func writeTree(w io.Writer, tree traceTree) error {
	return (&treeWriter{W: w}).WriteTree(tree)
}

type treeWriter struct {
	W io.Writer
	e error
}

func (p *treeWriter) WriteTree(t traceTree) error {
	p.writeTree(t, nil /* path */)
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
func (p *treeWriter) writeTree(t traceTree, path []int) {
	for i, child := range t.Children {
		p.writeTree(child, append(path, i))
	}

	p.writeTrace(t.Err, t.Trace, path)
}

func (p *treeWriter) writeTrace(err error, trace []traceFrame, path []int) {
	// A trace for a single error takes
	// the same form as a stack trace:
	//
	// error message
	//
	// func1
	// 	path/to/file.go:12
	// func2
	// 	path/to/file.go:34
	//
	// However, when path isn't empty, we're part of a tree,
	// so we need to add prefixes containers around the trace
	// to indicate the tree structure.
	//
	// We print in depth-first order, so we get:
	//
	//    +- error message 1
	//    |
	//    |  func5
	//    |  	path/to/file.go:90
	//    |  func6
	//    |  	path/to/file.go:12
	//    |
	//    +- error message 2
	//    |
	//    |  func7
	//    |  	path/to/file.go:34
	//    |  func8
	//    |  	path/to/file.go:56
	//    |
	// +- error message 3
	// |
	// |  func3
	// |  	path/to/file.go:57
	// |  func4
	// |  	path/to/file.go:78
	// |
	// error message 4
	//
	// func1
	// 	path/to/file.go:12
	// func2
	// 	path/to/file.go:34

	//   +- error message
	//   |
	//
	// The message may have newlines in it,
	// so we need to print each line separately.
	for i, line := range strings.Split(err.Error(), "\n") {
		if i == 0 {
			p.pipes(path, "+- ")
		} else {
			p.pipes(path, "|  ")
		}
		p.writeString(line)
		p.writeString("\n")
	}

	if len(trace) > 0 {
		// Empty line between the message and the trace.
		p.pipes(path, "|  ")
		p.writeString("\n")

		for _, frame := range trace {
			p.pipes(path, "|  ")
			p.writeString(frame.Name)
			p.writeString("\n")

			p.pipes(path, "|  ")
			p.printf("\t%s:%d\n", frame.File, frame.Line)
		}
	}

	// Connecting "|" lines when ending a trace
	// This is the "empty" line between traces.
	if len(path) > 0 {
		p.pipes(path, "|  ")
		p.writeString("\n")
	}
}

func (p *treeWriter) pipes(path []int, last string) {
	for depth, idx := range path {
		if depth < len(path)-1 && idx == 0 {
			// Don't draw a pipe for the first element in each layer
			// except the last layer.
			//
			// This omits extraneous "|" prefixes
			// that don't have anything to connect to.
			p.writeString("   ")
		} else if depth == len(path)-1 {
			p.writeString(last)
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
