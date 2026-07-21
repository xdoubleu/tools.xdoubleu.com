package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tools.xdoubleu.com/internal/oauthconn"
)

// App is a DigitalOcean App Platform app the connected account can access,
// offered as a pick-list option for the admin config picker.
type App struct {
	ID   string
	Name string
}

type appWire struct {
	ID   string `json:"id"`
	Spec struct {
		Name string `json:"name"`
	} `json:"spec"`
}

type appsWire struct {
	Apps []appWire `json:"apps"`
}

// ListApps returns the apps visible to the connected account, for the admin
// config picker. Unlike LatestDeployment, this must work before any app ID
// is picked, so a missing token is reported as ErrNotConnected, not
// ErrNotConfigured.
func (c *client) ListApps(ctx context.Context) ([]App, error) {
	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, oauthconn.ErrNotConnected
	}
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/v2/apps", baseURL)

	var wire appsWire
	if getErr := c.get(ctx, endpoint, token, &wire); getErr != nil {
		return nil, getErr
	}

	apps := make([]App, 0, len(wire.Apps))
	for _, w := range wire.Apps {
		apps = append(apps, App{ID: w.ID, Name: w.Spec.Name})
	}
	return apps, nil
}

// Option formats app as a single picker-list string ("id — name") so the
// admin config picker can show a friendly label while still round-tripping
// the exact app ID it needs to store.
func (a App) Option() string {
	return a.ID + " — " + a.Name
}

// AppIDFromOption extracts the app ID from a string previously produced by
// App.Option.
func AppIDFromOption(option string) string {
	id, _, _ := strings.Cut(option, " — ")
	return id
}
