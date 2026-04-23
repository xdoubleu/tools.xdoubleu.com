package hardcover

type ExternalBook struct {
	Provider    string
	ProviderID  string
	Title       string
	Authors     []string
	ISBN13      *string
	ISBN10      *string
	CoverURL    *string
	Description *string
}

// GraphQL response shapes

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []graphQLError `json:"errors,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// Search response

type searchData struct {
	Search searchResults `json:"search"`
}

type searchResults struct {
	Results searchHits `json:"results"`
}

type searchHits struct {
	Hits []searchHit `json:"hits"`
}

type searchHit struct {
	Document searchDocument `json:"document"`
}

type searchDocument struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Contributions  []contribution  `json:"contributions"`
	Description    *string         `json:"description"`
	DefaultEdition *defaultEdition `json:"default_physical_edition"`
}

type contribution struct {
	Author authorRef `json:"author"`
}

type authorRef struct {
	Name string `json:"name"`
}

type defaultEdition struct {
	ISBN13 *string   `json:"isbn_13"`
	ISBN10 *string   `json:"isbn_10"`
	Image  *imageRef `json:"image"`
}

type imageRef struct {
	URL string `json:"url"`
}

// GetByID response

type getBookData struct {
	Books []bookRecord `json:"books"`
}

type bookRecord struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Contributions  []contribution  `json:"contributions"`
	Description    *string         `json:"description"`
	DefaultEdition *defaultEdition `json:"default_physical_edition"`
}
