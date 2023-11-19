package main

import (
	"fmt"
	"runtime"
)

func ret1() uintptr
func retPreFP() uintptr
func retFromBP() uintptr

func main() {
	testcaller()
}

//go:noinline
func testcaller() {
	Wrap(1)
}

//go:noinline
func Wrap(i int) {
	printFrames("start-safe", getcallerpcsafe())
	printFrames("retPreFP", retPreFP())
	printFrames("retFromBP", retFromBP())
	printFrames("end-safe", getcallerpcsafe())
}

func getcallerpcsafe() uintptr {
	const skip = 1 + // frame for Callers
		1 + // frame for GetCaller
		1 // frame for our caller, which should be errtrace.Wrap

	var callers [1]uintptr
	n := runtime.Callers(skip, callers[:]) // skip getcallerpc + caller
	if n == 0 {
		return 0
	}
	return callers[0]
}

func printFrames(prefix string, pc uintptr) {
	fmt.Printf("%v = 0x%x\n", prefix, pc)
	frames := runtime.CallersFrames([]uintptr{pc})
	for {
		f, more := frames.Next()
		if f != (runtime.Frame{}) {
			fmt.Printf("  %v\n", f.Function)
			fmt.Printf("  at %v:%v\n", f.File, f.Line)

		}
		if !more {
			return
		}
	}
}
