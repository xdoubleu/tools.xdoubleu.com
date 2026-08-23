package observability

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrNoMetrics is returned when node_exporter's scrape response didn't carry
// enough of the expected series to compute a sample (e.g. no
// node_filesystem_size_bytes{mountpoint="/"} line).
var ErrNoMetrics = errors.New("hostmetrics: no usable metrics in scrape")

// HostSample is one point-in-time reading of host resource usage.
type HostSample struct {
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
}

// HostMetricsScraper GETs node_exporter's Prometheus text-exposition
// endpoint and parses just the handful of series needed for CPU/memory/disk
// percentages — no Prometheus server, no client library.
type HostMetricsScraper struct {
	url        string
	httpClient *http.Client
}

// NewHostMetricsScraper builds a scraper against url (typically
// "http://node-exporter:9100/metrics").
func NewHostMetricsScraper(url string, timeout time.Duration) *HostMetricsScraper {
	return &HostMetricsScraper{
		url:        url,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Scrape fetches and parses the current sample.
func (s *HostMetricsScraper) Scrape(ctx context.Context) (HostSample, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return HostSample{}, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return HostSample{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return HostSample{}, fmt.Errorf(
			"hostmetrics: scrape returned status %d", resp.StatusCode,
		)
	}

	return ParseNodeExporterMetrics(resp.Body)
}

// metricLine is one parsed "name{labels} value" line, with labels kept as a
// raw map for the handful of lookups callers need (mountpoint, mode).
type metricLine struct {
	name   string
	labels map[string]string
	value  float64
}

// ParseNodeExporterMetrics reads node_exporter's Prometheus text-exposition
// format and computes CPU/memory/disk percentages from it:
//   - CPU%: idle-based, from node_cpu_seconds_total summed per mode across
//     all CPUs — 100 * (1 - idle / total).
//   - Memory%: 100 * (1 - MemAvailable / MemTotal).
//   - Disk%: 100 * (1 - avail / size) for the root filesystem
//     (mountpoint="/").
func ParseNodeExporterMetrics(r io.Reader) (HostSample, error) {
	cpuSecondsByMode := map[string]float64{}
	var (
		memTotalVal, memAvailableVal, diskSize, diskAvail           float64
		haveMemTotal, haveMemAvailable, haveDiskSize, haveDiskAvail bool
	)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := parseMetricLine(scanner.Text())
		if line == nil {
			continue
		}

		switch line.name {
		case "node_cpu_seconds_total":
			cpuSecondsByMode[line.labels["mode"]] += line.value
		case "node_memory_MemTotal_bytes":
			memTotalVal, haveMemTotal = line.value, true
		case "node_memory_MemAvailable_bytes":
			memAvailableVal, haveMemAvailable = line.value, true
		case "node_filesystem_size_bytes":
			if line.labels["mountpoint"] == "/" {
				diskSize, haveDiskSize = line.value, true
			}
		case "node_filesystem_avail_bytes":
			if line.labels["mountpoint"] == "/" {
				diskAvail, haveDiskAvail = line.value, true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return HostSample{}, err
	}

	if len(cpuSecondsByMode) == 0 || !haveMemTotal || !haveMemAvailable ||
		!haveDiskSize || !haveDiskAvail {
		return HostSample{}, ErrNoMetrics
	}

	return HostSample{
		CPUPercent:    cpuPercentFromModes(cpuSecondsByMode),
		MemoryPercent: percentUsed(memTotalVal, memAvailableVal),
		DiskPercent:   percentUsed(diskSize, diskAvail),
	}, nil
}

// fullPercent is 100.0, factored out so the percentage math below doesn't
// read as unexplained magic numbers.
const fullPercent = 100.0

// percentUsed returns 100 * (1 - available/total), clamped to [0, 100].
func percentUsed(total, available float64) float64 {
	if total <= 0 {
		return 0
	}
	pct := fullPercent * (1 - available/total)
	return clampPercent(pct)
}

// cpuPercentFromModes returns 100 * (1 - idle / sum(all modes)).
func cpuPercentFromModes(byMode map[string]float64) float64 {
	var total float64
	for _, v := range byMode {
		total += v
	}
	if total <= 0 {
		return 0
	}
	pct := fullPercent * (1 - byMode["idle"]/total)
	return clampPercent(pct)
}

func clampPercent(pct float64) float64 {
	if pct < 0 {
		return 0
	}
	if pct > fullPercent {
		return fullPercent
	}
	return pct
}

// parseMetricLine parses one line of Prometheus text-exposition format:
// `metric_name{label="value",...} 123.45`. Returns nil for comments, blank
// lines, and lines it can't parse — the format allows arbitrary unknown
// series, which this scraper simply ignores.
func parseMetricLine(line string) *metricLine {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	var name, rest string
	labels := map[string]string{}

	if idx := strings.IndexByte(line, '{'); idx != -1 {
		end := strings.IndexByte(line, '}')
		if end == -1 || end < idx {
			return nil
		}
		name = strings.TrimSpace(line[:idx])
		labels = parseLabels(line[idx+1 : end])
		rest = strings.TrimSpace(line[end+1:])
	} else {
		const nameAndValue = 2
		fields := strings.Fields(line)
		if len(fields) < nameAndValue {
			return nil
		}
		name = fields[0]
		rest = fields[1]
	}

	valueField := strings.Fields(rest)
	if len(valueField) == 0 {
		return nil
	}
	value, err := strconv.ParseFloat(valueField[0], 64)
	if err != nil {
		return nil
	}

	return &metricLine{name: name, labels: labels, value: value}
}

// parseLabels parses the comma-separated `key="value"` pairs inside a metric
// line's `{...}` label block.
func parseLabels(raw string) map[string]string {
	const keyAndValue = 2
	labels := map[string]string{}
	for _, part := range splitLabels(raw) {
		kv := strings.SplitN(part, "=", keyAndValue)
		if len(kv) != keyAndValue {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		labels[key] = val
	}
	return labels
}

// splitLabels splits a label block on top-level commas, ignoring commas
// inside quoted values.
func splitLabels(raw string) []string {
	var parts []string
	var inQuotes bool
	start := 0
	for i, r := range raw {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				parts = append(parts, raw[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, raw[start:])
	return parts
}
