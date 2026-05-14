package services

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

const (
	ordinalSecond  = 2
	ordinalThird   = 3
	ordinalFourth  = 4
	ordinalFifth   = 5
	daysInWeek     = 7
	twoRuleParts   = 2
	threeRuleParts = 3
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
	tasks, err = s.attachSubtasks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	tasks, err = s.attachLinks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	return s.enrichWithShortcuts(ctx, userID, workspaceID, tasks), nil
}

func (s *TaskService) CountOpenPerSection(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) (map[string]int, error) {
	return s.tasks.CountOpenPerSection(ctx, userID, workspaceID)
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
	tasks, err = s.attachSubtasks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	tasks, err = s.attachLinks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	return s.enrichWithShortcuts(ctx, userID, workspaceID, tasks), nil
}

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

func (s *TaskService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Task, error) {
	task, err := s.tasks.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	// Build subtask tree and recalculate counts
	task.Subtasks = buildSubtaskTree(task.Subtasks)
	task.SubtaskTotal = countSubtasksRecursive(task.Subtasks)
	task.SubtaskDone = countDoneSubtasksRecursive(task.Subtasks)

	if len(task.Links) > 0 {
		tasks := s.enrichWithShortcuts(
			ctx, userID, task.WorkspaceID, []models.Task{*task},
		)
		*task = tasks[0]
	}
	return task, nil
}

func (s *TaskService) Create(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	dto dtos.SaveTaskDto,
) (*models.Task, error) {
	dueDate, deadline, recurDays, recurRule, err := parseScheduleDTO(dto, time.Now())
	if err != nil {
		return nil, err
	}

	labels := s.normalizeAndAddLabels(
		ctx,
		userID,
		workspaceID,
		parseLabelsInput(dto.Label),
	)

	//nolint:exhaustruct // ID, Status, timestamps set by DB
	t := models.Task{
		OwnerUserID: userID,
		Title:       dto.Title,
		Description: dto.Description,
		Labels:      labels,
		Priority:    dto.Priority,
		RecurDays:   recurDays,
		RecurRule:   recurRule,
		DueDate:     dueDate,
		Deadline:    deadline,
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
	workspaceID *uuid.UUID,
	dto dtos.SaveTaskDto,
) error {
	dueDate, deadline, recurDays, recurRule, err := parseScheduleDTO(dto, time.Now())
	if err != nil {
		return err
	}

	labels := s.normalizeAndAddLabels(
		ctx,
		userID,
		workspaceID,
		parseLabelsInput(dto.Label),
	)

	existing, err := s.tasks.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	existing.Title = dto.Title
	existing.Description = dto.Description
	existing.Labels = labels
	existing.Priority = dto.Priority
	existing.RecurDays = recurDays
	existing.RecurRule = recurRule
	existing.DueDate = dueDate
	existing.Deadline = deadline
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

func (s *TaskService) MoveSection(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	sectionID *uuid.UUID,
) error {
	return s.tasks.MoveSection(ctx, id, userID, sectionID)
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
	if task.RecurDays <= 0 && task.RecurRule == "" {
		return nil
	}
	due, recurDays := nextRecurringDue(now, task.RecurRule, task.RecurDays)
	if due == nil {
		return nil
	}
	//nolint:exhaustruct // ID/Status/timestamps set by DB
	newTask := models.Task{
		OwnerUserID: task.OwnerUserID,
		Title:       task.Title,
		Description: task.Description,
		Labels:      task.Labels,
		Priority:    task.Priority,
		RecurDays:   recurDays,
		RecurRule:   task.RecurRule,
		DueDate:     due,
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
	workspaceID *uuid.UUID,
	input string,
	description string,
	parentSubtaskID *uuid.UUID,
) (*models.Subtask, error) {
	if strings.TrimSpace(input) == "" {
		return nil, &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Subtask title cannot be empty",
		}
	}
	title, dto := parseQuickInput(input, nil, time.Now())
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(input)
	}
	labels := []string{}
	if dto.Label != "" {
		labels = s.normalizeAndAddLabels(
			ctx, userID, workspaceID, parseLabelsInput(dto.Label),
		)
	}

	if parentSubtaskID != nil {
		depth, err := s.getSubtaskDepth(ctx, taskID, *parentSubtaskID)
		if err != nil {
			return nil, err
		}
		const maxSubtaskParentDepth = 2
		if depth >= maxSubtaskParentDepth {
			return nil, &HTTPError{
				Status:  http.StatusUnprocessableEntity,
				Message: "Maximum subtask depth (3) reached",
			}
		}
	}

	return s.tasks.AddSubtask(
		ctx, taskID, userID,
		title, strings.TrimSpace(description),
		dto.Priority, labels,
		parseDatePtr(dto.DueDate),
		parseDatePtr(dto.Deadline),
		parentSubtaskID,
	)
}

func (s *TaskService) UpdateSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
	workspaceID *uuid.UUID,
	dto dtos.UpdateSubtaskDto,
) (*models.Subtask, error) {
	if strings.TrimSpace(dto.Title) == "" {
		return nil, &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Subtask title cannot be empty",
		}
	}
	labels := []string{}
	if dto.Label != "" {
		labels = s.normalizeAndAddLabels(
			ctx, userID, workspaceID, parseLabelsInput(dto.Label),
		)
	}
	return s.tasks.UpdateSubtask(
		ctx, id, taskID, userID,
		strings.TrimSpace(dto.Title),
		strings.TrimSpace(dto.Description),
		dto.Priority, labels,
		parseDatePtr(dto.DueDate),
		parseDatePtr(dto.Deadline),
	)
}

