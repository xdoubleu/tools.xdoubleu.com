package digitalocean

import "time"

// Deployment is the normalised representation of a DigitalOcean App Platform
// deployment.
type Deployment struct {
	ID        string
	Phase     string
	Cause     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// deploymentsWire is the subset of the DigitalOcean deployments API payload
// that is decoded. The list is newest-first, so element 0 is the latest.
type deploymentsWire struct {
	Deployments []deploymentWire `json:"deployments"`
}

type deploymentWire struct {
	ID        string    `json:"id"`
	Phase     string    `json:"phase"`
	Cause     string    `json:"cause"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (w deploymentWire) toDeployment() Deployment {
	return Deployment(w)
}

// LogType is a DigitalOcean App Platform deployment log phase. BUILD and
// DEPLOY cover "why did this deploy fail"; RUN and RUN_RESTARTED are the
// component's runtime logs, fetched the same way.
type LogType string

const (
	LogTypeBuild        LogType = "BUILD"
	LogTypeDeploy       LogType = "DEPLOY"
	LogTypeRun          LogType = "RUN"
	LogTypeRunRestarted LogType = "RUN_RESTARTED"
)

// ComponentLog is one service component's log text for one LogType of one
// deployment. Truncated is set when the log exceeded the size cap applied
// while fetching it.
type ComponentLog struct {
	Component string
	Type      LogType
	Content   string
	Truncated bool
}

// deploymentDetailWire is the subset of the single-deployment-get payload
// used to discover the app's service component names. Names come from the
// top-level "services" array, not "spec.services": DO returns spec as
// omitempty and does not reliably include it on the deployment-detail
// endpoint, whereas the top-level services list is always present.
type deploymentDetailWire struct {
	Deployment struct {
		Services []struct {
			Name string `json:"name"`
		} `json:"services"`
	} `json:"deployment"`
}

// deployLogsWire is the /logs endpoint's payload. HistoricURLs are
// pre-signed plain-GET URLs pointing at the actual log text, oldest first.
type deployLogsWire struct {
	HistoricURLs []string `json:"historic_urls"`
}
