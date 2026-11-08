package service

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cedar-policy/cedar-go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExperimentalCedarOnlyInChecks checks that checks.go is the only user of cedar-go's x/exp packages, which
// may break at any time: decisions must never depend on them.
func TestExperimentalCedarOnlyInChecks(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)

	var importers []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && strings.HasPrefix(entry.Name(), ".") && path != root:
			return filepath.SkipDir
		case entry.IsDir() || !strings.HasSuffix(path, ".go"):
			return nil
		}

		imports, err := importsExperimentalCedar(path)
		if err != nil || !imports {
			return err
		}
		rel, err := filepath.Rel(root, path)
		importers = append(importers, rel)

		return err
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"feature/policy/service/checks.go"}, importers)
}

// importsExperimentalCedar reports whether the Go file at path imports any of cedar-go's x/exp packages.
func importsExperimentalCedar(path string) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		return false, err
	}

	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return false, err
		}
		if strings.HasPrefix(imported, "github.com/cedar-policy/cedar-go/x/exp") {
			return true, nil
		}
	}

	return false, nil
}

// TestAppendReferencesOfValue checks that the groups and identities inside sets and records count too, each
// once, and that other entities don't, should Cedar ever hand them over as one value instead of a node for
// each element.
func TestAppendReferencesOfValue(t *testing.T) {
	t.Parallel()
	group := func(id string) types.EntityUID { return types.NewEntityUID("Group", types.String(id)) }
	identity := func(id string) types.EntityUID { return types.NewEntityUID("Identity", types.String(id)) }

	got := appendReferencesOfValue([]types.EntityUID{group("a")}, types.NewSet(
		group("a"),
		group("b"),
		identity("a"),
		types.NewEntityUID("APIKey", "c"),
		types.NewRecord(types.RecordMap{"d": group("d"), "b": types.NewSet(group("b"), identity("e"))}),
		types.String("f"),
	))

	assert.ElementsMatch(t, []types.EntityUID{group("a"), group("b"), identity("a"), group("d"), identity("e")}, got)
}
