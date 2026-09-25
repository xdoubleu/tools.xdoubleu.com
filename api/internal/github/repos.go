package github

import (
	"context"
	"errors"
	"fmt"

	"tools.xdoubleu.com/internal/oauthconn"
)

// Repo is a repository offered by the admin config picker.
type Repo struct {
	FullName string // "owner/name"
}

type repoWire struct {
	FullName string `json:"full_name"`
}

// ListRepos lists repos visible to the connection. It must work before a repo
// is picked, so a missing token is ErrNotConnected, not ErrNotConfigured.
func (c *client) ListRepos(ctx context.Context) ([]Repo, error) {
	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, oauthconn.ErrNotConnected
	}
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"%s/user/repos?per_page=100&sort=updated", baseURL,
	)

	var wires []repoWire
	if getErr := c.get(ctx, endpoint, token, &wires); getErr != nil {
		return nil, getErr
	}

	repos := make([]Repo, 0, len(wires))
	for _, w := range wires {
		repos = append(repos, Repo(w))
	}
	return repos, nil
}