func (s *TaskService) ReorderSubtasks(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	ids []uuid.UUID,
	parentSubtaskID *uuid.UUID,
) error {
	return s.tasks.ReorderSubtasks(ctx, taskID, userID, ids, parentSubtaskID)
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

// QuickAdd creates a task from a plain title, fancy Markdown link [Title](URL),
// or bare URL, parsing Todoist-style shortcuts: #section, @type, /setup,
// p1/p2/p3, natural-language due dates.
func (s *TaskService) QuickAdd(
	ctx context.Context,
	userID string,
	input string,
	description string,
	workspaceID *uuid.UUID,
	fallbackSectionID string,
) (*models.Task, error) {
	sectionList, err := s.sections.ListByUser(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}

	var title string
	var linkURL, linkLabel string
	var parsedDTO dtos.SaveTaskDto

	if fancyTitle, fancyURL, rest, ok := parseFancyURL(input); ok {
		title = fancyTitle
		linkURL = fancyURL
		linkLabel = s.matchURLPattern(ctx, userID, workspaceID, fancyURL)
		sectionList = s.ensureSections(ctx, userID, workspaceID, rest, sectionList)
		_, parsedDTO = parseQuickInput(rest, sectionList, time.Now())
	} else {
		sectionList = s.ensureSections(ctx, userID, workspaceID, input, sectionList)
		title, parsedDTO = parseQuickInput(input, sectionList, time.Now())
		linkURL, linkLabel = s.detectURLLink(ctx, userID, workspaceID, title)
		if linkURL != "" {
			title = urlToTitle(linkURL)
		}
	}

	if parsedDTO.Label != "" {
		normalized := s.normalizeAndAddLabels(
			ctx, userID, workspaceID, parseLabelsInput(parsedDTO.Label),
		)
		parsedDTO.Label = strings.Join(normalized, ",")
	}

	recurDays := parsedDTO.RecurDays
	recurRule := ""
	if parsedDTO.Recur != "" {
		if rd, rr, parseErr := parseRecurOnly(parsedDTO.Recur, time.Now()); parseErr == nil {
			recurDays = rd
			recurRule = rr
		}
	} else if recurDays > 0 {
		recurRule = "days:" + strconv.Itoa(recurDays)
	}

	var links []models.TaskLink
	if linkURL != "" {
		//nolint:exhaustruct // TaskID/ID/SortOrder set by DB or caller
		links = []models.TaskLink{{URL: linkURL, Label: linkLabel}}
	}

	sectionID := parseSectionID(parsedDTO.SectionID)
	if sectionID == nil {
		sectionID = parseSectionID(fallbackSectionID)
	}

	//nolint:exhaustruct // ID/Status/timestamps set by DB
	t := models.Task{
		OwnerUserID: userID,
		Title:       title,
		Description: strings.TrimSpace(description),
		Labels:      parseLabelsInput(parsedDTO.Label),
		Priority:    parsedDTO.Priority,
		RecurDays:   recurDays,
		RecurRule:   recurRule,
		DueDate:     parseDatePtr(parsedDTO.DueDate),
		SectionID:   sectionID,
		WorkspaceID: workspaceID,
	}
	created, err := s.tasks.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	if len(links) > 0 {
		links[0].TaskID = created.ID
		if err = s.tasks.ReplaceLinks(ctx, created.ID, links); err != nil {
			return nil, err
		}
		created.Links = links
	}
	return created, nil
}

// QuickUpdate updates an existing task from a quick-add style input string.
// Title, priority, labels, due date, deadline, section and recur are parsed
// from input. Description is taken from the desc argument. Links are preserved.
func (s *TaskService) QuickUpdate(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	workspaceID *uuid.UUID,
	input string,
	desc string,
) (*models.Task, error) {
	existing, err := s.tasks.GetByID(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}
	sections, err := s.sections.ListByUser(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	title, parsedDTO := parseQuickInput(input, sections, time.Now())
	if strings.TrimSpace(title) == "" {
		title = existing.Title
	}
	labels := []string{}
	if parsedDTO.Label != "" {
		labels = s.normalizeAndAddLabels(
			ctx, userID, workspaceID, parseLabelsInput(parsedDTO.Label),
		)
	}
	recurDays := parsedDTO.RecurDays
	recurRule := ""
	if parsedDTO.Recur != "" {
		if rd, rr, parseErr := parseRecurOnly(parsedDTO.Recur, time.Now()); parseErr == nil {
			recurDays = rd
			recurRule = rr
		}
	} else if recurDays > 0 {
		recurRule = "days:" + strconv.Itoa(recurDays)
	}
	existing.Title = title
	existing.Description = strings.TrimSpace(desc)
	existing.Labels = labels
	existing.Priority = parsedDTO.Priority
	existing.DueDate = parseDatePtr(parsedDTO.DueDate)
	existing.Deadline = parseDatePtr(parsedDTO.Deadline)
	existing.RecurDays = recurDays
	existing.RecurRule = recurRule
	if parsedDTO.SectionID != "" {
		existing.SectionID = parseSectionID(parsedDTO.SectionID)
	} else {
		existing.SectionID = nil
	}
	if err = s.tasks.Update(ctx, *existing); err != nil {
		return nil, err
	}
	return s.tasks.GetByID(ctx, taskID, userID)
}

func (s *TaskService) ensureSections(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	input string,
	sections []models.Section,
) []models.Section {
	for _, tok := range strings.Fields(input) {
		if !strings.HasPrefix(tok, "#") || len(tok) < 2 {
			continue
		}
		name := tok[1:]
		if findSection(sections, name) != nil {
			continue
		}
		//nolint:exhaustruct // SortOrder/ID/CreatedAt set by DB
		newSec, err := s.sections.Create(ctx, models.Section{
			OwnerUserID: userID,
			Name:        name,
			WorkspaceID: workspaceID,
		})
		if err == nil {
			sections = append(sections, *newSec)
		}
	}
	return sections
}

func parseLabelsInput(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			result = append(result, v)
		}
	}
	return result
}

