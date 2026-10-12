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
	recs       []models.Recommendation
	touched    []models.Recommendation
	rates      []models.HitRate
	replaced   bool
	at         time.Time
	note       string
	listErr    error
	touchedErr error
	savedOnly  bool
	calls      []string
}

func (r *recStore) List(
	_ context.Context, _, _, _ string, savedOnly bool,
) ([]models.Recommendation, error) {
	r.savedOnly = savedOnly
	return r.recs, r.listErr
}

func (r *recStore) Touched(context.Context, string) ([]models.Recommendation, error) {
	return r.touched, r.touchedErr
}

func (r *recStore) Get(
	_ context.Context, _, id string,
) (*models.Recommendation, error) {
	//nolint:exhaustruct // the id is enough
	return &models.Recommendation{ID: id}, r.listErr
}

func (r *recStore) SetFeedback(
	_ context.Context, _, id, feedback string, at time.Time,
) (*models.Recommendation, error) {
	r.calls = append(r.calls, fmt.Sprintf("feedback %s %s %s", id, feedback,
		at.Format(time.DateOnly)))
	//nolint:exhaustruct // what was set
	return &models.Recommendation{ID: id, Feedback: feedback}, nil
}

func (r *recStore) SetSaved(
	_ context.Context, _, id string, saved bool,
) (*models.Recommendation, error) {
	r.calls = append(r.calls, fmt.Sprintf("saved %s %t", id, saved))
	//nolint:exhaustruct // what was set
	return &models.Recommendation{ID: id, Saved: saved}, nil
}

func (r *recStore) SetAccepted(
	_ context.Context, _, id string,
) (*models.Recommendation, error) {
	r.calls = append(r.calls, "accepted "+id)
	//nolint:exhaustruct // what was set
	return &models.Recommendation{ID: id, Accepted: true}, nil
}

