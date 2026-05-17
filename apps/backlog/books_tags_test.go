package backlog_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/backlog/internal/dtos"
	"tools.xdoubleu.com/apps/backlog/internal/models"
)

func TestToggleTag(t *testing.T) {
	ub := addTestBook(t, "ToggleTagBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/tags",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ToggleTagDto{Tag: "classics"})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestToggleTag_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/not-a-uuid/tags",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ToggleTagDto{Tag: "classics"})
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestToggleTag_EmptyTag(t *testing.T) {
	ub := addTestBook(t, "EmptyTagBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/tags",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ToggleTagDto{Tag: ""})
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// TestToggleTag_HTMX exercises the HTMX branch of toggleTagHandler, which
// calls buildLibraryData and renders BooksLibraryPage inline.
func TestToggleTag_HTMX(t *testing.T) {
	ub := addTestBook(t, "HXToggleBook")

	body := strings.NewReader("tag=htmx-tag")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/tags",
		body,
	)
	req.Header.Set("Content-Type", string(test.FormContentType))
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&accessToken)

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestToggleTag_RemovesExistingTag calls toggleTagHandler with a tag the book
// already has, exercising the found=true / removal path in ToggleTag.
func TestToggleTag_RemovesExistingTag(t *testing.T) {
	ub := addTestBookWithStatus(
		t,
		"RemoveTagBook",
		models.StatusToRead,
		[]string{"existing-tag"},
	)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/tags",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ToggleTagDto{Tag: "existing-tag"})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}