func (s *TaskService) normalizeAndAddLabels(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	labels []string,
) []string {
	if len(labels) == 0 {
		return []string{}
	}
	presets, err := s.settings.GetLabelPresets(ctx, userID, workspaceID)
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		if label == "" {
			continue
		}
		if err == nil {
			for _, p := range presets.Labels {
				if strings.EqualFold(p.Value, label) {
					label = p.Value
					goto next
				}
			}
		}
		_ = s.settings.AddLabelPreset(
			ctx,
			userID,
			models.LabelCategory,
			label,
			workspaceID,
		)
	next:
		result = append(result, label)
	}
	return result
}

// FormatRecurRule converts a stored recur rule to its human-readable form,
// falling back to "every N days" if only a day-count is available.
func (s *TaskService) FormatRecurRule(rule string, fallbackDays int) string {
	if rule != "" {
		if r := recurRuleToInput(rule); r != "" {
			return r
		}
	}
	if fallbackDays > 0 {
		return "every " + strconv.Itoa(fallbackDays) + " days"
	}
	return ""
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

// ── quick-add input parser ────────────────────────────────────────────────────

// fancyURLPattern matches Edge-style "Copy as link": [Title](https://…)
// with an optional trailing string of shortcuts.
var fancyURLPattern = regexp.MustCompile(
	`^\[(.+)\]\(((?:https?://)?[^\s\)]+)\)(.*)$`,
)

// parseFancyURL detects a Markdown link at the start of input and returns
// (title, url, rest, true) when matched, or ("","","",false) otherwise.
func parseFancyURL(input string) (string, string, string, bool) {
	m := fancyURLPattern.FindStringSubmatch(strings.TrimSpace(input))
	if m == nil {
		return "", "", "", false
	}
	u := m[2]
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	return strings.TrimSpace(m[1]), u, strings.TrimSpace(m[3]), true
}

// parseDeadlineTok parses a "!<date>" token from the quick-add input.
// skip is the number of extra tokens consumed (0 or 1 for "!next <weekday>").
// ok is false when the token is not a valid deadline and should be kept as title text.
func parseDeadlineTok(
	tok string, tokens []string, i int, now time.Time,
) (string, int, bool) {
	raw := tok[1:]
	skip := 0
	if strings.EqualFold(raw, "next") && i+1 < len(tokens) {
		raw += " " + tokens[i+1]
		skip = 1
	}
	if d, _, err := parseHumanDate(raw, now, false); err == nil {
		return d.Format("2006-01-02"), skip, true
	}
	if d, err := time.Parse("2006-01-02", tok[1:]); err == nil {
		return d.Format("2006-01-02"), skip, true
	}
	return "", skip, false
}

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
	var labelTokens []string

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok == "p1":
			dto.Priority = models.PriorityP1
		case tok == "p2":
			dto.Priority = models.PriorityP2
		case tok == "p3":
			dto.Priority = models.PriorityP3
		case strings.HasPrefix(tok, "@") && len(tok) > 1:
			labelTokens = append(labelTokens, tok[1:])
		case strings.HasPrefix(tok, "~") && len(tok) > 1:
			titleTokens = append(titleTokens, tok)
		case strings.HasPrefix(tok, "#") && len(tok) > 1:
			if sec := findSection(sections, tok[1:]); sec != nil {
				dto.SectionID = sec.ID.String()
			} else {
				titleTokens = append(titleTokens, tok)
			}
		case strings.HasPrefix(tok, "!") && len(tok) > 1:
			dl, skip, dlOK := parseDeadlineTok(tok, tokens, i, now)
			i += skip
			if dlOK {
				dto.Deadline = dl
			} else {
				titleTokens = append(titleTokens, tok)
			}
		case isoDatePattern.MatchString(tok) && dto.DueDate == "":
			dto.DueDate = tok
		default:
			titleTokens = append(titleTokens, tok)
		}
	}

	dto.Label = strings.Join(labelTokens, ",")
	title := strings.Join(titleTokens, " ")
	title, due, recurRule, recurDays := parseDateFromTitle(title, now)
	if due != nil {
		dto.DueDate = due.Format("2006-01-02")
	}
	if recurRule != "" {
		dto.Recur = recurRuleToInput(recurRule)
		dto.RecurDays = recurDays
	}
	return strings.TrimSpace(title), dto
}

