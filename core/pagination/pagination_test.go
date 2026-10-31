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
