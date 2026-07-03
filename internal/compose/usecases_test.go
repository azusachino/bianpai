package compose

import (
	"path/filepath"
	"testing"
)

// TestUsecaseStacksLoad is a release gate: every stack under usecases/ must
// parse and load into a project with services. Adding a broken stack fails
// `make check` / CI, and the real-world stacks guard the loader against
// regressions.
func TestUsecaseStacksLoad(t *testing.T) {
	matches, err := filepath.Glob("../../usecases/*/compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no use-case stacks found under usecases/")
	}
	for _, path := range matches {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			project, err := Load(path, "")
			if err != nil {
				t.Fatalf("load %s: %v", path, err)
			}
			if len(project.Services) == 0 {
				t.Fatalf("%s loaded no services", path)
			}
		})
	}
}
