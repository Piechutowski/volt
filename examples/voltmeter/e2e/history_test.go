package e2e

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/collector"
)

func historyBuild(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("the collector reads its history with git, which is not on PATH:", err)
	}
	root, err := collector.RepoFind(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "Ada", "GIT_AUTHOR_EMAIL": "ada@voltmeter.test",
		"GIT_COMMITTER_NAME": "Ada", "GIT_COMMITTER_EMAIL": "ada@voltmeter.test",
	} {
		t.Setenv(k, v)
	}
	repo := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	gitRun("init", "-q")
	for i, fixture := range collector.FixturesDefault {
		treeCopy(t, filepath.Join(root, fixture), filepath.Join(repo, fixture))
		gitRun("add", "--", fixture)
		date := "2026-01-0" + string(rune('1'+i)) + "T12:00:00Z"
		t.Setenv("GIT_AUTHOR_DATE", date)
		t.Setenv("GIT_COMMITTER_DATE", date)
		gitRun("commit", "-q", "--no-verify", "-m", "Add "+fixture)
	}
	return repo
}

func treeCopy(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(dst, 0o755)
		case !d.Type().IsRegular():
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCollectRefusesATreeGitCannotRead proves a collection whose tree
// has no commit git can read, a copy without .git or a clone git
// refuses as another user's, fails before the sweep with git's own
// reason, since a run is stored under the commit it measured.
func TestCollectRefusesATreeGitCannotRead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH:", err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	e := envStart(t)
	repo := t.TempDir()
	cfg := collector.Config{Server: e.srv.URL, User: 1, Repo: repo, Sizes: []int{2}, Columns: 3, Reps: 1}
	got, err := collector.Collect(bg, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "nothing was measured") || !strings.Contains(err.Error(), "git") || len(got.Rows) != 0 {
		t.Errorf("collect over a tree with no commit = %v with %d rows", err, len(got.Rows))
	}
}
