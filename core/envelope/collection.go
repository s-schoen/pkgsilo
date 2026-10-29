package envelope

import "net/http"

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

// NewPagedCollection wraps the page of resource representations that starts at offset and holds
// at most limit resources. When hasMore is set, the next page starts right after data.
func NewPagedCollection[T any](data []T, offset, limit int, hasMore bool) Collection[T] {
	var nextOffset *int
	if hasMore {
		next := offset + len(data)
		nextOffset = &next
	}

	return NewCollection(data, Pagination{Offset: offset, Limit: limit, NextOffset: nextOffset})
}

// WriteCollection writes c as a collection response with status 200.
func WriteCollection[T any](w http.ResponseWriter, c Collection[T]) {
	writeJSON(w, http.StatusOK, jsonMediaType, c)
}
