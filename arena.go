package errtrace

import (
	"sync"
	"sync/atomic"
)

// arena is a lock-free allocator for a fixed-size type.
// It is intended to be used for allocating errTrace objects in batches.
type arena[T any] struct {
	slabSize int
	pool     sync.Pool
}

func newArena[T any](slabSize int) *arena[T] {
	return &arena[T]{
		slabSize: slabSize,
	}
}

// Take returns a pointer to a new object from the arena.
func (a *arena[T]) Take() *T {
	for {
		slab, ok := a.pool.Get().(*arenaSlab[T])
		if !ok {
			slab = newArenaSlab[T](a.slabSize)
		}

		if e, ok := slab.take(); ok {
			a.pool.Put(slab)
			return e
		}
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
