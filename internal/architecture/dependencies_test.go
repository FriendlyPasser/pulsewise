package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Keep the dependency rules executable: neither core package may import a host,
// and browser/API code must stay testable without Wails.
func TestPackageBoundaries(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	allowed := map[string][]string{
		"stats":  {},
		"update": {},
		"panel":  {"stats", "update"},
		"web":    {"panel", "stats"},
		"gui":    {"panel", "stats", "update"},
	}
	const internal = "github.com/FriendlyPasser/pulsewise/internal/"
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(filepath.Join(root, "internal"), filepath.Dir(path))
		if err != nil {
			return err
		}
		permitted, known := allowed[rel]
		if !known {
			t.Errorf("define dependency rules for package %s", rel)
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range parsed.Imports {
			dependency, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(dependency, "github.com/wailsapp/") && rel != "gui" {
				t.Errorf("%s imports Wails outside the native host", path)
			}
			if pkg, ok := strings.CutPrefix(dependency, internal); ok && !slices.Contains(permitted, pkg) {
				t.Errorf("%s imports %s; permitted internal dependencies: %v", path, pkg, permitted)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
