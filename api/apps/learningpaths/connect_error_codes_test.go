package learningpaths_test

import (
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func TestErrorCodes(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()
	unknown := uuid.NewString()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"get unknown path", func() error {
			_, err := client.GetLearningPath(
				ctx, &learningpathsv1.GetLearningPathRequest{Id: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"delete unknown path", func() error {
			_, err := client.DeleteLearningPath(
				ctx, &learningpathsv1.DeleteLearningPathRequest{Id: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"update unknown path", func() error {
			_, err := client.UpdateLearningPath(
				ctx, &learningpathsv1.UpdateLearningPathRequest{
					Id: unknown, Title: "codes",
				},
			)
			return err
		}, connect.CodeNotFound},
		{"pause unknown path", func() error {
			_, err := client.SetLearningPathPaused(
				ctx, &learningpathsv1.SetLearningPathPausedRequest{
					Id: unknown, Paused: true,
				},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid path id", func() error {
			_, err := client.GetLearningPath(
				ctx, &learningpathsv1.GetLearningPathRequest{Id: "nope"},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"resource linked to unknown book", func() error {
			_, err := client.CreateLearningPath(
				ctx, &learningpathsv1.CreateLearningPathRequest{
					Title: "codes",
					Resources: []*learningpathsv1.Resource{
						{Text: "book", LinkedBookId: &unknown},
					},
				},
			)
			return err
		}, connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
