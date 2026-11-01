package services

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database"
)

func TestNewCodeIsTwoGroupsOfFromTheAlphabet(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{5}-[0-9A-HJKMNP-TV-Z]{5}$`)
	seen := map[string]bool{}
	for range 200 {
		code, err := newCode()
		require.NoError(t, err)
		assert.Regexp(t, pattern, code)
		seen[code] = true
	}
	assert.Greater(t, len(seen), 190, "codes are random, not repeated")
}

func TestNormaliseCode(t *testing.T) {
	for in, want := range map[string]string{
		"abcde-fghjk": "ABCDEFGHJK",
		"ABCDE FGHJK": "ABCDEFGHJK",
		"0O0O0-0O0O0": "0000000000",
		"1I1L1-1l1i1": "1111111111",
		"":            "",
		"a-b-c":       "ABC",
	} {
		assert.Equal(t, want, normaliseCode(in), in)
	}
}

func TestNormaliseCodeIgnoresSurroundingSpace(t *testing.T) {
	assert.Equal(t, "ABCDEFGHJK", normaliseCode("\tabcde fghjk \n"))
}

func TestHashSecretIsStableAndDistinct(t *testing.T) {
	assert.Equal(t, hashSecret("a"), hashSecret("a"))
	assert.NotEqual(t, hashSecret("a"), hashSecret("b"))
	assert.Len(t, hashSecret("a"), 64)
	assert.NotContains(t, hashSecret("secret"), "secret")
}

func TestNewTokenHasThePrefixAndIsRandom(t *testing.T) {
	a, err := newToken()
	require.NoError(t, err)
	b, err := newToken()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(a, TokenPrefix))
	assert.Len(t, a, len(TokenPrefix)+43)
	assert.NotEqual(t, a, b)
}

func TestValidRunnerName(t *testing.T) {
	got, ok := ValidRunnerName("  Home Mac ")
	assert.True(t, ok)
	assert.Equal(t, "Home Mac", got)

	_, ok = ValidRunnerName(strings.Repeat("a", maxRunnerName))
	assert.True(t, ok)
	_, ok = ValidRunnerName(strings.Repeat("é", maxRunnerName))
	assert.True(t, ok, "characters, not bytes")
	for _, bad := range []string{
		"", "  ", strings.Repeat("a", maxRunnerName+1), "two\nlines", "nul\x00",
	} {
		_, ok = ValidRunnerName(bad)
		assert.False(t, ok, bad)
	}
}

// These need no database: each is refused before the repository is reached.
func TestRunnerServiceRefusesBadInputBeforeTheDatabase(t *testing.T) {
	svc := &RunnerService{repo: nil, now: nil}
	ctx := context.Background()

	_, _, err := svc.Create(ctx, "human:x", " ")
	assert.Equal(t, http.StatusBadRequest, status(t, err))

	for _, code := range []string{"", "ABC", "ABCDE-FGHJK-M", strings.Repeat("A", 9)} {
		_, _, err = svc.Pair(ctx, code, "1")
		assert.Equal(t, http.StatusBadRequest, status(t, err), code)
	}

	for _, token := range []string{"", "no-prefix", "FRT_upper", "Bearer frt_x"} {
		_, err = svc.Authenticate(ctx, token)
		assert.ErrorIs(t, err, database.ErrResourceNotFound, token)
	}
}

func TestPairingCodeLengthIsTen(t *testing.T) {
	code, err := newCode()
	require.NoError(t, err)
	assert.Len(t, normaliseCode(code), codeLength)
}