func parsePositiveInt(s string) (int, bool) {
	n, atoiErr := strconv.Atoi(s)
	return n, atoiErr == nil && n > 0
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

var recurringInTitlePattern = regexp.MustCompile(
	`(?i)\bevery\s+((first|second|third|fourth|fifth|last)\s+)?` +
		`(monday|tuesday|wednesday|thursday|friday|saturday|sunday|\d+\s+days?)\b`,
)

// parseDateFromTitle extracts a natural-language due date phrase from title and
// returns cleaned title, due date, and optional recurrence metadata.
func parseDateFromTitle(title string, now time.Time) (string, *time.Time, string, int) {
	lower := strings.ToLower(title)

	if loc := recurringInTitlePattern.FindStringIndex(lower); loc != nil {
		phrase := strings.TrimSpace(title[loc[0]:loc[1]])
		if d, recurring, err := parseHumanDate(phrase, now, true); err == nil {
			cleaned := strings.TrimSpace(title[:loc[0]] + title[loc[1]:])
			return cleaned, d, recurring.recurRule, recurring.recurDays
		}
	}

	if loc := weekdayPattern.FindStringIndex(lower); loc != nil {
		m := weekdayPattern.FindStringSubmatch(lower)
		wd, _ := weekdayByName(m[1])
		d := nextWeekday(now, wd)
		cleaned := strings.TrimSpace(title[:loc[0]] + title[loc[1]:])
		return cleaned, &d, "", 0
	}

	if idx := strings.Index(lower, "tomorrow"); idx >= 0 {
		d := now.AddDate(0, 0, 1)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		cleaned := strings.TrimSpace(title[:idx] + title[idx+len("tomorrow"):])
		return cleaned, &t, "", 0
	}

	if idx := strings.Index(lower, "today"); idx >= 0 {
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cleaned := strings.TrimSpace(title[:idx] + title[idx+len("today"):])
		return cleaned, &t, "", 0
	}

	return title, nil, "", 0
}

func nextWeekday(from time.Time, wd time.Weekday) time.Time {
	days := int(wd) - int(from.Weekday())
	if days <= 0 {
		days += 7
	}
	d := from.AddDate(0, 0, days)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, from.Location())
}

