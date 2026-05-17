package app

// HTTPError carries an HTTP status alongside a user-facing message.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }
