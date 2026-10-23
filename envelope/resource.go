package envelope

// Resource is a successful response containing one resource representation.
type Resource[T any] struct {
	Data T `json:"data"`
}

// NewResource wraps one resource representation in a response envelope.
func NewResource[T any](data T) Resource[T] {
	return Resource[T]{Data: data}
}
