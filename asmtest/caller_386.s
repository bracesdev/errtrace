//go:build 386

#include "textflag.h"

//func ret1() uintptr
TEXT ·ret1(SB),NOSPLIT|NOFRAME,$0-4
	MOVL $1337, AX
	MOVL AX, ret+0(FP)
	RET

// func retPreFP() uintptr
TEXT ·retPreFP(SB),NOSPLIT|NOFRAME,$0-4
	MOVL ra-4(FP), AX
	MOVL AX, ret+0(FP)
	RET

// func retFromBP() uintptr
TEXT ·retFromBP(SB),NOSPLIT|NOFRAME,$0-4
	MOVL x+4(FP), AX
	MOVL AX, ret+0(FP)
	RET
