package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

type rawClient struct {
	mocks.StubClient
	raw []byte
	err error
}

func (c *rawClient) RawVehicle(
	context.Context, string, time.Time,
) ([]byte, error) {
	return c.raw, c.err
}

func TestIrailService_Diagnostic(t *testing.T) {
	ctx := context.Background()
	client := &rawClient{raw: []byte(`{"a":1}`)} //nolint:exhaustruct //stub
	svc := services.NewIrailService(client)

	out, err := svc.Diagnostic(ctx, "", "")
	require.NoError(t, err)
	assert.Nil(t, out.Vehicle, "no vehicle asked for")

	out, err = svc.Diagnostic(ctx, "IC530", "2026-10-23")
	require.NoError(t, err)
	assert.Equal(t, irail.VehicleID("IC530"), out.Vehicle["id"])
	assert.Equal(t, map[string]any{"a": float64(1)}, out.Vehicle["raw"])

	_, err = svc.Diagnostic(ctx, "IC530", "yesterday")
	require.ErrorIs(t, err, services.ErrInvalidDate)

	client.err = errors.New("boom")
	out, err = svc.Diagnostic(ctx, "IC530", "")
	require.NoError(t, err, "an iRail failure is the diagnostic")
	assert.Equal(t, "boom", out.Vehicle["error"])

	client.err, client.raw = nil, []byte("not json")
	out, err = svc.Diagnostic(ctx, "IC530", "")
	require.NoError(t, err)
	assert.Contains(t, out.Vehicle, "error")
}
