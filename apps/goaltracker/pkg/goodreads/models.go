package goodreads

import "time"

type Book struct {
	ID        int64
	Shelf     string
	Tags      []string
	Title     string
	Author    string
	DatesRead []time.Time
}
