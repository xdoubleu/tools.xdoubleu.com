//nolint:testpackage // testing the unexported redactKoboToken helper directly
package books

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactKoboToken(t *testing.T) {
	assert.Equal(t,
		"/books/kobo/redacted/v1/library/sync",
		redactKoboToken("/books/kobo/AbC123secret/v1/library/sync", "AbC123secret"),
	)
	assert.Equal(t,
		"/books/kobo//v1/x",
		redactKoboToken("/books/kobo//v1/x", ""),
		"empty token must leave the path unchanged",
	)
}
