package dtos

type AddBookDto struct {
	ProviderID  string `schema:"provider_id"`
	Provider    string `schema:"provider"`
	Title       string `schema:"title"`
	Author      string `schema:"author"`
	ISBN13      string `schema:"isbn13"`
	CoverURL    string `schema:"cover_url"`
	Description string `schema:"description"`
	Status      string `schema:"status"`
}

type UpdateBookStatusDto struct {
	Status string `schema:"status"`
}
