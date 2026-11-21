package http_test

import (
	"crypto/rsa"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/platform/identity/oauth2as"
)

func newTestStore(t *testing.T) (*oauth2as.Store, *pgxpool.Pool) {
	t.Helper()
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	return oauth2as.NewStore(db), db
}

// oauthConfig is the authorization server's view of the app config.
func oauthConfig(cfg config.Config) oauth2as.Config {
	return oauth2as.Config{
		HMACSecret: cfg.OAuthHMACSecret,
		Issuer:     cfg.AuthIssuer,
		APIURL:     cfg.APIURL,
		WebURL:     cfg.WebURL,
	}
}

// testOIDCKeyOnce shares one generated RSA key across tests (generation is slow).
//
//nolint:gochecknoglobals // one shared RSA signing key across the package's tests
var (
	testOIDCKeyOnce sync.Once
	testOIDCKeyVal  *rsa.PrivateKey
)

func testOIDCKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testOIDCKeyOnce.Do(func() {
		key, _, err := oauth2as.LoadOrGenerateOIDCKey("")
		require.NoError(t, err)
		testOIDCKeyVal = key
	})
	return testOIDCKeyVal
}
