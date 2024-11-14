package errtrace

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
)

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
	Trace []runtime.Frame

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
		if frame, inner, ok := UnwrapFrame(err); ok {
			current.Trace = append(current.Trace, frame)
			err = inner
			continue
		}

		// We unwrap errors manually instead of using errors.As
		// because we don't want to accidentally skip over multi-errors
		// or interpret them as part of a single error chain.
		switch x := err.(type) {
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

	slices.Reverse(current.Trace)
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

func (p *treeWriter) writeTrace(err error, trace []runtime.Frame, path []int) {
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
			p.writeString(frame.Function)
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

// pipes draws the "| | |" pipes prefix.
//
// path is a slice of indexes leading to the current node.
// For example, the path [1, 3, 2] says that the current node is
// the 2nd child of the 3rd child of the 1st child of the root.
//
// last is the last "|" component in this grouping;
// it'll normally be "|  " or "+- ".
//
// In combination, path and last tell us how to draw the pipes.
// More often than not, we just draw:
//
//	|  |  |
//
// However, for the first line of a message,
// we need to connect to the following line so we use "+- "
// which gives us:
//
//	|  |  +- msg
//	|  |  |
//
// Lastly, when drawing the tree,
// if any of the intermediate positions in the path are 0,
// (i.e. the first child of a parent),
// we don't draw a pipe because it won't have
// anything above it to connect to.
// For example:
//
//	0  1  2          For some x > 0
//	-------
//	|     +- msg     path = [x, 0, 0]
//	|     |
//	|     +- msg     path = [x, 0, 1]
//	|     |
//	|  +- msg        path = [x, 0]
//	|  |
//	|  +- msg        path = [x, 1]
//
// Note that for cases where path[1] == 0,
// we don't draw a pipe if len(path) > 2.
func (p *treeWriter) pipes(path []int, last string) {
	for depth, idx := range path {
		if depth == len(path)-1 {
			p.writeString(last)
		} else if idx == 0 {
			// First child of the parent at this layer.
			// Nothing to connect to above us.
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
