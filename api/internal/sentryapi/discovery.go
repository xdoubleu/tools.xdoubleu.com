package sentryapi

import (
	"context"
	"errors"
	"fmt"

	"tools.xdoubleu.com/internal/oauthconn"
)

// Org is a Sentry organization offered by the admin config picker.
type Org struct {
	Slug string
}

// Project is a Sentry project offered by the admin config picker.
type Project struct {
	Slug string
}

type orgWire struct {
	Slug string `json:"slug"`
}

type projectWire struct {
	Slug string `json:"slug"`
}

// ListOrgs lists orgs for the picker. It must work before anything is picked,
// so a missing token is ErrNotConnected, not ErrNotConfigured.
func (c *client) ListOrgs(ctx context.Context) ([]Org, error) {
	token, err := c.resolveToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/0/organizations/", baseURL)

	var wires []orgWire
	if getErr := c.get(ctx, endpoint, token, &wires); getErr != nil {
		return nil, getErr
	}

	orgs := make([]Org, 0, len(wires))
	for _, w := range wires {
		orgs = append(orgs, Org(w))
	}
	return orgs, nil
}

// ListProjects lists org's projects for the picker.
func (c *client) ListProjects(ctx context.Context, org string) ([]Project, error) {
	token, err := c.resolveToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"%s/api/0/organizations/%s/projects/", baseURL, org,
	)

	var wires []projectWire
	if getErr := c.get(ctx, endpoint, token, &wires); getErr != nil {
		return nil, getErr
	}

	projects := make([]Project, 0, len(wires))
	for _, w := range wires {
		projects = append(projects, Project(w))
	}
	return projects, nil
}

func (c *client) resolveToken(ctx context.Context) (string, error) {
	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return "", oauthconn.ErrNotConnected
	}
	return token, err
}
