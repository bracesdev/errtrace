//go:build !safe && amd64

#include "textflag.h"

// func getcallerpc() uintptr
TEXT ·getcallerpc(SB),NOSPLIT|NOFRAME,$0-8
	MOVQ 8(BP), AX
	MOVQ AX, ret+0(FP)
	RET
