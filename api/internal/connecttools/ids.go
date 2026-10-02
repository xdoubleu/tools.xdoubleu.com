package connecttools

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// ParseClientID parses a Create request's optional client-generated ID (used
// so an offline replay returns the existing row). An empty string yields an
// invalid NullUUID, meaning the database generates the ID.
func ParseClientID(raw string) (uuid.NullUUID, error) {
	if raw == "" {
		return uuid.NullUUID{UUID: uuid.Nil, Valid: false}, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.NullUUID{UUID: uuid.Nil, Valid: false}, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid ID"),
		)
	}
	return uuid.NullUUID{UUID: id, Valid: true}, nil
}
