package mcpserver_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/mcpserver"
	"pkgsilo/core/pagination"
	"pkgsilo/core/request"
)

// listTool registers test_list, which lists the numbers from 1 to 5 as strings and reports the page that it
// was asked for to got.
func listTool(got *request.Pagination) func(reg *mcpserver.Registry) {
	numbers := []int{1, 2, 3, 4, 5}

	return func(reg *mcpserver.Registry) {
		reg.Add(mcpserver.Tool{Name: "test_list", Description: "Lists numbers.", Kind: mcpserver.Read},
			func(_ context.Context, _ uuid.UUID, in mcpserver.ListInput) (mcpserver.ListOutput[string], error) {
				*got = in.Pagination()
				page := pagination.FromAll(numbers, got.Offset, got.Limit)

				return mcpserver.NewListOutput(page, got.Offset, strconv.Itoa), nil
			})
	}
}

func TestListInput_Schema(t *testing.T) {
	t.Parallel()
	s := serve(t, listTool(new(request.Pagination)))

	tools := s.mcp.Tools()

	require.Len(t, tools, 1)
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"offset": {
				"type": "integer",
				"description": "Items to skip: leave it out for the first page, then pass the nextOffset of the previous page."
			},
			"limit": {
				"type": "integer",
				"description": "The largest number of items to return, from 1 to 100. Defaults to 50."
			}
		},
		"additionalProperties": false
	}`, marshal(t, tools[0].InputSchema))
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"items": {"type": ["null", "array"], "items": {"type": "string"}, "description": "The items of the page."},
			"nextOffset": {
				"type": ["null", "integer"],
				"description": "The offset of the next page, to pass to the next call. It's absent on the last page."
			}
		},
		"required": ["items"],
		"additionalProperties": false
	}`, marshal(t, tools[0].OutputSchema))
}

// TestListInput_Defaults checks that a list without arguments requests the first page of the HTTP API's
// default size, and that the description of the limit names the limits of the HTTP API, which a tag can't
// refer to.
func TestListInput_Defaults(t *testing.T) {
	t.Parallel()

	assert.Equal(t, request.Pagination{Offset: 0, Limit: 50}, mcpserver.ListInput{}.Pagination())
	assert.Equal(t, 50, request.DefaultLimit, "the description of ListInput.Limit names the default")
	assert.Equal(t, 100, request.MaxLimit, "the description of ListInput.Limit names the maximum")
}

func TestListOutput_Pages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
		want request.Pagination
		// wantOutput is the output, with a nextOffset if more items follow.
		wantOutput string
	}{
		{name: "defaults", args: "", want: request.Pagination{Limit: 50}, wantOutput: `{"items":["1","2","3","4","5"]}`},
		{
			name: "first page", args: `{"limit":2}`, want: request.Pagination{Limit: 2},
			wantOutput: `{"items":["1","2"],"nextOffset":2}`,
		},
		{
			name: "middle page", args: `{"offset":2,"limit":2}`, want: request.Pagination{Offset: 2, Limit: 2},
			wantOutput: `{"items":["3","4"],"nextOffset":4}`,
		},
		{
			name: "last page", args: `{"offset":4,"limit":2}`, want: request.Pagination{Offset: 4, Limit: 2},
			wantOutput: `{"items":["5"]}`,
		},
		{
			name: "past the end", args: `{"offset":9}`, want: request.Pagination{Offset: 9, Limit: 50},
			wantOutput: `{"items":[]}`,
		},
		{
			name: "largest limit", args: `{"limit":100}`, want: request.Pagination{Limit: 100},
			wantOutput: `{"items":["1","2","3","4","5"]}`,
		},
		{
			name: "zero limit is the default", args: `{"limit":0}`, want: request.Pagination{Limit: 50},
			wantOutput: `{"items":["1","2","3","4","5"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got request.Pagination
			s := serve(t, listTool(&got))

			s.mcp.Call("test_list", tt.args).AssertOutput(tt.wantOutput)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListInput_Invalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
		want validation.Errors
	}{
		{
			name: "negative offset", args: `{"offset":-1}`,
			want: validation.Errors{"offset": errors.New("must be no less than 0")},
		},
		{
			name: "negative limit", args: `{"limit":-1}`,
			want: validation.Errors{"limit": errors.New("must be no less than 1")},
		},
		{
			name: "limit too large", args: `{"limit":101}`,
			want: validation.Errors{"limit": errors.New("must be no greater than 100")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := serve(t, listTool(new(request.Pagination)))

			s.mcp.Call("test_list", tt.args).AssertToolError(mcpserver.InvalidArgumentsError(tt.want))
		})
	}
}
