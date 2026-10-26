package envelope

import (
	"fmt"
	"net/http"
)

// Pagination describes a page of a collection and how to continue through it.
type Pagination struct {
	// Offset is the zero-based position of the first resource in the page.
	Offset int `json:"offset"`
	// Limit is the effective maximum page size; the page may contain fewer resources.
	Limit int `json:"limit"`
	// NextOffset is the offset of the next page, or nil when this is the final page.
	NextOffset *int `json:"nextOffset"`
	// Total is the exact number of matching resources before pagination, when the endpoint counts them.
	Total *int `json:"total,omitempty"`
}

// Collection is a successful response containing a page of resource representations.
type Collection[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// NewCollection wraps a page of resource representations and its pagination metadata.
func NewCollection[T any](data []T, p Pagination) Collection[T] {
	// A nil slice would encode as null, but data is always an array.
	if data == nil {
		data = []T{}
	}

	return Collection[T]{Data: data, Pagination: p}
}

// NewUnpaginatedCollection wraps a complete collection in an envelope that has no following page.
func NewUnpaginatedCollection[T any](data []T) Collection[T] {
	total := len(data)

	return NewCollection(data, Pagination{
		Offset: 0,
		// An empty collection uses the minimum valid limit.
		Limit: max(1, total),
		Total: &total,
	})
}

// LookaheadOptions describes the page a lookahead query was made for.
type LookaheadOptions struct {
	Offset int
	Limit  int
	// Total is the optional exact number of matching resources before pagination.
	Total *int
}

// NewCollectionFromLookahead creates a collection from a query result that fetched up to Limit+1 rows.
// The extra row is dropped and only signals that a next page exists.
//
// It returns an error when rows contains more than Limit+1 resources.
func NewCollectionFromLookahead[T any](rows []T, o LookaheadOptions) (Collection[T], error) {
	if len(rows) > o.Limit+1 {
		return Collection[T]{}, fmt.Errorf("lookahead result has %d rows, more than limit+1 (%d)", len(rows), o.Limit+1)
	}

	data := rows[:min(len(rows), o.Limit)]

	var nextOffset *int
	if len(rows) == o.Limit+1 {
		next := o.Offset + len(data)
		nextOffset = &next
	}

	return NewCollection(data, Pagination{
		Offset:     o.Offset,
		Limit:      o.Limit,
		NextOffset: nextOffset,
		Total:      o.Total,
	}), nil
}

// WriteCollection writes c as a collection response with status 200.
func WriteCollection[T any](w http.ResponseWriter, c Collection[T]) {
	writeJSON(w, http.StatusOK, jsonMediaType, c)
}
