package feeds

import (
	"context"
	"net/url"
	"strings"

	"tools.xdoubleu.com/internal/consumption"
)

// FindSource confirms a recommended site exists by fetching it, for the taste
// app; nil when siteURL isn't an http(s) URL or can't be fetched. Known is
// set when one of userID's feeds is on the same host.
func (a *Feeds) FindSource(
	ctx context.Context,
	userID, title, siteURL string,
) (*consumption.CatalogMatch, error) {
	host := hostOf(siteURL)
	if host == "" {
		return nil, nil //nolint:nilnil // not a site URL is no match
	}
	feeds, err := a.Services.Feeds.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	known := false
	for _, f := range feeds {
		known = known || hostOf(f.URL) == host
	}
	final, err := a.Services.Feeds.Probe(ctx, siteURL)
	if err != nil {
		return nil, nil //nolint:nilnil,nilerr // an unreachable site is no match
	}
	return &consumption.CatalogMatch{
		Ref: final, Title: title, Creator: "", Year: 0, ImageURL: "",
		URL: final, Known: known,
	}, nil
}

// hostOf is rawURL's host without "www.", or "" when it isn't http(s).
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
