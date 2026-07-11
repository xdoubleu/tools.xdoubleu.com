package kobogateway

// StatusResponse is the GET /status payload.
type StatusResponse struct {
	Version int    `json:"version"`
	Release string `json:"release"`
	Kobos   []Kobo `json:"kobos"`
}

// ConfigureRequest is the POST /configure payload.
type ConfigureRequest struct {
	SyncURL    string `json:"syncUrl"`
	VolumePath string `json:"volumePath"`
}

// ConfigureResponse is the POST /configure result.
type ConfigureResponse struct {
	Serial           string `json:"serial"`
	OriginalEndpoint string `json:"originalEndpoint"`
}

// RevertRequest is the POST /revert payload.
type RevertRequest struct {
	TargetEndpoint string `json:"targetEndpoint"`
	VolumePath     string `json:"volumePath"`
}

// RevertResponse is the POST /revert result.
type RevertResponse struct {
	Serial string `json:"serial"`
}

// UpdateResponse is the POST /update result; the gateway re-execs into the
// freshly downloaded binary right after sending it.
type UpdateResponse struct {
	Updating bool `json:"updating"`
}

// ErrorResponse is the uniform error payload of every endpoint.
type ErrorResponse struct {
	Error string `json:"error"`
}
