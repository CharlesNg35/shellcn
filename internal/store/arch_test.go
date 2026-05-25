package store_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestGormDoesNotLeak enforces the architecture invariant: only internal/store
// imports gorm. Any other package importing it is a layering violation.
func TestGormDoesNotLeak(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "node_modules" || base == "web" || base == ".git" || base == "bin" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// internal/store is the one allowed home for gorm.
		if strings.Contains(filepath.ToSlash(path), "/internal/store/") {
			return nil
		}

		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil // unparseable files are not our concern here
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == "gorm.io/gorm" || strings.HasPrefix(p, "gorm.io/") || strings.HasPrefix(p, "github.com/glebarez/") {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports %q — gorm must not leak outside internal/store", rel, p)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}
}
