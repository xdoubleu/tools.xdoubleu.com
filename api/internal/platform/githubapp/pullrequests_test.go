package githubapp_test

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/githubapp"
)

// pulls serves one pull request on the fake GitHub, for any repo.
type pulls struct {
	pull    string
	checks  string
	reviews string
	// pullStatus answers the pull request call when it isn't 200.
	pullStatus int
	asked      atomic.Int32
	tokens     []string
	// checkPages and reviewPages, when set, answer by the page asked for.
	checkPages  []string
	reviewPages []string
	// checkAsked and reviewAsked count the calls for those lists.
	checkAsked  atomic.Int32
	reviewAsked atomic.Int32
}

func newPulls(pull, checks, reviews string) *pulls {
	return &pulls{
		pull: pull, checks: checks, reviews: reviews, pullStatus: 0,
		asked: atomic.Int32{}, tokens: nil, checkPages: nil, reviewPages: nil,
		checkAsked: atomic.Int32{}, reviewAsked: atomic.Int32{},
	}
}

func failingPulls(status int) *pulls {
	p := newPulls("", "", "")
	p.pullStatus = status
	return p
}

func servePulls(f *fakeGitHub, p *pulls) {
	record := func(r *http.Request) {
		p.asked.Add(1)
		p.tokens = append(p.tokens, r.Header.Get("Authorization"))
	}
	f.mux.HandleFunc("GET /repos/{o}/{r}/pulls/{n}",
		func(w http.ResponseWriter, r *http.Request) {
			record(r)
			if p.pullStatus != 0 {
				w.WriteHeader(p.pullStatus)
				return
			}
			_, _ = io.WriteString(w, p.pull)
		})
	f.mux.HandleFunc("GET /repos/{o}/{r}/commits/{sha}/check-runs",
		func(w http.ResponseWriter, r *http.Request) {
			record(r)
			assert.Equal(f.t, "abc123", r.PathValue("sha"))
			p.checkAsked.Add(1)
			_, _ = io.WriteString(w, pageOf(r, p.checkPages, p.checks))
		})
	f.mux.HandleFunc("GET /repos/{o}/{r}/pulls/{n}/reviews",
		func(w http.ResponseWriter, r *http.Request) {
			record(r)
			p.reviewAsked.Add(1)
			_, _ = io.WriteString(w, pageOf(r, p.reviewPages, p.reviews))
		})
}

// pageOf is the body for the page the request asks for, or fallback if the fake
// isn't paged.
func pageOf(r *http.Request, pages []string, fallback string) string {
	if pages == nil {
		return fallback
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if n < 1 || n > len(pages) {
		return `{"check_runs":[]}`
	}
	return pages[n-1]
}

const (
	openPull   = `{"state":"open","merged":false,"head":{"sha":"abc123"}}`
	noChecks   = `{"check_runs":[]}`
	mergedPull = `{"state":"closed","merged":true,"head":{"sha":"abc123"}}`
	closedPull = `{"state":"closed","merged":false,"head":{"sha":"abc123"}}`
)

func status(t *testing.T, p *pulls) githubapp.PullRequestStatus {
	t.Helper()
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	servePulls(f, p)
	c := newClient(t, f, pkcs1)
	got, err := c.PullRequest(t.Context(), "xdoubleu", "tools.xdoubleu.com", 7)
	require.NoError(t, err)
	return got
}

func ended(state string) githubapp.PullRequestStatus {
	return githubapp.PullRequestStatus{State: state, CI: "", Review: "", Mergeable: "",
		MergeSHA: ""}
}

func TestAMergedOrClosedPullRequestNeedsNoChecksOrReviews(t *testing.T) {
	merged := newPulls(mergedPull, "", "")
	assert.Equal(t, ended("merged"), status(t, merged))
	assert.EqualValues(t, 1, merged.asked.Load(), "nothing else is asked once over")

	assert.Equal(t, ended("closed"), status(t, newPulls(closedPull, "", "")))
}

func TestOnlyMergedPullRequestsAreListedWithTheirBodies(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	f.mux.HandleFunc("GET /repos/{o}/{r}/pulls",
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "closed", r.URL.Query().Get("state"))
			_, _ = io.WriteString(w, `[
				{"html_url":"u1","body":"Task: a (x)","merged_at":"2026-11-10T09:00:00Z",
					"merge_commit_sha":"s1"},
				{"html_url":"u2","body":"closed","merged_at":null},
				{"html_url":"u3","body":null,"merged_at":"2026-11-10T09:01:00Z"}]`)
		})
	c := newClient(t, f, pkcs1)

	got, err := c.MergedPullRequests(t.Context(), "xdoubleu", "tools.xdoubleu.com")

	require.NoError(t, err)
	assert.Equal(t, []githubapp.MergedPullRequest{
		{URL: "u1", Body: "Task: a (x)", MergeSHA: "s1"}, {URL: "u3", Body: "",
			MergeSHA: ""},
	}, got)
}

