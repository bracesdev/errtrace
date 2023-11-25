# Safety

To achieve the performance above,
errtrace makes use of unsafe operations using Go assembly
to read the caller information directly from the stack.
This is part of the reason why we have the disclaimer on top.

errtrace includes an opt-in safe mode
that drops these unsafe operations in exchange for poorer performance.
To opt into safe mode,
use the `safe` build tag when compiling code that uses errtrace.

```bash
go build -tags safe
```
