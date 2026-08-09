// Package sentrytools contains all sorts of tools for using Sentry.
package sentrytools

import (
	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/gateway/internal/config"
)

// Init initializes Sentry and returns a hub clone suitable for use on
// background contexts. Returns nil, nil when the DSN is empty or env is
// [config.TestEnv] — in those cases no initialization is performed.
func Init(env string, options sentry.ClientOptions) (*sentry.Hub, error) {
	if env == config.TestEnv || options.Dsn == "" {
		return nil, nil //nolint:nilnil //Sentry disabled is not an error
	}

	if err := sentry.Init(options); err != nil {
		return nil, err
	}

	return sentry.CurrentHub().Clone(), nil
}
