package services

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/openrouter"
)

const (
	// keepPerList is how many judged picks each domain and track keeps.
	keepPerList        = 3
	verifyConcurrency  = 4
	generatorMaxTokens = 8000
	judgeMaxTokens     = 3000
	searchMaxResults   = 5
	searchMaxTotal     = 20
)

type (
	// TitleFinder looks a title up; games and podcasts.
	TitleFinder func(
		ctx context.Context, userID, title string,
	) (*consumption.CatalogMatch, error)
	// CreatorFinder looks a title by a creator up; books.
	CreatorFinder func(
		ctx context.Context, userID, title, creator string,
	) (*consumption.CatalogMatch, error)
	// MediaFinder looks a "movie" or "series" up; year 0 when unknown.
	MediaFinder func(
		ctx context.Context, userID, mediaType, title string, year int,
	) (*consumption.CatalogMatch, error)
	// SiteFinder confirms a site exists; feeds.
	SiteFinder func(
		ctx context.Context, userID, title, url string,
	) (*consumption.CatalogMatch, error)
)

// Catalogs look candidates up per domain; a nil finder skips that domain.
type Catalogs struct {
	Books    CreatorFinder
	Movies   MediaFinder
	Games    TitleFinder
	Podcasts TitleFinder
	Feeds    SiteFinder
}

// appOf is the app a domain's items live in.
func appOf(d consumption.Domain) string {
	if d == consumption.DomainSeries {
		return string(consumption.DomainMovies)
	}
	return string(d)
}

// Candidate is a pick as the generator proposed it.
type Candidate struct {
	Domain    string `json:"domain"`
	Track     string `json:"track"`
	Title     string `json:"title"`
	Creator   string `json:"creator"`
	Year      int    `json:"year"`
	URL       string `json:"url"`
	Why       string `json:"why"`
	Neighbour string `json:"neighbour"`
}

type recommendationStore interface {
	List(
		ctx context.Context,
		userID, domain, track string,
	) ([]models.Recommendation, error)
	Replace(
		ctx context.Context,
		userID string,
		recs []models.Recommendation,
		at time.Time,
	) error
	SaveError(ctx context.Context, userID, note string) error
	Status(ctx context.Context, userID string) (*time.Time, string, error)
}

// RecommendService proposes picks with the generator, verifies them against
// the catalogs and ranks them with the judge.
type RecommendService struct {
	log       *LogService
	profiles  profileStore
	recs      recommendationStore
	catalogs  Catalogs
	llm       openrouter.Client
	generator string
	judge     string
	logger    *slog.Logger
	now       func() time.Time
}

func NewRecommendService(
	log *LogService,
	profiles profileStore,
	recs recommendationStore,
	catalogs Catalogs,
	llm openrouter.Client,
	generator, judge string,
	logger *slog.Logger,
) *RecommendService {
	return &RecommendService{
		log: log, profiles: profiles, recs: recs, catalogs: catalogs, llm: llm,
		generator: generator, judge: judge, logger: logger, now: time.Now,
	}
}

// List returns userID's picks with when they were made and the last run's
// error note.
func (s *RecommendService) List(
	ctx context.Context,
	userID, domain, track string,
) ([]models.Recommendation, *time.Time, string, error) {
	recs, err := s.recs.List(ctx, userID, domain, track)
	if err != nil {
		return nil, nil, "", err
	}
	at, note, err := s.recs.Status(ctx, userID)
	return recs, at, note, err
}

// Generate replaces userID's picks for the apps include accepts. A failure is
// noted and returned, keeping the previous picks.
func (s *RecommendService) Generate(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) error {
	recs, err := s.generate(ctx, userID, include)
	var f failure
	if errors.As(err, &f) {
		return errors.Join(err, s.recs.SaveError(ctx, userID, f.note))
	}
	return s.recs.Replace(ctx, userID, recs, s.now())
}

