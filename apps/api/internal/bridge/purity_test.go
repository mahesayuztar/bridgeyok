package bridge

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestProductionPackageHasNoExternalOrImpureImports(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	allowed := map[string]struct{}{
		"encoding/binary": {},
		"fmt":             {},
		"io":              {},
		"reflect":         {},
		"sort":            {},
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v", entry.Name(), err)
		}
		for _, importSpec := range file.Imports {
			path, err := strconv.Unquote(importSpec.Path.Value)
			if err != nil {
				t.Fatalf("Unquote(%s) error = %v", importSpec.Path.Value, err)
			}
			if _, ok := allowed[path]; !ok {
				t.Errorf("%s imports dependency outside the pure-engine allowlist: %s", entry.Name(), path)
			}
		}
	}
}
