package app

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// update rewrites the golden files with what the tests get, for a deliberate change of what they pin.
var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// TestSchema pins the Cedar schema that saved policies are validated against, the public contract of
// authorization: the actions are named after the authz tags of the features' types, so renaming a field
// renames its actions, and a stored policy that names the old action no longer matches it. Such a change
// must be deliberate, by rewriting the golden file with -update, and shows up in its diff.
func TestSchema(t *testing.T) {
	t.Parallel()
	golden := filepath.Join("testdata", "schema.cedarschema")

	got := schema()

	if *update {
		require.NoError(t, os.WriteFile(golden, []byte(got), 0o600))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), got,
		"the schema changed, which can break stored policies; if that's intended, run go test ./app -run TestSchema -update")
}
