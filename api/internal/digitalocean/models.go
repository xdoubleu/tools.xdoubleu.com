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

// LogType is a DigitalOcean App Platform deployment log phase. Only BUILD and
// DEPLOY are fetched — RUN/RUN_RESTARTED are runtime logs, a different and
// much noisier concern than "why did this deploy fail".
type LogType string

const (
	LogTypeBuild  LogType = "BUILD"
	LogTypeDeploy LogType = "DEPLOY"
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
// used to discover the app's service component names.
type deploymentDetailWire struct {
	Deployment struct {
		Spec struct {
			Services []struct {
				Name string `json:"name"`
			} `json:"services"`
		} `json:"spec"`
	} `json:"deployment"`
}

// deployLogsWire is the /logs endpoint's payload. HistoricURLs are
// pre-signed plain-GET URLs pointing at the actual log text, oldest first.
type deployLogsWire struct {
	HistoricURLs []string `json:"historic_urls"`
}
