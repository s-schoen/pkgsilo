package envelope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollection(t *testing.T) {
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
	c := NewCollection[user](nil, Pagination{Offset: 10, Limit: 5})

	assert.JSONEq(t, `{"data":[],"pagination":{"offset":10,"limit":5,"nextOffset":null}}`, encode(t, c))
}

func TestNewUnpaginatedCollection(t *testing.T) {
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
			assert.JSONEq(t, tt.want, encode(t, NewUnpaginatedCollection(tt.data)))
		})
	}
}

func TestNewCollectionFromLookahead(t *testing.T) {
	rows := []int{1, 2, 3, 4}
	total := 10
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name     string
		rows     []int
		opts     LookaheadOptions
		wantData []int
		wantNext *int
	}{
		{name: "fewer than limit", rows: rows[:2], opts: LookaheadOptions{Offset: 4, Limit: 3}, wantData: []int{1, 2}},
		{name: "exactly limit is final", rows: rows[:3], opts: LookaheadOptions{Offset: 4, Limit: 3}, wantData: []int{1, 2, 3}},
		{
			name: "lookahead row trimmed", rows: rows, opts: LookaheadOptions{Offset: 4, Limit: 3},
			wantData: []int{1, 2, 3}, wantNext: intPtr(7),
		},
		{name: "empty", rows: nil, opts: LookaheadOptions{Offset: 20, Limit: 3}, wantData: []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCollectionFromLookahead(tt.rows, tt.opts)
			require.NoError(t, err)

			assert.Equal(t, tt.wantData, c.Data)
			assert.Equal(t, Pagination{Offset: tt.opts.Offset, Limit: tt.opts.Limit, NextOffset: tt.wantNext}, c.Pagination)
		})
	}

	t.Run("total is passed through", func(t *testing.T) {
		c, err := NewCollectionFromLookahead(rows, LookaheadOptions{Limit: 3, Total: &total})
		require.NoError(t, err)

		assert.JSONEq(t, `{"data":[1,2,3],"pagination":{"offset":0,"limit":3,"nextOffset":3,"total":10}}`, encode(t, c))
	})

	t.Run("more than limit+1 rows", func(t *testing.T) {
		_, err := NewCollectionFromLookahead(rows, LookaheadOptions{Limit: 2})

		assert.Error(t, err)
	})
}

func TestWriteCollection(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteCollection(rec, NewCollection[user](nil, Pagination{Offset: 10, Limit: 5}))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"data":[],"pagination":{"offset":10,"limit":5,"nextOffset":null}}`, rec.Body.String())
}
