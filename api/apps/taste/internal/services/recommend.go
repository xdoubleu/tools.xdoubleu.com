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
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/openrouter"
)

const (
	// keepPerList is how many scored picks each domain and track keeps.
	keepPerList        = 3
	verifyConcurrency  = 4
	generatorMaxTokens = 8000
	searchMaxResults   = 5
	searchMaxTotal     = 20
	// replyPrefixLen is how much of an unparseable reply is logged.
	replyPrefixLen = 200
	// servicesBonus is added to the score of a pick streaming on the user's
	// services.
	servicesBonus = 10
	// nichePenalty is taken off the score of a pick with a small following.
	nichePenalty = 15
	maxScore     = 100
)

// A pick's reach as the generator judged it.
const (
	reachNiche   = "niche"
	reachKnown   = "known"
	reachPopular = "popular"
	reachUnknown = "unknown"
)

// nicheBelow is the catalog audience under which a pick is niche: TMDB votes
// for movies and series, Hardcover readers for books, Steam reviews for games.
//
//nolint:gochecknoglobals,mnd // read-only lookup table of thresholds
var nicheBelow = map[consumption.Domain]int{
	consumption.DomainMovies:   300,
	consumption.DomainSeries:   100,
	consumption.DomainBooks:    100,
	consumption.DomainGames:    500,
	consumption.DomainFeeds:    0,
	consumption.DomainPodcasts: 0,
}

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
	// Wanter adds an item, by the catalog ref its finder returned, to userID's
	// backlog in the owning app.
	Wanter func(ctx context.Context, userID, ref string) error
)

// Catalogs look candidates up per domain, where a nil finder skips that
// domain, and add accepted picks to a backlog, where a nil Wanter saves them
// instead.
type Catalogs struct {
	Books    CreatorFinder
	Movies   MediaFinder
	Games    TitleFinder
	Podcasts TitleFinder
	Feeds    SiteFinder
	WantBook Wanter
	// WantTitle takes movies and series.
	WantTitle Wanter
}

// AppOf is the app a domain's items live in.
func AppOf(d consumption.Domain) string {
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
	// Score is the model's 0-100 estimate of how much they'd enjoy it.
	Score int `json:"score"`
	// Reach is the model's estimate of its following, e.g. reachNiche.
	Reach string `json:"reach"`
}

type recommendationStore interface {
	List(
		ctx context.Context,
		userID, domain, track string,
		savedOnly bool,
	) ([]models.Recommendation, error)
	Touched(ctx context.Context, userID string) ([]models.Recommendation, error)
	Get(ctx context.Context, userID, id string) (*models.Recommendation, error)
	SetFeedback(
		ctx context.Context,
		userID, id, feedback string,
		at time.Time,
	) (*models.Recommendation, error)
	SetSaved(
		ctx context.Context,
		userID, id string,
		saved bool,
	) (*models.Recommendation, error)
	SetAccepted(ctx context.Context, userID, id string) (*models.Recommendation, error)
	HitRates(ctx context.Context, userID string) ([]models.HitRate, error)
	Replace(
		ctx context.Context,
		userID string,
		recs []models.Recommendation,
		at time.Time,
	) error
	SaveError(ctx context.Context, userID, note string, at time.Time) error
	Status(ctx context.Context, userID string) (models.PicksStatus, error)
	Request(ctx context.Context, userID string, at, notBefore time.Time) (bool, error)
	Due(ctx context.Context, userIDs []string, retryBefore time.Time) ([]string, error)
}

// ErrNoProfile means there is no profile to write picks from yet.
var ErrNoProfile = errors.New("there's no profile to write picks from yet")

// RecommendService proposes picks with the generator, verifies them against
// the catalogs and ranks them by the score the generator gave each.
type RecommendService struct {
	log       *LogService
	profiles  profileStore
	recs      recommendationStore
	catalogs  Catalogs
	llm       openrouter.Client
	generator string
	logger    *slog.Logger
	now       func() time.Time
}

func NewRecommendService(
	log *LogService,
	profiles profileStore,
	recs recommendationStore,
	catalogs Catalogs,
	llm openrouter.Client,
	generator string,
	logger *slog.Logger,
) *RecommendService {
	return &RecommendService{
		log: log, profiles: profiles, recs: recs, catalogs: catalogs, llm: llm,
		generator: generator, logger: logger, now: time.Now,
	}
}