func ordinalByName(name string) (int, bool) {
	m := map[string]int{
		"first":  1,
		"second": ordinalSecond,
		"third":  ordinalThird,
		"fourth": ordinalFourth,
		"fifth":  ordinalFifth,
		"last":   -1,
	}
	v, ok := m[strings.ToLower(name)]
	return v, ok
}

func ordinalToName(o int) string {
	switch o {
	case 1:
		return "first"
	case ordinalSecond:
		return "second"
	case ordinalThird:
		return "third"
	case ordinalFourth:
		return "fourth"
	case ordinalFifth:
		return "fifth"
	case -1:
		return "last"
	default:
		return ""
	}
}

func nthWeekdayOfMonth(
	year int,
	month time.Month,
	wd time.Weekday,
	ordinal int,
	loc *time.Location,
) (time.Time, bool) {
	if ordinal == 0 || ordinal < -1 || ordinal > 5 {
		return time.Time{}, false
	}
	if ordinal == -1 {
		last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc)
		diff := (int(last.Weekday()) - int(wd) + daysInWeek) % daysInWeek
		d := last.AddDate(0, 0, -diff)
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc), true
	}
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	diff := (int(wd) - int(first.Weekday()) + daysInWeek) % daysInWeek
	day := 1 + diff + (ordinal-1)*daysInWeek
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if day > lastDay {
		return time.Time{}, false
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc), true
}

func nextMonthlyWeekday(
	from time.Time,
	wd time.Weekday,
	ordinal int,
) (time.Time, bool) {
	base := time.Date(
		from.Year(),
		from.Month(),
		from.Day(),
		0,
		0,
		0,
		0,
		from.Location(),
	)
	for offset := 0; offset < 24; offset++ {
		m := from.AddDate(0, offset, 0)
		candidate, ok := nthWeekdayOfMonth(
			m.Year(), m.Month(), wd, ordinal, from.Location(),
		)
		if !ok {
			continue
		}
		if candidate.After(base) {
			return candidate, true
		}
	}
	return time.Time{}, false
}

func nextRecurringDue(now time.Time, rule string, fallbackDays int) (*time.Time, int) {
	if rule == "" {
		if fallbackDays <= 0 {
			return nil, 0
		}
		d := now.AddDate(0, 0, fallbackDays)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return &t, fallbackDays
	}
	parts := strings.Split(rule, ":")
	switch parts[0] {
	case "days":
		if len(parts) != twoRuleParts {
			return nil, fallbackDays
		}
		n, ok := parsePositiveInt(parts[1])
		if !ok {
			return nil, fallbackDays
		}
		d := now.AddDate(0, 0, n)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return &t, n
	case "weekday":
		if len(parts) != twoRuleParts {
			return nil, fallbackDays
		}
		w, err := strconv.Atoi(parts[1])
		if err != nil || w < 0 || w > 6 {
			return nil, fallbackDays
		}
		t := nextWeekday(now, time.Weekday(w))
		return &t, daysInWeek
	case "monthweekday":
		if len(parts) != threeRuleParts {
			return nil, fallbackDays
		}
		ordinal, err1 := strconv.Atoi(parts[1])
		w, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || w < 0 || w > 6 {
			return nil, fallbackDays
		}
		t, ok := nextMonthlyWeekday(now, time.Weekday(w), ordinal)
		if !ok {
			return nil, fallbackDays
		}
		return &t, 0
	default:
		return nil, fallbackDays
	}
}

