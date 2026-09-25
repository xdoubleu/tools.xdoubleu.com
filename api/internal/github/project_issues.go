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

	"tools.xdoubleu.com/internal/oauthconn"
)

// ProjectIssue is an open issue on a Projects (v2) board with its Status value.
type ProjectIssue struct {
	Number int64
	Title  string
	URL    string
	Status string
}

const issueStateOpen = "OPEN"

// projectItemsPageSize: boards stay well under this, so no cursor pagination.
const projectItemsPageSize = 100

// projectIssuesByStatusQuery uses GraphQL because the REST API can't resolve a
// user-owned (non-org) project's custom fields.
const projectIssuesByStatusQuery = `
query($login: String!, $number: Int!, $pageSize: Int!) {
  user(login: $login) {
    projectV2(number: $number) {
      items(first: $pageSize) {
        nodes {
          status: fieldValueByName(name: "Status") {
            ... on ProjectV2ItemFieldSingleSelectValue {
              name
            }
          }
          content {
            ... on Issue {
              number
              title
              url
              state
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
			ProjectV2 struct {
				Items struct {
					Nodes []projectItemNodeWire `json:"nodes"`
				} `json:"items"`
			} `json:"projectV2"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type projectItemNodeWire struct {
	Status struct {
		Name string `json:"name"`
	} `json:"status"`
	Content struct {
		Number int64  `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		State  string `json:"state"`
	} `json:"content"`
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

	var resp projectIssuesByStatusResponse
	graphQLErr := c.postGraphQL(ctx, token, projectIssuesByStatusQuery, map[string]any{
		"login":    owner,
		"number":   projectNumber,
		"pageSize": projectItemsPageSize,
	}, &resp)
	if graphQLErr != nil {
		return nil, graphQLErr
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("github GraphQL error: %s", resp.Errors[0].Message)
	}

	return filterProjectIssuesByStatus(
		resp.Data.User.ProjectV2.Items.Nodes,
		status,
	), nil
}

func filterProjectIssuesByStatus(
	nodes []projectItemNodeWire, status string,
) []ProjectIssue {
	issues := make([]ProjectIssue, 0, len(nodes))
	for _, n := range nodes {
		if n.Content.Number == 0 || n.Content.State != issueStateOpen {
			continue
		}
		if !strings.EqualFold(n.Status.Name, status) {
			continue
		}
		issues = append(issues, ProjectIssue{
			Number: n.Content.Number,
			Title:  n.Content.Title,
			URL:    n.Content.URL,
			Status: n.Status.Name,
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
				"github API returned %d: %s", resp.StatusCode, string(raw),
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