// List returns userID's latest picks, or every saved one, with the state of
// the latest run.
func (s *RecommendService) List(
	ctx context.Context,
	userID, domain, track string,
	savedOnly bool,
) ([]models.Recommendation, models.PicksStatus, error) {
	var st models.PicksStatus
	recs, err := s.recs.List(ctx, userID, domain, track, savedOnly)
	if err != nil {
		return nil, st, err
	}
	st, err = s.recs.Status(ctx, userID)
	return recs, st, err
}

// Request queues new picks from the stored profile unless they were requested
// within RefreshCooldown; it reports whether it did.
func (s *RecommendService) Request(ctx context.Context, userID string) (bool, error) {
	profile, err := s.profiles.Get(ctx, userID)
	if err != nil {
		return false, err
	}
	if profile == nil || profile.GeneratedAt == nil {
		return false, ErrNoProfile
	}
	now := s.now()
	return s.recs.Request(ctx, userID, now, now.Add(-RefreshCooldown))
}

// Get returns userID's pick id.
func (s *RecommendService) Get(
	ctx context.Context,
	userID, id string,
) (*models.Recommendation, error) {
	return s.recs.Get(ctx, userID, id)
}

// SetFeedback records feedback on userID's pick id.
func (s *RecommendService) SetFeedback(
	ctx context.Context,
	userID, id, feedback string,
) (*models.Recommendation, error) {
	return s.recs.SetFeedback(ctx, userID, id, feedback, s.now())
}

// SetSaved saves or unsaves userID's pick id.
func (s *RecommendService) SetSaved(
	ctx context.Context,
	userID, id string,
	saved bool,
) (*models.Recommendation, error) {
	return s.recs.SetSaved(ctx, userID, id, saved)
}

// Accept adds rec to its app's backlog, or saves it where there is none.
func (s *RecommendService) Accept(
	ctx context.Context,
	userID string,
	rec models.Recommendation,
) (*models.Recommendation, error) {
	want := s.wanter(consumption.Domain(rec.Domain))
	if want == nil || !rec.CanAccept() {
		return s.recs.SetSaved(ctx, userID, rec.ID, true)
	}
	if err := want(ctx, userID, rec.CatalogRef); err != nil {
		return nil, err
	}
	return s.recs.SetAccepted(ctx, userID, rec.ID)
}

func (s *RecommendService) wanter(d consumption.Domain) Wanter {
	switch d { //nolint:exhaustive // other domains have no backlog
	case consumption.DomainBooks:
		return s.catalogs.WantBook
	case consumption.DomainMovies, consumption.DomainSeries:
		return s.catalogs.WantTitle
	}
	return nil
}

// HitRates counts userID's hits and misses per domain and track, in display
// order.
func (s *RecommendService) HitRates(
	ctx context.Context,
	userID string,
) ([]models.HitRate, error) {
	rates, err := s.recs.HitRates(ctx, userID)
	if err != nil {
		return nil, err
	}
	order := map[consumption.Domain]int{}
	for i, d := range consumption.Domains() {
		order[d] = i
	}
	slices.SortFunc(rates, func(a, b models.HitRate) int {
		da, db := consumption.Domain(a.Domain), consumption.Domain(b.Domain)
		return cmp.Or(
			cmp.Compare(order[da], order[db]),
			cmp.Compare(trackOrder(a.Track), trackOrder(b.Track)),
		)
	})
	return rates, nil
}

func trackOrder(t string) int {
	if t == models.TrackMore {
		return 0
	}
	return 1
}

// Due returns the users among userIDs whose picks were requested, or are
// missing or older than their profile and weren't attempted since it or within
// PicksRetryAfter.
func (s *RecommendService) Due(
	ctx context.Context,
	userIDs []string,
) ([]string, error) {
	return s.recs.Due(ctx, userIDs, s.now().Add(-PicksRetryAfter))
}

