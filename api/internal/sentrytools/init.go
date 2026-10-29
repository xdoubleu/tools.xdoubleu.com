package sentrytools

import "github.com/getsentry/sentry-go"

// testEnv matches every component's config.TestEnv literal.
const testEnv = "test"

// Init initializes Sentry and returns a hub clone for background contexts.
// Returns nil, nil when the DSN is empty or env is "test".
func Init(env string, options sentry.ClientOptions) (*sentry.Hub, error) {
	if env == testEnv || options.Dsn == "" {
		return nil, nil //nolint:nilnil //Sentry disabled is not an error
	}

	if err := sentry.Init(options); err != nil {
		return nil, err
	}

	return sentry.CurrentHub().Clone(), nil
}
