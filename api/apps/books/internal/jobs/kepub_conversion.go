package jobs

import (
	"context"
	"log/slog"
	"time"
)

const kepubConversionInterval = time.Minute

type kepubDrainer interface {
	Drain(ctx context.Context) (int, error)
}

// KEPUBConversionJob converts the books queued by request handlers.
type KEPUBConversionJob struct {
	queue kepubDrainer
}

func NewKEPUBConversionJob(queue kepubDrainer) *KEPUBConversionJob {
	return &KEPUBConversionJob{queue: queue}
}

func (j *KEPUBConversionJob) ID() string { return "books-kepub-conversion" }

func (j *KEPUBConversionJob) RunEvery() time.Duration { return kepubConversionInterval }

func (j *KEPUBConversionJob) Run(ctx context.Context, logger *slog.Logger) error {
	n, err := j.queue.Drain(ctx)
	if n > 0 {
		logger.InfoContext(ctx, "drained kepub conversions", slog.Int("books", n))
	}
	return err
}
