# atomicval

[![Go](https://github.com/rhallora-heidelberg/atomicval/actions/workflows/go.yml/badge.svg)](https://github.com/rhallora-heidelberg/atomicval/actions/workflows/go.yml)
[![GoDoc](https://pkg.go.dev/badge/github.com/rhallora-heidelberg/atomicval)](https://pkg.go.dev/github.com/rhallora-heidelberg/atomicval)
[![Go Report Card](https://goreportcard.com/badge/github.com/rhallora-heidelberg/atomicval)](https://goreportcard.com/report/github.com/rhallora-heidelberg/atomicval)

A safer, friendlier `atomic.Value` alternative with type parameters.

## Features

- **Explicit Typing**: Type parameters ensure consistent usage.
- **Familiar API**: Identical API to stdlib (type parameters aside), allowing largely drop-in replacement.
- **Panic-Free**: All panics found in the stdlib implementation are eliminated.
- **Safe Zero-Value**: Zero-values are always safe to use, and no operations on them will produce panics or unintuitive results.
- **Allows Mixed Interface Implementations**: For a `Value` of some interface type, inputs will be compared correctly and may be implemented by mixed concrete types.
- **Performant**: More lightweight than stdlib, with similar or slightly better performance in most observed cases.
- **Extra Safeguards**: Prevents invalid type conversions (compile-time) and copies (via `go vet`), with no impact on size/performance.

## Performance

This implementation is a little more lightweight than stdlib in terms of code and struct size, and appears to see some performance benefit from that. However, overall performance is fairly similar and I would not expect real applications to see a noticeable boost outside of very niche circumstances.

Microbenchmarks are included and make attempts at accuracy/impartiality, though as always your results may vary. The below sample results compare this implementation to several possible alternatives as a baseline:
- `stdlib_baseline`: stdlib implementation without added safety features
- `stdlib_thinWrapper`: wrapped stdlib with incomplete safety feature additions (similar to other third-party libraries)
- `mutexValue`: simple lock-based implementation (apples-to-oranges, but interesting to see)

```
goos: linux
goarch: amd64
pkg: github.com/rhallora-heidelberg/atomicval
cpu: AMD Ryzen 7 3700X 8-Core Processor             
BenchmarkLoad/Value-16                  1000000000               0.2195 ns/op          0 B/op          0 allocs/op
BenchmarkLoad/stdlib_baseline-16        1000000000               0.3180 ns/op          0 B/op          0 allocs/op
BenchmarkLoad/stdlib_thinWrapper-16     1000000000               0.3229 ns/op          0 B/op          0 allocs/op
BenchmarkLoad/mutexValue-16             23641336                51.42 ns/op            0 B/op          0 allocs/op

BenchmarkStore/Value-16                 34796320                33.13 ns/op           32 B/op          1 allocs/op
BenchmarkStore/stdlib_baseline-16       31221872                34.56 ns/op           32 B/op          1 allocs/op
BenchmarkStore/stdlib_thinWrapper-16    30506049                36.08 ns/op           32 B/op          1 allocs/op
BenchmarkStore/mutexValue-16            18810124                64.64 ns/op            0 B/op          0 allocs/op

BenchmarkSwap/Value-16                  34508912                34.24 ns/op           32 B/op          1 allocs/op
BenchmarkSwap/stdlib_baseline-16        27829462                40.47 ns/op           32 B/op          1 allocs/op
BenchmarkSwap/stdlib_thinWrapper-16     24618009                41.53 ns/op           32 B/op          1 allocs/op
BenchmarkSwap/mutexValue-16             16995601                71.05 ns/op            0 B/op          0 allocs/op

BenchmarkCompareAndSwap/Value-16                        17737754                57.25 ns/op           50 B/op          1 allocs/op
BenchmarkCompareAndSwap/stdlib_baseline-16              14334254                72.12 ns/op           64 B/op          2 allocs/op
BenchmarkCompareAndSwap/stdlib_thinWrapper-16           16981998                69.41 ns/op           64 B/op          2 allocs/op
BenchmarkCompareAndSwap/mutexValue-16                    8364978               143.9 ns/op             0 B/op          0 allocs/op

BenchmarkCompareAndSwap_retries/Value-16                         5105161               241.1 ns/op            70 B/op          2 allocs/op
BenchmarkCompareAndSwap_retries/stdlib_baseline-16               4834880               253.1 ns/op            81 B/op          2 allocs/op
BenchmarkCompareAndSwap_retries/stdlib_thinWrapper-16            4571679               264.3 ns/op            81 B/op          2 allocs/op
BenchmarkCompareAndSwap_retries/mutexValue-16                   16380373                72.48 ns/op            0 B/op          0 allocs/op

BenchmarkMedley/Value-16                                         4587736               242.6 ns/op           236 B/op          7 allocs/op
BenchmarkMedley/stdlib_baseline-16                               3723897               302.8 ns/op           256 B/op          8 allocs/op
BenchmarkMedley/stdlib_thinWrapper-16                            3605121               308.2 ns/op           256 B/op          8 allocs/op
BenchmarkMedley/mutexValue-16                                    1333707               910.2 ns/op             0 B/op          0 allocs/op
```
