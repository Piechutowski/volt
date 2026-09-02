package itest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/gen/router"
)

// voltGen runs the real CLI ('go run ./cmd/volt gen dir') from the
// module root, returning combined output and the exit error, if any.
func voltGen(t *testing.T, dir string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/volt", "gen", dir)
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestCLIClobberProtection: 'volt gen' refuses to overwrite a file that
// lacks the generated-code header, and writes nothing at all when any
// target is refused (all or nothing).
func TestCLIClobberProtection(t *testing.T) {
	if testing.Short() {
		t.Skip("execs the toolchain")
	}
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module clobber\n")
	write(filepath.Join("app", "r.volt"), "package app\n\nScope / {\n\tget / Home.Index\n}\n")

	// A hand-written file squatting on a generated name.
	hand := "package app\n\n// hand-written: mine, not the generator's\n"
	write(filepath.Join("app", "volt_router.go"), hand)

	out, err := voltGen(t, appDir)
	if err == nil {
		t.Fatalf("gen succeeded over a hand-written volt_router.go:\n%s", out)
	}
	if !strings.Contains(out, "refusing to overwrite") {
		t.Errorf("refusal not reported: %q", out)
	}
	got, readErr := os.ReadFile(filepath.Join(appDir, "volt_router.go"))
	if readErr != nil || string(got) != hand {
		t.Errorf("hand-written file was modified: %q", got)
	}
	for _, name := range router.Files {
		if name == "volt_router.go" {
			continue
		}
		if _, err := os.Stat(filepath.Join(appDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s was written despite the refusal (want all-or-nothing)", name)
		}
	}

	// Clear the squatter: gen now writes all four, each with the marker.
	if err := os.Remove(filepath.Join(appDir, "volt_router.go")); err != nil {
		t.Fatal(err)
	}
	out, err = voltGen(t, appDir)
	if err != nil {
		t.Fatalf("gen failed on a clean package: %v\n%s", err, out)
	}
	for _, name := range router.Files {
		src, err := os.ReadFile(filepath.Join(appDir, name))
		if err != nil {
			t.Fatalf("%s not written: %v", name, err)
		}
		if !strings.HasPrefix(string(src), "// Code generated ") {
			t.Errorf("%s lacks the generated-code header", name)
		}
	}

	// Re-running over its own output is fine (the marker authorizes it).
	if out, err = voltGen(t, appDir); err != nil {
		t.Fatalf("gen is not idempotent over its own output: %v\n%s", err, out)
	}
}
