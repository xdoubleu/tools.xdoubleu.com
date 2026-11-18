package todoist

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSection_ReturnsID(t *testing.T) {
	var gotPath string
	var gotBody createSectionRequest
	c := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		_ = json.NewEncoder(w).Encode(idResponse{ID: "sec-1"})
	})

	id, err := c.CreateSection(t.Context(), "proj-1", "Go concurrency")
	require.NoError(t, err)
	assert.Equal(t, "sec-1", id)
	assert.Equal(t, "/sections", gotPath)
	assert.Equal(t, "Go concurrency", gotBody.Name)
	assert.Equal(t, "proj-1", gotBody.ProjectID)
}

func TestSectionExists(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   bool
		err    bool
	}{
		{"found", http.StatusOK, true, false},
		{"deleted", http.StatusNotFound, false, false},
		{"server error", http.StatusInternalServerError, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/sections/sec-1", r.URL.Path)
				w.WriteHeader(tc.status)
			})

			got, err := c.SectionExists(t.Context(), "sec-1")
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.err, err != nil)
		})
	}
}

func TestDeleteSection_NotFoundSucceeds(t *testing.T) {
	var gotMethod, gotPath string
	c := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNotFound)
	})

	require.NoError(t, c.DeleteSection(t.Context(), "sec-1"))
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/sections/sec-1", gotPath)
}
