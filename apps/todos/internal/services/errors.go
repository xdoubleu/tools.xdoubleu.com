package services

// HTTPError carries an HTTP status and user-facing message from the service layer.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }
