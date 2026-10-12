package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/openrouter"
)

// scriptLLM answers each schema with its scripted content or error.
type scriptLLM struct {
	answers   map[string]string
	errs      map[string]error
	citations []openrouter.Citation
	requests  []openrouter.Request
}

func (s *scriptLLM) Complete(
	_ context.Context, req openrouter.Request,
) (*openrouter.Response, error) {
	s.requests = append(s.requests, req)
	if err := s.errs[req.SchemaName]; err != nil {
		return nil, err
	}
	//nolint:exhaustruct // usage is irrelevant here
	return &openrouter.Response{
		Model: req.Model, Content: s.answers[req.SchemaName], Citations: s.citations,
	}, nil
}

// script answers each schema with its content.
func script(answers map[string]string) *scriptLLM {
	return &scriptLLM{
		answers:   answers,
		errs:      map[string]error{},
		citations: nil,
		requests:  nil,
	}
}

// failingAt answers with answers but fails schema with err.
func failingAt(answers map[string]string, schema string, err error) *scriptLLM {
	s := script(answers)
	s.errs[schema] = err
	return s
}

func (s *scriptLLM) schemas() []string {
	out := make([]string, len(s.requests))
	for i, r := range s.requests {
		out[i] = r.SchemaName
	}
	return out
}

// recStore is an in-memory recommendation store.
type recStore struct {
	recs     []models.Recommendation
	replaced bool
	at       time.Time
	note     string
	listErr  error
}

func (r *recStore) List(
	context.Context, string, string, string,
) ([]models.Recommendation, error) {
	return r.recs, r.listErr
}

func (r *recStore) Replace(
	_ context.Context, _ string, recs []models.Recommendation, at time.Time,
) error {
	r.recs, r.replaced, r.at = recs, true, at
	return nil
}

func (r *recStore) SaveError(_ context.Context, _, note string) error {
	r.note = note
	return nil
}

func (r *recStore) Status(context.Context, string) (*time.Time, string, error) {
	return &r.at, r.note, nil
}

// emptyStore holds no picks; listErr fails List.
func emptyStore(listErr error) *recStore {
	return &recStore{
		recs:     nil,
		replaced: false,
		at:       time.Time{},
		note:     "",
		listErr:  listErr,
	}
}

func picksJSON(picks ...Candidate) string {
	b, _ := json.Marshal(map[string]any{"picks": picks})
	return string(b)
}

func pick(domain, track, title string) Candidate {
	return Candidate{
		Domain: domain, Track: track, Title: title, Creator: "C", Year: 2020,
		URL: "", Why: "because " + title, Neighbour: "explorer",
	}
}

func match(title string, known bool) *consumption.CatalogMatch {
	return &consumption.CatalogMatch{
		Ref: "ref:" + title, Title: title, Creator: "", Year: 0,
		ImageURL: "img/" + title, URL: "url/" + title, Known: known,
	}
}

// catalogs finds every title in found; Known titles start with "Known", and
// "Broken" fails.
func catalogs(found ...string) Catalogs {
	lookup := func(title string) (*consumption.CatalogMatch, error) {
		if title == "Broken" {
			return nil, errors.New("catalog down")
		}
		for _, f := range found {
			if f == title {
				return match(title, len(title) > 5 && title[:5] == "Known"), nil
			}
		}
		return nil, nil //nolint:nilnil // not found
	}
	return Catalogs{
		Books: func(
			_ context.Context, _, title, _ string,
		) (*consumption.CatalogMatch, error) {
			return lookup(title)
		},
		Movies: func(
			_ context.Context, _, mediaType, title string, _ int,
		) (*consumption.CatalogMatch, error) {
			return lookup(mediaType + ":" + title)
		},
		Games: func(
			_ context.Context, _, title string,
		) (*consumption.CatalogMatch, error) {
			return lookup(title)
		},
		Podcasts: func(
			_ context.Context, _, title string,
		) (*consumption.CatalogMatch, error) {
			return lookup(title)
		},
		Feeds: func(
			_ context.Context, _, title, url string,
		) (*consumption.CatalogMatch, error) {
			return lookup(title + "@" + url)
		},
	}
}

func recService(
	llm openrouter.Client,
	cats Catalogs,
	store *recStore,
	src eventsSource,
	profile *models.Profile,
) *RecommendService {
	//nolint:exhaustruct // only Get is used
	profiles := &memStore{profile: profile}
	s := NewRecommendService(NewLogService([]Source{src}), profiles, store, cats, llm,
		"gen", "judge", logging.NewNopLogger())
	s.now = func() time.Time { return clock }
	return s
}

