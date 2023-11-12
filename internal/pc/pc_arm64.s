//go:build !safe && arm64

#include "textflag.h"

// func getcallerpc() uintptr
TEXT ·getcallerpc(SB),NOSPLIT|NOFRAME,$0-8
	MOVD x+0(SP), R20
	MOVD R20, ret+0(FP)
	RET
