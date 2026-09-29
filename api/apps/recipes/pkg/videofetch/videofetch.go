// Package videofetch reads the title and description of a YouTube video, the
// only short-form source whose recipe text is reachable from a datacenter IP
// without a login or API key.
package videofetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/safedial"
)

var (
	// ErrUnsupportedURL is returned for anything but a YouTube video URL.
	ErrUnsupportedURL = errors.New("not a YouTube video URL")
	// ErrInstagram is returned for Instagram URLs: captions need a login.
	ErrInstagram = errors.New(
		"instagram captions can't be read without a login",
	)
	// ErrNotFound is returned when the video is unavailable or has no text.
	ErrNotFound = errors.New("video unavailable or without description")
	// ErrFetch wraps transport, HTTP status and decoding failures.
	ErrFetch = errors.New("fetching video failed")
)

const (
	youtubeBaseURL = "https://www.youtube.com"
	requestTimeout = 10 * time.Second
	maxRedirects   = 3
	maxBodyBytes   = 5 << 20

	// The watch page and player endpoint hit a bot wall from datacenter IPs;
	// the web client's "next" endpoint still returns the description.
	nextBody = `{"videoId":%q,"context":{"client":` +
		`{"clientName":"WEB","clientVersion":"2.20240726.00.00","hl":"en"}}}`
)

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// Video is the recipe-relevant text of one video.
type Video struct {
	Platform    string
	URL         string
	Title       string
	Description string
}

// Fetcher fetches video text through safedial.
type Fetcher struct {
	http    *http.Client
	baseURL string
}

// New returns a Fetcher; allowPrivate must be false in production.
func New(allowPrivate bool) *Fetcher {
	return &Fetcher{
		http:    safedial.Client(requestTimeout, maxRedirects, allowPrivate),
		baseURL: youtubeBaseURL,
	}
}

// Fetch returns the title and description of the video at rawURL.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Video, error) {
	id, err := youTubeID(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		f.baseURL+"/youtubei/v1/next?prettyPrint=false",
		bytes.NewBufferString(fmt.Sprintf(nextBody, id)))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrFetch, resp.StatusCode)
	}

	var next nextResponse
	if err = json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).
		Decode(&next); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}

	video := next.video()
	if video.Title == "" && video.Description == "" {
		return nil, ErrNotFound
	}
	video.URL = youtubeBaseURL + "/shorts/" + id
	return video, nil
}

// youTubeID extracts the video id from a shorts, watch or youtu.be URL.
func youTubeID(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedURL, rawURL)
	}

	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	var id string
	switch host {
	case "instagram.com":
		return "", ErrInstagram
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "m.youtube.com":
		if rest, ok := strings.CutPrefix(u.Path, "/shorts/"); ok {
			id = strings.Trim(rest, "/")
		} else if u.Path == "/watch" {
			id = u.Query().Get("v")
		}
	}
	if !videoIDPattern.MatchString(id) {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedURL, rawURL)
	}
	return id, nil
}

type nextResponse struct {
	Contents struct {
		TwoColumnWatchNextResults struct {
			Results struct {
				Results struct {
					Contents []struct {
						Primary *struct {
							Title struct {
								Runs []struct {
									Text string `json:"text"`
								} `json:"runs"`
							} `json:"title"`
						} `json:"videoPrimaryInfoRenderer"`
						Secondary *struct {
							AttributedDescription struct {
								Content string `json:"content"`
							} `json:"attributedDescription"`
						} `json:"videoSecondaryInfoRenderer"`
					} `json:"contents"`
				} `json:"results"`
			} `json:"results"`
		} `json:"twoColumnWatchNextResults"`
	} `json:"contents"`
}

func (r nextResponse) video() *Video {
	//nolint:exhaustruct // URL is set by the caller
	video := &Video{Platform: "youtube"}
	for _, c := range r.Contents.TwoColumnWatchNextResults.Results.Results.Contents {
		if c.Primary != nil {
			var title strings.Builder
			for _, run := range c.Primary.Title.Runs {
				title.WriteString(run.Text)
			}
			video.Title = title.String()
		}
		if c.Secondary != nil {
			video.Description = c.Secondary.AttributedDescription.Content
		}
	}
	return video
}
