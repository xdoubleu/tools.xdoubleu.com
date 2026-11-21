package services

import (
	"time"

	"tools.xdoubleu.com/apps/movies/internal/repositories"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/family"
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
	familyRepo *family.Repository,
) *Services {
	return &Services{
		Auth:   authService,
		Movies: NewMovieService(repos.Movies, familyRepo, tmdbClient, time.Now),
	}
}
