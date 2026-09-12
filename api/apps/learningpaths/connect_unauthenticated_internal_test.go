package learningpaths

// Package-internal test: getUser(ctx) returns nil (and each handler returns
// CodeUnauthenticated) before it ever touches h.app, so a nil app is safe
// here — this exercises a branch the HTTP-level tests in connect_test.go
// can't reach, since the mocked auth service there always authenticates the
// caller as some user.

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func TestListLearningPaths_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.ListLearningPaths(
		context.Background(),
		connect.NewRequest(&learningpathsv1.ListLearningPathsRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestGetLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.GetLearningPath(
		context.Background(),
		connect.NewRequest(&learningpathsv1.GetLearningPathRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestCreateLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.CreateLearningPath(
		context.Background(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestUpdateLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.UpdateLearningPath(
		context.Background(),
		connect.NewRequest(&learningpathsv1.UpdateLearningPathRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestDeleteLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.DeleteLearningPath(
		context.Background(),
		connect.NewRequest(&learningpathsv1.DeleteLearningPathRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRecordItemProgress_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.RecordItemProgress(
		context.Background(),
		connect.NewRequest(&learningpathsv1.RecordItemProgressRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
