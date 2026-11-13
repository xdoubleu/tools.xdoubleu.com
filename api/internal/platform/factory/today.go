package factory

import (
	"context"
	"fmt"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// AttentionTask is a task waiting on a person, for the Today page.
type AttentionTask struct {
	Ref    string
	Title  string
	Status string
}

// ListAttentionTasks returns up to limit tasks that need you or are in testing.
func (app *Factory) ListAttentionTasks(
	ctx context.Context,
	limit int,
) ([]AttentionTask, error) {
	tasks, err := app.services.Tasks.List(ctx, "")
	if err != nil {
		return nil, err
	}

	out := make([]AttentionTask, 0, limit)
	for _, t := range tasks {
		if t.Status != models.StatusNeedsYou && t.Status != models.StatusTesting {
			continue
		}
		if len(out) == limit {
			break
		}
		out = append(out, AttentionTask{
			Ref:    fmt.Sprintf("T-%d", t.Number),
			Title:  t.Title,
			Status: string(t.Status),
		})
	}
	return out, nil
}
