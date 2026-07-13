package hardcover

// ExternalBook is the normalised representation returned by this package.
// Fields are nil/empty when the Hardcover record has no value for them. The
// shape matches googlebooks.ExternalBook so the resync candidate mapping is
// identical across providers.
type ExternalBook struct {
	Title    string
	Authors  []string
	ISBN13   *string
	CoverURL *string
	// Description may contain Markdown/HTML — callers strip tags when plain
	// text is needed.
	Description *string
	PageCount   *int
}

// --- Hardcover GraphQL request/response types ---

// graphQLRequest is the POST body sent to the GraphQL endpoint.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// graphQLError is one entry in a GraphQL response's top-level errors array. A
// GraphQL endpoint returns HTTP 200 even for query errors, reporting them here.
type graphQLError struct {
	Message string `json:"message"`
}

// isbnResponse is the response shape for the GetByISBN query.
type isbnResponse struct {
	Data struct {
		Editions []edition `json:"editions"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// searchResponse is the response shape for the Search query.
type searchResponse struct {
	Data struct {
		Books []book `json:"books"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// edition is a specific published edition of a book. It carries the ISBN and
// may override the parent book's title/pages/cover.
type edition struct {
	Title  string       `json:"title"`
	Pages  int          `json:"pages"`
	ISBN13 string       `json:"isbn_13"`
	Image  *cachedImage `json:"image"`
	Book   *book        `json:"book"`
}

// book is a work-level record. cached_image and cached_contributors are
// denormalised JSON fields Hardcover exposes so callers avoid deep relation
// joins that would exceed the API's max query depth of 3.
type book struct {
	Title             string              `json:"title"`
	Pages             int                 `json:"pages"`
	Description       string              `json:"description"`
	CachedImage       *cachedImage        `json:"cached_image"`
	CachedContributor []cachedContributor `json:"cached_contributors"`
}

// cachedImage is the denormalised image JSON ({"url": "..."}). Also the shape
// of the edition-level image relation's selected fields.
type cachedImage struct {
	URL string `json:"url"`
}

// cachedContributor is one entry of a book's cached_contributors JSON array.
// Hardcover nests the author under an "author" object; a flat "name" is
// tolerated as a fallback in case the cached shape changes.
type cachedContributor struct {
	Author struct {
		Name string `json:"name"`
	} `json:"author"`
	Name string `json:"name"`
}

// name returns the contributor's display name, preferring the nested author
// name and falling back to a flat name field.
func (c cachedContributor) name() string {
	if c.Author.Name != "" {
		return c.Author.Name
	}
	return c.Name
}
