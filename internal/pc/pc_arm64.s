//go:build !safe && arm64

#include "textflag.h"

// func GetCaller() uintptr
TEXT ·GetCaller(SB),NOSPLIT|NOFRAME,$0-8
	// R29 is the frame pointer, documented in https://pkg.go.dev/cmd/internal/obj/arm64
	// and used in https://github.com/golang/go/blob/go1.21.4/src/runtime/asm_arm64.s#L1571
	// The return address sits one word above, hence we evaluate `*(R29+8)`.
	MOVD 8(R29), R20
	MOVD R20, ret+0(FP)
	RET


// func GetCallerSkip1() uintptr
TEXT ·GetCallerSkip1(SB),NOSPLIT|NOFRAME,$0-8
	// R29 contains the frame pointer, dereference it to skip a frame.
	MOVD (R29), R20
	MOVD 8(R20), R20
	MOVD R20, ret+0(FP)
	RET
