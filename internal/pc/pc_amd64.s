//go:build !safe && amd64

#include "textflag.h"

// func getcallerpc() uintptr
TEXT ·getcallerpc(SB),NOSPLIT|NOFRAME,$0-8
	MOVQ	argp+0(FP), AX
	MOVQ AX, ret+0(FP)
	RET
