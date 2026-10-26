package envelope

import "net/http"

// Resource is a successful response containing one resource representation.
type Resource[T any] struct {
	Data T `json:"data"`
}

// NewResource wraps one resource representation in a response envelope.
func NewResource[T any](data T) Resource[T] {
	return Resource[T]{Data: data}
}

// WriteResource writes data as a resource response with status 200.
func WriteResource[T any](w http.ResponseWriter, data T) {
	writeJSON(w, http.StatusOK, jsonMediaType, NewResource(data))
}

// WriteCreated writes data as a resource response with status 201 and sets the Location
// header to location, the URI of the created resource.
func WriteCreated[T any](w http.ResponseWriter, location string, data T) {
	w.Header().Set("Location", location)
	writeJSON(w, http.StatusCreated, jsonMediaType, NewResource(data))
}
