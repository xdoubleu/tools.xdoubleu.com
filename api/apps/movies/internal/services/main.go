package services

import (
	"tools.xdoubleu.com/apps/movies/internal/repositories"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth   auth.Service
	Movies *MovieService
}

// New wires the services; tmdbClient is nil when no API key is configured.
func New(
	repos *repositories.Repositories,
	authService auth.Service,
	tmdbClient tmdb.Client,
) *Services {
	return &Services{
		Auth: authService,
		//nolint:exhaustruct // the provider cache starts empty
		Movies: &MovieService{repo: repos.Movies, tmdb: tmdbClient},
	}
}