func TestOpenPullRequestsAreListedWithTheirNumbersAndBodies(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	f.mux.HandleFunc("GET /repos/{o}/{r}/pulls",
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "open", r.URL.Query().Get("state"))
			_, _ = io.WriteString(w, `[
				{"html_url":"u1","number":7,"body":"Task: a (x)"},
				{"html_url":"u2","number":8,"body":null}]`)
		})
	c := newClient(t, f, pkcs1)

	got, err := c.OpenPullRequests(t.Context(), "xdoubleu", "tools.xdoubleu.com")

	require.NoError(t, err)
	assert.Equal(t, []githubapp.OpenPullRequest{
		{URL: "u1", Number: 7, Body: "Task: a (x)"}, {URL: "u2", Number: 8, Body: ""},
	}, got)
}

func TestACommitIsDeployedWhenTheReleaseIsAheadOfItOrTheSame(t *testing.T) {
	for status, want := range map[string]bool{
		"ahead": true, "identical": true, "behind": false, "diverged": false,
	} {
		t.Run(status, func(t *testing.T) {
			key, pkcs1, _ := newKey(t)
			f := newFake(t, key)
			f.mux.HandleFunc("GET /repos/{o}/{r}/compare/{range}",
				func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "merge...release", r.PathValue("range"))
					_, _ = io.WriteString(w, `{"status":"`+status+`"}`)
				})
			c := newClient(t, f, pkcs1)

			got, err := c.Deployed(t.Context(), "xdoubleu", "tools.xdoubleu.com",
				"merge", "release")

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestADeployCheckGitHubRefusesIsAnErrorNotAnAnswer(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	f.mux.HandleFunc("GET /repos/{o}/{r}/compare/{range}",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	c := newClient(t, f, pkcs1)

	_, err := c.Deployed(t.Context(), "xdoubleu", "tools.xdoubleu.com", "gone", "release")

	require.Error(t, err)
}

func TestAPageOfMergedPullRequestsOverOneMebibyteIsRead(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	long := strings.Repeat("x", 30<<10)
	f.mux.HandleFunc("GET /repos/{o}/{r}/pulls",
		func(w http.ResponseWriter, _ *http.Request) {
			items := make([]string, 100)
			for i := range items {
				items[i] = `{"html_url":"u","body":"` + long +
					`","merged_at":"2026-11-10T09:00:00Z"}`
			}
			_, _ = io.WriteString(w, "["+strings.Join(items, ",")+"]")
		})
	c := newClient(t, f, pkcs1)

	got, err := c.MergedPullRequests(t.Context(), "xdoubleu", "tools.xdoubleu.com")

	require.NoError(t, err)
	assert.Len(t, got, 100)
}

func run(state, conclusion string) string {
	return `{"status":"` + state + `","conclusion":"` + conclusion + `"}`
}

func runs(items ...string) string {
	return `{"check_runs":[` + strings.Join(items, ",") + `]}`
}

func TestAnOpenPullRequestRollsItsChecksUp(t *testing.T) {
	green := run("completed", "success")
	for name, tc := range map[string]struct{ checks, want string }{
		"none": {noChecks, ""},
		"green": {
			runs(green, run("completed", "skipped"), run("completed", "neutral")),
			"passing",
		},
		"one pending": {runs(green, run("in_progress", "")), "pending"},
		"queued":      {runs(run("queued", "")), "pending"},
		"failure":     {runs(green, run("completed", "failure")), "failing"},
		"failure first": {
			runs(run("in_progress", ""), run("completed", "failure")), "failing",
		},
		"timed out":       {runs(run("completed", "timed_out")), "failing"},
		"cancelled":       {runs(run("completed", "cancelled")), "failing"},
		"action required": {runs(run("completed", "action_required")), "failing"},
		"startup failure": {runs(run("completed", "startup_failure")), "failing"},
		"stale":           {runs(run("completed", "stale")), "failing"},
	} {
		got := status(t, newPulls(openPull, tc.checks, `[]`))
		assert.Equal(t, "open", got.State, name)
		assert.Equal(t, tc.want, got.CI, name)
	}
}

func review(login, state string) string {
	return `{"user":{"login":"` + login + `"},"state":"` + state + `"}`
}

func reviews(items ...string) string { return "[" + strings.Join(items, ",") + "]" }

func TestAnOpenPullRequestRollsItsReviewsUp(t *testing.T) {
	for name, tc := range map[string]struct{ reviews, want string }{
		"none":         {`[]`, ""},
		"comment only": {reviews(review("a", "COMMENTED")), ""},
		"approved":     {reviews(review("a", "APPROVED")), "approved"},
		"changes":      {reviews(review("a", "CHANGES_REQUESTED")), "changes_requested"},
		"changes win": {
			reviews(review("a", "APPROVED"), review("b", "CHANGES_REQUESTED")),
			"changes_requested",
		},
		"latest word counts": {
			reviews(review("a", "CHANGES_REQUESTED"), review("a", "APPROVED")),
			"approved",
		},
		"commenting keeps it": {
			reviews(review("a", "APPROVED"), review("a", "COMMENTED")), "approved",
		},
		"dismissed is gone": {
			reviews(review("a", "CHANGES_REQUESTED"), review("a", "DISMISSED")), "",
		},
		"pending is ignored": {reviews(review("a", "PENDING")), ""},
	} {
		got := status(t, newPulls(openPull, noChecks, tc.reviews))
		assert.Equal(t, tc.want, got.Review, name)
	}
}

func TestThePullRequestIsReadWithAReadOnlyTokenForThatRepoOnly(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	p := newPulls(openPull, noChecks, `[]`)
	servePulls(f, p)
	c := newClient(t, f, pkcs1)

	_, err := c.PullRequest(t.Context(), "xdoubleu", "tools.xdoubleu.com", 7)
	require.NoError(t, err)

	assert.Equal(t, []any{"tools.xdoubleu.com"}, f.lastMint["repositories"])
	assert.Equal(t,
		map[string]any{"pull_requests": "read", "checks": "read", "contents": "read"},
		f.lastMint["permissions"], "reads only: no write permission is asked for")
	for _, auth := range p.tokens {
		assert.Equal(t, "Bearer ghs_minted", auth)
	}
}

func TestTheReadTokenIsReusedUntilItIsAboutToExpire(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	servePulls(f, newPulls(openPull, noChecks, `[]`))

	// The clock is 09:00; the minted token expires at 10:00.
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	c, err := githubapp.New("12345", pkcs1,
		githubapp.WithBaseURL(f.server.URL), githubapp.WithHTTPClient(f.server.Client()),
		githubapp.WithClock(func() time.Time { return now }))
	require.NoError(t, err)

	for range 3 {
		_, err = c.PullRequest(t.Context(), "o", "r", 1)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, f.mints.Load(), "one token for three calls")

	// Five minutes before it expires it is replaced.
	now = time.Date(2026, 11, 4, 9, 54, 59, 0, time.UTC)
	_, err = c.PullRequest(t.Context(), "o", "r", 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.mints.Load(), "still good at 09:54:59")
	now = time.Date(2026, 11, 4, 9, 55, 0, 0, time.UTC)
	_, err = c.PullRequest(t.Context(), "o", "r", 1)
	require.NoError(t, err)
	assert.EqualValues(t, 2, f.mints.Load(), "replaced at 09:55")

	// Another repo gets its own.
	_, err = c.PullRequest(t.Context(), "o", "other", 1)
	require.NoError(t, err)
	assert.EqualValues(t, 3, f.mints.Load())
}

func TestAReadTokenThatCannotBeMintedIsNotRemembered(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	servePulls(f, newPulls(openPull, noChecks, `[]`))
	c := newClient(t, f, pkcs1)

	f.mintStatus = http.StatusInternalServerError
	_, err := c.PullRequest(t.Context(), "o", "r", 1)
	require.Error(t, err)

	f.mintStatus = http.StatusCreated
	_, err = c.PullRequest(t.Context(), "o", "r", 1)
	require.NoError(t, err)
}

func TestPullRequestFailures(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	read := func(p *pulls) error {
		f := newFake(t, key)
		servePulls(f, p)
		_, err := newClient(t, f, pkcs1).PullRequest(t.Context(), "o", "r", 1)
		return err
	}

	require.ErrorIs(t, read(failingPulls(http.StatusNotFound)),
		githubapp.ErrPullRequestGone)
	err := read(failingPulls(http.StatusInternalServerError))
	require.Error(t, err)
	assert.NotErrorIs(t, err, githubapp.ErrPullRequestGone)

	// Checks or reviews that can't be read.
	require.ErrorContains(t, read(newPulls(openPull, `nope`, `[]`)), "checks")
	require.ErrorContains(t, read(newPulls(openPull, noChecks, `nope`)), "reviews")

	// A head without a sha has no checks to read.
	f := newFake(t, key)
	servePulls(f, newPulls(`{"state":"open","head":{}}`, "", `[]`))
	got, err := newClient(t, f, pkcs1).PullRequest(t.Context(), "o", "r", 1)
	require.NoError(t, err)
	assert.Empty(t, got.CI)

	// Bad input never reaches GitHub.
	c := newClient(t, f, pkcs1)
	for _, n := range []int{0, -1} {
		_, err = c.PullRequest(t.Context(), "o", "r", n)
		require.Error(t, err)
	}
	_, err = c.PullRequest(t.Context(), "..", "r", 1)
	require.Error(t, err)

	// An App that isn't installed says so.
	f = newFake(t, key)
	f.installStatus = http.StatusNotFound
	_, err = newClient(t, f, pkcs1).PullRequest(t.Context(), "o", "r", 1)
	require.ErrorIs(t, err, githubapp.ErrNotInstalled)
}

func TestChecksAndReviewsBeyondTheFirstPageAreRead(t *testing.T) {
	// 100 passing runs on page one and a failing one on page two.
	hundred := make([]string, 100)
	for i := range hundred {
		hundred[i] = run("completed", "success")
	}
	p := newPulls(openPull, "", "")
	p.checkPages = []string{
		`{"total_count":101,"check_runs":[` + strings.Join(hundred, ",") + `]}`,
		`{"total_count":101,"check_runs":[` + run("completed", "failure") + `]}`,
	}
	p.reviewPages = []string{`[]`}
	assert.Equal(t, "failing", status(t, p).CI)

	// A total that is never reached stops at an empty page.
	p = newPulls(openPull, "", "")
	p.checkPages = []string{
		`{"total_count":900,"check_runs":[` + run("completed", "success") + `]}`,
	}
	p.reviewPages = []string{`[]`}
	assert.Equal(t, "passing", status(t, p).CI)

	// 100 comments on page one, a change request on page two.
	comments := make([]string, 100)
	for i := range comments {
		comments[i] = review("someone", "COMMENTED")
	}
	p = newPulls(openPull, noChecks, "")
	p.reviewPages = []string{
		reviews(comments...),
		reviews(review("boss", "CHANGES_REQUESTED")),
	}
	assert.Equal(t, "changes_requested", status(t, p).Review)
}

func TestReadingStopsAtTheLastPageAndAtTheBound(t *testing.T) {
	passing := run("completed", "success")
	page := func(runs ...string) string {
		return `{"total_count":2,"check_runs":[` + strings.Join(runs, ",") + `]}`
	}

	// Two runs on two pages: exactly two requests, none for a third.
	p := newPulls(openPull, "", "")
	p.checkPages = []string{page(passing), page(passing)}
	p.reviewPages = []string{`[]`}
	assert.Equal(t, "passing", status(t, p).CI)
	assert.EqualValues(t, 2, p.checkAsked.Load())

	// The fifth page is read, the sixth is not.
	many := func(last string) []string {
		pages := make([]string, 6)
		for i := range pages {
			pages[i] = `{"total_count":600,"check_runs":[` + passing + `]}`
		}
		pages[5] = last
		return pages
	}
	p = newPulls(openPull, "", "")
	p.checkPages = many(
		`{"total_count":600,"check_runs":[` + run("completed", "failure") + `]}`,
	)
	p.reviewPages = []string{`[]`}
	assert.Equal(t, "passing", status(t, p).CI, "page six is past the bound")
	assert.EqualValues(t, 5, p.checkAsked.Load())

	fifth := many("")
	fifth[4] = `{"total_count":600,"check_runs":[` + run("completed", "failure") + `]}`
	p = newPulls(openPull, "", "")
	p.checkPages = fifth
	p.reviewPages = []string{`[]`}
	assert.Equal(t, "failing", status(t, p).CI, "page five is read")

	// Reviews the same: a full page asks for the next, a short one does not.
	full := make([]string, 100)
	for i := range full {
		full[i] = review("someone", "COMMENTED")
	}
	p = newPulls(openPull, noChecks, "")
	p.reviewPages = []string{reviews(full...), reviews(review("a", "APPROVED"))}
	assert.Equal(t, "approved", status(t, p).Review)
	assert.EqualValues(t, 2, p.reviewAsked.Load())

	p = newPulls(openPull, noChecks, "")
	pages := make([]string, 6)
	for i := range pages {
		pages[i] = reviews(full...)
	}
	pages[4] = reviews(append([]string{review("a", "APPROVED")}, full[1:]...)...)
	p.reviewPages = pages
	assert.Equal(t, "approved", status(t, p).Review, "page five is read")
	assert.EqualValues(t, 5, p.reviewAsked.Load(), "and not a sixth")
}

func TestOnlyAnArmedPullRequestThatCannotMergeReportsWhy(t *testing.T) {
	armed := `"auto_merge":{"merge_method":"squash"}`
	cases := map[string]struct {
		extra string
		want  string
	}{
		"conflict":    {`"mergeable_state":"dirty",` + armed, "conflicting"},
		"behind":      {`"mergeable_state":"behind",` + armed, "behind"},
		"clean":       {`"mergeable_state":"clean",` + armed, ""},
		"blocked":     {`"mergeable_state":"blocked",` + armed, ""},
		"not armed":   {`"mergeable_state":"dirty","auto_merge":null`, ""},
		"unknown yet": {`"mergeable_state":"unknown",` + armed, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pull := `{"state":"open","merged":false,"head":{"sha":"abc123"},` + tc.extra + `}`
			got := status(t, newPulls(pull, noChecks, `[]`))
			assert.Equal(t, tc.want, got.Mergeable)
		})
	}
}