func recurRuleToInput(rule string) string {
	parts := strings.Split(rule, ":")
	switch parts[0] {
	case "days":
		if len(parts) == twoRuleParts {
			return "every " + parts[1] + " days"
		}
	case "weekday":
		if len(parts) == twoRuleParts {
			if w, err := strconv.Atoi(parts[1]); err == nil && w >= 0 && w <= 6 {
				return "every " + strings.ToLower(time.Weekday(w).String())
			}
		}
	case "monthweekday":
		if len(parts) == threeRuleParts {
			o, err1 := strconv.Atoi(parts[1])
			w, err2 := strconv.Atoi(parts[2])
			if err1 == nil && err2 == nil && w >= 0 && w <= 6 {
				return "every " + ordinalToName(o) + " " +
					strings.ToLower(time.Weekday(w).String())
			}
		}
	}
	return ""
}

// ── helpers ───────────────────────────────────────────────────────────────────

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
		flat := subtaskMap[tasks[i].ID]
		tasks[i].Subtasks = buildSubtaskTree(flat)
		tasks[i].SubtaskTotal = countSubtasksRecursive(tasks[i].Subtasks)
		tasks[i].SubtaskDone = countDoneSubtasksRecursive(tasks[i].Subtasks)
	}
	return tasks, nil
}

// countSubtasksRecursive counts all subtasks in a tree (including nested ones).
func countSubtasksRecursive(subtasks []models.Subtask) int {
	count := len(subtasks)
	for _, s := range subtasks {
		count += countSubtasksRecursive(s.Children)
	}
	return count
}

// countDoneSubtasksRecursive counts done subtasks in a tree (including nested ones).
func countDoneSubtasksRecursive(subtasks []models.Subtask) int {
	count := 0
	for _, s := range subtasks {
		if s.Done {
			count++
		}
		count += countDoneSubtasksRecursive(s.Children)
	}
	return count
}

func (s *TaskService) attachLinks(
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
	linkMap, err := s.tasks.ListLinksForTasks(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		tasks[i].Links = linkMap[tasks[i].ID]
	}
	return tasks, nil
}

func urlToTitle(rawURL string) string {
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

type recurringParseResult struct {
	recurDays int
	recurRule string
}

var everyPattern = regexp.MustCompile(
	`(?i)^\s*every\s+((first|second|third|fourth|fifth|last)\s+)?([a-z]+|\d+\s+days?)\s*$`,
)

var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func parseScheduleDTO(
	dto dtos.SaveTaskDto,
	now time.Time,
) (*time.Time, *time.Time, int, string, error) {
	due, dueRecurring, err := parseHumanDate(dto.DueDate, now, true)
	if err != nil {
		return nil, nil, 0, "", &HTTPError{
			Status: http.StatusBadRequest,
			Message: "Invalid due date. Use e.g. today, tomorrow, next thursday," +
				" every thursday, or every first sunday.",
		}
	}
	deadline, _, err := parseHumanDate(dto.Deadline, now, false)
	if err != nil {
		return nil, nil, 0, "", &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid deadline. Use e.g. today, tomorrow, next thursday, or YYYY-MM-DD.",
		}
	}

	recurDays := dto.RecurDays
	recurRule := ""
	if strings.TrimSpace(dto.Recur) != "" {
		recurDays, recurRule, err = parseRecurOnly(dto.Recur, now)
		if err != nil {
			return nil, nil, 0, "", &HTTPError{
				Status: http.StatusBadRequest,
				Message: "Invalid recur value." +
					" Use e.g. every thursday, every first sunday, or every 7 days.",
			}
		}
	}
	if dueRecurring.recurDays > 0 && strings.TrimSpace(dto.Recur) == "" {
		recurDays = dueRecurring.recurDays
	}
	if dueRecurring.recurRule != "" && strings.TrimSpace(dto.Recur) == "" {
		recurRule = dueRecurring.recurRule
	}
	if recurRule == "" && recurDays > 0 {
		recurRule = "days:" + strconv.Itoa(recurDays)
	}

	return due, deadline, recurDays, recurRule, nil
}

