package swaparoo

import (
	"testing"

	"github.com/zeebo/assert"
)

func TestCounter(t *testing.T) {
	ch := make(chan bool, 2)
	var ctr counter
	ctr.Acquire()
	go func() {
		ctr.Wait()
		ch <- false
	}()
	for range 10 {
		ctr.Acquire()
		ctr.Release()
	}
	ch <- true
	ctr.Release()
	assert.That(t, <-ch)
}
