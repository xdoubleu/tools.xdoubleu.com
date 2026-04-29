//nolint:testpackage // tests unexported helpers
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURLToTitle_LastSegment(t *testing.T) {
	assert.Equal(t, "CR-1234", urlToTitle("https://jira.company.com/browse/CR-1234"))
	assert.Equal(t, "42", urlToTitle("https://github.com/org/repo/pull/42"))
	assert.Equal(t, "https://example.com", urlToTitle("https://example.com"))
}

func TestURLToTitle_TrailingSlash(t *testing.T) {
	assert.Equal(t, "CR-1234", urlToTitle("https://jira.company.com/browse/CR-1234/"))
}

func TestURLToTitle_InvalidURL(t *testing.T) {
	assert.Equal(t, "not a url", urlToTitle("not a url"))
}

func TestParseDatePtr_ValidDate(t *testing.T) {
	p := parseDatePtr("2026-05-01")
	assert.NotNil(t, p)
	assert.Equal(t, 2026, p.Year())
	assert.Equal(t, 5, int(p.Month()))
	assert.Equal(t, 1, p.Day())
}

func TestParseDatePtr_Empty(t *testing.T) {
	assert.Nil(t, parseDatePtr(""))
}

func TestParseDatePtr_Invalid(t *testing.T) {
	assert.Nil(t, parseDatePtr("not-a-date"))
}
