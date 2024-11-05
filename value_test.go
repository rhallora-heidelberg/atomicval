package atomicval

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

type ex struct {
	a int
	b string
	c complex128
}

func TestValue_Load(t *testing.T) {
	requireZero(t, new(Value[int]).Load())
	requireZero(t, new(Value[[2]int]).Load())
	requireZero(t, new(Value[ex]).Load())

	requireZero(t, new(Value[*int]).Load())
	requireZero(t, new(Value[*ex]).Load())
	requireZero(t, new(Value[chan int]).Load())
	requireZero(t, new(Value[io.Reader]).Load())
}

type fakeWriter struct{}

func (fakeWriter) Write(p []byte) (int, error) { return len(p), nil }

type fakeWriter2 fakeWriter

func (fakeWriter2) Write(p []byte) (int, error) { return len(p), nil }

func TestValue_LoadAndStore(t *testing.T) {
	var a Value[uint64]
	a.Store(1)
	requireEqual(t, uint64(1), a.Load())
	a.Store(math.MaxUint64)
	requireEqual(t, uint64(math.MaxUint64), a.Load())

	var b Value[[2]int]
	b.Store([2]int{0, 1})
	requireEqual(t, [2]int{0, 1}, b.Load())

	var c Value[ex]
	c.Store(ex{3, "3", 3i})
	requireEqual(t, ex{3, "3", 3i}, c.Load())

	var ptrVal Value[*int]
	ptrVal.Store(nil)
	requireZero(t, ptrVal.Load())
	ptr := new(int)
	ptrVal.Store(ptr)
	requireEqual(t, ptr, ptrVal.Load())
	requireZero(t, *ptrVal.Load())
	*ptr = 1
	requireEqual(t, ptr, ptrVal.Load())

	var chanVal Value[chan int]
	chanVal.Store(nil)
	requireZero(t, chanVal.Load())
	intChan := make(chan int)
	chanVal.Store(intChan)
	requireEqual(t, intChan, chanVal.Load())

	t.Run("interfaces", func(t *testing.T) {
		var av Value[io.Writer]
		av.Store(nil)
		requireZero(t, av.Load())

		fw := fakeWriter{}
		fw2 := fakeWriter2{}

		av.Store(fw)
		res1 := av.Load()
		requireEqual[io.Writer](t, fw, res1)

		// mixed underlying types
		av.Store(fw2)
		res2 := av.Load()
		requireEqual[io.Writer](t, fw2, res2)

		// distinguish between mixed types but same data (none, in this case)
		requireNotEqual(t, res1, res2)
	})

	// based on stdlib test
	t.Run("concurrent", func(t *testing.T) {
		// generate a slice of random data -- lengths/sizes are mostly arbitrary
		randArr := func() [3]uint64 {
			return [3]uint64{rand.Uint64(), rand.Uint64(), rand.Uint64()}
		}
		data := [][3]uint64{randArr(), randArr(), randArr(), randArr()}

		paralellism := 100 * runtime.GOMAXPROCS(0)
		iters := 50000
		if testing.Short() {
			iters = 10000
		}

		wg := sync.WaitGroup{}
		wg.Add(paralellism)
		failChan := make(chan error)
		go func() {
			wg.Wait()
			close(failChan)
		}()

		var av Value[[3]uint64]
		for range paralellism {
			go func() {
				defer wg.Done()
				for range iters {
					x := data[rand.IntN(len(data))]
					av.Store(x)
					x = av.Load()

					if !slices.Contains(data, x) {
						failChan <- fmt.Errorf("value %+v not in test data set: %+v", x, data)
					}
				}
			}()
		}

		for err := range failChan {
			t.Fatal(err)
		}
	})
}

func TestValue_Swap(t *testing.T) {
	var a Value[uint64]
	requireEqual(t, uint64(0), a.Swap(1))
	requireEqual(t, uint64(1), a.Swap(math.MaxUint64))
	requireEqual(t, uint64(math.MaxUint64), a.Swap(0))

	var b Value[*int]
	ptrA := new(int)
	requireZero(t, b.Swap(ptrA))
	requireEqual(t, ptrA, b.Swap(new(int)))
	requireNotZero(t, b.Swap(nil))
	requireZero(t, b.Swap(ptrA))

	var c Value[io.Writer]
	requireZero(t, c.Swap(io.Discard))
	requireEqual(t, io.Discard, c.Swap(io.Discard))
	buf := bytes.NewBuffer(nil)
	requireEqual(t, io.Discard, c.Swap(buf))
	requireEqual[io.Writer](t, buf, c.Swap(nil))

	// based on stdlib test
	t.Run("concurrent", func(t *testing.T) {
		// calculate the sum of integers 1...N-1 by addition, splitting the range
		// into chunks per goroutine, then compare the result against a known
		// formula.
		var chunks, chunkSize uint64 = 1000, 100000
		if testing.Short() {
			chunks, chunkSize = 1000, 1000
		}
		N := chunkSize * chunks

		// 1 + 2 + 3 ... N-1 = N * (N - 1) / 2
		expected := (N - 1) * N / 2

		var wg sync.WaitGroup
		var sum atomic.Uint64
		var av Value[uint64]
		for start := uint64(0); start < N; start += chunkSize {
			wg.Add(1)
			go func(start uint64) {
				var localSum uint64 // avoid [atomic.Uint64.Add] bottleneck
				for x := range chunkSize {
					localSum += av.Swap(start + x)
				}
				sum.Add(localSum)
				wg.Done()
			}(uint64(start))
		}
		wg.Wait()

		// the value swapped in by the last [Value.Swap] won't have been returned by a
		// subsequent call, so add it here from [Value.Load]
		sum.Add(av.Load())
		requireEqual(t, expected, sum.Load())
	})
}

