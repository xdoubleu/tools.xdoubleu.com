package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"tools.xdoubleu.com/internal/oauthconn"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://api.todoist.com/api/v1"

const requestTimeout = 15 * time.Second

// apiError is a non-2xx response from the Todoist API.
type apiError struct {
	status int
	body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("todoist API returned %d: %s", e.status, e.body)
}

type client struct {
	httpClient *http.Client
	tokenFn    oauthconn.TokenFunc
}

var _ Client = (*client)(nil)

// NewClient builds a Client that resolves a live bearer token via tokenFn
// (an oauthconn.TokenFunc bound to one specific user's connection — see
// api/apps/learningpaths/internal/repositories/oauth_connections.go) on
// every call, so a rotated/refreshed token is always used without the
// caller having to manage it.
func NewClient(tokenFn oauthconn.TokenFunc) Client {
	return &client{
		httpClient: &http.Client{Timeout: requestTimeout},
		tokenFn:    tokenFn,
	}
}

type createTaskRequest struct {
	Content   string `json:"content"`
	DueString string `json:"due_string,omitempty"`
}

type createTaskResponse struct {
	ID string `json:"id"`
}

func (c *client) CreateTask(
	ctx context.Context, content, dueString string,
) (string, error) {
	token, err := c.tokenFn(ctx)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(createTaskRequest{
		Content:   content,
		DueString: dueString,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/tasks", bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(resp.Body)
		return "", &apiError{status: resp.StatusCode, body: string(raw)}
	}

	var out createTaskResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&out); decErr != nil {
		return "", decErr
	}
	return out.ID, nil
}