// Generate replaces userID's picks for the apps include accepts. A failure is
// noted and returned, keeping the previous picks.
func (s *RecommendService) Generate(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) error {
	at := s.now()
	recs, err := s.generate(ctx, userID, include)
	var f failure
	if errors.As(err, &f) {
		return errors.Join(err, s.recs.SaveError(ctx, userID, f.note, at))
	}
	return s.recs.Replace(ctx, userID, recs, at)
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
	if len(domains) == 0 {
		return nil, nil
	}
	previous, err := s.recs.List(ctx, userID, "", "", false)
	if err != nil {
		return nil, failure{note: "Your earlier picks couldn't be read.", err: err}
	}
	touched, err := s.recs.Touched(ctx, userID)
	if err != nil {
		return nil, failure{note: "Your earlier picks couldn't be read.", err: err}
	}
	touched = slices.DeleteFunc(touched, func(r models.Recommendation) bool {
		return !include(AppOf(consumption.Domain(r.Domain)))
	})
	profile, err := s.profiles.Get(ctx, userID)
	if err != nil {
		return nil, failure{note: "Your profile couldn't be read.", err: err}
	}

	candidates, sources, err := s.propose(
		ctx, events, domains, profile, mergePicks(previous, touched), touched,
	)
	if err != nil {
		return nil, err
	}
	verified, failed := s.verify(ctx, userID, candidates, events, touched)
	if len(verified) == 0 {
		// Keep the earlier picks rather than replace them with nothing.
		if failed > 0 {
			return nil, failure{
				note: "The catalogs couldn't be reached.",
				err:  fmt.Errorf("%d of %d lookups failed", failed, len(candidates)),
			}
		}
		return nil, failure{
			note: "None of the proposed picks checked out.",
			err:  fmt.Errorf("none of %d candidates verified", len(candidates)),
		}
	}
	return rankPicks(verified, sources), nil
}

