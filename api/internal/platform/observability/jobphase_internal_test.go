package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveJobPhaseRecordsHistogram(t *testing.T) {
	ObserveJobPhase("fake-job", "fetch", 250*time.Millisecond)

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, mf := range families {
		if mf.GetName() != "job_phase_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := labelMap(m.GetLabel())
			if labels["job"] == "fake-job" && labels["phase"] == "fetch" {
				assert.Positive(t, m.GetHistogram().GetSampleCount())
				return
			}
		}
	}
	t.Fatal("no job_phase_duration_seconds series observed for fake-job/fetch")
}

func labelMap(pairs []*dto.LabelPair) map[string]string {
	out := make(map[string]string, len(pairs))
	for _, l := range pairs {
		out[l.GetName()] = l.GetValue()
	}
	return out
}
