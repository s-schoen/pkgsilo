// Package pagination holds the pages that services return for paginated collections.
package pagination

// MaxAllRows is the most rows that a list that loads all rows of a table may load. It caps the memory of
// a request and shows when a table has outgrown this way of listing it.
const MaxAllRows = 10_000

// Page is a page of a collection.
type Page[T any] struct {
	Items []T
	// HasMore reports whether further items follow this page.
	HasMore bool
}

// FromLookahead creates a page of up to limit items from rows queried with a limit of limit+1.
// The extra row is dropped and only signals that further items follow.
func FromLookahead[T any](rows []T, limit int) Page[T] {
	return Page[T]{
		Items:   rows[:min(len(rows), limit)],
		HasMore: len(rows) > limit,
	}
}

// FromAll creates the page of up to limit items that skips the first offset, from all rows of a
// collection.
func FromAll[T any](rows []T, offset, limit int) Page[T] {
	return FromLookahead(rows[min(len(rows), offset):], limit)
}
