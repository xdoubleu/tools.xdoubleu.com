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

// TestCreateTask_RequestFails covers the httpClient.Do error branch without
// a real Todoist network call: baseURL points at a closed local port, so the
// connection is refused immediately.
func TestCreateTask_RequestFails(t *testing.T) {
	withTestBaseURL(t, "http://127.0.0.1:1")

	c := NewClient(oauthconn.TokenFunc(func(context.Context) (string, error) {
		return "token", nil
	}))

	_, err := c.CreateTask(t.Context(), "content", "")
	assert.Error(t, err)
}

// TestCreateTask_DecodeError covers a 2xx response whose body isn't valid
// JSON — Todoist returning something unexpected.
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
