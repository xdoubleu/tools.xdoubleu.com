//nolint:testpackage // tests unexported helpers
package shoppinglist

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	iapp "tools.xdoubleu.com/internal/app"
)

// ── mapError ──────────────────────────────────────────────────────────────────

func TestMapError_Nil(t *testing.T) {
	assert.Nil(t, mapError(nil))
}

func TestMapError_ResourceNotFound(t *testing.T) {
	err := mapError(database.ErrResourceNotFound)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestMapError_HTTPBadRequest(t *testing.T) {
	err := mapError(&iapp.HTTPError{Status: http.StatusBadRequest, Message: "bad"})
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestMapError_HTTPNotFound(t *testing.T) {
	err := mapError(&iapp.HTTPError{Status: http.StatusNotFound, Message: "not found"})
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestMapError_HTTPForbidden(t *testing.T) {
	err := mapError(&iapp.HTTPError{Status: http.StatusForbidden, Message: "forbidden"})
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestMapError_HTTPOther(t *testing.T) {
	err := mapError(
		&iapp.HTTPError{Status: http.StatusInternalServerError, Message: "oops"},
	)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

func TestMapError_GenericError(t *testing.T) {
	err := mapError(errors.New("some error"))
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

// ── exportWindow ──────────────────────────────────────────────────────────────

func TestExportWindow_BeforeBreakfastEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotBreakfastEnd-1, 0, 0, 0, time.UTC)
	today, pastSlots := exportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Empty(t, pastSlots)
}

func TestExportWindow_AtBreakfastEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotBreakfastEnd, 0, 0, 0, time.UTC)
	today, pastSlots := exportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Equal(t, []string{slotBreakfast}, pastSlots)
}

func TestExportWindow_AtNoonEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotNoonEnd, 0, 0, 0, time.UTC)
	today, pastSlots := exportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Equal(t, []string{slotBreakfast, slotNoon}, pastSlots)
}

func TestExportWindow_AtEveningEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotEveningEnd, 0, 0, 0, time.UTC)
	today, pastSlots := exportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Equal(t, []string{slotBreakfast, slotNoon, slotEvening}, pastSlots)
}
