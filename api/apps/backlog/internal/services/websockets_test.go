package services_test

import (
	"context"
	"testing"

	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/backlog/internal/services"
)

// newTestWebSocketService creates a WebSocketService wired to a real (but
// idle) JobQueue. It is usable for testing UpdateState / UpdateProgress
// without a real HTTP connection.
func newTestWebSocketService(t *testing.T) *services.WebSocketService {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const workers = 1
	const queueSize = 10
	logger := logging.NewNopLogger()
	jq := threading.NewJobQueue(ctx, logger, workers, queueSize)

	return services.NewWebSocketService(ctx, logger, []string{"*"}, jq)
}

// TestUpdateProgress_UnknownTopic verifies that calling UpdateProgress for a
// topic that has not been registered is a silent no-op (no panic).
func TestUpdateProgress_UnknownTopic(t *testing.T) {
	svc := newTestWebSocketService(t)
	// Must not panic — topic "unknown" was never registered.
	svc.UpdateProgress("unknown", 5, 10)
}

// TestUpdateState_UnknownTopic verifies the symmetric no-op for UpdateState.
func TestUpdateState_UnknownTopic(t *testing.T) {
	svc := newTestWebSocketService(t)
	svc.UpdateState("unknown", true, nil)
}
