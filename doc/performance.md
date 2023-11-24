# Performance

errtrace is designed to have very low overhead
on supported systems.

Benchmark results for linux/amd64 on an Intel Core i5-13600 (best of 10):
```
BenchmarkFmtErrorf      11574928               103.5 ns/op            40 B/op          2 allocs/op
# default build, uses Go assembly.
BenchmarkWrap           78173496                14.70 ns/op           24 B/op          0 allocs/op
# build with -tags safe to avoid assembly.
BenchmarkWrap            5958579               198.5 ns/op            24 B/op          0 allocs/op

# benchext compares capturing stacks using pkg/errors vs errtrace
# both tests capture ~10 frames,
BenchmarkErrtrace        6388651               188.4 ns/op           280 B/op          1 allocs/op
BenchmarkPkgErrors       1673145               716.8 ns/op           304 B/op          3 allocs/op
```

Stack traces have a large initial cost,
while errtrace scales with each frame that an error is returned through.
