package jobs

import (
	"context"
	"log/slog"
	"time"
)

const (
	figureAnalysisInterval = 15 * time.Minute
	// figureAnalysisBatch bounds one run's downloads and parsing; the
	// existing library drains over successive runs.
	figureAnalysisBatch = 10
	// figureAnalysisDeadline allows for a batch of large PDFs; one batch has
	// taken about 14 minutes.
	figureAnalysisDeadline = time.Hour
)

type figureAnalyser interface {
	AnalysePending(ctx context.Context, limit int) (int, error)
}

// FigureAnalysisJob measures the figure density of not-yet-analysed book
// files, which drives the library's PDF-or-EPUB reading suggestion.
type FigureAnalysisJob struct {
	figures figureAnalyser
}

func NewFigureAnalysisJob(figures figureAnalyser) *FigureAnalysisJob {
	return &FigureAnalysisJob{figures: figures}
}

func (j *FigureAnalysisJob) ID() string { return "books-figure-analysis" }

func (j *FigureAnalysisJob) RunEvery() time.Duration { return figureAnalysisInterval }

func (j *FigureAnalysisJob) RunDeadline() time.Duration {
	return figureAnalysisDeadline
}

func (j *FigureAnalysisJob) Run(ctx context.Context, logger *slog.Logger) error {
	n, err := j.figures.AnalysePending(ctx, figureAnalysisBatch)
	if n > 0 {
		logger.InfoContext(ctx, "analysed book file figures", slog.Int("files", n))
	}
	return err
}
