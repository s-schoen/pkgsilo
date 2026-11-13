package request

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldsDeniedProblem(t *testing.T) {
	t.Parallel()

	b, err := json.Marshal(FieldsDeniedProblem("username", "a/b"))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"type": "https://pkgsilo/problems/fields-denied",
		"title": "Fields denied",
		"status": 403,
		"detail": "The request body sets members that the caller may not set.",
		"errors": [
			{"pointer": "#/username", "detail": "may not be set by the caller"},
			{"pointer": "#/a~1b", "detail": "may not be set by the caller"}
		]
	}`, string(b))
}