// domains are the domains whose app include accepts and has a catalog.
func (s *RecommendService) domains(include func(string) bool) []consumption.Domain {
	var out []consumption.Domain
	for _, d := range consumption.Domains() {
		if include(AppOf(d)) && s.hasCatalog(d) {
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
	previous, touched []models.Recommendation,
) ([]Candidate, []models.Source, error) {
	// The web-search call can't be held to the schema (models answer it in
	// prose), so the format call turns its notes into strict JSON.
	research, err := s.llm.Complete(ctx, openrouter.Request{
		Model: s.generator,
		Messages: []openrouter.Message{
			openrouter.System(generatorPrompt(domains)),
			openrouter.User(generatorInput(events, profile, previous, touched, s.now())),
		},
		SchemaName: "",
		Schema:     nil,
		WebSearch: &openrouter.WebSearch{
			MaxResults: searchMaxResults, MaxTotalResults: searchMaxTotal,
		},
		MaxTokens: generatorMaxTokens,
	})
	if err != nil {
		return nil, nil, failure{note: noteModelFailed, err: err}
	}
	if strings.TrimSpace(research.Content) == "" {
		return nil, nil, failure{
			note: noteAnswerUnreadable,
			err:  errors.New("the research answer is empty"),
		}
	}
	resp, err := s.llm.Complete(ctx, openrouter.Request{
		Model: s.generator,
		Messages: []openrouter.Message{
			openrouter.System(formatterPrompt),
			openrouter.User(research.Content),
		},
		SchemaName: "taste_candidates",
		Schema:     candidatesSchema(),
		WebSearch:  nil,
		MaxTokens:  generatorMaxTokens,
	})
	if err != nil {
		return nil, nil, failure{note: noteModelFailed, err: err}
	}
	var out struct {
		Picks []Candidate `json:"picks"`
	}
	if err = json.Unmarshal([]byte(resp.Content), &out); err != nil {
		s.logger.WarnContext(ctx, "taste picks reply is not valid JSON",
			slog.String("model", resp.Model),
			slog.String("prefix", prefix(resp.Content, replyPrefixLen)))
		return nil, nil, failure{note: noteAnswerUnreadable, err: err}
	}
	sources := make([]models.Source, 0, len(research.Citations))
	for _, c := range research.Citations {
		sources = append(sources, models.Source{URL: c.URL, Title: c.Title})
	}
	allowed := map[string]bool{}
	for _, d := range domains {
		allowed[string(d)] = true
	}
	valid := slices.DeleteFunc(out.Picks, func(c Candidate) bool {
		return !allowed[c.Domain] || !models.IsTrack(c.Track) ||
			strings.TrimSpace(c.Title) == ""
	})
	return valid, sources, nil
}

// verified is a candidate its catalog confirmed and the user doesn't have.
type verified struct {
	Candidate
	match consumption.CatalogMatch
}

// verify keeps candidates their catalog confirms, dropping what the user
// already has, consumed or acted on as a pick, and duplicates; a failed
// lookup only drops that candidate, and is counted.
func (s *RecommendService) verify(
	ctx context.Context,
	userID string,
	candidates []Candidate,
	events []consumption.Event,
	touched []models.Recommendation,
) ([]verified, int) {
	matches := make([]*consumption.CatalogMatch, len(candidates))
	var failed atomic.Int32
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(verifyConcurrency)
	for i, c := range candidates {
		g.Go(func() error {
			m, err := s.find(gctx, userID, c)
			if err != nil {
				s.logger.WarnContext(gctx, "taste candidate lookup failed",
					slog.String("domain", c.Domain), slog.Any("error", err))
				failed.Add(1)
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
		if m == nil || m.Known || consumed(events, c.Domain, m.Title) ||
			pickedBefore(touched, c.Domain, *m) {
			continue
		}
		key := c.Domain + "|" + cmp.Or(m.Ref, strings.ToLower(m.Title))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, verified{Candidate: c, match: *m})
	}
	return out, int(failed.Load())
}

// niche reports whether the pick has a small following: by its catalog
// audience where the catalog counts one, else by the generator's reach.
func (v verified) niche() bool {
	if v.match.Audience != nil {
		return *v.match.Audience < nicheBelow[consumption.Domain(v.Domain)]
	}
	return v.Reach == reachNiche
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
	app := AppOf(consumption.Domain(domain))
	return slices.ContainsFunc(events, func(e consumption.Event) bool {
		return AppOf(e.Domain) == app && consumption.SameTitle(e.Title, title)
	})
}

// pickedBefore reports whether m is a pick the user already acted on.
func pickedBefore(
	touched []models.Recommendation,
	domain string,
	m consumption.CatalogMatch,
) bool {
	app := AppOf(consumption.Domain(domain))
	return slices.ContainsFunc(touched, func(r models.Recommendation) bool {
		if AppOf(consumption.Domain(r.Domain)) != app {
			return false
		}
		return (m.Ref != "" && r.CatalogRef == m.Ref) ||
			consumption.SameTitle(r.Title, m.Title)
	})
}

// mergePicks is previous plus the touched picks not in it, newest feedback
// first, capped at maxAlreadySuggested.
func mergePicks(previous, touched []models.Recommendation) []models.Recommendation {
	out := slices.Clone(previous)
	seen := map[string]bool{}
	for _, p := range previous {
		seen[p.ID] = true
	}
	for _, t := range touched {
		if !seen[t.ID] {
			out = append(out, t)
		}
	}
	return out[:min(len(out), maxAlreadySuggested)]
}

// rankPicks keeps the best-scored keepPerList picks per domain and track,
// boosting those on the user's services and holding back niche ones; each
// carries the cited sources that name it.
func rankPicks(picks []verified, sources []models.Source) []models.Recommendation {
	recs := make([]models.Recommendation, 0, len(picks))
	for _, p := range picks {
		score := min(max(p.Score, 0), maxScore)
		if p.match.OnMyServices {
			score = min(score+servicesBonus, maxScore)
		}
		if p.niche() {
			score = max(score-nichePenalty, 0)
		}
		recs = append(recs, models.Recommendation{
			ID: "", Domain: p.Domain, Track: p.Track, Title: p.match.Title,
			Creator: cmp.Or(p.match.Creator, p.Creator), Year: cmp.Or(p.match.Year, p.Year),
			CatalogRef: p.match.Ref, URL: p.match.URL, ImageURL: p.match.ImageURL,
			Why: p.Why, Sources: sourcesFor(p.match.Title, sources), Score: score,
			GeneratedAt: time.Time{}, Feedback: models.FeedbackNone, Saved: false,
			Accepted: false, OnMyServices: p.match.OnMyServices,
		})
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

// sourcesFor keeps the cited sources whose title or URL names title, each
// URL once.
func sourcesFor(title string, sources []models.Source) []models.Source {
	name := strings.ToLower(strings.TrimSpace(title))
	slug := strings.ReplaceAll(name, " ", "-")
	seen := map[string]bool{}
	out := []models.Source{}
	for _, src := range sources {
		hay := strings.ToLower(src.Title + " " + src.URL)
		if name == "" || seen[src.URL] ||
			(!strings.Contains(hay, name) && !strings.Contains(hay, slug)) {
			continue
		}
		seen[src.URL] = true
		out = append(out, src)
	}
	return out
}

// prefix is the first n bytes of s, cut at a rune boundary.
func prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
