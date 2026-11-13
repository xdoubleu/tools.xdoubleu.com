package threading

import (
	"context"
	"sync"
)

// inflight counts running work. Unlike sync.WaitGroup, work may start while
// a wait is in progress, and a wait can give up.
type inflight struct {
	mu   sync.Mutex
	n    int
	idle chan struct{} // closed when n drops to 0; nil while nobody waits
}

func newInflight() *inflight {
	//nolint:exhaustruct //mu, n and idle start zero-valued
	return &inflight{}
}

func (f *inflight) add() {
	f.mu.Lock()
	f.n++
	f.mu.Unlock()
}

func (f *inflight) done() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.n--
	if f.n == 0 && f.idle != nil {
		close(f.idle)
		f.idle = nil
	}
}

// wait blocks until no work is running or ctx is done.
func (f *inflight) wait(ctx context.Context) error {
	f.mu.Lock()
	if f.n == 0 {
		f.mu.Unlock()
		return nil
	}
	if f.idle == nil {
		f.idle = make(chan struct{})
	}
	idle := f.idle
	f.mu.Unlock()

	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
