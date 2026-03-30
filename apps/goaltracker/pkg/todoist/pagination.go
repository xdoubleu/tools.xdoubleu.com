package todoist

import (
	"context"
	"net/http"
)

type PaginatedResponse[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
}

func getPaginatedResults[T any](
	ctx context.Context,
	client client,
	endpoint string,
	query string,
	dst *[]T,
) error {
	var paginatedResponse PaginatedResponse[T]

	for ok := true; ok; ok = paginatedResponse.NextCursor != "" {
		err := client.sendRequest(
			ctx,
			http.MethodGet,
			endpoint,
			query,
			nil,
			&paginatedResponse,
		)
		if err != nil {
			return err
		}

		*dst = append(*dst, paginatedResponse.Results...)
	}

	return nil
}