func TestValue_CompareAndSwap(t *testing.T) {
	var a Value[int]
	requireEqual(t, true, a.CompareAndSwap(0, 1))
	requireEqual(t, true, a.CompareAndSwap(1, 2))
	requireEqual(t, false, a.CompareAndSwap(3, 4))

	var b Value[io.Writer]
	requireEqual(t, true, b.CompareAndSwap(nil, io.Discard))
	requireEqual(t, false, b.CompareAndSwap(nil, io.Discard))
	requireEqual(t, true, b.CompareAndSwap(io.Discard, nil))
	requireEqual(t, false, b.CompareAndSwap(io.Discard, nil))

	t.Run("concurrent", func(t *testing.T) {
		n := 10000
		if testing.Short() {
			n = 1000
		}

		var wg sync.WaitGroup
		wg.Add(n)

		var av Value[int]

		// "i" counts down to maximize contention
		for i := n - 1; i >= 0; i-- {
			go func(i int) {
				// each goroutine must succeed exactly once (no values are "missed")
				for !av.CompareAndSwap(i, i+1) {
					runtime.Gosched()
				}
				wg.Done()
			}(i)
		}

		wg.Wait()
	})
}

// avoid dependency on testify etc., since we have simple needs here

func requireZero[T comparable](t *testing.T, v T) {
	t.Helper()

	var zeroVal T
	if v != zeroVal {
		t.Fatalf("expected zero-value, got %+v", v)
	}
}

func requireNotZero[T comparable](t *testing.T, v T) {
	t.Helper()

	var zeroVal T
	if v == zeroVal {
		t.Fatalf("expected zero-value, got %+v", v)
	}
}

func requireEqual[T comparable](t *testing.T, expected, got T) {
	t.Helper()

	if expected != got {
		t.Fatalf("not equal:\n   expected: %+v,\n   got: %+v", expected, got)
	}
}

func requireNotEqual[T comparable](t *testing.T, expected, got T) {
	t.Helper()

	if expected == got {
		t.Fatalf("unexpected value: %+v", got)
	}
}

func BenchmarkLoad(b *testing.B) {
	const paralellism = 100

	type tt [32]uint8

	x := tt{1}

	b.Run("Value", func(b *testing.B) {
		var av Value[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Load())
			}
		})
	})

	b.Run("stdlib_baseline", func(b *testing.B) {
		var av atomic.Value
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Load().(tt))
			}
		})
	})

	b.Run("stdlib_thinWrapper", func(b *testing.B) {
		var av thinWrapper[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Load())
			}
		})
	})

	b.Run("mutexValue", func(b *testing.B) {
		var av mutexValue[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Load())
			}
		})
	})
}

func BenchmarkStore(b *testing.B) {
	const paralellism = 100

	type tt [32]uint8

	x := tt{1}

	b.Run("Value", func(b *testing.B) {
		var av Value[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
			}
		})
	})

	b.Run("stdlib_baseline", func(b *testing.B) {
		var av atomic.Value

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
			}
		})
	})

	b.Run("stdlib_thinWrapper", func(b *testing.B) {
		var av thinWrapper[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
			}
		})
	})

	b.Run("mutexValue", func(b *testing.B) {
		var av mutexValue[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
			}
		})
	})
}

func BenchmarkSwap(b *testing.B) {
	const paralellism = 100

	type tt [32]uint8

	x := tt{1}

	b.Run("Value", func(b *testing.B) {
		var av Value[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Swap(x))
			}
		})
	})

	b.Run("stdlib_baseline", func(b *testing.B) {
		var av atomic.Value
		av.Store(x) // avoid panic in type conversion below

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Swap(x).(tt))
			}
		})
	})

	b.Run("stdlib_thinWrapper", func(b *testing.B) {
		var av thinWrapper[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Swap(x))
			}
		})
	})

	b.Run("mutexValue", func(b *testing.B) {
		var av mutexValue[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.Swap(x))
			}
		})
	})
}

