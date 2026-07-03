package models

// BooksTypeID keys books-read rows in the books.progress table.
// The value must stay stable across deploys (rows were written under the
// former backlog schema with this same type ID).
const BooksTypeID string = "1"
