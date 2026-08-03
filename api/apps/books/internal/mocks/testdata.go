package mocks

// Shared canned values for the metadata-provider mocks. Keeping them in one
// place satisfies goconst (the same string appears across every provider mock)
// and keeps the "The Odyssey by Homer" fixture consistent everywhere.
const (
	testBookTitle  = "The Odyssey"
	testBookAuthor = "Homer"
	testBookDesc   = "A test book."
	testBookISBN13 = "9780140447934"
)
