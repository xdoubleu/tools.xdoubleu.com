//nolint:testpackage // tests unexported helpers
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// ── parseFancyURL ─────────────────────────────────────────────────────────────

func TestParseFancyURL_Valid(t *testing.T) {
	title, rawURL, rest, ok := parseFancyURL(
		"[Fix homepage bug](https://jira.example.com/PROJ-42)",
	)
	require.True(t, ok)
	assert.Equal(t, "Fix homepage bug", title)
	assert.Equal(t, "https://jira.example.com/PROJ-42", rawURL)
	assert.Equal(t, "", rest)
}

func TestParseFancyURL_WithTrailingShortcuts(t *testing.T) {
	title, rawURL, rest, ok := parseFancyURL(
		"[Fix bug](https://jira.example.com/PROJ-42) p1 @cr",
	)
	require.True(t, ok)
	assert.Equal(t, "Fix bug", title)
	assert.Equal(t, "https://jira.example.com/PROJ-42", rawURL)
	assert.Equal(t, "p1 @cr", rest)
}

func TestParseFancyURL_PlainURL(t *testing.T) {
	_, _, _, ok := parseFancyURL("https://example.com/path")
	assert.False(t, ok)
}

func TestParseFancyURL_PlainTitle(t *testing.T) {
	_, _, _, ok := parseFancyURL("buy milk today")
	assert.False(t, ok)
}

func TestParseFancyURL_MissingURL(t *testing.T) {
	_, _, _, ok := parseFancyURL("[Title](not-a-url)")
	assert.False(t, ok)
}

// ── shortcutQueryPattern ──────────────────────────────────────────────────────

func TestShortcutQueryPattern_Matches(t *testing.T) {
	m := shortcutQueryPattern.FindStringSubmatch("DCP1234")
	require.NotNil(t, m)
	assert.Equal(t, "DCP", m[1])
	assert.Equal(t, "1234", m[2])
}

func TestShortcutQueryPattern_WithDash(t *testing.T) {
	m := shortcutQueryPattern.FindStringSubmatch("PROJ-42")
	require.NotNil(t, m)
	assert.Equal(t, "PROJ", m[1])
	assert.Equal(t, "-42", m[2])
}

func TestShortcutQueryPattern_NoMatch_LowerCase(t *testing.T) {
	assert.Nil(t, shortcutQueryPattern.FindStringSubmatch("dcp1234"))
}

func TestShortcutQueryPattern_NoMatch_PlainText(t *testing.T) {
	assert.Nil(t, shortcutQueryPattern.FindStringSubmatch("fix bug"))
}