func BenchmarkCompareAndSwap(b *testing.B) {
	const paralellism = 100

	type tt [32]uint8
	var x, y tt
	y[len(y)-1] = 1

	b.Run("Value", func(b *testing.B) {
		var av Value[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.CompareAndSwap(x, y))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
			}
		})
	})

	b.Run("stdlib_baseline", func(b *testing.B) {
		var av atomic.Value
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.CompareAndSwap(x, y))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
			}
		})
	})

	b.Run("stdlib_thinWrapper", func(b *testing.B) {
		var av thinWrapper[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.CompareAndSwap(x, y))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
			}
		})
	})

	b.Run("mutexValue", func(b *testing.B) {
		var av mutexValue[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				runtime.KeepAlive(av.CompareAndSwap(x, y))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
			}
		})
	})
}

// benchmark CompareAndSwap in the case where we retry until it succeeds
func BenchmarkCompareAndSwap_retries(b *testing.B) {
	const paralellism = 100

	type tt [32]uint8
	var x, y tt
	y[len(y)-1] = 1

	b.Run("Value", func(b *testing.B) {
		var av Value[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				for !av.CompareAndSwap(x, y) {
					runtime.Gosched()
				}
				for !av.CompareAndSwap(y, x) {
					runtime.Gosched()
				}
			}
		})
	})

	b.Run("stdlib_baseline", func(b *testing.B) {
		var av atomic.Value
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				for !av.CompareAndSwap(x, y) {
					runtime.Gosched()
				}
				for !av.CompareAndSwap(y, x) {
					runtime.Gosched()
				}
			}
		})
	})

	b.Run("stdlib_thinWrapper", func(b *testing.B) {
		var av thinWrapper[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				for !av.CompareAndSwap(x, y) {
					runtime.Gosched()
				}
				for !av.CompareAndSwap(y, x) {
					runtime.Gosched()
				}
			}
		})
	})

	b.Run("mutexValue", func(b *testing.B) {
		var av mutexValue[tt]
		av.Store(x)

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				for !av.CompareAndSwap(x, y) {
					runtime.Gosched()
				}
				for !av.CompareAndSwap(y, x) {
					runtime.Gosched()
				}
			}
		})
	})
}

// benchmark mixed methods being called concurrently -- exact mix is entirely arbitrary
func BenchmarkMedley(b *testing.B) {
	const paralellism = 100

	type tt [32]uint8
	var x, y tt
	y[len(y)-1] = 1

	b.Run("Value", func(b *testing.B) {
		var av Value[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
				runtime.KeepAlive(av.Load())
				av.Store(y)
				runtime.KeepAlive(av.Load())
				runtime.KeepAlive(av.Swap(y))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
				av.Store(x)
				runtime.KeepAlive(av.Load())
				av.Store(y)
				runtime.KeepAlive(av.Load())
				runtime.KeepAlive(av.Swap(x))
				runtime.KeepAlive(av.CompareAndSwap(x, y))
			}
		})
	})

	b.Run("stdlib_baseline", func(b *testing.B) {
		var av atomic.Value

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
				runtime.KeepAlive(av.Load().(tt))
				av.Store(y)
				runtime.KeepAlive(av.Load().(tt))
				runtime.KeepAlive(av.Swap(y).(tt))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
				av.Store(x)
				runtime.KeepAlive(av.Load().(tt))
				av.Store(y)
				runtime.KeepAlive(av.Load().(tt))
				runtime.KeepAlive(av.Swap(x).(tt))
				runtime.KeepAlive(av.CompareAndSwap(x, y))
			}
		})
	})

	b.Run("stdlib_thinWrapper", func(b *testing.B) {
		var av thinWrapper[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
				runtime.KeepAlive(av.Load())
				av.Store(y)
				runtime.KeepAlive(av.Load())
				runtime.KeepAlive(av.Swap(y))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
				av.Store(x)
				runtime.KeepAlive(av.Load())
				av.Store(y)
				runtime.KeepAlive(av.Load())
				runtime.KeepAlive(av.Swap(x))
				runtime.KeepAlive(av.CompareAndSwap(x, y))
			}
		})
	})

	b.Run("mutexValue", func(b *testing.B) {
		var av mutexValue[tt]

		b.SetParallelism(paralellism)
		runtime.GC()
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				av.Store(x)
				runtime.KeepAlive(av.Load())
				av.Store(y)
				runtime.KeepAlive(av.Load())
				runtime.KeepAlive(av.Swap(y))
				runtime.KeepAlive(av.CompareAndSwap(y, x))
				av.Store(x)
				runtime.KeepAlive(av.Load())
				av.Store(y)
				runtime.KeepAlive(av.Load())
				runtime.KeepAlive(av.Swap(x))
				runtime.KeepAlive(av.CompareAndSwap(x, y))
			}
		})
	})
}
