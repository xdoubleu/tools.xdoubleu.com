package runner

import (
	"fmt"
	"time"
)

// phaseTimes adds up where a job's time goes across its attempts.
type phaseTimes struct {
	agent, publish, ci, merge time.Duration
}

// addWatch splits one watch into the wait for CI and the wait for the merge
// after it; greenAt is zero when CI never passed.
func (p *phaseTimes) addWatch(began, greenAt, ended time.Time) {
	if greenAt.IsZero() {
		p.ci += ended.Sub(began)
		return
	}
	p.ci += greenAt.Sub(began)
	p.merge += ended.Sub(greenAt)
}

func (p *phaseTimes) summary(total time.Duration) string {
	round := func(d time.Duration) time.Duration { return d.Round(time.Second) }
	return fmt.Sprintf(
		"Phase times: agent %s, push/PR %s, CI %s, merge wait %s (total %s)",
		round(p.agent), round(p.publish), round(p.ci), round(p.merge), round(total))
}
