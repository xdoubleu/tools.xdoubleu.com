package dtos_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/icsproxy/internal/dtos"
)

func TestHideSeries_Empty(t *testing.T) {
	dto := &dtos.CreateFilterDto{} //nolint:exhaustruct // test only uses relevant fields
	result := dto.HideSeries(url.Values{})
	assert.Empty(t, result)
}

func TestHideSeries_WithMatchingKeys(t *testing.T) {
	dto := &dtos.CreateFilterDto{} //nolint:exhaustruct // test only uses relevant fields
	form := url.Values{
		"hide_rec_Daily Standup": {"1"},
		"hide_rec_Weekly Review": {"1"},
		"source_url":             {"https://example.com"},
	}
	result := dto.HideSeries(form)
	assert.True(t, result["Daily Standup"])
	assert.True(t, result["Weekly Review"])
	assert.NotContains(t, result, "source_url")
}

func TestHideSeries_NoMatchingKeys(t *testing.T) {
	dto := &dtos.CreateFilterDto{} //nolint:exhaustruct // test only uses relevant fields
	form := url.Values{
		"source_url": {"https://example.com"},
		"token":      {"abc"},
	}
	result := dto.HideSeries(form)
	assert.Empty(t, result)
}
