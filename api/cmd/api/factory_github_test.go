package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
)

func TestTheFactoryGitHubAppIsWiredOnlyWhenUsable(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))

	cases := map[string]struct {
		id, key string
		options int
		warns   bool
	}{
		"unset":   {id: "", key: "", options: 0, warns: false},
		"usable":  {id: "123", key: keyPEM, options: 1, warns: false},
		"no key":  {id: "123", key: "", options: 0, warns: true},
		"no id":   {id: "", key: keyPEM, options: 0, warns: true},
		"bad key": {id: "123", key: "not a key", options: 0, warns: true},
	}
	for name, tc := range cases {
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		cfg := config.Config{} //nolint:exhaustruct // only the App's id and key matter
		cfg.FactoryGitHubAppID, cfg.FactoryGitHubAppKey = tc.id, tc.key

		got := factoryGitHubOptions(cfg, logger)

		assert.Len(t, got, tc.options, name)
		assert.Equal(t, tc.warns, logs.Len() > 0, name)
		assert.NotContains(t, logs.String(), "not a key", name, "the key is never logged")
	}
}
