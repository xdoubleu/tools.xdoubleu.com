package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/openrouter"
)

const (
	// RefreshCooldown is how long a user waits between profile refreshes.
	RefreshCooldown = 6 * time.Hour
	// ProfileMaxAge is when the job regenerates a profile unasked.
	ProfileMaxAge = 7 * 24 * time.Hour

	profileMaxTokens  = 2000
	maxBullets        = 6
	maxErrorLength    = 300
)

// ErrNotConfigured means no OpenRouter key is set.
var ErrNotConfigured = openrouter.ErrNotConfigured

var errNothingConsumed = errors.New("Nothing consumed yet.") //nolint:staticcheck,revive // shown as is

type profileStore interface {
	Get(ctx context.Context, userID string) (*models.Profile, error)
	Request(ctx context.Context, userID string, at, notBefore time.Time) (bool, error)
	SaveGenerated(
		ctx context.Context, userID string, sections models.ProfileSections,
		model string, at time.Time,
	) error
	SaveError(ctx context.Context, userID, message string, at time.Time) error
	Due(ctx context.Context, userIDs []string, staleBefore time.Time) ([]string, error)
}

// ProfileService writes each user's taste profile with the generator model.
type ProfileService struct {
	log        *LogService
	profiles   profileStore
	llm        openrouter.Client
	model      string
	configured bool
	now        func() time.Time
}

func NewProfileService(
	log *LogService,
	profiles profileStore,
	llm openrouter.Client,
	model string,
	configured bool,
) *ProfileService {
	return &ProfileService{
		log: log, profiles: profiles, llm: llm, model: model,
		configured: configured, now: time.Now,
	}
}

// Configured reports whether an OpenRouter key is set.
func (s *ProfileService) Configured() bool {
	return s.configured
}

// Get returns userID's profile, or nil before the first request.
func (s *ProfileService) Get(ctx context.Context, userID string) (*models.Profile, error) {
	return s.profiles.Get(ctx, userID)
}

// Request queues a regeneration unless one was requested within
// RefreshCooldown; it reports whether it did.
func (s *ProfileService) Request(ctx context.Context, userID string) (bool, error) {
	if !s.configured {
		return false, ErrNotConfigured
	}
	now := s.now()
	return s.profiles.Request(ctx, userID, now, now.Add(-RefreshCooldown))
}

// Due returns the users among userIDs whose profile should be generated now.
func (s *ProfileService) Due(ctx context.Context, userIDs []string) ([]string, error) {
	return s.profiles.Due(ctx, userIDs, s.now().Add(-ProfileMaxAge))
}

// Generate writes userID's profile from the sources include accepts. A
// failure is stored on the profile as well as returned; an empty log is
// stored but isn't a failure.
func (s *ProfileService) Generate(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) error {
	sections, model, err := s.generate(ctx, userID, include)
	if errors.Is(err, errNothingConsumed) {
		return s.profiles.SaveError(ctx, userID, err.Error(), s.now())
	}
	if err != nil {
		saveErr := s.profiles.SaveError(ctx, userID, errorText(err), s.now())
		return errors.Join(err, saveErr)
	}
	return s.profiles.SaveGenerated(ctx, userID, *sections, model, s.now())
}

func (s *ProfileService) generate(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) (*models.ProfileSections, string, error) {
	events, err := s.log.Events(ctx, userID, include)
	if err != nil {
		return nil, "", err
	}
	if len(events) == 0 {
		return nil, "", errNothingConsumed
	}

	resp, err := s.llm.Complete(ctx, openrouter.Request{
		Model: s.model,
		Messages: []openrouter.Message{
			{Role: "system", Content: profileSystemPrompt},
			{Role: "user", Content: Digest(events, s.now())},
		},
		SchemaName: "taste_profile",
		Schema:     profileSchema(),
		WebSearch:  nil,
		MaxTokens:  profileMaxTokens,
	})
	if err != nil {
		return nil, "", err
	}
	sections, err := parseSections(resp.Content)
	if err != nil {
		return nil, "", err
	}
	return sections, resp.Model, nil
}

func parseSections(content string) (*models.ProfileSections, error) {
	var sections models.ProfileSections
	if err := json.Unmarshal([]byte(content), &sections); err != nil {
		return nil, fmt.Errorf("profile is not valid JSON: %w", err)
	}
	if strings.TrimSpace(sections.Summary) == "" {
		return nil, errors.New("profile has no summary")
	}
	for _, list := range []*[]string{
		&sections.Know, &sections.Like, &sections.Drifting, &sections.Bounce,
	} {
		*list = (*list)[:min(len(*list), maxBullets)]
	}
	return &sections, nil
}

func errorText(err error) string {
	msg := err.Error()
	if len(msg) > maxErrorLength {
		msg = msg[:maxErrorLength] + "…"
	}
	return msg
}

const profileSystemPrompt = `You write a person's taste profile from their ` +
	`consumption log across books, articles (feeds), games, movies, series and ` +
	`podcasts. Each item has a signal from -2 (strong dislike) to +2 (strong ` +
	`like), normalized within its domain.

Write for the person themselves, in second person, plainly and specifically: ` +
	`name concrete genres, themes, eras, forms, creators and moods, and connect ` +
	`patterns across domains. Never invent items that aren't in the log.

- summary: two or three sentences capturing the overall taste.
- know: what they know well — subjects and fields they have gone deep on.
- like: what reliably works for them, including cross-domain patterns.
- drifting: where their taste has moved recently compared with before.
- bounce: what they tend to drop or dislike.

Each list holds at most 6 short bullets; use an empty list when the log ` +
	`gives no evidence.`

func profileSchema() map[string]any {
	list := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "know", "like", "drifting", "bounce"},
		"properties": map[string]any{
			"summary":  map[string]any{"type": "string"},
			"know":     list,
			"like":     list,
			"drifting": list,
			"bounce":   list,
		},
	}
}
