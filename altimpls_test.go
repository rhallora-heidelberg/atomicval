package atomicval

import (
	"sync"
	"sync/atomic"
)

// Defined for benchmark comparison. Uses a lock to mimic the [atomic.Value] methods.
type mutexValue[T comparable] struct {
	mu    sync.Mutex
	inner T
}

func (v *mutexValue[T]) Load() (val T) {
	v.mu.Lock()
	val = v.inner
	v.mu.Unlock()
	return
}

func (v *mutexValue[T]) Store(val T) {
	v.mu.Lock()
	v.inner = val
	v.mu.Unlock()
}

func (v *mutexValue[T]) Swap(new T) (old T) {
	v.mu.Lock()
	old = v.inner
	v.inner = new
	v.mu.Unlock()
	return old
}

// note that we can't replicate the [atomic.Value] behavior here as well as we can
// for the other methods. In short, Lock() and TryLock() would have very different
// performance characteristics and be more/less appropriate for different workloads,
// but neither one is an apples-to-apples comparison with [atomic.Value] or [Value].
func (v *mutexValue[T]) CompareAndSwap(old, new T) (swapped bool) {
	v.mu.Lock()
	if v.inner == old {
		v.inner = new
		v.mu.Unlock()
		return true
	}

	v.mu.Unlock()
	return false
}

// Defined for benchmark comparison. Wraps [atomic.Value] while reducing some of
// the potential pain points.
type thinWrapper[T comparable] struct {
	atomic.Value
}

func (v *thinWrapper[T]) Load() (val T) {
	if out := v.Value.Load(); out != nil {
		return out.([1]T)[0]
	}

	return val
}

func (v *thinWrapper[T]) Store(val T) { v.Value.Store([1]T{val}) }

func (v *thinWrapper[T]) Swap(new T) (old T) {
	if out := v.Value.Swap([1]T{new}); out != nil {
		return out.([1]T)[0]
	}

	return old
}

func (v *thinWrapper[T]) CompareAndSwap(old, new T) (swapped bool) {
	return v.Value.CompareAndSwap([1]T{old}, [1]T{new})
}
