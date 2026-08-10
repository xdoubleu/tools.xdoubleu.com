package sentrytools

import "github.com/getsentry/sentry-go"

// testEnv is the env value that skips initialization entirely. Callers pass
// their own module's env string (e.g. config.TestEnv) straight through —
// every component in this repo defines it as this same literal.
const testEnv = "test"

// Init initializes Sentry and returns a hub clone suitable for use on
// background contexts (e.g. job queues). Returns nil, nil when the DSN is
// empty or env is the literal "test" (every component's config.TestEnv) —
// in those cases no initialization is performed.
func Init(env string, options sentry.ClientOptions) (*sentry.Hub, error) {
	if env == testEnv || options.Dsn == "" {
		return nil, nil //nolint:nilnil //Sentry disabled is not an error
	}

	if err := sentry.Init(options); err != nil {
		return nil, err
	}

	return sentry.CurrentHub().Clone(), nil
}
