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

	profileMaxTokens = 2000
	maxBullets       = 6
)

// ErrNotConfigured means no OpenRouter key is set.
var ErrNotConfigured = openrouter.ErrNotConfigured

var errNothingConsumed = errors.New("nothing consumed yet")

// nothingConsumedNote is noted on the profile for an empty log.
const nothingConsumedNote = "There's nothing in your history to write a " +
	"profile from yet."

type profileStore interface {
	Get(ctx context.Context, userID string) (*models.Profile, error)
	Request(ctx context.Context, userID string, at, notBefore time.Time) (bool, error)
	SaveGenerated(
		ctx context.Context, userID string, sections models.ProfileSections,
		model string, startedAt time.Time,
	) error
	SaveError(ctx context.Context, userID, note string, startedAt time.Time) error
	SaveEmpty(ctx context.Context, userID, note string, startedAt time.Time) error
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
func (s *ProfileService) Get(
	ctx context.Context,
	userID string,
) (*models.Profile, error) {
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

// failure pairs an error with the note the user sees; the details are only
// returned, for the job's log.
type failure struct {
	note string
	err  error
}

func (f failure) Error() string { return f.err.Error() }

func (f failure) Unwrap() error { return f.err }

// Generate writes userID's profile from the sources include accepts. A
// failure is noted on the profile as well as returned; an empty log clears
// the profile but isn't a failure.
func (s *ProfileService) Generate(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) error {
	startedAt := s.now()
	sections, model, err := s.generate(ctx, userID, include)
	if errors.Is(err, errNothingConsumed) {
		return s.profiles.SaveEmpty(ctx, userID, nothingConsumedNote, startedAt)
	}
	var f failure
	if errors.As(err, &f) {
		saveErr := s.profiles.SaveError(ctx, userID, f.note, startedAt)
		return errors.Join(err, saveErr)
	}
	return s.profiles.SaveGenerated(ctx, userID, *sections, model, startedAt)
}

func (s *ProfileService) generate(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) (*models.ProfileSections, string, error) {
	events, err := s.log.Events(ctx, userID, include)
	if err != nil {
		return nil, "", failure{note: "Your history couldn't be read.", err: err}
	}
	if len(events) == 0 {
		return nil, "", errNothingConsumed
	}

	resp, err := s.llm.Complete(ctx, openrouter.Request{
		Model: s.model,
		Messages: []openrouter.Message{
			openrouter.System(profileSystemPrompt),
			openrouter.User(Digest(events, s.now())),
		},
		SchemaName: "taste_profile",
		Schema:     profileSchema(),
		WebSearch:  nil,
		MaxTokens:  profileMaxTokens,
	})
	if err != nil {
		return nil, "", failure{note: "The model service failed.", err: err}
	}
	sections, err := parseSections(resp.Content)
	if err != nil {
		return nil, "", failure{note: "The model's answer couldn't be read.", err: err}
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
	list := arraySchema(typeSchema("string"))
	return objectSchema(map[string]any{
		"summary":  typeSchema("string"),
		"know":     list,
		"like":     list,
		"drifting": list,
		"bounce":   list,
	})
}
