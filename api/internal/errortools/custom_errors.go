package errortools

// BadRequestError is used to return a bad request response.
type BadRequestError struct {
	err error
}

// UnauthorizedError is used to return an unauthorized response.
type UnauthorizedError struct {
	err error
}

// NewBadRequestError creates a new [BadRequestError].
func NewBadRequestError(err error) BadRequestError {
	return BadRequestError{
		err: err,
	}
}

func (err BadRequestError) Error() string {
	return err.err.Error()
}

// NewUnauthorizedError creates a new [UnauthorizedError].
func NewUnauthorizedError(err error) UnauthorizedError {
	return UnauthorizedError{
		err: err,
	}
}

func (err UnauthorizedError) Error() string {
	return err.err.Error()
}