func parseEveryDate(
	m []string,
	now time.Time,
) (*time.Time, recurringParseResult, error) {
	empty := recurringParseResult{recurDays: 0, recurRule: ""}
	ordinalWord := strings.ToLower(strings.TrimSpace(m[2]))
	everyBody := strings.ToLower(strings.TrimSpace(m[3]))
	wd, wdOK := weekdayByName(everyBody)
	if wdOK && ordinalWord == "" {
		d := nextWeekday(now, wd)
		return &d, recurringParseResult{
			recurDays: daysInWeek,
			recurRule: "weekday:" + strconv.Itoa(int(wd)),
		}, nil
	}
	if wdOK && ordinalWord != "" {
		ordinal, ordOK := ordinalByName(ordinalWord)
		if !ordOK {
			return nil, empty, strconv.ErrSyntax
		}
		d, monthlyOK := nextMonthlyWeekday(now, wd, ordinal)
		if !monthlyOK {
			return nil, empty, strconv.ErrSyntax
		}
		return &d, recurringParseResult{
			recurDays: 0,
			recurRule: "monthweekday:" + strconv.Itoa(ordinal) +
				":" + strconv.Itoa(int(wd)),
		}, nil
	}
	parts := strings.Fields(everyBody)
	if len(parts) == twoRuleParts && strings.HasPrefix(parts[1], "day") {
		if n, ok := parsePositiveInt(parts[0]); ok {
			d := now.AddDate(0, 0, n)
			t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
			return &t, recurringParseResult{
				recurDays: n,
				recurRule: "days:" + strconv.Itoa(n),
			}, nil
		}
	}
	return nil, empty, strconv.ErrSyntax
}

func parseHumanDate(
	input string,
	now time.Time,
	allowRecurring bool,
) (*time.Time, recurringParseResult, error) {
	empty := recurringParseResult{recurDays: 0, recurRule: ""}
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, empty, nil
	}

	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
		return &d, empty, nil
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case "today":
		d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &d, empty, nil
	case "tomorrow":
		d := now.AddDate(0, 0, 1)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return &t, empty, nil
	}

	if m := everyPattern.FindStringSubmatch(lower); m != nil {
		if !allowRecurring {
			return nil, empty, strconv.ErrSyntax
		}
		return parseEveryDate(m, now)
	}

	if wd, ok := weekdayByName(lower); ok {
		d := nextWeekday(now, wd)
		return &d, empty, nil
	}

	if m := weekdayPattern.FindStringSubmatch(lower); m != nil {
		wd, ok := weekdayByName(m[1])
		if !ok {
			return nil, empty, strconv.ErrSyntax
		}
		d := nextWeekday(now, wd)
		return &d, empty, nil
	}

	return nil, empty, strconv.ErrSyntax
}

func parseRecurOnly(input string, now time.Time) (int, string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, "", nil
	}
	if n, ok := parsePositiveInt(trimmed); ok {
		return n, "days:" + strconv.Itoa(n), nil
	}
	_, recurring, err := parseHumanDate(trimmed, now, true)
	if err != nil {
		return 0, "", err
	}
	if recurring.recurRule == "" {
		return 0, "", strconv.ErrSyntax
	}
	return recurring.recurDays, recurring.recurRule, nil
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

// getSubtaskDepth retrieves the depth of a subtask in the tree (via repository).
func (s *TaskService) getSubtaskDepth(
	ctx context.Context,
	taskID uuid.UUID,
	subtaskID uuid.UUID,
) (int, error) {
	return s.tasks.GetSubtaskDepth(ctx, taskID, subtaskID)
}

// buildSubtaskTree recursively builds a tree structure from a flat list,
// limiting depth to 3 levels. Only top-level subtasks (ParentSubtaskID == nil)
// are returned with their Children populated.
func buildSubtaskTree(flat []models.Subtask) []models.Subtask {
	const maxDepth = 3
	idToSubtask := make(map[uuid.UUID]*models.Subtask)

	// Build a map for quick lookup
	for i := range flat {
		idToSubtask[flat[i].ID] = &flat[i]
	}

	var result []models.Subtask
	for i := range flat {
		s := &flat[i]
		if s.ParentSubtaskID == nil {
			// Top-level subtask
			populateChildren(s, idToSubtask, 0, maxDepth)
			result = append(result, *s)
		}
	}

	return result
}

// populateChildren recursively populates the Children field of a subtask.
// It stops at maxDepth to prevent deep nesting.
func populateChildren(
	parent *models.Subtask,
	idToSubtask map[uuid.UUID]*models.Subtask,
	currentDepth int,
	maxDepth int,
) {
	if currentDepth >= maxDepth {
		return
	}

	for _, candidate := range idToSubtask {
		if candidate.ParentSubtaskID != nil && *candidate.ParentSubtaskID == parent.ID {
			// Make a copy to avoid shared references
			child := *candidate
			populateChildren(&child, idToSubtask, currentDepth+1, maxDepth)
			parent.Children = append(parent.Children, child)
		}
	}
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
