package models

import "time"

const (
	TrackMore    = "more"
	TrackBroaden = "broaden"
)

// IsTrack reports whether t is a known track.
func IsTrack(t string) bool {
	return t == TrackMore || t == TrackBroaden
}

// Source is a web page the generator cited for a pick.
type Source struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Recommendation is a verified, judged pick.
type Recommendation struct {
	ID          string
	Domain      string
	Track       string
	Title       string
	Creator     string
	Year        int
	CatalogRef  string
	URL         string
	ImageURL    string
	Why         string
	Sources     []Source
	Score       int
	GeneratedAt time.Time
}
