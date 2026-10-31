package envelope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCollection(t *testing.T) {
	t.Parallel()
	next, total := 2, 3
	c := NewCollection([]user{{ID: "user-42", Name: "Ada"}, {ID: "user-43", Name: "Grace"}}, Pagination{
		Offset: 0, Limit: 2, NextOffset: &next, Total: &total,
	})

	assert.JSONEq(t, `{
		"data": [{"id":"user-42","name":"Ada"},{"id":"user-43","name":"Grace"}],
		"pagination": {"offset":0,"limit":2,"nextOffset":2,"total":3}
	}`, encode(t, c))
}

func TestNewCollection_NilDataAndFinalPage(t *testing.T) {
	t.Parallel()
	c := NewCollection[user](nil, Pagination{Offset: 10, Limit: 5})

	assert.JSONEq(t, `{"data":[],"pagination":{"offset":10,"limit":5,"nextOffset":null}}`, encode(t, c))
}

func TestNewUnpaginatedCollection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []user
		want string
	}{
		{
			name: "resources",
			data: []user{{ID: "user-42", Name: "Ada"}, {ID: "user-43", Name: "Grace"}},
			want: `{
				"data": [{"id":"user-42","name":"Ada"},{"id":"user-43","name":"Grace"}],
				"pagination": {"offset":0,"limit":2,"nextOffset":null,"total":2}
			}`,
		},
		{
			name: "empty uses minimum limit",
			data: nil,
			want: `{"data":[],"pagination":{"offset":0,"limit":1,"nextOffset":null,"total":0}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.JSONEq(t, tt.want, encode(t, NewUnpaginatedCollection(tt.data)))
		})
	}
}

func TestNewPagedCollection(t *testing.T) {
	t.Parallel()
	next := 7

	tests := []struct {
		name    string
		data    []int
		hasMore bool
		want    Collection[int]
	}{
		{
			name: "final page", data: []int{1, 2},
			want: Collection[int]{Data: []int{1, 2}, Pagination: Pagination{Offset: 4, Limit: 3}},
		},
		{
			name: "more pages", data: []int{1, 2, 3}, hasMore: true,
			want: Collection[int]{Data: []int{1, 2, 3}, Pagination: Pagination{Offset: 4, Limit: 3, NextOffset: &next}},
		},
		{
			name: "empty", data: nil,
			want: Collection[int]{Data: []int{}, Pagination: Pagination{Offset: 4, Limit: 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NewPagedCollection(tt.data, 4, 3, tt.hasMore))
		})
	}
}

func TestWriteCollection(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteCollection(rec, NewCollection[user](nil, Pagination{Offset: 10, Limit: 5}))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"data":[],"pagination":{"offset":10,"limit":5,"nextOffset":null}}`, rec.Body.String())
}
