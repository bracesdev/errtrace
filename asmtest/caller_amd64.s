//go:build amd64

#include "textflag.h"

// func ret1() int
TEXT ·ret1(SB),NOSPLIT|NOFRAME,$0-8
	MOVQ	$1337, ret+0(FP)
	RET


// func retPreFP() uintptr
TEXT ·retPreFP(SB),NOSPLIT|NOFRAME,$0-8
	MOVQ ra-8(FP), AX
	MOVQ AX, ret+0(FP)
	RET

// func retFromBP() uintptr
TEXT ·retFromBP(SB),NOSPLIT|NOFRAME,$0-8
	MOVQ 8(BP), AX
	MOVQ AX, ret+0(FP)
	RET
