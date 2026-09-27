package todoist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauthconn"
)

func withTestBaseURL(t *testing.T, url string) {
	t.Helper()
	orig := baseURL
	baseURL = url
	t.Cleanup(func() { baseURL = orig })
}

func TestCreateTask_Success(t *testing.T) {
	var gotAuth, gotContentType, gotPath string
	var gotBody createTaskRequest

	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotContentType = r.Header.Get("Content-Type")
			gotPath = r.URL.Path
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(createTaskResponse{ID: "task-42"})
		}),
	)
	defer srv.Close()
	withTestBaseURL(t, srv.URL)

	tokenFn := oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token-abc", nil
	})
	c := NewClient(tokenFn)

	taskID, err := c.CreateTask(t.Context(), "Learn Go: read ch3", "every Monday")
	require.NoError(t, err)
	assert.Equal(t, "task-42", taskID)
	assert.Equal(t, "Bearer token-abc", gotAuth)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, "/tasks", gotPath)
	assert.Equal(t, "Learn Go: read ch3", gotBody.Content)
	assert.Equal(t, "every Monday", gotBody.DueString)
}

func TestCreateTask_OmitsEmptyDueString(t *testing.T) {
	var gotBody createTaskRequest

	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(createTaskResponse{ID: "task-1"})
		}),
	)
	defer srv.Close()
	withTestBaseURL(t, srv.URL)

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	_, err := c.CreateTask(t.Context(), "content only", "")
	require.NoError(t, err)
	assert.Empty(t, gotBody.DueString)
}

func TestCreateTask_TokenFuncError(t *testing.T) {
	tokenErr := errors.New("not connected")
	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "", tokenErr
	}))

	_, err := c.CreateTask(t.Context(), "content", "")
	assert.ErrorIs(t, err, tokenErr)
}

func TestCreateTask_NonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid token"}`))
		}),
	)
	defer srv.Close()
	withTestBaseURL(t, srv.URL)

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "bad-token", nil
	}))

	_, err := c.CreateTask(t.Context(), "content", "")
	require.Error(t, err)
	var apiErr *apiError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusUnauthorized, apiErr.status)
	assert.Contains(t, apiErr.Error(), "401")
	assert.Contains(t, apiErr.Error(), "invalid token")
}

// baseURL points at a closed port, so the connection is refused.
func TestCreateTask_RequestFails(t *testing.T) {
	withTestBaseURL(t, "http://127.0.0.1:1")

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	_, err := c.CreateTask(t.Context(), "content", "")
	assert.Error(t, err)
}

func TestCreateTask_DecodeError(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not json"))
		}),
	)
	defer srv.Close()
	withTestBaseURL(t, srv.URL)

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	_, err := c.CreateTask(t.Context(), "content", "")
	assert.Error(t, err)
}

func TestDeleteTask_Success(t *testing.T) {
	var gotAuth, gotPath string

	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	defer srv.Close()
	withTestBaseURL(t, srv.URL)

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	err := c.DeleteTask(t.Context(), "task-7")
	require.NoError(t, err)
	assert.Equal(t, "Bearer token", gotAuth)
	assert.Equal(t, "/tasks/task-7", gotPath)
}

// TestDeleteTask_NotFoundSucceeds: a task Todoist already completed or deleted
// is 404; the idempotent pipeline treats that as success.
func TestDeleteTask_NotFoundSucceeds(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	defer srv.Close()
	withTestBaseURL(t, srv.URL)

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	require.NoError(t, c.DeleteTask(t.Context(), "task-missing"))
}

func TestDeleteTask_PropagatesOtherError(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		}),
	)
	defer srv.Close()
	withTestBaseURL(t, srv.URL)

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	require.Error(t, c.DeleteTask(t.Context(), "task-7"))
}

func TestDeleteTask_TokenFuncError(t *testing.T) {
	tokenErr := errors.New("not connected")
	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "", tokenErr
	}))

	assert.ErrorIs(t, c.DeleteTask(t.Context(), "task-7"), tokenErr)
}

// baseURL points at a closed port, so the delete request is refused.
func TestDeleteTask_RequestFails(t *testing.T) {
	withTestBaseURL(t, "http://127.0.0.1:1")

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	assert.Error(t, c.DeleteTask(t.Context(), "task-7"))
}
