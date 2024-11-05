// Package atomicval provides [Value]: an atomic value store which is a safer,
// friendlier, and often faster alternative to [atomic.Value]. Relative to
// the standard library, it:
//   - will not raise panics
//   - is safe for any type T allowed by the constraint ([unsafe.Pointer] shenanigans
//     aside, perhaps)
//   - does not prohibit/panic on mixed concrete types for the same interface type
//   - properly handles nils as a zero-value for applicable types (e.g.
//     `Store(nil)`, or [Value.CompareAndSwap] on an uninitialized [Value].)
package atomicval

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// Value provides atomic operations for values of a given type. It is based
// on [atomic.Value], but is designed to be safer and more user-friendly in
// that it will not panic, treats an uninitialized state as equivalent to
// a zero-value of the given type, and allows interfaces of mixed underlying
// types.
//
// Must not be copied after first use.
type Value[T comparable] struct {
	// cause copy attempts to be caught by `go vet`
	_ [0]sync.Mutex

	// prevent unruly type conversions
	_ [0]*T

	v unsafe.Pointer
}

// Load returns the value set by the most recent Store. Returns the zero value
// if no value has been set.
func (v *Value[T]) Load() (val T) {
	dp := atomic.LoadPointer(&v.v)
	if dp == nil {
		return val
	}

	return (*[1]T)(dp)[0]
}

// Store sets the value of the [Value] v to val.
func (v *Value[T]) Store(val T) {
	atomic.StorePointer(&v.v, unsafe.Pointer(&[1]T{val}))
}

// Swap stores new into Value and returns the previous value. Returns the zero value
// if no value has been set.
func (v *Value[T]) Swap(new T) (old T) {
	dp := atomic.SwapPointer(&v.v, unsafe.Pointer(&[1]T{new}))
	if dp == nil {
		return old
	}

	return (*[1]T)(dp)[0]
}

// CompareAndSwap executes the compare-and-swap operation for the [Value]. All
// values of type T are valid inputs. If no value has been set, old is compared
// against the zero-value for type T.
func (v *Value[T]) CompareAndSwap(old, new T) (swapped bool) {
	dp := atomic.LoadPointer(&v.v)
	if dp == nil {
		// treat nil as a zero-value, otherwise proceeding as below
		var zeroVal T
		if old != zeroVal {
			return false
		}

		return atomic.CompareAndSwapPointer(&v.v, dp, unsafe.Pointer(&[1]T{new}))
	}

	// Perform a runtime equality check between old and the current value
	if *(*[1]T)(dp) != [1]T{old} {
		return false
	}

	// [atomic.CompareAndSwapPointer] ensures that changes haven't occured since the
	// [atomic.LoadPointer] call above
	return atomic.CompareAndSwapPointer(&v.v, dp, unsafe.Pointer(&[1]T{new}))
}
