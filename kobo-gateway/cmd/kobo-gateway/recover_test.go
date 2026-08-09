package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportFatalDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { reportFatal(errors.New("boom")) })
}

func TestGuardSwallowsPanic(_ *testing.T) {
	func() {
		defer guard("test")

		panic("boom")
	}()
	// Reaching here means guard recovered the panic instead of letting it
	// propagate.
}

func TestGuardNoopWithoutPanic(t *testing.T) {
	called := false

	func() {
		defer guard("test")

		called = true
	}()

	if !called {
		t.Fatal("function body did not run")
	}
}

func TestRecoverGoSwallowsPanic(t *testing.T) {
	ran := false

	recoverGo("test", func() {
		ran = true

		panic("boom")
	})

	if !ran {
		t.Fatal("fn did not run")
	}
}

func TestReportAndRepanicRepanics(t *testing.T) {
	defer func() {
		r := recover()
		if r != "boom" {
			t.Fatalf("expected re-panicked value %q, got %v", "boom", r)
		}
	}()

	func() {
		defer reportAndRepanic()

		panic("boom")
	}()
}

func TestReportAndRepanicNoopWithoutPanic(_ *testing.T) {
	func() {
		defer reportAndRepanic()
	}()
}
