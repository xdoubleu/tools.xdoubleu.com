package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

func TestAPersonWithoutTheAdminRoleCantManageRoutines(t *testing.T) {
	s := new(JobService)
	person := models.Actor{
		Kind: models.ActorHuman, ID: "u", Browser: true, Admin: false, Via: "",
	}
	ctx := context.Background()

	_, err := s.SaveRoutine(ctx, person, RoutineInput{
		Name: "noop", RepoID: uuid.Nil, Enabled: true, IntervalMinutes: 5,
	})
	assert.Equal(t, http.StatusForbidden, status(t, err))
	assert.Equal(t, http.StatusForbidden, status(t, s.DeleteRoutine(ctx, person, "noop")))
	_, err = s.RunRoutine(ctx, person, "noop")
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.StopRoutine(ctx, person, "noop")
	assert.Equal(t, http.StatusForbidden, status(t, err))
}
