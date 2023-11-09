package errtrace

import (
	"sync"
	"sync/atomic"
)

// arena is a lock-free allocator for a fixed-size type.
// It is intended to be used for allocating errTrace objects in batches.
type arena[T any] struct {
	// slab is the current slab of objects.
	//
	// When this runs out, a new slab will be swapped in.
	slab atomic.Pointer[arenaSlab[T]]
}

// newArena returns a new arena with the given slab size.
func newArena[T any](sz int) *arena[T] {
	var a arena[T]
	a.slab.Store(newArenaSlab[T](sz))
	return &a
}

// Take returns a pointer to a new object from the arena.
func (a *arena[T]) Take() *T {
	for {
		slab := a.slab.Load()
		if e, ok := slab.take(); ok {
			return e
		}

		// Slab is exhausted, replace it.
		// The sync.Once ensures that
		// the first goroutine to get here does the replacement,
		// and all others either wait for it to finish
		// or arrive after the replacement is done.
		//
		// Everyone tries with the new slab in the next iteration.
		slab.replace.Do(func() {
			newSlab := newArenaSlab[T](len(slab.buf))
			a.slab.CompareAndSwap(slab, newSlab)
		})
	}
}

// arenaSlab is a slab of objects in an arena.
//
// Each slab has a fixed number of objects in it.
// Pointers are taken from the slab in order.
type arenaSlab[T any] struct {
	// Full list of objects in the slab.
	buf []T

	// Index of the next object to be taken.
	idx atomic.Int64

	// Ensures that the slab is replaced only once.
	replace sync.Once
}

func newArenaSlab[T any](sz int) *arenaSlab[T] {
	return &arenaSlab[T]{buf: make([]T, sz)}
}

func (a *arenaSlab[T]) take() (*T, bool) {
	idx := a.idx.Add(1) - 1 // 0-indexed
	if int(idx) >= len(a.buf) {
		return nil, false
	}
	return &a.buf[idx], true
}
