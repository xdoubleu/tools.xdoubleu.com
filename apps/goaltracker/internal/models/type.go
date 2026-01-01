//nolint:mnd //no magic numbers
package models

type ViewType int

const (
	List  ViewType = iota
	Graph ViewType = iota
)

type Type struct {
	ID       int64    `json:"id"`
	ViewType ViewType `json:"viewType"`
	Name     string   `json:"name"`
}

//nolint:gochecknoglobals //ok
var SteamCompletionRate = Type{
	ID:       0,
	ViewType: Graph,
	Name:     "Steam completion rate",
}

//nolint:gochecknoglobals //ok
var FinishedBooksThisYear = Type{
	ID:       1,
	ViewType: Graph,
	Name:     "Finished books this year",
}

//nolint:gochecknoglobals //ok
var BooksFromSpecificTag = Type{
	ID:       3,
	ViewType: List,
	Name:     "Books from specific tag",
}

//nolint:gochecknoglobals //ok
var Types = map[int64]Type{
	SteamCompletionRate.ID:   SteamCompletionRate,
	FinishedBooksThisYear.ID: FinishedBooksThisYear,
	BooksFromSpecificTag.ID:  BooksFromSpecificTag,
}