func (s *RecommendService) generate(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) ([]models.Recommendation, error) {
	events, err := s.log.Events(ctx, userID, include)
	if err != nil {
		return nil, failure{note: "Your history couldn't be read.", err: err}
	}
	if len(events) == 0 {
		return nil, nil
	}
	domains := s.domains(include)
	previous, err := s.recs.List(ctx, userID, "", "")
	if err != nil {
		return nil, failure{note: "Your earlier picks couldn't be read.", err: err}
	}
	profile, err := s.profiles.Get(ctx, userID)
	if err != nil {
		return nil, failure{note: "Your profile couldn't be read.", err: err}
	}

	candidates, sources, err := s.propose(ctx, events, domains, profile, previous)
	if err != nil {
		return nil, err
	}
	verified := s.verify(ctx, userID, candidates, events)
	if len(verified) == 0 {
		return nil, nil
	}
	scores, err := s.score(ctx, profile, verified)
	if err != nil {
		return nil, err
	}
	return rankPicks(verified, scores, sources), nil
}

// domains are the domains whose app include accepts and has a catalog.
func (s *RecommendService) domains(include func(string) bool) []consumption.Domain {
	var out []consumption.Domain
	for _, d := range consumption.Domains() {
		if include(appOf(d)) && s.hasCatalog(d) {
			out = append(out, d)
		}
	}
	return out
}

func (s *RecommendService) hasCatalog(d consumption.Domain) bool {
	switch d {
	case consumption.DomainBooks:
		return s.catalogs.Books != nil
	case consumption.DomainMovies, consumption.DomainSeries:
		return s.catalogs.Movies != nil
	case consumption.DomainGames:
		return s.catalogs.Games != nil
	case consumption.DomainPodcasts:
		return s.catalogs.Podcasts != nil
	case consumption.DomainFeeds:
		return s.catalogs.Feeds != nil
	}
	return false
}

func (s *RecommendService) propose(
	ctx context.Context,
	events []consumption.Event,
	domains []consumption.Domain,
	profile *models.Profile,
	previous []models.Recommendation,
) ([]Candidate, []models.Source, error) {
	resp, err := s.llm.Complete(ctx, openrouter.Request{
		Model: s.generator,
		Messages: []openrouter.Message{
			openrouter.System(generatorPrompt(domains)),
			openrouter.User(generatorInput(events, profile, previous, s.now())),
		},
		SchemaName: "taste_candidates",
		Schema:     candidatesSchema(),
		WebSearch: &openrouter.WebSearch{
			MaxResults: searchMaxResults, MaxTotalResults: searchMaxTotal,
		},
		MaxTokens: generatorMaxTokens,
	})
	if err != nil {
		return nil, nil, failure{note: "The model service failed.", err: err}
	}
	var out struct {
		Picks []Candidate `json:"picks"`
	}
	if err = json.Unmarshal([]byte(resp.Content), &out); err != nil {
		return nil, nil, failure{note: "The model's answer couldn't be read.", err: err}
	}
	sources := make([]models.Source, 0, len(resp.Citations))
	for _, c := range resp.Citations {
		sources = append(sources, models.Source{URL: c.URL, Title: c.Title})
	}
	allowed := map[string]bool{}
	for _, d := range domains {
		allowed[string(d)] = true
	}
	valid := slices.DeleteFunc(out.Picks, func(c Candidate) bool {
		return !allowed[c.Domain] || (c.Track != models.TrackMore &&
			c.Track != models.TrackBroaden) || strings.TrimSpace(c.Title) == ""
	})
	return valid, sources, nil
}

// verified is a candidate its catalog confirmed and the user doesn't have.
type verified struct {
	Candidate
	match consumption.CatalogMatch
}

// verify keeps candidates their catalog confirms, dropping what the user
// already has or consumed and duplicates; a failed lookup only drops that
// candidate.
func (s *RecommendService) verify(
	ctx context.Context,
	userID string,
	candidates []Candidate,
	events []consumption.Event,
) []verified {
	matches := make([]*consumption.CatalogMatch, len(candidates))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(verifyConcurrency)
	for i, c := range candidates {
		g.Go(func() error {
			m, err := s.find(gctx, userID, c)
			if err != nil {
				s.logger.WarnContext(gctx, "taste candidate lookup failed",
					slog.String("domain", c.Domain), slog.Any("error", err))
				return nil
			}
			matches[i] = m
			return nil
		})
	}
	_ = g.Wait()

	seen := map[string]bool{}
	var out []verified
	for i, m := range matches {
		c := candidates[i]
		if m == nil || m.Known || consumed(events, c.Domain, m.Title) {
			continue
		}
		key := c.Domain + "|" + cmp.Or(m.Ref, strings.ToLower(m.Title))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, verified{Candidate: c, match: *m})
	}
	return out
}

