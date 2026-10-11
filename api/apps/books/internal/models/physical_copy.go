package models

// PhysicalAnchor pins a page of the physical copy to where it starts in the
// EPUB.
type PhysicalAnchor struct {
	Page   int    `json:"page"`
	Href   string `json:"href"`
	Offset int    `json:"offset"`
}

// PhysicalCopy is a user's physical copy of a book, for mapping its pages to
// reading positions (ADR-0031). A zero PageCount is unknown.
type PhysicalCopy struct {
	PageCount     int
	TextStartPage int
	Anchors       []PhysicalAnchor
	// Syncable is true when the book is tagged own-physical and own-digital.
	Syncable bool
}
