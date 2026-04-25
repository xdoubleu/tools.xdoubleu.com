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
	OwnPhysical bool   `schema:"own_physical"`
	OwnDigital  bool   `schema:"own_digital"`
}

type UpdateBookStatusDto struct {
	Status    string `schema:"status"`
	Rating    string `schema:"rating"`
	Notes     string `schema:"notes"`
	Favourite bool   `schema:"favourite"`
}

type ToggleTagDto struct {
	Tag string `schema:"tag"`
}