func TestRecommend_EndToEnd(t *testing.T) {
	llm := &scriptLLM{
		answers: map[string]string{
			"taste_candidates": picksJSON(
				pick("books", "more", "B1"), pick("books", "more", "B2"),
				pick("books", "more", "B3"), pick("books", "more", "B4"),
				pick("books", "broaden", "KnownBook"),
				pick("books", "broaden", "Dune"), // consumed
				pick("books", "broaden", "Missing"),
				pick("books", "broaden", "Broken"),
				pick("books", "more", "B1"), // duplicate
				pick("music", "more", "Song"),
				pick("books", "sideways", "B5"),
				pick("books", "more", "  "),
				pick("series", "broaden", "Show"),
				pick("feeds", "broaden", "Blog"),
			),
			"taste_scores": `{"scores": [{"id": 0, "score": 40}, {"id": 1, "score": 90},
				{"id": 2, "score": 150}, {"id": 3, "score": -5}, {"id": 4, "score": 70}]}`,
		},
		errs: map[string]error{},
		citations: []openrouter.Citation{
			{URL: "https://a.example/b3", Title: "Why B3 works"},
			{URL: "https://a.example/b3", Title: "Why B3 works"},
			{URL: "https://b.example/other", Title: "Unrelated"},
		},
		requests: nil,
	}
	cats := catalogs("B1", "B2", "B3", "B4", "KnownBook", "Dune", "series:Show")
	cats.Feeds = nil
	store := emptyStore(nil)
	//nolint:exhaustruct // sections only
	profile := &models.Profile{Sections: &models.ProfileSections{
		Summary: "Loves SF", Know: []string{"k"}, Like: []string{"l"},
		Drifting: []string{"d"}, Bounce: []string{"b"},
	}}
	s := recService(llm, cats, store, oneBook, profile)

	require.NoError(t, s.Generate(context.Background(), "u", all))
	require.True(t, store.replaced)
	assert.Equal(t, clock, store.at)
	assert.Equal(t, []string{"taste_candidates", "taste_scores"}, llm.schemas())

	got := make([]string, len(store.recs))
	for i, r := range store.recs {
		got[i] = fmt.Sprintf("%s/%s/%s/%d", r.Domain, r.Track, r.Title, r.Score)
	}
	// B3's 150 clamps to 100, B4's -5 to 0; the fourth "more" book is cut.
	assert.Equal(t, []string{
		"books/more/B3/100", "books/more/B2/90", "series/broaden/series:Show/70",
		"books/more/B1/40",
	}, got)
	first := store.recs[0]
	assert.Equal(t, "ref:B3", first.CatalogRef)
	assert.Equal(t, "url/B3", first.URL)
	assert.Equal(t, "img/B3", first.ImageURL)
	assert.Equal(t, "C", first.Creator, "the generator's creator fills a gap")
	assert.Equal(t, 2020, first.Year)
	assert.Equal(t, "because B3", first.Why)
	assert.Equal(t, []models.Source{{URL: "https://a.example/b3", Title: "Why B3 works"}},
		first.Sources, "only the sources naming it, once")
	assert.Empty(t, store.recs[1].Sources)

	gen := llm.requests[0]
	assert.Equal(t, "gen", gen.Model)
	require.NotNil(t, gen.WebSearch)
	assert.Equal(t, searchMaxResults, gen.WebSearch.MaxResults)
	assert.Equal(t, searchMaxTotal, gen.WebSearch.MaxTotalResults)
	assert.Equal(t, generatorMaxTokens, gen.MaxTokens)
	assert.Contains(t, gen.Messages[0].Content,
		"Cover these domains: books, games, movies, series, podcasts.")
	assert.Contains(t, gen.Messages[1].Content, "## Profile\nLoves SF\nKnows: k\n")
	assert.Contains(t, gen.Messages[1].Content, "Bounces off: b\n")
	assert.Contains(t, gen.Messages[1].Content, "## History\nToday is")
	assert.NotContains(t, gen.Messages[1].Content, "Already suggested")

	judge := llm.requests[1]
	assert.Equal(t, "judge", judge.Model)
	assert.Nil(t, judge.WebSearch)
	assert.Equal(t, judgeMaxTokens, judge.MaxTokens)
	assert.Contains(
		t,
		judge.Messages[1].Content,
		"0 | books | more | B1 — C | because B1\n",
	)
	assert.Contains(t, judge.Messages[1].Content, "## Profile\nLoves SF\nLikes: l\n")

	profile.Sections.Drifting = nil
	assert.NotContains(t, generatorInput(nil, profile, nil, clock), "Drifting toward",
		"an empty section is left out")
}

