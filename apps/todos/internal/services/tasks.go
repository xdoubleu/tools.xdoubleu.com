package services

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

type TaskService struct {
	tasks    *repositories.TasksRepository
	settings *repositories.SettingsRepository
	sections *repositories.SectionsRepository
}

func (s *TaskService) ListOpen(
	ctx context.Context,
	userID string,
	sectionID *uuid.UUID,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	tasks, err := s.tasks.ListOpen(ctx, userID, sectionID, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.attachSubtasks(ctx, tasks)
}

func (s *TaskService) List(
	ctx context.Context,
	userID string,
	status string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	tasks, err := s.tasks.ListByStatus(ctx, userID, status, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.attachSubtasks(ctx, tasks)
}

func (s *TaskService) Search(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	return s.tasks.ListArchived(ctx, userID, query, workspaceID)
}

func (s *TaskService) SearchAll(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	return s.tasks.SearchAll(ctx, userID, query, workspaceID)
}

func (s *TaskService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Task, error) {
	return s.tasks.GetByID(ctx, id, userID)
}

func (s *TaskService) Create(
	ctx context.Context,
	userID string,
	dto dtos.SaveTaskDto,
) (*models.Task, error) {
	//nolint:exhaustruct // ID, Status, timestamps set by DB
	t := models.Task{
		OwnerUserID: userID,
		Title:       dto.Title,
		Description: dto.Description,
		SetupLabel:  dto.SetupLabel,
		TypeLabel:   dto.TypeLabel,
		Priority:    dto.Priority,
		RecurDays:   dto.RecurDays,
		DueDate:     parseDatePtr(dto.DueDate),
		SectionID:   parseSectionID(dto.SectionID),
	}
	created, err := s.tasks.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	links := dtoToLinks(dto, created.ID)
	if err = s.tasks.ReplaceLinks(ctx, created.ID, links); err != nil {
		return nil, err
	}
	created.Links = links
	return created, nil
}

func (s *TaskService) Update(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	dto dtos.SaveTaskDto,
) error {
	existing, err := s.tasks.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	existing.Title = dto.Title
	existing.Description = dto.Description
	existing.SetupLabel = dto.SetupLabel
	existing.TypeLabel = dto.TypeLabel
	existing.Priority = dto.Priority
	existing.RecurDays = dto.RecurDays
	existing.DueDate = parseDatePtr(dto.DueDate)
	existing.SectionID = parseSectionID(dto.SectionID)
	if err = s.tasks.Update(ctx, *existing); err != nil {
		return err
	}
	return s.tasks.ReplaceLinks(ctx, id, dtoToLinks(dto, id))
}

func (s *TaskService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.tasks.Delete(ctx, id, userID)
}

func (s *TaskService) Complete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	task, err := s.tasks.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err = s.tasks.SetStatus(
		ctx, id, userID, models.StatusDone, &now, nil,
	); err != nil {
		return err
	}
	if task.RecurDays <= 0 {
		return nil
	}
	d := now.AddDate(0, 0, task.RecurDays)
	due := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
	//nolint:exhaustruct // ID/Status/timestamps set by DB
	newTask := models.Task{
		OwnerUserID: task.OwnerUserID,
		Title:       task.Title,
		Description: task.Description,
		SetupLabel:  task.SetupLabel,
		TypeLabel:   task.TypeLabel,
		Priority:    task.Priority,
		RecurDays:   task.RecurDays,
		DueDate:     &due,
		SectionID:   task.SectionID,
		WorkspaceID: task.WorkspaceID,
	}
	created, err := s.tasks.Create(ctx, newTask)
	if err != nil {
		return err
	}
	if len(task.Links) == 0 {
		return nil
	}
	links := make([]models.TaskLink, len(task.Links))
	for i, l := range task.Links {
		//nolint:exhaustruct // ID set by DB
		links[i] = models.TaskLink{
			TaskID:    created.ID,
			URL:       l.URL,
			Label:     l.Label,
			SortOrder: l.SortOrder,
		}
	}
	return s.tasks.ReplaceLinks(ctx, created.ID, links)
}

func (s *TaskService) Reopen(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.tasks.SetStatus(ctx, id, userID, models.StatusOpen, nil, nil)
}

func (s *TaskService) Reorder(
	ctx context.Context,
	userID string,
	ids []uuid.UUID,
) error {
	return s.tasks.ReorderTasks(ctx, userID, ids)
}

func (s *TaskService) AddSubtask(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	title string,
) error {
	if title == "" {
		return &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Subtask title cannot be empty",
		}
	}
	return s.tasks.AddSubtask(ctx, taskID, userID, title)
}

func (s *TaskService) ToggleSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
) error {
	return s.tasks.ToggleSubtask(ctx, id, taskID, userID)
}

func (s *TaskService) DeleteSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
) error {
	return s.tasks.DeleteSubtask(ctx, id, taskID, userID)
}

