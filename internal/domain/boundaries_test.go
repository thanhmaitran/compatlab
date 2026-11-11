package domain

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The domain package is the dependency root: it may import only the standard
// library, so the core engine never depends on HTTP, SQL or UI code.
func TestDomainImportsOnlyStandardLibrary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if first := strings.Split(path, "/")[0]; strings.Contains(first, ".") {
				t.Errorf("%s imports non-standard package %q", f, path)
			}
		}
	}
}