func TestRecommend_PassesPreviousPicks(t *testing.T) {
	llm := script(map[string]string{"taste_candidates": picksJSON()})
	store := emptyStore(nil)
	//nolint:exhaustruct // only what the prompt reads
	store.recs = []models.Recommendation{{Title: "Old Pick", Domain: "books"}}
	s := recService(llm, catalogs(), store, oneBook, nil)
	require.Error(t, s.Generate(context.Background(), "u", all))
	assert.Contains(t, llm.requests[0].Messages[1].Content,
		"## Already suggested\n- Old Pick (books)\n")
	assert.NotContains(t, llm.requests[0].Messages[1].Content, "## Profile")
	assert.Equal(t, []string{"taste_candidates"}, llm.schemas(), "no judge without picks")
	assert.False(t, store.replaced, "the earlier picks stay")
	assert.Equal(t, "None of the proposed picks checked out.", store.note)
}

func TestRecommend_OnlyAccessibleDomains(t *testing.T) {
	llm := script(map[string]string{
		"taste_candidates": picksJSON(pick("books", "more", "B1"),
			pick("movies", "more", "M")),
		"taste_scores": `{"scores": [{"id": 0, "score": 50}]}`,
	})
	store := emptyStore(nil)
	s := recService(llm, catalogs("B1", "movie:M"), store, oneBook, nil)
	onlyMovies := func(app string) bool { return app == "movies" }
	// The book source is hidden, so the log is empty: nothing to recommend.
	require.NoError(t, s.Generate(context.Background(), "u", onlyMovies))
	assert.Empty(t, llm.requests)
	assert.True(t, store.replaced)

	noFeeds := func(app string) bool { return app != "feeds" && app != "games" }
	require.NoError(t, s.Generate(context.Background(), "u", noFeeds))
	assert.Contains(t, llm.requests[0].Messages[0].Content,
		"Cover these domains: books, movies, series, podcasts.")
}

func TestRecommend_ConsumedAcrossMoviesAndSeries(t *testing.T) {
	watched := eventsSource{events: []consumption.Event{{
		Domain: consumption.DomainSeries, ItemID: "1", Title: "Dark", Creator: "",
		Tags: nil, At: clock, Signal: consumption.Positive, Reason: "watched",
	}}, err: nil}
	llm := &scriptLLM{
		answers: map[string]string{
			"taste_candidates": picksJSON(pick("movies", "more", "Dark")),
		},
		errs: map[string]error{}, citations: nil, requests: nil,
	}
	cats := catalogs()
	cats.Movies = func(
		context.Context, string, string, string, int,
	) (*consumption.CatalogMatch, error) {
		return match("Dark", false), nil
	}
	store := emptyStore(nil)
	require.Error(t, recService(llm, cats, store, watched, nil).
		Generate(context.Background(), "u", all))
	assert.False(t, store.replaced, "nothing new, so the earlier picks stay")
}

func TestRecommend_Failures(t *testing.T) {
	tests := []struct {
		name  string
		llm   *scriptLLM
		store *recStore
		src   eventsSource
		note  string
	}{
		{
			"history", script(nil), emptyStore(nil),
			eventsSource{events: nil, err: errors.New("db")},
			"Your history couldn't be read.",
		},
		{
			"previous picks", script(nil), emptyStore(errors.New("db")), oneBook,
			"Your earlier picks couldn't be read.",
		},
		{
			"generator", failingAt(nil, "taste_candidates", errors.New("503")),
			emptyStore(nil), oneBook, "The model service failed.",
		},
		{
			"generator answer", script(map[string]string{"taste_candidates": "nope"}),
			emptyStore(nil), oneBook, "The model's answer couldn't be read.",
		},
		{
			"judge", failingAt(
				map[string]string{
					"taste_candidates": picksJSON(pick("books", "more", "B1")),
				},
				"taste_scores", errors.New("503"),
			), emptyStore(nil), oneBook, "The scoring model failed.",
		},
		{
			"judge answer", script(map[string]string{
				"taste_candidates": picksJSON(pick("books", "more", "B1")),
				"taste_scores":     "nope",
			}), emptyStore(nil), oneBook, "The scoring model's answer couldn't be read.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := recService(tt.llm, catalogs("B1"), tt.store, tt.src, nil)
			require.Error(t, s.Generate(context.Background(), "u", all))
			assert.Equal(t, tt.note, tt.store.note)
			assert.False(t, tt.store.replaced, "earlier picks stay")
		})
	}
}

