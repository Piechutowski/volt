package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// TestFixtureWritesACheckableProject proves `volt fixture` writes both
// layouts at a small size, that what it writes checks clean with the
// tables asked for, and that it refuses a directory holding anything
// (D80). The command runs in-process: exit errors are returned, not
// exited on.
func TestFixtureWritesACheckableProject(t *testing.T) {
	volt := func(args ...string) error {
		cmd := command()
		cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
		return cmd.Run(context.Background(), append([]string{"volt", "fixture"}, args...))
	}
	for _, tc := range []struct {
		args   []string
		tables int
	}{
		{[]string{"-packages", "2", "-tables", "3", "-columns", "9"}, 6},
		{[]string{"-tables", "3", "-columns", "9", "-single"}, 3},
	} {
		dir := filepath.Join(t.TempDir(), "big")
		if err := volt(append(tc.args, dir)...); err != nil {
			t.Fatalf("fixture %v: %v", tc.args, err)
		}
		pr, err := lang.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if diags := lang.Check(pr); diag.HasErrors(diags) {
			t.Fatalf("fixture %v does not check:\n%v", tc.args, diags)
		}
		tables := 0
		for _, pkg := range pr.Packages {
			if pkg.HasSchema() {
				tables += len(pkg.Schema().Tables)
			}
		}
		if tables != tc.tables {
			t.Errorf("fixture %v: %d tables, want %d", tc.args, tables, tc.tables)
		}
		err = volt("-tables", "1", "-columns", "1", dir)
		if err == nil || !strings.Contains(err.Error(), "not empty") {
			t.Errorf("fixture over %s: %v, want a refusal", dir, err)
		}
	}
}
