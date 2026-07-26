package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/sandgorgon/tui/cell"
)

// update controls whether Golden writes fixtures instead of comparing
// against them. Registered here (not in the importing _test.go files)
// so every caller gets the same flag for free: `go test ./... -update`.
var update = flag.Bool("update", false, "update golden fixtures instead of comparing against them")

// Golden compares buf's text-grid dump (cell.Buffer.String(), see its
// doc comment) against the fixture at testdata/golden/<name>.golden,
// relative to the calling test's package directory — the "golden-
// buffer tests" docs/DESIGN.md §10 calls for, used where the render<->
// vt round trip isn't the natural fit because it's the specific
// content/shape of a widget's output being protected against
// regression, not the renderer's byte-level correctness.
//
// Run `go test -update ./...` to create or refresh fixtures after an
// intentional rendering change.
func Golden(t *testing.T, name string, buf *cell.Buffer) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".golden")
	got := buf.String() + "\n"

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("Golden(%s): mkdir: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("Golden(%s): write fixture: %v", name, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Golden(%s): read fixture (run `go test -update` to create it): %v", name, err)
	}
	if got != string(want) {
		t.Errorf("Golden(%s): mismatch (run `go test -update` to refresh)\n--- want ---\n%s--- got ---\n%s", name, want, got)
	}
}
