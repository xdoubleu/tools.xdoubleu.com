package hardcover

// ExternalBook is the normalised Hardcover result, shaped like the other
// providers' ExternalBook.
type ExternalBook struct {
	Title    string
	Authors  []string
	ISBN13   *string
	CoverURL *string
	// Description may contain Markdown/HTML.
	Description *string
	PageCount   *int
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// graphQLError is a GraphQL error; the endpoint returns 200 even for these.
type graphQLError struct {
	Message string `json:"message"`
}

type isbnResponse struct {
	Data struct {
		Editions []edition `json:"editions"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type searchResponse struct {
	Data struct {
		Books []book `json:"books"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// searchIDsResponse is the Typesense half of Search. IDs are assumed []int.
type searchIDsResponse struct {
	Data struct {
		Search struct {
			IDs []int `json:"ids"`
		} `json:"search"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// edition carries the ISBN and may override the book's title/pages/cover.
type edition struct {
	Title  string       `json:"title"`
	Pages  int          `json:"pages"`
	ISBN13 string       `json:"isbn_13"`
	Image  *cachedImage `json:"image"`
	Book   *book        `json:"book"`
}

// book is a work-level record; the cached_* fields avoid joins past the
// API's max query depth of 3.
type book struct {
	// ID is only selected by booksByIDsQuery.
	ID          int          `json:"id"`
	Title       string       `json:"title"`
	Pages       int          `json:"pages"`
	Description string       `json:"description"`
	CachedImage *cachedImage `json:"cached_image"`
	// Editions is only selected by booksByIDsQuery (one, for its ISBN13).
	Editions          []edition           `json:"editions"`
	CachedContributor []cachedContributor `json:"cached_contributors"`
}

type cachedImage struct {
	URL string `json:"url"`
}

// cachedContributor nests the author; a flat "name" is a fallback.
type cachedContributor struct {
	Author struct {
		Name string `json:"name"`
	} `json:"author"`
	Name string `json:"name"`
}

func (c cachedContributor) name() string {
	if c.Author.Name != "" {
		return c.Author.Name
	}
	return c.Name
}
