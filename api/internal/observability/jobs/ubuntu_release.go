package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
)

// currentUbuntuLTSVersion is the Ubuntu LTS release the Hetzner VPS is
// currently provisioned with (chosen manually via the Hetzner console per
// infra/README.md — not tracked anywhere machine-readable). Bump this by
// hand after any real do-release-upgrade, or this job keeps alerting on the
// release it was just upgraded to.
const currentUbuntuLTSVersion = "24.04"

// ubuntuMetaReleaseURL is Canonical's plain-text feed of every Ubuntu LTS
// release ever published.
const ubuntuMetaReleaseURL = "https://changelogs.ubuntu.com/meta-release-lts"

// ubuntuReleaseRunEvery is deliberately much slower than IssueNotifierJob's
// 5 minutes — a new Ubuntu LTS ships roughly every two years, so daily is
// more than enough to catch one promptly.
const ubuntuReleaseRunEvery = 24 * time.Hour

const ubuntuReleaseHTTPTimeout = 10 * time.Second

// versionFieldCount bounds strings.SplitN when parsing a "major.minor"
// version string into at most two fields.
const versionFieldCount = 2

// ltsRelease is one entry parsed out of the meta-release feed.
type ltsRelease struct {
	name    string
	version string
}

// UbuntuReleaseJob emails an admin the first time Canonical's meta-release
// feed lists a newer Ubuntu LTS than currentUbuntuLTSVersion (issue #1134).
// unattended-upgrades (issue #1050) only ever patches within the current
// release — it deliberately never runs do-release-upgrade, since automating
// a full OS release upgrade on a single-instance box with no HA is too
// risky — so without this, a new LTS becoming available could go unnoticed
// indefinitely. The box owner rarely SSHes in, so this emails rather than
// relying on the SSH login MOTD. Deduped via the same global.notified_issues
// table IssueNotifierJob uses, keyed on the new version string, so it only
// emails once per release until currentUbuntuLTSVersion is bumped by hand.
type UbuntuReleaseJob struct {
	httpClient     *http.Client
	metaReleaseURL string
	notifications  *notifications.Service
	notified       notifiedRepo
}

func NewUbuntuReleaseJob(
	notifications *notifications.Service,
	notified notifiedRepo,
) *UbuntuReleaseJob {
	return &UbuntuReleaseJob{
		httpClient:     &http.Client{Timeout: ubuntuReleaseHTTPTimeout},
		metaReleaseURL: ubuntuMetaReleaseURL,
		notifications:  notifications,
		notified:       notified,
	}
}

func (j *UbuntuReleaseJob) ID() string {
	return "notify-ubuntu-lts-release"
}

func (j *UbuntuReleaseJob) RunEvery() time.Duration {
	return ubuntuReleaseRunEvery
}

func (j *UbuntuReleaseJob) Run(ctx context.Context, logger *slog.Logger) error {
	release, err := j.latestLTSRelease(ctx)
	if err != nil {
		logger.WarnContext(ctx, "ubuntu-release: failed to fetch meta-release feed",
			essentialogger.ErrAttr(err))
		return nil
	}
	if release == nil || !isNewerVersion(release.version, currentUbuntuLTSVersion) {
		return nil
	}

	key := "ubuntu-lts:" + release.version
	exists, err := j.notified.Exists(ctx, key)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	subject := fmt.Sprintf("[Ubuntu] %s is now available", release.name)
	body := fmt.Sprintf(
		"A new Ubuntu LTS release is available: %s (%s).\n\n"+
			"The Hetzner VPS is currently on %s. unattended-upgrades only "+
			"patches within the current release — a do-release-upgrade to "+
			"%s must be run manually.",
		release.name, release.version, currentUbuntuLTSVersion, release.version,
	)
	j.notifications.Enqueue(subject, body, func(ctx context.Context, err error) error {
		if errors.Is(err, mailer.ErrNotConfigured) {
			return nil
		}
		if err != nil {
			return err
		}
		return j.notified.Insert(ctx, key)
	})
	return nil
}

// latestLTSRelease fetches and parses the meta-release feed, returning the
// LTS release with the highest version number (the feed lists releases
// oldest-first, but this doesn't rely on that ordering).
func (j *UbuntuReleaseJob) latestLTSRelease(ctx context.Context) (*ltsRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.metaReleaseURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"unexpected status %d from %s", resp.StatusCode, j.metaReleaseURL,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return latestRelease(parseMetaRelease(string(body))), nil
}

// parseMetaRelease parses the meta-release feed's "Field: value" blocks,
// separated by blank lines, into one ltsRelease per block that has both a
// Name and a Version.
func parseMetaRelease(body string) []ltsRelease {
	var releases []ltsRelease
	var name, version string

	flush := func() {
		if name != "" && version != "" {
			releases = append(releases, ltsRelease{name: name, version: version})
		}
		name, version = "", ""
	}

	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "Name: "):
			name = strings.TrimPrefix(line, "Name: ")
		case strings.HasPrefix(line, "Version: "):
			version = strings.TrimSuffix(strings.TrimPrefix(line, "Version: "), " LTS")
		case line == "":
			flush()
		}
	}
	flush()

	return releases
}

// latestRelease returns the release with the highest version among
// releases, or nil if releases is empty.
func latestRelease(releases []ltsRelease) *ltsRelease {
	var latest *ltsRelease
	for i := range releases {
		r := releases[i]
		if latest == nil || isNewerVersion(r.version, latest.version) {
			latest = &r
		}
	}
	return latest
}

// isNewerVersion reports whether candidate is a newer Ubuntu release
// version than current, comparing major.minor numerically (not
// lexicographically, since e.g. "6.06" must sort before "10.04").
func isNewerVersion(candidate, current string) bool {
	candMajor, candMinor := splitVersion(candidate)
	curMajor, curMinor := splitVersion(current)
	if candMajor != curMajor {
		return candMajor > curMajor
	}
	return candMinor > curMinor
}

func splitVersion(v string) (int, int) {
	parts := strings.SplitN(v, ".", versionFieldCount)
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}