func (s *RecommendService) find(
	ctx context.Context,
	userID string,
	c Candidate,
) (*consumption.CatalogMatch, error) {
	switch consumption.Domain(c.Domain) {
	case consumption.DomainBooks:
		return s.catalogs.Books(ctx, userID, c.Title, c.Creator)
	case consumption.DomainMovies:
		return s.catalogs.Movies(ctx, userID, "movie", c.Title, c.Year)
	case consumption.DomainSeries:
		return s.catalogs.Movies(ctx, userID, "series", c.Title, c.Year)
	case consumption.DomainGames:
		return s.catalogs.Games(ctx, userID, c.Title)
	case consumption.DomainPodcasts:
		return s.catalogs.Podcasts(ctx, userID, c.Title)
	case consumption.DomainFeeds:
		return s.catalogs.Feeds(ctx, userID, c.Title, c.URL)
	}
	return nil, fmt.Errorf("unknown domain %q", c.Domain)
}

// consumed reports whether the log already has title in the domain's app.
func consumed(events []consumption.Event, domain, title string) bool {
	app := appOf(consumption.Domain(domain))
	return slices.ContainsFunc(events, func(e consumption.Event) bool {
		return appOf(e.Domain) == app && consumption.SameTitle(e.Title, title)
	})
}

func (s *RecommendService) score(
	ctx context.Context,
	profile *models.Profile,
	picks []verified,
) (map[int]int, error) {
	resp, err := s.llm.Complete(ctx, openrouter.Request{
		Model: s.judge,
		Messages: []openrouter.Message{
			openrouter.System(judgePrompt),
			openrouter.User(judgeInput(profile, picks)),
		},
		SchemaName: "taste_scores",
		Schema:     scoresSchema(),
		WebSearch:  nil,
		MaxTokens:  judgeMaxTokens,
	})
	if err != nil {
		return nil, failure{note: "The scoring model failed.", err: err}
	}
	var out struct {
		Scores []struct {
			ID    int `json:"id"`
			Score int `json:"score"`
		} `json:"scores"`
	}
	if err = json.Unmarshal([]byte(resp.Content), &out); err != nil {
		return nil, failure{note: "The scoring model's answer couldn't be read.", err: err}
	}
	scores := map[int]int{}
	for _, sc := range out.Scores {
		scores[sc.ID] = min(max(sc.Score, 0), 100) //nolint:mnd // a 0-100 score
	}
	return scores, nil
}

// rankPicks keeps the best-scored keepPerList picks per domain and track; every
// pick carries the generation's cited sources.
func rankPicks(
	picks []verified,
	scores map[int]int,
	sources []models.Source,
) []models.Recommendation {
	recs := make([]models.Recommendation, len(picks))
	for i, p := range picks {
		recs[i] = models.Recommendation{
			ID: "", Domain: p.Domain, Track: p.Track, Title: p.match.Title,
			Creator: cmp.Or(p.match.Creator, p.Creator), Year: cmp.Or(p.match.Year, p.Year),
			CatalogRef: p.match.Ref, URL: p.match.URL, ImageURL: p.match.ImageURL,
			Why: p.Why, Sources: sources, Score: scores[i], GeneratedAt: time.Time{},
		}
	}
	slices.SortStableFunc(recs, func(a, b models.Recommendation) int {
		return cmp.Compare(b.Score, a.Score)
	})
	kept := map[string]int{}
	return slices.DeleteFunc(recs, func(r models.Recommendation) bool {
		key := r.Domain + "|" + r.Track
		kept[key]++
		return kept[key] > keepPerList
	})
}
