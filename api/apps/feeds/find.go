package feeds

import (
	"context"
	"net/url"
	"strings"

	"tools.xdoubleu.com/internal/consumption"
)

// FindSource confirms a recommended site exists by fetching it, for the taste
// app; nil when siteURL isn't an http(s) URL or can't be fetched. Known is
// set when one of userID's feeds is on the site's host, before or after
// redirects, ignoring www. and feed subdomains.
func (a *Feeds) FindSource(
	ctx context.Context,
	userID, title, siteURL string,
) (*consumption.CatalogMatch, error) {
	if siteHost(siteURL) == "" {
		return nil, nil //nolint:nilnil // not a site URL is no match
	}
	feeds, err := a.Services.Feeds.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	final, err := a.Services.Feeds.Probe(ctx, siteURL)
	if err != nil {
		return nil, nil //nolint:nilnil,nilerr // an unreachable site is no match
	}
	hosts := map[string]bool{siteHost(siteURL): true, siteHost(final): true}
	known := false
	for _, f := range feeds {
		known = known || hosts[siteHost(f.URL)]
	}
	return &consumption.CatalogMatch{
		Ref: final, Title: title, Creator: "", Year: 0, ImageURL: "",
		URL: final, Known: known,
		OnMyServices: false,
	}, nil
}

// siteHost is rawURL's host without a www. or feed subdomain, or "" when it
// isn't http(s).
func siteHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	for _, prefix := range []string{"www.", "feeds.", "feed.", "rss."} {
		host = strings.TrimPrefix(host, prefix)
	}
	return host
}
