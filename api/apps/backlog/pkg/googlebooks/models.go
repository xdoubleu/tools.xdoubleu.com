package googlebooks

// ExternalBook is the normalised representation returned by this package.
// Fields are nil/empty when the Google Books record has no value for them.
type ExternalBook struct {
	Title    string
	Authors  []string
	ISBN13   *string
	ISBN10   *string
	CoverURL *string
	// Description may be HTML — callers should strip tags if plain text is needed.
	Description *string
	PageCount   *int
}

// --- Google Books API JSON types ---

type volumesResponse struct {
	TotalItems int      `json:"totalItems"`
	Items      []volume `json:"items"`
}

type volume struct {
	VolumeInfo volumeInfo `json:"volumeInfo"`
}

type volumeInfo struct {
	Title               string               `json:"title"`
	Authors             []string             `json:"authors"`
	Description         string               `json:"description"`
	PageCount           int                  `json:"pageCount"`
	ImageLinks          *imageLinks          `json:"imageLinks"`
	IndustryIdentifiers []industryIdentifier `json:"industryIdentifiers"`
}

type imageLinks struct {
	Thumbnail string `json:"thumbnail"`
}

type industryIdentifier struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}
