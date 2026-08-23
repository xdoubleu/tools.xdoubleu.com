package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/observability"
)

// sampleNodeExporterOutput is a trimmed but realistic node_exporter scrape:
// two CPUs' worth of node_cpu_seconds_total across the standard modes, plus
// the memory and root-filesystem series the parser looks for.
const sampleNodeExporterOutput = `# HELP node_cpu_seconds_total CPU time per mode.
# TYPE node_cpu_seconds_total counter
node_cpu_seconds_total{cpu="0",mode="idle"} 700.5
node_cpu_seconds_total{cpu="0",mode="system"} 50.1
node_cpu_seconds_total{cpu="0",mode="user"} 249.4
node_cpu_seconds_total{cpu="1",mode="idle"} 800.0
node_cpu_seconds_total{cpu="1",mode="system"} 40.0
node_cpu_seconds_total{cpu="1",mode="user"} 160.0
# HELP node_memory_MemTotal_bytes Total memory.
# TYPE node_memory_MemTotal_bytes gauge
node_memory_MemTotal_bytes 1.6e+10
# HELP node_memory_MemAvailable_bytes Available memory.
# TYPE node_memory_MemAvailable_bytes gauge
node_memory_MemAvailable_bytes 4e+09
# HELP node_filesystem_size_bytes Filesystem size in bytes.
# TYPE node_filesystem_size_bytes gauge
node_filesystem_size_bytes{device="tmpfs",fstype="tmpfs",mountpoint="/dev/shm"} 1e+09
node_filesystem_size_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 1e+11
# HELP node_filesystem_avail_bytes Filesystem space available.
# TYPE node_filesystem_avail_bytes gauge
node_filesystem_avail_bytes{device="tmpfs",fstype="tmpfs",mountpoint="/dev/shm"} 1e+09
node_filesystem_avail_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 2.5e+10
`

func TestParseNodeExporterMetrics(t *testing.T) {
	sample, err := observability.ParseNodeExporterMetrics(
		strings.NewReader(sampleNodeExporterOutput),
	)
	require.NoError(t, err)

	// idle = 700.5+800 = 1500.5, total = 1500.5+50.1+249.4+40+160 = 2000
	assert.InDelta(t, 100*(1-1500.5/2000), sample.CPUPercent, 0.001)
	// 100 * (1 - 4e9/1.6e10) = 75
	assert.InDelta(t, 75, sample.MemoryPercent, 0.001)
	// 100 * (1 - 2.5e10/1e11) = 75
	assert.InDelta(t, 75, sample.DiskPercent, 0.001)
}

func TestParseNodeExporterMetrics_IgnoresUnknownSeriesAndComments(t *testing.T) {
	input := "# just a comment\n\nnode_unrelated_total 42\n" + sampleNodeExporterOutput
	sample, err := observability.ParseNodeExporterMetrics(strings.NewReader(input))
	require.NoError(t, err)
	assert.InDelta(t, 75, sample.MemoryPercent, 0.001)
}

func TestParseNodeExporterMetrics_MissingSeriesErrors(t *testing.T) {
	_, err := observability.ParseNodeExporterMetrics(strings.NewReader(
		"node_memory_MemTotal_bytes 1000\n",
	))
	require.ErrorIs(t, err, observability.ErrNoMetrics)
}

func TestHostMetricsScraper_Scrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(sampleNodeExporterOutput))
		},
	))
	defer srv.Close()

	scraper := observability.NewHostMetricsScraper(srv.URL, time.Second)
	sample, err := scraper.Scrape(t.Context())
	require.NoError(t, err)
	assert.InDelta(t, 75, sample.MemoryPercent, 0.001)
}

func TestHostMetricsScraper_Scrape_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer srv.Close()

	scraper := observability.NewHostMetricsScraper(srv.URL, time.Second)
	_, err := scraper.Scrape(t.Context())
	require.Error(t, err)
}
