package kobogateway_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/kobogateway"
)

func TestDiffKobosConnect(t *testing.T) {
	kobo := kobogateway.Kobo{VolumePath: "/Volumes/KOBOeReader", Serial: "S1"}

	events := kobogateway.DiffKobos(nil, []kobogateway.Kobo{kobo})

	assert.Equal(t, []kobogateway.KoboEvent{{Connected: true, Kobo: kobo}}, events)
}

func TestDiffKobosDisconnect(t *testing.T) {
	kobo := kobogateway.Kobo{VolumePath: "/Volumes/KOBOeReader", Serial: "S1"}

	events := kobogateway.DiffKobos([]kobogateway.Kobo{kobo}, nil)

	assert.Equal(t, []kobogateway.KoboEvent{{Connected: false, Kobo: kobo}}, events)
}

func TestDiffKobosNoChange(t *testing.T) {
	kobo := kobogateway.Kobo{VolumePath: "/Volumes/KOBOeReader", Serial: "S1"}

	events := kobogateway.DiffKobos(
		[]kobogateway.Kobo{kobo},
		[]kobogateway.Kobo{kobo},
	)

	assert.Empty(t, events)
}

func TestDiffKobosMultiple(t *testing.T) {
	stays := kobogateway.Kobo{VolumePath: "/Volumes/STAYS", Serial: "S1"}
	leaves := kobogateway.Kobo{VolumePath: "/Volumes/LEAVES", Serial: "S2"}
	arrives := kobogateway.Kobo{VolumePath: "/Volumes/ARRIVES", Serial: "S3"}

	events := kobogateway.DiffKobos(
		[]kobogateway.Kobo{stays, leaves},
		[]kobogateway.Kobo{stays, arrives},
	)

	assert.ElementsMatch(t, []kobogateway.KoboEvent{
		{Connected: false, Kobo: leaves},
		{Connected: true, Kobo: arrives},
	}, events)
}

func TestKoboTooltip(t *testing.T) {
	connected := kobogateway.KoboEvent{
		Connected: true,
		Kobo:      kobogateway.Kobo{Serial: "N418ABCD1234"},
	}
	assert.Equal(t,
		"Kobo Gateway — Kobo connected (N418ABCD1234)",
		kobogateway.KoboTooltip(connected),
	)

	disconnected := kobogateway.KoboEvent{Connected: false}
	assert.Equal(t,
		"Kobo Gateway — no Kobo connected",
		kobogateway.KoboTooltip(disconnected),
	)
}

func TestKoboMenuLine(t *testing.T) {
	connected := kobogateway.KoboEvent{
		Connected: true,
		Kobo:      kobogateway.Kobo{Serial: "N418ABCD1234"},
	}
	assert.Equal(t, "Kobo connected: N418ABCD1234", kobogateway.KoboMenuLine(connected))

	disconnected := kobogateway.KoboEvent{Connected: false}
	assert.Equal(t, "No Kobo connected", kobogateway.KoboMenuLine(disconnected))
}

func TestKoboNotification(t *testing.T) {
	connected := kobogateway.KoboEvent{
		Connected: true,
		Kobo:      kobogateway.Kobo{Serial: "N418ABCD1234"},
	}
	title, body := kobogateway.KoboNotification(connected)
	assert.Equal(t, "Kobo connected", title)
	assert.Equal(t, "Serial N418ABCD1234", body)

	disconnected := kobogateway.KoboEvent{Connected: false}
	title, body = kobogateway.KoboNotification(disconnected)
	assert.Equal(t, "Kobo disconnected", title)
	assert.Equal(t, "", body)
}

func recvEvent(t *testing.T, events <-chan kobogateway.KoboEvent) kobogateway.KoboEvent {
	t.Helper()

	select {
	case ev := <-events:
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for KoboEvent")

		return kobogateway.KoboEvent{}
	}
}

func TestWatchEmitsConnectAndDisconnect(t *testing.T) {
	root := t.TempDir()
	volumePath := filepath.Join(root, "KOBOeReader")
	confDir := filepath.Join(volumePath, ".kobo", "Kobo")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := kobogateway.Watch(ctx, root, 10*time.Millisecond)

	require.NoError(t, os.MkdirAll(confDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(confDir, "Kobo eReader.conf"),
		[]byte("[NickelConf]\n"),
		0o644,
	))

	connect := recvEvent(t, events)
	assert.True(t, connect.Connected)
	assert.Equal(t, volumePath, connect.Kobo.VolumePath)

	require.NoError(t, os.RemoveAll(volumePath))

	disconnect := recvEvent(t, events)
	assert.False(t, disconnect.Connected)
	assert.Equal(t, volumePath, disconnect.Kobo.VolumePath)

	cancel()

	select {
	case _, ok := <-events:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for events channel to close")
	}
}
