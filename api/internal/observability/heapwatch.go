package observability

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	heapProfileTopN = 6
	// heapStackFrames keeps a stack's innermost frames; deeper ones are noise.
	heapStackFrames = 5
	// heapFrameFields is a frame line's "#", address, function and file:line.
	heapFrameFields = 4
)

// WatchHeap logs the biggest in-use allocation stacks the first time heap in
// use exceeds threshold bytes, so an OOM-killed process (which logs nothing)
// leaves evidence of what grew. It fires once per process and returns.
func WatchHeap(
	ctx context.Context,
	logger *slog.Logger,
	threshold uint64,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		if ms.HeapInuse < threshold {
			continue
		}
		var buf bytes.Buffer
		err := pprof.Lookup("heap").WriteTo(&buf, 1)
		if err != nil {
			logger.WarnContext(ctx, "heap watchdog: profile failed", "err", err)
			return
		}
		logger.WarnContext(ctx, "heap watchdog: heap in use above threshold",
			slog.Uint64("heap_inuse_bytes", ms.HeapInuse),
			slog.Uint64("threshold_bytes", threshold),
			slog.Int("goroutines", runtime.NumGoroutine()),
			slog.Any("top_stacks", topHeapStacks(buf.String(), heapProfileTopN)),
		)
		return
	}
}

// topHeapStacks reads pprof's legacy text heap profile ("N: B [N: B] @ ..."
// followed by "#\t0xaddr\tfunc+0xoff\tfile:line" frame lines) and returns the n
// stacks holding the most in-use bytes as "bytes: frame < frame ...".
func topHeapStacks(profile string, n int) []string {
	type stack struct {
		bytes  int64
		frames []string
	}
	var stacks []stack
	for _, line := range strings.Split(profile, "\n") {
		if strings.HasPrefix(line, "#") {
			if len(stacks) == 0 {
				continue
			}
			s := &stacks[len(stacks)-1]
			if len(s.frames) >= heapStackFrames {
				continue
			}
			if fields := strings.Split(line, "\t"); len(fields) >= heapFrameFields {
				s.frames = append(s.frames, fields[2]+" "+fields[3])
			}
			continue
		}
		head, _, ok := strings.Cut(line, " @ ")
		if !ok {
			continue
		}
		// head is "count: bytes [count: bytes]"; the first pair is in use.
		_, rest, _ := strings.Cut(head, ": ")
		b, _, _ := strings.Cut(rest, " ")
		size, err := strconv.ParseInt(b, 10, 64)
		if err != nil {
			continue
		}
		stacks = append(stacks, stack{bytes: size, frames: nil})
	}
	sort.SliceStable(stacks, func(i, j int) bool {
		return stacks[i].bytes > stacks[j].bytes
	})
	out := make([]string, 0, n)
	for _, s := range stacks[:min(n, len(stacks))] {
		out = append(out, strconv.FormatInt(s.bytes, 10)+": "+
			strings.Join(s.frames, " < "))
	}
	return out
}
