package mcpserver

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/pagination"
	"pkgsilo/core/request"
)

// ListInput is the input of a tool that lists a collection one page at a time, as the offset and limit query
// parameters are for an endpoint of the HTTP API. A list tool without further arguments takes it as its
// input. Both members are optional: the first page holds request.DefaultLimit items.
type ListInput struct {
	Offset int `json:"offset,omitempty" jsonschema:"Items to skip: leave it out for the first page, then pass the nextOffset of the previous page."`
	Limit  int `json:"limit,omitempty"  jsonschema:"The largest number of items to return, from 1 to 100. Defaults to 50."`
}

// Validate reports a negative offset and a limit outside 1 to request.MaxLimit as validation.Errors. A limit
// of 0 is left out.
func (in ListInput) Validate() error {
	return validation.ValidateStruct(&in,
		validation.Field(&in.Offset, validation.Min(0)),
		validation.Field(&in.Limit, validation.Min(1), validation.Max(request.MaxLimit)),
	)
}

// Pagination returns the page that in requests, with request.DefaultLimit for a limit that it leaves out.
func (in ListInput) Pagination() request.Pagination {
	limit := in.Limit
	if limit == 0 {
		limit = request.DefaultLimit
	}

	return request.Pagination{Offset: in.Offset, Limit: limit}
}

// ListOutput is the output of a list tool: a page of items, each the output of the tool that gets one of
// them, and the offset of the next page, which an agent copies into its next call.
type ListOutput[T any] struct {
	Items      []T  `json:"items"                jsonschema:"The items of the page."`
	NextOffset *int `json:"nextOffset,omitempty" jsonschema:"The offset of the next page, to pass to the next call. It's absent on the last page."`
}

// NewListOutput returns the output of a list tool for page, the page of a service that skips the first offset
// items, with each item converted to its output by convert. Its NextOffset follows the page's last item if
// more items follow.
func NewListOutput[S, T any](page pagination.Page[S], offset int, convert func(S) T) ListOutput[T] {
	items := make([]T, len(page.Items))
	for i, item := range page.Items {
		items[i] = convert(item)
	}

	out := ListOutput[T]{Items: items}
	if page.HasMore {
		out.NextOffset = new(offset + len(items))
	}

	return out
}
