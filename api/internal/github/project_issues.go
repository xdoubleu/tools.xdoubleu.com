package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/oauthconn"
)

// ProjectIssue is an open issue on a Projects (v2) board with its Status value.
// BodyEditedAfterStatus: someone other than the repo owner edited the body
// after Status was last set.
type ProjectIssue struct {
	Number                int64
	Title                 string
	URL                   string
	Status                string
	AuthorLogin           string
	AuthorAssociation     string
	StatusUpdatedAt       time.Time
	BodyHasHTMLComment    bool
	BodyEditedAfterStatus bool
}

const issueStateOpen = "OPEN"

// projectItemsPageSize is GitHub's per-page maximum for ProjectV2 items.
const projectItemsPageSize = 100

// projectItemsMaxPages bounds pagination; a larger board errors rather than
// returning a silently truncated column.
const projectItemsMaxPages = 50

// ErrInsufficientScopes means the GitHub token lacks a scope the query needs
// (read:project for project boards).
var ErrInsufficientScopes = errors.New("github: token lacks required scope")

// projectIssuesByStatusQuery uses GraphQL because the REST API can't resolve a
// user-owned (non-org) project's custom fields.
const projectIssuesByStatusQuery = `
query($login: String!, $number: Int!, $pageSize: Int!, $after: String) {
  user(login: $login) {
    projectV2(number: $number) {
      items(first: $pageSize, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          status: fieldValueByName(name: "Status") {
            ... on ProjectV2ItemFieldSingleSelectValue {
              name
              updatedAt
            }
          }
          content {
            ... on Issue {
              number
              title
              url
              state
              body
              lastEditedAt
              author { __typename login }
              authorAssociation
              editor { login }
            }
          }
        }
      }
    }
  }
}`

type projectIssuesByStatusResponse struct {
	Data struct {
		User struct {
			ProjectV2 *struct {
				Items struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []projectItemNodeWire `json:"nodes"`
				} `json:"items"`
			} `json:"projectV2"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"errors"`
}

type projectItemNodeWire struct {
	Status struct {
		Name      string    `json:"name"`
		UpdatedAt time.Time `json:"updatedAt"`
	} `json:"status"`
	Content struct {
		Number            int64      `json:"number"`
		Title             string     `json:"title"`
		URL               string     `json:"url"`
		State             string     `json:"state"`
		Body              string     `json:"body"`
		LastEditedAt      *time.Time `json:"lastEditedAt"`
		Author            actorWire  `json:"author"`
		AuthorAssociation string     `json:"authorAssociation"`
		Editor            actorWire  `json:"editor"`
	} `json:"content"`
}

type actorWire struct {
	Typename string `json:"__typename"`
	Login    string `json:"login"`
}

// restLogin matches the REST API, which suffixes App logins with "[bot]";
// GraphQL omits the suffix.
func (a actorWire) restLogin() string {
	if a.Typename == "Bot" {
		return a.Login + "[bot]"
	}
	return a.Login
}

// ListProjectIssuesByStatus returns open issues on the owner's Projects (v2)
// board projectNumber whose Status matches (case-insensitive).
func (c *client) ListProjectIssuesByStatus(
	ctx context.Context, projectNumber int64, status string,
) ([]ProjectIssue, error) {
	repo, err := c.resolveRepo(ctx)
	if err != nil {
		return nil, err
	}
	owner, _, ok := strings.Cut(repo, "/")
	if !ok || owner == "" {
		return nil, fmt.Errorf("github: malformed repo %q", repo)
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	nodes, err := c.listProjectItems(ctx, token, owner, projectNumber)
	if err != nil {
		return nil, err
	}
	return filterProjectIssuesByStatus(nodes, status, owner), nil
}

// listProjectItems follows the items cursor until the board is exhausted.
func (c *client) listProjectItems(
	ctx context.Context, token, owner string, projectNumber int64,
) ([]projectItemNodeWire, error) {
	var nodes []projectItemNodeWire
	var after *string
	for range projectItemsMaxPages {
		var resp projectIssuesByStatusResponse
		err := c.postGraphQL(ctx, token, projectIssuesByStatusQuery, map[string]any{
			"login":    owner,
			"number":   projectNumber,
			"pageSize": projectItemsPageSize,
			"after":    after,
		}, &resp)
		if err != nil {
			return nil, err
		}
		if len(resp.Errors) > 0 {
			e := resp.Errors[0]
			if e.Type == "INSUFFICIENT_SCOPES" {
				return nil, fmt.Errorf("%w: %s", ErrInsufficientScopes, e.Message)
			}
			return nil, fmt.Errorf("github GraphQL error: %s", e.Message)
		}
		project := resp.Data.User.ProjectV2
		if project == nil {
			return nil, fmt.Errorf(
				"github: project %d not found for %s", projectNumber, owner,
			)
		}
		nodes = append(nodes, project.Items.Nodes...)
		if !project.Items.PageInfo.HasNextPage {
			return nodes, nil
		}
		cursor := project.Items.PageInfo.EndCursor
		after = &cursor
	}
	return nil, fmt.Errorf(
		"github: project %d has more than %d items",
		projectNumber, projectItemsMaxPages*projectItemsPageSize,
	)
}

func filterProjectIssuesByStatus(
	nodes []projectItemNodeWire, status, repoOwner string,
) []ProjectIssue {
	issues := make([]ProjectIssue, 0, len(nodes))
	for _, n := range nodes {
		if n.Content.Number == 0 || n.Content.State != issueStateOpen {
			continue
		}
		if !strings.EqualFold(n.Status.Name, status) {
			continue
		}
		edited := n.Content.LastEditedAt
		issues = append(issues, ProjectIssue{
			Number:             n.Content.Number,
			Title:              n.Content.Title,
			URL:                n.Content.URL,
			Status:             n.Status.Name,
			AuthorLogin:        n.Content.Author.restLogin(),
			AuthorAssociation:  n.Content.AuthorAssociation,
			StatusUpdatedAt:    n.Status.UpdatedAt,
			BodyHasHTMLComment: strings.Contains(n.Content.Body, "<!--"),
			BodyEditedAfterStatus: edited != nil &&
				edited.After(n.Status.UpdatedAt) &&
				!strings.EqualFold(n.Content.Editor.Login, repoOwner),
		})
	}
	return issues
}

// postGraphQL posts a GraphQL query into dst, with doWithRetry's retries.
func (c *client) postGraphQL(
	ctx context.Context, token, query string, variables map[string]any, dst any,
) error {
	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return err
	}

	return c.doWithRetry(ctx, func() (bool, error) {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodPost, baseURL+"/graphql", bytes.NewReader(body),
		)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(resp.Body)
			return true, fmt.Errorf(
				"%w: github API returned %d: %s",
				errTransientStatus, resp.StatusCode, string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"github API returned %d: %s", resp.StatusCode, string(raw),
			)
		}

		return false, json.NewDecoder(resp.Body).Decode(dst)
	})
}
