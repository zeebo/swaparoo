package swaparoo

import (
	"sync/atomic"
	"testing"

	"github.com/zeebo/assert"
)

func TestSwaparoo(t *testing.T) {
	tr := NewTracker()

	for i := 0; i < 10; i++ {
		token := tr.Acquire()
		assert.Equal(t, token.Gen(), i)
		token.Release()
		assert.Equal(t, tr.Increment(), i)
	}

	assert.Equal(t, tr.Increment(), 10)
	assert.Equal(t, tr.Increment(), 11)
	tr.Acquire().Release()
	assert.Equal(t, tr.Increment(), 12)
	assert.Equal(t, tr.Increment(), 13)
}

func BenchmarkSwaparoo(b *testing.B) {
	b.Run("Acquire", func(b *testing.B) {
		tr := NewTracker()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			tr.Acquire().Release()
		}
	})

	b.Run("Increment", func(b *testing.B) {
		tr := NewTracker()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			tr.Increment()
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.Run("Acquire", func(b *testing.B) {
			tr := NewTracker()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					tr.Acquire().Release()
				}
			})
		})

		b.Run("Increment", func(b *testing.B) {
			first := new(uint64)
			tr := NewTracker()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				// only a single thread can be calling Increment
				if atomic.CompareAndSwapUint64(first, 0, 1) {
					for pb.Next() {
						tr.Increment()
					}
				} else {
					for pb.Next() {
						tr.Acquire().Release()
					}
				}
			})
		})
	})
}