// QuickAdd creates a task from a plain title or URL, parsing Todoist-style
// shortcuts: #section, @type, /setup, p1/p2/p3, natural-language due dates.
func (s *TaskService) QuickAdd(
	ctx context.Context,
	userID string,
	input string,
	workspaceID *uuid.UUID,
) error {
	sectionList, err := s.sections.ListByUser(ctx, userID, workspaceID)
	if err != nil {
		return err
	}

	title, parsedDTO := parseQuickInput(input, sectionList, time.Now())

	linkURL, linkLabel := s.detectURLLink(ctx, userID, workspaceID, title)
	if linkURL != "" {
		title = urlToTitle(linkURL)
	}

	var links []models.TaskLink
	if linkURL != "" {
		//nolint:exhaustruct // TaskID/ID/SortOrder set by DB or caller
		links = []models.TaskLink{{URL: linkURL, Label: linkLabel}}
	}

	//nolint:exhaustruct // ID/Status/timestamps set by DB
	t := models.Task{
		OwnerUserID: userID,
		Title:       title,
		TypeLabel:   parsedDTO.TypeLabel,
		SetupLabel:  parsedDTO.SetupLabel,
		Priority:    parsedDTO.Priority,
		RecurDays:   parsedDTO.RecurDays,
		DueDate:     parseDatePtr(parsedDTO.DueDate),
		SectionID:   parseSectionID(parsedDTO.SectionID),
		WorkspaceID: workspaceID,
	}
	created, err := s.tasks.Create(ctx, t)
	if err != nil {
		return err
	}
	if len(links) > 0 {
		links[0].TaskID = created.ID
		return s.tasks.ReplaceLinks(ctx, created.ID, links)
	}
	return nil
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

// ── quick-add input parser ────────────────────────────────────────────────────

// parseQuickInput extracts Todoist-style shortcuts from a raw input string and
// returns the cleaned title plus a partially-filled SaveTaskDto.
func parseQuickInput(
	input string,
	sections []models.Section,
	now time.Time,
) (string, dtos.SaveTaskDto) {
	var dto dtos.SaveTaskDto
	tokens := strings.Fields(input)
	var titleTokens []string

	for _, tok := range tokens {
		switch {
		case tok == "p1":
			dto.Priority = models.PriorityP1
		case tok == "p2":
			dto.Priority = models.PriorityP2
		case tok == "p3":
			dto.Priority = models.PriorityP3
		case strings.HasPrefix(tok, "@") && len(tok) > 1:
			dto.TypeLabel = tok[1:]
		case strings.HasPrefix(tok, "/") && len(tok) > 1:
			dto.SetupLabel = tok[1:]
		case strings.HasPrefix(tok, "~") && len(tok) > 1:
			if n, atoiErr := strconv.Atoi(tok[1:]); atoiErr == nil && n > 0 {
				dto.RecurDays = n
			} else {
				titleTokens = append(titleTokens, tok)
			}
		case strings.HasPrefix(tok, "#") && len(tok) > 1:
			if sec := findSection(sections, tok[1:]); sec != nil {
				dto.SectionID = sec.ID.String()
			} else {
				titleTokens = append(titleTokens, tok)
			}
		default:
			titleTokens = append(titleTokens, tok)
		}
	}

	title := strings.Join(titleTokens, " ")
	title, due := parseDateFromTitle(title, now)
	if due != nil {
		dto.DueDate = due.Format("2006-01-02")
	}
	return strings.TrimSpace(title), dto
}

func findSection(sections []models.Section, name string) *models.Section {
	for i := range sections {
		if strings.EqualFold(sections[i].Name, name) {
			return &sections[i]
		}
	}
	return nil
}

// weekdayPattern matches "next <weekday>" with an optional "at HH" or "at HH:MM".
//

var weekdayPattern = regexp.MustCompile(
	`(?i)\bnext\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)` +
		`(?:\s+at\s+(\d{1,2})(?::(\d{2}))?)?\b`,
)

func weekdayByName(name string) (time.Weekday, bool) {
	m := map[string]time.Weekday{
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
		"sunday":    time.Sunday,
	}
	wd, ok := m[strings.ToLower(name)]
	return wd, ok
}

// parseDateFromTitle extracts a natural-language date phrase from the title and
// returns the cleaned title and parsed date. Supported: "today", "tomorrow",
// "next <weekday>", "next <weekday> at HH", "next <weekday> at HH:MM".
func parseDateFromTitle(title string, now time.Time) (string, *time.Time) {
	lower := strings.ToLower(title)

	if loc := weekdayPattern.FindStringIndex(lower); loc != nil {
		m := weekdayPattern.FindStringSubmatch(lower)
		wd, _ := weekdayByName(m[1])
		d := nextWeekday(now, wd)
		cleaned := strings.TrimSpace(title[:loc[0]] + title[loc[1]:])
		return cleaned, &d
	}

	if idx := strings.Index(lower, "tomorrow"); idx >= 0 {
		d := now.AddDate(0, 0, 1)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		cleaned := strings.TrimSpace(title[:idx] + title[idx+len("tomorrow"):])
		return cleaned, &t
	}

	if idx := strings.Index(lower, "today"); idx >= 0 {
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cleaned := strings.TrimSpace(title[:idx] + title[idx+len("today"):])
		return cleaned, &t
	}

	return title, nil
}

func nextWeekday(from time.Time, wd time.Weekday) time.Time {
	days := int(wd) - int(from.Weekday())
	if days <= 0 {
		days += 7
	}
	d := from.AddDate(0, 0, days)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, from.Location())
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *TaskService) attachSubtasks(
	ctx context.Context,
	tasks []models.Task,
) ([]models.Task, error) {
	if len(tasks) == 0 {
		return tasks, nil
	}
	ids := make([]uuid.UUID, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	subtaskMap, err := s.tasks.ListSubtasksForTasks(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		tasks[i].Subtasks = subtaskMap[tasks[i].ID]
	}
	return tasks, nil
}

func urlToTitle(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parts := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return rawURL
}

func parseDatePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func parseSectionID(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

func dtoToLinks(dto dtos.SaveTaskDto, taskID uuid.UUID) []models.TaskLink {
	var links []models.TaskLink
	for i, u := range dto.LinkURLs {
		if u == "" {
			continue
		}
		label := ""
		if i < len(dto.LinkLabels) {
			label = dto.LinkLabels[i]
		}
		//nolint:exhaustruct // ID set by DB
		links = append(links, models.TaskLink{
			TaskID:    taskID,
			URL:       u,
			Label:     label,
			SortOrder: i,
		})
	}
	return links
}
