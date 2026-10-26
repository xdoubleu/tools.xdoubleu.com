package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// NewClient builds a Client that resolves a fresh token via tokenFn (bound to
// one user's connection) on every call.
func NewClient(tokenFn oauthconn.TokenFunc) Client {
	return &client{
		httpClient: &http.Client{Timeout: requestTimeout},
		tokenFn:    tokenFn,
	}
}

// errNotFound is a 404: the task or project no longer exists in Todoist.
var errNotFound = errors.New("todoist: not found")

type createTaskRequest struct {
	Content   string `json:"content"`
	DueString string `json:"due_string,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

type createProjectRequest struct {
	Name string `json:"name"`
}

type idResponse struct {
	ID string `json:"id"`
}

func (c *client) CreateTask(
	ctx context.Context, content, dueString, projectID string,
) (string, error) {
	var out idResponse
	err := c.send(ctx, http.MethodPost, "/tasks", createTaskRequest{
		Content:   content,
		DueString: dueString,
		ProjectID: projectID,
	}, &out)
	return out.ID, err
}

// DeleteTask deletes the task at /tasks/{id}. A 404 (already completed or
// deleted in Todoist) counts as success so retries are idempotent.
func (c *client) DeleteTask(ctx context.Context, taskID string) error {
	return ignoreNotFound(c.send(ctx, http.MethodDelete, "/tasks/"+taskID, nil, nil))
}

func (c *client) CreateProject(ctx context.Context, name string) (string, error) {
	var out idResponse
	err := c.send(
		ctx, http.MethodPost, "/projects", createProjectRequest{Name: name}, &out,
	)
	return out.ID, err
}

func (c *client) ProjectExists(ctx context.Context, projectID string) (bool, error) {
	err := c.send(ctx, http.MethodGet, "/projects/"+projectID, nil, nil)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (c *client) DeleteProject(ctx context.Context, projectID string) error {
	return ignoreNotFound(
		c.send(ctx, http.MethodDelete, "/projects/"+projectID, nil, nil),
	)
}

func ignoreNotFound(err error) error {
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}

// send issues an authenticated request with an optional JSON body and decodes
// the JSON response into out when non-nil. A 404 returns errNotFound.
func (c *client) send(
	ctx context.Context, method, path string, in, out any,
) error {
	token, err := c.tokenFn(ctx)
	if err != nil {
		return err
	}

	var body io.Reader
	if in != nil {
		raw, marshalErr := json.Marshal(in)
		if marshalErr != nil {
			return marshalErr
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(resp.Body)
		return &apiError{status: resp.StatusCode, body: string(raw)}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
