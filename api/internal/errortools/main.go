// Package errortools holds reusable errors and error helpers.
package errortools

//nolint:lll //can't make these lines shorter,the errors are clear
const (
	MessageInternalServerError = "the server encountered a problem and could not process your request"
	MessageTooManyRequests     = "rate limit exceeded"
	MessageForbidden           = "user has no access to this resource"
)
