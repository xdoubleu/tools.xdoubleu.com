package learningpaths

import "context"

// ActivePath is an unpaused learning path, for the Today page.
type ActivePath struct {
	ID    string
	Title string
}

// ListActivePaths returns up to limit of userID's unpaused learning paths.
func (app *LearningPaths) ListActivePaths(
	ctx context.Context,
	userID string,
	limit int32,
) ([]ActivePath, error) {
	paths, _, err := app.services.LearningPaths.List(ctx, userID, limit, 0)
	if err != nil {
		return nil, err
	}

	out := make([]ActivePath, 0, len(paths))
	for _, p := range paths {
		if p.Paused {
			continue
		}
		out = append(out, ActivePath{ID: p.ID.String(), Title: p.Title})
	}
	return out, nil
}
