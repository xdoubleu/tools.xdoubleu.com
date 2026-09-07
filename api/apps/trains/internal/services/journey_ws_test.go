package services_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/internal/logging"
)

// fakeDetailFetcher is a minimal journeyDetailFetcher stand-in so
// JourneyWSService can be tested without a real database.
type fakeDetailFetcher struct {
	calls int32
	err   error
}

func (f *fakeDetailFetcher) GetJourneyDetail(
	_ context.Context, journeyID string,
) (*models.JourneyDetail, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.err != nil {
		return nil, f.err
	}
	//nolint:exhaustruct //only Legs is exercised by these tests
	return &models.JourneyDetail{
		Legs: []models.LegDetail{{TripShortName: journeyID}},
	}, nil
}

func TestJourneyWSService_EnsureTopic_IsIdempotent(t *testing.T) {
	fake := &fakeDetailFetcher{} //nolint:exhaustruct //calls/err zero-valued deliberately
	svc := services.NewJourneyWSService(
		context.Background(), logging.NewNopLogger(), []string{"*"}, fake,
	)

	svc.EnsureTopic("journey-1")
	svc.EnsureTopic("journey-1")
	svc.EnsureTopic("journey-1")

	// EnsureTopic itself doesn't call the fetcher — only a subscribe or
	// PushAll does — so registering the same topic 3 times must not create
	// 3 separate topics under the hood. PushAll (which fans out over every
	// registered topic) is the observable proxy for that: it must only
	// invoke the fetcher once for "journey-1", not 3 times.
	svc.PushAll(context.Background())
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.calls))
}

func TestJourneyWSService_PushAll_SkipsFailingTopicsAndContinues(t *testing.T) {
	fake := &fakeDetailFetcher{} //nolint:exhaustruct //calls/err zero-valued deliberately
	svc := services.NewJourneyWSService(
		context.Background(), logging.NewNopLogger(), []string{"*"}, fake,
	)
	svc.EnsureTopic("journey-a")
	svc.EnsureTopic("journey-b")

	fake.err = errors.New("boom")
	// Must not panic even though every topic's rebuild fails.
	svc.PushAll(context.Background())
	assert.GreaterOrEqual(t, atomic.LoadInt32(&fake.calls), int32(2))
}

func TestJourneyWSService_Handler_ReturnsNonNilHandlerFunc(t *testing.T) {
	fake := &fakeDetailFetcher{} //nolint:exhaustruct //calls/err zero-valued deliberately
	svc := services.NewJourneyWSService(
		context.Background(), logging.NewNopLogger(), []string{"*"}, fake,
	)
	require.NotNil(t, svc.Handler())
}
