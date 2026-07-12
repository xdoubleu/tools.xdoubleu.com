//go:build !darwin

package main

import (
	"testing"
	"time"

	"tools.xdoubleu.com/gateway/internal/kobogateway"
)

func TestRunUIStubReturnsOnStop(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	events := make(chan kobogateway.KoboEvent)

	go func() {
		runUI("dev", stop, events, "/home/test", "/usr/local/bin/kobo-gateway")
		close(done)
	}()

	close(stop)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runUI did not return after stop closed")
	}
}
