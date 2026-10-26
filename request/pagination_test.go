package request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		query string
		want  Pagination
	}{
		{query: "", want: Pagination{Offset: 0, Limit: DefaultLimit}},
		{query: "offset=20", want: Pagination{Offset: 20, Limit: DefaultLimit}},
		{query: "offset=20&limit=5", want: Pagination{Offset: 20, Limit: 5}},
		{query: "limit=1", want: Pagination{Offset: 0, Limit: 1}},
		{query: "limit=100", want: Pagination{Offset: 0, Limit: MaxLimit}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			rec := httptest.NewRecorder()

			got, ok := ParsePagination(rec, httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil))

			require.True(t, ok)
			assert.Equal(t, tt.want, got)
			assert.Zero(t, rec.Body.Len(), "nothing must be written on success")
		})
	}
}

func TestParsePagination_Problems(t *testing.T) {
	tests := []struct {
		query      string
		wantErrors string
	}{
		{query: "offset=-1", wantErrors: `[{"detail":"must be no less than 0","parameter":"offset"}]`},
		{query: "limit=0", wantErrors: `[{"detail":"must be no less than 1","parameter":"limit"}]`},
		{query: "limit=101", wantErrors: `[{"detail":"must be no greater than 100","parameter":"limit"}]`},
		{query: "limit=ten", wantErrors: `[{"detail":"must be an integer","parameter":"limit"}]`},
		{
			query: "offset=&limit=1.5",
			wantErrors: `[
				{"detail":"must be an integer","parameter":"limit"},
				{"detail":"must be an integer","parameter":"offset"}
			]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			rec := httptest.NewRecorder()

			_, ok := ParsePagination(rec, httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil))

			assert.False(t, ok)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request contains invalid query parameters.",
				"errors":`+tt.wantErrors+`}`, rec.Body.String())
		})
	}
}
