package httptools_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/errortools"
)

func do(
	t *testing.T,
	fn func(w http.ResponseWriter, r *http.Request),
) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	fn(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) errortools.ErrorDto {
	t.Helper()
	var dto errortools.ErrorDto
	require.NoError(t, httptools.ReadJSON(rec.Body, &dto))
	return dto
}

func TestServerErrorResponse(t *testing.T) {
	t.Parallel()

	rec := do(t, func(w http.ResponseWriter, r *http.Request) {
		httptools.ServerErrorResponse(w, r, errors.New("boom"))
	})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	dto := decode(t, rec)
	assert.Equal(t, errortools.MessageInternalServerError, dto.Message)
}

func TestServerErrorResponse_ShowsErrorsWhenEnabled(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(contexttools.WithShownErrors(req.Context()))
	httptools.ServerErrorResponse(rec, req, errors.New("boom"))

	dto := decode(t, rec)
	assert.Equal(t, "boom", dto.Message)
}

func TestBadRequestResponse(t *testing.T) {
	t.Parallel()

	rec := do(t, func(w http.ResponseWriter, r *http.Request) {
		httptools.BadRequestResponse(w, r, errors.New("bad input"))
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	dto := decode(t, rec)
	assert.Equal(t, "bad input", dto.Message)
}

func TestRateLimitExceededResponse(t *testing.T) {
	t.Parallel()

	rec := do(t, func(w http.ResponseWriter, r *http.Request) {
		httptools.RateLimitExceededResponse(w, r)
	})
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	dto := decode(t, rec)
	assert.Equal(t, errortools.MessageTooManyRequests, dto.Message)
}

func TestForbiddenResponse(t *testing.T) {
	t.Parallel()

	rec := do(t, func(w http.ResponseWriter, r *http.Request) {
		httptools.ForbiddenResponse(w, r)
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	dto := decode(t, rec)
	assert.Equal(t, errortools.MessageForbidden, dto.Message)
}

func TestFailedValidationResponse(t *testing.T) {
	t.Parallel()

	rec := do(t, func(w http.ResponseWriter, r *http.Request) {
		httptools.FailedValidationResponse(
			w,
			r,
			map[string]string{"field": "must be provided"},
		)
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestHandleError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			"bad request",
			errortools.NewBadRequestError(errors.New("bad")),
			http.StatusBadRequest,
		},
		{
			"unauthorized",
			errortools.NewUnauthorizedError(errors.New("no token")),
			http.StatusUnauthorized,
		},
		{"other", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, func(w http.ResponseWriter, r *http.Request) {
				httptools.HandleError(w, r, tt.err)
			})
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
