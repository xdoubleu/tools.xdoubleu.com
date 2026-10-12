package learningpaths

// A nil app is safe: handlers return CodeUnauthenticated before touching it.
// The HTTP-level tests can't reach this, as mock auth always authenticates.

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func TestListLearningPaths_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.ListLearningPaths(
		context.Background(),
		&learningpathsv1.ListLearningPathsRequest{},
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestGetLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.GetLearningPath(
		context.Background(),
		&learningpathsv1.GetLearningPathRequest{},
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestCreateLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.CreateLearningPath(
		context.Background(),
		&learningpathsv1.CreateLearningPathRequest{},
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestUpdateLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.UpdateLearningPath(
		context.Background(),
		&learningpathsv1.UpdateLearningPathRequest{},
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestDeleteLearningPath_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.DeleteLearningPath(
		context.Background(),
		&learningpathsv1.DeleteLearningPathRequest{},
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRecordItemProgress_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.RecordItemProgress(
		context.Background(),
		&learningpathsv1.RecordItemProgressRequest{},
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestSetLearningPathPaused_Unauthenticated(t *testing.T) {
	h := &learningPathsConnectHandler{app: nil}
	_, err := h.SetLearningPathPaused(
		context.Background(),
		&learningpathsv1.SetLearningPathPausedRequest{},
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
