package todoist

import "golang.org/x/oauth2"

// OAuthConfig builds the Todoist OAuth2 config used to let a user connect
// their own account (issue #1475). Unlike github/sentryapi's OAuthConfig
// (one shared admin-authorized connection per provider), this is built once
// per app but exchanged per user — the resulting token is stored in
// learningpaths.oauth_connections, keyed by (user_id, provider), not
// global.oauth_connections.
//
// Spike findings, confirmed via direct WebFetch against
// developer.todoist.com this session (the prior research pass recorded on
// issue #1475 was blocked by this environment's egress proxy and could only
// corroborate via search-engine snippets):
//   - Todoist's current, actively-documented API surface is "API v1" at
//     base path https://api.todoist.com/api/v1/ — this is what
//     developer.todoist.com/api/v1/ (the docs' own landing page) describes,
//     including task creation (POST /api/v1/tasks, see client.go). Its
//     "Migrating from v9" section is written for callers moving onto v1,
//     not away from it. The older /rest/v2/ docs are still reachable but
//     aren't the current documented entry point, so this client targets v1.
//   - Rate limit: 1000 requests / 15 minutes per user access token,
//     confirmed against the docs' own Request Limits section.
//   - due_string (see interface.go) is natural-language only — e.g. "every
//     Monday" — confirmed there is no separate structured recurrence field;
//     Todoist parses recurrence out of due_string itself.
//   - The OAuth authorize endpoint is app.todoist.com (not api.todoist.com),
//     and Todoist expects comma-separated scopes in that request (unlike
//     golang.org/x/oauth2's default space-joined "scope" param) — moot here
//     since only one scope is requested, but worth a flag if a second scope
//     is ever added.
func OAuthConfig(clientID, clientSecret, apiURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		//nolint:exhaustruct,gosec // endpoint URLs, not credentials
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://app.todoist.com/oauth/authorize",
			TokenURL: "https://api.todoist.com/oauth/access_token",
		},
		RedirectURL: apiURL + "/learningpaths/oauth/todoist/callback",
		// task:add is create-only — this app never reads a user's existing
		// Todoist tasks/projects, matching "no two-way sync".
		Scopes: []string{"task:add"},
	}
}
