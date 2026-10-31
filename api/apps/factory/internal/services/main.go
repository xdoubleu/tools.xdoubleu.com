// Package services holds the factory's task board rules.
package services

import (
	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth  auth.Service
	Tasks *TaskService
	Repos *RepoService
}

func New(repos *repositories.Repositories, authService auth.Service) *Services {
	return &Services{
		Auth:  authService,
		Tasks: &TaskService{repo: repos.Tasks},
		Repos: &RepoService{repo: repos.Repos},
	}
}
