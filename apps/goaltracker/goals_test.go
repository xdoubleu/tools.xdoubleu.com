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

func TestEditGoalHandler(t *testing.T) {
	//nolint:exhaustruct,errcheck //other fields are optional
	defer testApp.Repositories.Goals.Delete(
		context.Background(),
		&models.Goal{ID: goalID},
		userID,
	)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		fmt.Sprintf("/%s/api/goals/%s/edit", testApp.GetName(), goalID),
	)

	tReq.SetFollowRedirect(false)

	tReq.AddCookie(&accessToken)
	tReq.AddCookie(&refreshToken)

	targetValue := int64(50)
	tag := ""

	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.LinkGoalDto{
		TypeID:      models.SteamCompletionRate.ID,
		TargetValue: &targetValue,
		Tag:         &tag,
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestUnlinkGoalHandler(t *testing.T) {
	_, err := testApp.Repositories.Goals.Upsert(
		context.Background(),
		goalID,
		userID,
		"Goal",
		"1",
		nil,
		1,
	)
	if err != nil {
		panic(err)
	}
	//nolint:exhaustruct,errcheck //other fields are optional
	defer testApp.Repositories.Goals.Delete(
		context.Background(),
		&models.Goal{ID: goalID},
		userID,
	)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		fmt.Sprintf("/%s/api/goals/%s/unlink", testApp.GetName(), goalID),
	)

	tReq.SetFollowRedirect(false)

	tReq.AddCookie(&accessToken)
	tReq.AddCookie(&refreshToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestCompleteGoalHandler(t *testing.T) {
	_, err := testApp.Repositories.Goals.Upsert(
		context.Background(),
		goalID,
		userID,
		"Goal",
		"1",
		nil,
		1,
	)
	if err != nil {
		panic(err)
	}
	//nolint:exhaustruct,errcheck //other fields are optional
	defer testApp.Repositories.Goals.Delete(
		context.Background(),
		&models.Goal{ID: goalID},
		userID,
	)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		fmt.Sprintf("/%s/api/goals/%s/complete", testApp.GetName(), goalID),
	)

	tReq.SetFollowRedirect(false)

	tReq.AddCookie(&accessToken)
	tReq.AddCookie(&refreshToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}
