package progressws_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/progressws"
)

// newTestService uses an idle JobQueue; its nil db is never touched.
func newTestService(t *testing.T) *progressws.Service {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const workers = 1
	const queueSize = 10
	logger := logging.NewNopLogger()
	jq := jobqueue.NewJobQueue(ctx, logger, workers, queueSize, nil)

	return progressws.NewService(ctx, logger, []string{"*"}, jq)
}

func TestUpdateProgress_UnknownTopic(t *testing.T) {
	svc := newTestService(t)
	svc.UpdateProgress("unknown", 5, 10)
}

func TestUpdateState_UnknownTopic(t *testing.T) {
	svc := newTestService(t)
	svc.UpdateState("unknown", true, nil)
}

func TestSubscribeMessageDtoTopic(t *testing.T) {
	dto := progressws.SubscribeMessageDto{Subject: "steam"}
	assert.Equal(t, "steam", dto.Topic())
}

func TestSubscribeMessageDtoValidate(t *testing.T) {
	dto := progressws.SubscribeMessageDto{Subject: "steam"}
	ok, errs := dto.Validate()
	assert.True(t, ok)
	assert.Empty(t, errs)
}
