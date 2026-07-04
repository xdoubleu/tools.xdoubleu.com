package services

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

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

func findSection(sections []models.Section, name string) *models.Section {
	for i := range sections {
		if strings.EqualFold(sections[i].Name, name) {
			return &sections[i]
		}
	}
	return nil
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
		_ = s.settings.CreateLabelPreset(
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

func urlToTitle(rawURL string) string {
	return rawURL
}
