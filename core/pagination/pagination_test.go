package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromLookahead(t *testing.T) {
	t.Parallel()
	rows := []int{1, 2, 3, 4}

	tests := []struct {
		name string
		rows []int
		want Page[int]
	}{
		{name: "fewer than limit", rows: rows[:2], want: Page[int]{Items: []int{1, 2}}},
		{name: "exactly limit is final", rows: rows[:3], want: Page[int]{Items: []int{1, 2, 3}}},
		{name: "lookahead row dropped", rows: rows, want: Page[int]{Items: []int{1, 2, 3}, HasMore: true}},
		{name: "empty", rows: nil, want: Page[int]{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, FromLookahead(tt.rows, 3))
		})
	}
}

func TestFromAll(t *testing.T) {
	t.Parallel()
	rows := []int{1, 2, 3, 4, 5}

	tests := []struct {
		name          string
		rows          []int
		offset, limit int
		want          Page[int]
	}{
		{name: "first page", rows: rows, offset: 0, limit: 2, want: Page[int]{Items: []int{1, 2}, HasMore: true}},
		{name: "middle page", rows: rows, offset: 2, limit: 2, want: Page[int]{Items: []int{3, 4}, HasMore: true}},
		{name: "short last page", rows: rows, offset: 4, limit: 2, want: Page[int]{Items: []int{5}}},
		{name: "full last page", rows: rows, offset: 3, limit: 2, want: Page[int]{Items: []int{4, 5}}},
		{name: "offset at the end", rows: rows, offset: 5, limit: 2, want: Page[int]{Items: []int{}}},
		{name: "offset past the end", rows: rows, offset: 6, limit: 2, want: Page[int]{Items: []int{}}},
		{name: "no rows", rows: nil, offset: 0, limit: 2, want: Page[int]{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, FromAll(tt.rows, tt.offset, tt.limit))
		})
	}
}