func (r *recStore) HitRates(context.Context, string) ([]models.HitRate, error) {
	return r.rates, r.listErr
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
		recs:       nil,
		touched:    nil,
		rates:      nil,
		replaced:   false,
		at:         time.Time{},
		note:       "",
		listErr:    listErr,
		touchedErr: nil,
		savedOnly:  false,
		calls:      nil,
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
		OnMyServices: false,
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
	//nolint:exhaustruct // no backlogs; accept tests set them
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
	assert.NotContains(t, generatorInput(nil, profile, nil, nil, clock), "Drifting toward",
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
			"touched picks", script(nil), &recStore{
				recs: nil, touched: nil, rates: nil, replaced: false, at: time.Time{},
				note: "", listErr: nil, touchedErr: errors.New("db"),
				savedOnly: false, calls: nil,
			}, oneBook, "Your earlier picks couldn't be read.",
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
		List(context.Background(), "u", "", "", true)
	require.NoError(t, err)
	assert.True(t, store.savedOnly)
	assert.Len(t, recs, 1)
	assert.Equal(t, clock, *at)
	assert.Equal(t, "n", note)

	store.listErr = errors.New("db")
	_, _, _, err = recService(script(nil), catalogs(), store, oneBook, nil).
		List(context.Background(), "u", "", "", false)
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
	none := Catalogs{
		Books: nil, Movies: nil, Games: nil, Podcasts: nil, Feeds: nil,
		WantBook: nil, WantTitle: nil,
	}
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

// touchedPick is a pick the user acted on.
func touchedPick(
	domain, title, ref, feedback string,
	saved bool,
) models.Recommendation {
	//nolint:exhaustruct // what verification and the prompt read
	return models.Recommendation{
		ID: title, Domain: domain, Track: "broaden", Title: title, CatalogRef: ref,
		Feedback: feedback, Saved: saved,
	}
}

func TestRecommend_TouchedPicksShapeTheNextRun(t *testing.T) {
	llm := script(map[string]string{
		"taste_candidates": picksJSON(
			pick("books", "more", "B1"),
			pick("books", "more", "B2"),        // same ref as a known pick
			pick("books", "more", "Saved One"), // same title as a saved pick
			pick("series", "more", "Old Hit"),  // a hit, but in another app
		),
		"taste_scores": `{"scores": [{"id": 0, "score": 50}, {"id": 1, "score": 60}]}`,
	})
	store := emptyStore(nil)
	store.touched = []models.Recommendation{
		touchedPick("games", "Hidden Hit", "ref:g", models.FeedbackHit, false),
		touchedPick("books", "Old Hit", "ref:x", models.FeedbackHit, false),
		touchedPick("books", "Already", "ref:B2", models.FeedbackKnown, false),
		touchedPick("books", "Saved One", "", models.FeedbackNone, true),
	}
	//nolint:exhaustruct // only what the prompt reads
	store.recs = []models.Recommendation{
		{ID: "Old Hit", Title: "Old Hit", Domain: "books"},
	}
	s := recService(llm, catalogs("B1", "B2", "Saved One", "series:Old Hit"),
		store, oneBook, nil)
	noGames := func(app string) bool { return app != "games" }
	require.NoError(t, s.Generate(context.Background(), "u", noGames))

	titles := make([]string, len(store.recs))
	for i, r := range store.recs {
		titles[i] = r.Title
	}
	assert.Equal(t, []string{"series:Old Hit", "B1"}, titles)
	in := llm.requests[0].Messages[1].Content
	assert.Contains(t, in, "## Already suggested\n- Old Hit (books)\n"+
		"- Already (books)\n- Saved One (books)\n", "each pick once")
	assert.Contains(t, in, "## Feedback on earlier picks\n"+
		"- Old Hit (books, broaden): hit, loved it\n"+
		"- Already (books, broaden): already knew it\n")
	assert.NotContains(t, in, "Saved One (books, broaden)", "saving isn't feedback")
	assert.NotContains(t, in, "Hidden Hit", "only apps the user can open")
}

func TestGeneratorInput_CapsFeedback(t *testing.T) {
	touched := make([]models.Recommendation, maxFeedback+1)
	for i := range touched {
		title := fmt.Sprintf("T%d", i)
		touched[i] = touchedPick("games", title, "", models.FeedbackMiss, false)
	}
	in := generatorInput(nil, nil, nil, touched, clock)
	assert.Contains(t, in, fmt.Sprintf("- T%d (games, broaden): miss", maxFeedback-1))
	assert.NotContains(t, in, fmt.Sprintf("T%d ", maxFeedback))
}

func TestMergePicks_CapsAndDeduplicates(t *testing.T) {
	//nolint:exhaustruct // ids are enough
	previous := []models.Recommendation{{ID: "a"}, {ID: "b"}}
	//nolint:exhaustruct // ids are enough
	touched := []models.Recommendation{{ID: "b"}, {ID: "c"}}
	ids := func(recs []models.Recommendation) []string {
		out := make([]string, len(recs))
		for i, r := range recs {
			out[i] = r.ID
		}
		return out
	}
	assert.Equal(t, []string{"a", "b", "c"}, ids(mergePicks(previous, touched)))

	many := make([]models.Recommendation, maxAlreadySuggested+5)
	for i := range many {
		many[i].ID = fmt.Sprint(i)
	}
	assert.Len(t, mergePicks(many, nil), maxAlreadySuggested)
}

func TestRecommend_BoostsPicksOnMyServices(t *testing.T) {
	llm := script(map[string]string{
		"taste_candidates": picksJSON(
			pick("movies", "more", "Near"), pick("movies", "more", "Far"),
			pick("movies", "more", "Top"),
		),
		"taste_scores": `{"scores": [{"id": 0, "score": 50}, {"id": 1, "score": 55},
			{"id": 2, "score": 95}]}`,
	})
	cats := catalogs()
	cats.Movies = func(
		_ context.Context, _, _, title string, _ int,
	) (*consumption.CatalogMatch, error) {
		m := match(title, false)
		m.OnMyServices = title != "Far"
		return m, nil
	}
	store := emptyStore(nil)
	require.NoError(t, recService(llm, cats, store, oneBook, nil).
		Generate(context.Background(), "u", all))
	got := make([]string, len(store.recs))
	for i, r := range store.recs {
		got[i] = fmt.Sprintf("%s/%d/%t", r.Title, r.Score, r.OnMyServices)
	}
	assert.Equal(t, []string{"Top/100/true", "Near/60/true", "Far/55/false"}, got)
}

func TestRecommend_Accept(t *testing.T) {
	var wanted []string
	want := func(name string, err error) Wanter {
		return func(_ context.Context, userID, ref string) error {
			wanted = append(wanted, name+" "+userID+" "+ref)
			return err
		}
	}
	cats := catalogs()
	cats.WantBook = want("book", nil)
	cats.WantTitle = want("title", nil)
	store := emptyStore(nil)
	s := recService(script(nil), cats, store, oneBook, nil)
	//nolint:exhaustruct // what Accept reads
	accept := func(domain, ref string) *models.Recommendation {
		rec, err := s.Accept(context.Background(), "u",
			models.Recommendation{ID: domain + ref, Domain: domain, CatalogRef: ref})
		require.NoError(t, err)
		return rec
	}

	assert.True(t, accept("books", "hardcover:978").Accepted)
	assert.True(t, accept("movies", "movie:1").Accepted)
	assert.True(t, accept("series", "series:2").Accepted)
	assert.True(t, accept("games", "620").Saved, "no games backlog: saved")
	assert.True(t, accept("books", "").Saved, "no ref: saved")
	assert.Equal(t, []string{
		"book u hardcover:978", "title u movie:1", "title u series:2",
	}, wanted)
	assert.Equal(t, []string{
		"accepted bookshardcover:978", "accepted moviesmovie:1",
		"accepted seriesseries:2", "saved games620 true", "saved books true",
	}, store.calls)

	cats.WantBook = want("book", errors.New("hardcover down"))
	s = recService(script(nil), cats, emptyStore(nil), oneBook, nil)
	//nolint:exhaustruct // what Accept reads
	_, err := s.Accept(context.Background(), "u",
		models.Recommendation{ID: "x", Domain: "books", CatalogRef: "hardcover:1"})
	assert.ErrorContains(t, err, "hardcover down")
}

func TestRecommend_FeedbackSavedGet(t *testing.T) {
	store := emptyStore(nil)
	s := recService(script(nil), catalogs(), store, oneBook, nil)
	ctx := context.Background()

	rec, err := s.Get(ctx, "u", "id1")
	require.NoError(t, err)
	assert.Equal(t, "id1", rec.ID)
	rec, err = s.SetFeedback(ctx, "u", "id1", models.FeedbackHit)
	require.NoError(t, err)
	assert.Equal(t, models.FeedbackHit, rec.Feedback)
	rec, err = s.SetSaved(ctx, "u", "id1", true)
	require.NoError(t, err)
	assert.True(t, rec.Saved)
	assert.Equal(t, []string{"feedback id1 hit 2026-10-12", "saved id1 true"}, store.calls)
}

func TestRecommend_HitRatesInDisplayOrder(t *testing.T) {
	store := emptyStore(nil)
	store.rates = []models.HitRate{
		{Domain: "series", Track: "broaden", Hits: 1, Misses: 0},
		{Domain: "books", Track: "broaden", Hits: 2, Misses: 1},
		{Domain: "books", Track: "more", Hits: 3, Misses: 0},
	}
	rates, err := recService(script(nil), catalogs(), store, oneBook, nil).
		HitRates(context.Background(), "u")
	require.NoError(t, err)
	got := make([]string, len(rates))
	for i, r := range rates {
		got[i] = r.Domain + "/" + r.Track
	}
	assert.Equal(t, []string{"books/more", "books/broaden", "series/broaden"}, got)

	store.listErr = errors.New("db")
	_, err = recService(script(nil), catalogs(), store, oneBook, nil).
		HitRates(context.Background(), "u")
	assert.Error(t, err)
}
