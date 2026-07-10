package kobogateway_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kobogateway"
)

// makeKoboVolume creates a fake mounted Kobo under root and returns its
// volume path.
func makeKoboVolume(t *testing.T, root, name, conf, version string) string {
	t.Helper()

	volumePath := filepath.Join(root, name)
	confDir := filepath.Join(volumePath, ".kobo", "Kobo")
	require.NoError(t, os.MkdirAll(confDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(confDir, "Kobo eReader.conf"),
		[]byte(conf),
		0o644,
	))

	if version != "" {
		require.NoError(t, os.WriteFile(
			filepath.Join(volumePath, ".kobo", "version"),
			[]byte(version),
			0o644,
		))
	}

	return volumePath
}

func TestFindKobosNone(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Macintosh HD"), 0o755))

	kobos, err := kobogateway.FindKobos(root)

	require.NoError(t, err)
	assert.Empty(t, kobos)
}

func TestFindKobosOne(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "USBSTICK"), 0o755))
	volumePath := makeKoboVolume(
		t,
		root,
		"KOBOeReader",
		sampleConf,
		"N418ABCD1234,4.38.21908,extra",
	)

	kobos, err := kobogateway.FindKobos(root)

	require.NoError(t, err)
	assert.Equal(t, []kobogateway.Kobo{{
		VolumePath:      volumePath,
		Serial:          "N418ABCD1234",
		CurrentEndpoint: "https://storeapi.kobo.com",
	}}, kobos)
}

func TestFindKobosMultiple(t *testing.T) {
	root := t.TempDir()
	makeKoboVolume(t, root, "KOBO1", sampleConf, "SERIAL1")
	makeKoboVolume(t, root, "KOBO2", sampleConf, "SERIAL2")

	kobos, err := kobogateway.FindKobos(root)

	require.NoError(t, err)
	assert.Len(t, kobos, 2)
}

func TestFindKobosMissingRoot(t *testing.T) {
	_, err := kobogateway.FindKobos(
		filepath.Join(t.TempDir(), "does-not-exist"),
	)

	assert.Error(t, err)
}

func TestReadSerialMissingVersionFile(t *testing.T) {
	root := t.TempDir()
	volumePath := makeKoboVolume(t, root, "KOBOeReader", sampleConf, "")

	assert.Equal(t, "", kobogateway.ReadSerial(volumePath))
}

func TestReadSerialGarbled(t *testing.T) {
	root := t.TempDir()
	volumePath := makeKoboVolume(
		t,
		root,
		"KOBOeReader",
		sampleConf,
		"  N418XYZ , trailing",
	)

	assert.Equal(t, "N418XYZ", kobogateway.ReadSerial(volumePath))
}
