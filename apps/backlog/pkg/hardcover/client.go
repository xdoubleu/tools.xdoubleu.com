package hardcover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

const baseURL = "https://api.hardcover.app/v1/graphql"

type client struct {
	logger   *slog.Logger
	apiToken string
}

func New(logger *slog.Logger, apiToken string) Client {
	return client{
		logger:   logger,
		apiToken: apiToken,
	}
}

func (c client) Search(ctx context.Context, query string) ([]ExternalBook, error) {
	gqlQuery := `
		query SearchBooks($query: String!) {
			search(query: $query, query_type: "Book", per_page: 20) {
				results
			}
		}
	`

	var resp graphQLResponse[searchData]
	if err := c.do(ctx, gqlQuery, map[string]any{"query": query}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("hardcover search error: %s", resp.Errors[0].Message)
	}

	books := make([]ExternalBook, 0, len(resp.Data.Search.Results.Hits))
	for _, hit := range resp.Data.Search.Results.Hits {
		books = append(books, toExternalBook(hit.Document))
	}

	return books, nil
}

func (c client) GetByID(ctx context.Context, id string) (*ExternalBook, error) {
	gqlQuery := `
		query GetBook($id: Int!) {
			books(where: {id: {_eq: $id}}) {
				id
				title
				contributions { author { name } }
				description
				default_physical_edition {
					isbn_13
					isbn_10
					image { url }
				}
			}
		}
	`

	intID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid hardcover book id %q: %w", id, err)
	}

	var resp graphQLResponse[getBookData]
	if err = c.do(ctx, gqlQuery, map[string]any{"id": intID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("hardcover get book error: %s", resp.Errors[0].Message)
	}
	if len(resp.Data.Books) == 0 {
		return nil, nil
	}

	rec := resp.Data.Books[0]
	doc := searchDocument{
		ID:             rec.ID,
		Title:          rec.Title,
		Contributions:  rec.Contributions,
		Description:    rec.Description,
		DefaultEdition: rec.DefaultEdition,
	}
	book := toExternalBook(doc)

	return &book, nil
}

func (c client) do(ctx context.Context, query string, variables map[string]any, dst any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hardcover API returned %d: %s", resp.StatusCode, string(raw))
	}

	return json.NewDecoder(resp.Body).Decode(dst)
}

func toExternalBook(doc searchDocument) ExternalBook {
	authors := make([]string, 0, len(doc.Contributions))
	for _, c := range doc.Contributions {
		if c.Author.Name != "" {
			authors = append(authors, c.Author.Name)
		}
	}

	var coverURL *string
	if doc.DefaultEdition != nil && doc.DefaultEdition.Image != nil && doc.DefaultEdition.Image.URL != "" {
		coverURL = &doc.DefaultEdition.Image.URL
	}

	var isbn13, isbn10 *string
	if doc.DefaultEdition != nil {
		isbn13 = doc.DefaultEdition.ISBN13
		isbn10 = doc.DefaultEdition.ISBN10
	}

	return ExternalBook{
		Provider:    "hardcover",
		ProviderID:  doc.ID,
		Title:       doc.Title,
		Authors:     authors,
		ISBN13:      isbn13,
		ISBN10:      isbn10,
		CoverURL:    coverURL,
		Description: doc.Description,
	}
}
