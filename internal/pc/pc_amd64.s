//go:build !safe && amd64

#include "textflag.h"

// func GetCaller() uintptr
TEXT ·GetCaller(SB),NOSPLIT|NOFRAME,$0-8
	// BP is the hardware register frame pointer, as used in:
	// https://github.com/golang/go/blob/go1.21.4/src/runtime/asm_amd64.s#L2091-L2093
	// The return address sits one word above, hence we evaluate `*(BP+8)`.
	MOVQ 8(BP), AX
	MOVQ AX, ret+0(FP)
	RET
