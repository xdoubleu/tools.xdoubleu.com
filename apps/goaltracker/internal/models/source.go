package models

type Source struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Types []Type `json:"types"`
}

//nolint:gochecknoglobals //ok
var Sources = []Source{
	SteamSource,
	GoodreadsSource,
}

//nolint:gochecknoglobals //ok
var SourcesTypeIDMap = map[int64]Source{
	SteamCompletionRate.ID:   SteamSource,
	FinishedBooksThisYear.ID: GoodreadsSource,
	BooksFromSpecificTag.ID:  GoodreadsSource,
}

//nolint:gochecknoglobals //ok
var SteamSource = Source{
	ID:   0,
	Name: "Steam",
	Types: []Type{
		SteamCompletionRate,
	},
}

//nolint:gochecknoglobals //ok
var GoodreadsSource = Source{
	ID:   1,
	Name: "Goodreads",
	Types: []Type{
		FinishedBooksThisYear,
		BooksFromSpecificTag,
	},
}
