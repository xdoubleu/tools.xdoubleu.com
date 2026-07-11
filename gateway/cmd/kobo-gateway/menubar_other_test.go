//go:build !darwin

package main

import (
	"testing"
	"time"
)

func TestRunUIStubReturnsOnStop(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		runUI("dev", stop)
		close(done)
	}()

	close(stop)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runUI did not return after stop closed")
	}
}
