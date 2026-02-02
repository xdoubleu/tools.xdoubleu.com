package goaltracker_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v2/pkg/test"
	"tools.xdoubleu.com/apps/goaltracker/internal/dtos"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
)

func TestRoot(t *testing.T) {
	err := testApp.Services.Goals.ImportStatesFromTodoist(context.Background(), userID)
	if err != nil {
		panic(err)
	}

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		testApp.GetName(),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestLink(t *testing.T) {
	err := testApp.Services.Goals.ImportGoalsFromTodoist(
		context.Background(),
		testApp.Config.SupabaseUserID,
	)
	if err != nil {
		panic(err)
	}

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		fmt.Sprintf("/%s/edit/123", testApp.GetName()),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestGoalProgressGraph(t *testing.T) {
	err := testApp.Services.Goals.ImportGoalsFromTodoist(
		context.Background(),
		testApp.Config.SupabaseUserID,
	)
	if err != nil {
		panic(err)
	}

	val := int64(50)
	err = testApp.Services.Goals.LinkGoal(
		context.Background(),
		goalID,
		userID,
		&dtos.LinkGoalDto{
			TypeID:      models.SteamCompletionRate.ID,
			TargetValue: &val,
			Tag:         nil,
		},
	)
	if err != nil {
		panic(err)
	}

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		fmt.Sprintf("/%s/goals/123", testApp.GetName()),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestGoalProgressList(t *testing.T) {
	err := testApp.Services.Goals.ImportGoalsFromTodoist(
		context.Background(),
		testApp.Config.SupabaseUserID,
	)
	if err != nil {
		panic(err)
	}

	val := int64(50)
	valStr := "fiction"
	err = testApp.Services.Goals.LinkGoal(
		context.Background(),
		goalID,
		userID,
		&dtos.LinkGoalDto{
			TypeID:      models.BooksFromSpecificTag.ID,
			TargetValue: &val,
			Tag:         &valStr,
		},
	)
	if err != nil {
		panic(err)
	}

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		fmt.Sprintf("/%s/goals/123", testApp.GetName()),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
