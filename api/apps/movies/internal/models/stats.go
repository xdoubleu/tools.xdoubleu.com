package models

// Stats summarises a user's watching. A title counts as watched once it has
// any watch: a movie watch, or a ticked counted season.
type Stats struct {
	// Months is the current month and the 11 before it, oldest first, in
	// Europe/Brussels; only dated watches count, rewatches included.
	Months []MonthWatches
	// Genres is the top 10 across watched titles, each title counted once.
	Genres []GenreCount
	// Ratings counts rated titles per star; index 0 is 1 star.
	Ratings        [5]int
	MoviesWatched  int
	SeriesWatched  int
	SeasonsWatched int
}

type MonthWatches struct {
	// Month is YYYY-MM.
	Month   string
	Movies  int
	Seasons int
}

type GenreCount struct {
	Genre string
	Count int
}
