package swaparoo

import "testing"

func TestCounter(t *testing.T) {
	var ctr counter
	ctr.Wait()
	for i := 0; i < 10; i++ {
		ctr.Acquire()
		go ctr.Release()
	}
	ctr.Wait()
}
