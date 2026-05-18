package services

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

func (s *TaskService) Search(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	if tasks, ok, err := s.searchByShortcut(ctx, userID, query, workspaceID); ok {
		return tasks, err
	}
	return s.tasks.ListArchived(ctx, userID, query, workspaceID)
}

func (s *TaskService) SearchAll(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	if tasks, ok, err := s.searchByShortcut(ctx, userID, query, workspaceID); ok {
		return tasks, err
	}
	return s.tasks.SearchAll(ctx, userID, query, workspaceID)
}

// shortcutQueryPattern matches strings like "DCP1234" or "PROJ-42".
var shortcutQueryPattern = regexp.MustCompile(`^([A-Z]+)([0-9A-Za-z-]+)$`)

// searchByShortcut checks if query matches a configured URL shortcut
// (e.g. "DCP1234") and, if so, searches by the reconstructed link URL.
// Returns (tasks, true, err) when a shortcut matched; (nil, false, nil) otherwise.
func (s *TaskService) searchByShortcut(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, bool, error) {
	m := shortcutQueryPattern.FindStringSubmatch(strings.ToUpper(query))
	if m == nil {
		return nil, false, nil
	}
	shortcut := m[1]
	identifier := strings.ToUpper(query)[len(shortcut):]

	patterns, err := s.settings.GetURLPatterns(ctx, userID, workspaceID)
	if err != nil {
		return nil, false, err
	}
	if len(patterns) == 0 {
		return nil, false, nil
	}
	for _, p := range patterns {
		if strings.ToUpper(p.Shortcut) != shortcut {
			continue
		}
		linkURL := p.URLPrefix + identifier
		tasks, searchErr := s.tasks.SearchByLinkURL(
			ctx, userID, workspaceID, linkURL,
		)
		return tasks, true, searchErr
	}
	return nil, false, nil
}

func (s *TaskService) detectURLLink(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	input string,
) (string, string) {
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		return "", ""
	}
	return input, s.matchURLPattern(ctx, userID, workspaceID, input)
}

func (s *TaskService) matchURLPattern(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	rawURL string,
) string {
	patterns, err := s.settings.GetURLPatterns(ctx, userID, workspaceID)
	if err != nil || len(patterns) == 0 {
		return ""
	}
	for _, p := range patterns {
		if strings.HasPrefix(rawURL, p.URLPrefix) {
			return p.PlatformName
		}
	}
	return ""
}

// resolveShortcutBadge returns the shortcut ID string for a URL when it matches
// a configured pattern with a non-empty shortcut (e.g. "DCP1234"), else "".
func resolveShortcutBadge(rawURL string, patterns []models.URLPattern) string {
	for _, p := range patterns {
		if p.Shortcut == "" || !strings.HasPrefix(rawURL, p.URLPrefix) {
			continue
		}
		return p.Shortcut + rawURL[len(p.URLPrefix):]
	}
	return ""
}

// enrichWithShortcuts fills ShortcutBadge on every link in tasks whose URL
// matches a configured URL pattern that has a non-empty Shortcut field.
func (s *TaskService) enrichWithShortcuts(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	tasks []models.Task,
) []models.Task {
	patterns, err := s.settings.GetURLPatterns(ctx, userID, workspaceID)
	if err != nil || len(patterns) == 0 {
		return tasks
	}
	for i := range tasks {
		links := tasks[i].Links
		for j, link := range links {
			links[j].ShortcutBadge = resolveShortcutBadge(link.URL, patterns)
		}
	}
	return tasks
}