func TestRecommend_ProfileReadFailure(t *testing.T) {
	store := emptyStore(nil)
	//nolint:exhaustruct // only Get fails
	profiles := &memStore{err: errors.New("db")}
	s := NewRecommendService(NewLogService([]Source{oneBook}), profiles, store,
		catalogs(), script(nil), "gen", "judge", logging.NewNopLogger())
	require.Error(t, s.Generate(context.Background(), "u", all))
	assert.Equal(t, "Your profile couldn't be read.", store.note)
}

func TestRecommend_List(t *testing.T) {
	store := emptyStore(nil)
	//nolint:exhaustruct // a title is enough
	store.recs = []models.Recommendation{{Title: "X"}}
	store.at, store.note = clock, "n"
	recs, at, note, err := recService(script(nil), catalogs(), store, oneBook, nil).
		List(context.Background(), "u", "", "")
	require.NoError(t, err)
	assert.Len(t, recs, 1)
	assert.Equal(t, clock, *at)
	assert.Equal(t, "n", note)

	store.listErr = errors.New("db")
	_, _, _, err = recService(script(nil), catalogs(), store, oneBook, nil).
		List(context.Background(), "u", "", "")
	assert.Error(t, err)
}

func TestAppOfAndFindUnknownDomain(t *testing.T) {
	assert.Equal(t, "movies", AppOf(consumption.DomainSeries))
	assert.Equal(t, "books", AppOf(consumption.DomainBooks))
	s := recService(script(nil), catalogs(), emptyStore(nil), oneBook, nil)
	_, err := s.find(context.Background(), "u", pick("music", "more", "x"))
	assert.ErrorContains(t, err, "unknown domain")
	assert.False(t, s.hasCatalog("music"))
}

func TestFindRoutesEachDomain(t *testing.T) {
	s := recService(script(nil), catalogs(
		"B", "movie:M", "series:S", "G", "P", "F@https://f.example",
	), emptyStore(nil), oneBook, nil)
	for _, c := range []Candidate{
		pick("books", "more", "B"), pick("movies", "more", "M"),
		pick("series", "more", "S"), pick("games", "more", "G"),
		pick("podcasts", "more", "P"),
		{
			Domain: "feeds", Track: "more", Title: "F", Creator: "", Year: 0,
			URL: "https://f.example", Why: "", Neighbour: "",
		},
	} {
		m, err := s.find(context.Background(), "u", c)
		require.NoError(t, err)
		assert.NotNil(t, m, c.Domain)
	}
}

func TestRecommend_AllLookupsFailed(t *testing.T) {
	llm := script(map[string]string{
		"taste_candidates": picksJSON(pick("books", "more", "Broken")),
	})
	store := emptyStore(nil)
	err := recService(llm, catalogs(), store, oneBook, nil).
		Generate(context.Background(), "u", all)
	require.ErrorContains(t, err, "1 of 1 lookups failed")
	assert.Equal(t, "The catalogs couldn't be reached.", store.note)
	assert.False(t, store.replaced)
}

func TestRecommend_NoCatalogsNoCall(t *testing.T) {
	llm := script(nil)
	store := emptyStore(nil)
	none := Catalogs{Books: nil, Movies: nil, Games: nil, Podcasts: nil, Feeds: nil}
	require.NoError(t, recService(llm, none, store, oneBook, nil).
		Generate(context.Background(), "u", all))
	assert.Empty(t, llm.requests)
	assert.True(t, store.replaced)
}

func TestRecommend_DropsUnjudgedPicks(t *testing.T) {
	llm := script(map[string]string{
		"taste_candidates": picksJSON(
			pick("books", "more", "B1"),
			pick("books", "more", "B2"),
		),
		"taste_scores": `{"scores": [{"id": 1, "score": 0}, {"id": 7, "score": 99}]}`,
	})
	store := emptyStore(nil)
	require.NoError(t, recService(llm, catalogs("B1", "B2"), store, oneBook, nil).
		Generate(context.Background(), "u", all))
	require.Len(t, store.recs, 1)
	assert.Equal(t, "B2", store.recs[0].Title)
	assert.Zero(t, store.recs[0].Score)
}

func TestSourcesFor(t *testing.T) {
	sources := []models.Source{
		{URL: "https://x.example/children-of-time-review", Title: "A review"},
		{URL: "https://y.example/", Title: "Best SF: Children of Time and more"},
		{URL: "https://x.example/children-of-time-review", Title: "dup"},
		{URL: "https://z.example/", Title: "Other"},
	}
	got := sourcesFor("Children of Time", sources)
	require.Len(t, got, 2)
	assert.Equal(t, "https://x.example/children-of-time-review", got[0].URL)
	assert.Equal(t, "https://y.example/", got[1].URL)
	assert.Empty(t, sourcesFor("  ", sources))
}
