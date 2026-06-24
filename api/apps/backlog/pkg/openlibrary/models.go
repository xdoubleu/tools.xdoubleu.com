package openlibrary

import "encoding/json"

type ExternalBook struct {
	Provider    string
	ProviderID  string
	Title       string
	Authors     []string
	ISBN13      *string
	ISBN10      *string
	CoverURL    *string
	Description *string
	PageCount   *int
}

// Search response (/search.json)

type searchResponse struct {
	Docs []searchDoc `json:"docs"`
}

type searchDoc struct {
	Key                 string   `json:"key"`
	Title               string   `json:"title"`
	AuthorName          []string `json:"author_name"`
	CoverID             *int     `json:"cover_i"`
	ISBN                []string `json:"isbn"`
	NumberOfPagesMedian *int     `json:"number_of_pages_median"`
}

// Books/details response (/api/books?...&jscmd=details). The top-level object is
// keyed by "ISBN:<isbn>", so it is decoded into a map.

type booksDetailsEntry struct {
	Details bookDetails `json:"details"`
}

type workRef struct {
	Key string `json:"key"`
}

type bookDetails struct {
	Title         string      `json:"title"`
	Description   description `json:"description"`
	NumberOfPages *int        `json:"number_of_pages"`
	Covers        []int       `json:"covers"`
	ISBN13        []string    `json:"isbn_13"`
	ISBN10        []string    `json:"isbn_10"`
	Works         []workRef   `json:"works"`
}

// workResponse is the subset of the Open Library Work record
// (GET /works/OL…W.json) that we care about.
type workResponse struct {
	Description description `json:"description"`
}

// description handles Open Library's two description encodings: a bare string or
// an object of the form {"type": "/type/text", "value": "..."}.
type description struct {
	Value string
}

func (d *description) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		return json.Unmarshal(data, &d.Value)
	}
	var obj struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	d.Value = obj.Value
	return nil
}
