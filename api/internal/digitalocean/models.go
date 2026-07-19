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
