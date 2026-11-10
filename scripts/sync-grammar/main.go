// sync-grammar publishes grammar/ to a local git mirror and points the
// Zed extension at it (docs/editor.md section 3): Zed loads a dev
// extension's grammar from a git repository, so the tool regenerates
// src/parser.c when the pinned tree-sitter CLI is present, mirrors
// grammar/ into ~/.cache/volt/tree-sitter-volt-git (VOLT_GRAMMAR_MIRROR
// overrides it) and commits there, rewrites [grammars.volt] in
// zed-extension/extension.toml with the mirror's file:// URL and
// commit, and replays Zed's checkout of that commit so a failure shows
// here with its real message. Run it from the repository root after
// cloning and after every grammar change:
//
//	go run ./scripts/sync-grammar
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	grammar := filepath.Join(root, "grammar")
	toml := filepath.Join(root, "zed-extension", "extension.toml")
	if _, err := os.Stat(filepath.Join(grammar, "grammar.js")); err != nil {
		return fmt.Errorf("run it from the repository root: %w", err)
	}
	mirror, err := mirrorPath()
	if err != nil {
		return err
	}
	if err := parserRegenerate(grammar); err != nil {
		return err
	}
	sha, err := mirrorSync(grammar, mirror)
	if err != nil {
		return err
	}
	url := "file://" + mirror
	if err := tomlRewrite(toml, url, sha); err != nil {
		return err
	}
	if err := checkoutPreflight(root, url, sha); err != nil {
		return err
	}
	fmt.Printf("Grammar mirrored at: %s\n", mirror)
	fmt.Printf("extension.toml now pins commit %s\n", sha[:12])
	fmt.Println("Preflight OK: Zed-style checkout works; parser.c exports tree_sitter_volt.")
	fmt.Println()
	fmt.Println("Next: in Zed run 'zed: extensions' and click Install Dev Extension")
	fmt.Println("(first time, pointing at zed-extension/ — remove any old Volt dev")
	fmt.Println("extension entry first) or Rebuild on the Volt extension.")
	fmt.Println()
	fmt.Println("If Zed still reports \"failed to compile grammar 'volt'\", run")
	fmt.Println("'zed: open log' — the usual causes left are Zed failing to download")
	fmt.Println("its wasi-sdk toolchain (network/proxy; set WASI_SDK_PATH to a local")
	fmt.Println("install to skip the download) or a git URL rewrite in ~/.gitconfig.")
	fmt.Println("Note: extension.toml's grammar lines are machine-local; avoid")
	fmt.Println("committing that change.")
	return nil
}

func mirrorPath() (string, error) {
	if m := os.Getenv("VOLT_GRAMMAR_MIRROR"); m != "" {
		return filepath.Abs(m)
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cache = filepath.Join(home, ".cache")
	}
	return filepath.Join(cache, "volt", "tree-sitter-volt-git"), nil
}

func parserRegenerate(grammar string) error {
	lock, err := os.ReadFile(filepath.Join(grammar, "package-lock.json"))
	if err != nil {
		return err
	}
	var pkgs struct {
		Packages map[string]struct{ Version string } `json:"packages"`
	}
	if err := json.Unmarshal(lock, &pkgs); err != nil {
		return fmt.Errorf("grammar/package-lock.json: %w", err)
	}
	pinned := pkgs.Packages["node_modules/tree-sitter-cli"].Version
	if pinned == "" {
		return errors.New("grammar/package-lock.json pins no tree-sitter-cli")
	}
	cands := []string{filepath.Join(grammar, "node_modules", ".bin", "tree-sitter")}
	if p, err := exec.LookPath("tree-sitter"); err == nil {
		cands = append(cands, p)
	}
	for _, c := range cands {
		out, err := exec.Command(c, "--version").Output()
		if err != nil {
			continue
		}
		fields := strings.Fields(string(out))
		if len(fields) < 2 || fields[1] != pinned {
			have := strings.TrimSpace(string(out))
			fmt.Printf("Skipping %s: %s, not the pinned %s.\n", c, have, pinned)
			continue
		}
		fmt.Printf("Regenerating parser with tree-sitter %s (%s)...\n", pinned, c)
		cmd := exec.Command(c, "generate")
		cmd.Dir, cmd.Stdout, cmd.Stderr = grammar, os.Stdout, os.Stderr
		return cmd.Run()
	}
	fmt.Printf("tree-sitter %s not found (cd grammar && npm ci installs it); using committed src/ as-is.\n", pinned)
	return nil
}

var mirrorSkipped = map[string]bool{".git": true, "node_modules": true, "build": true}

func mirrorSync(grammar, mirror string) (string, error) {
	if err := os.MkdirAll(mirror, 0o755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(mirror)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			if err := os.RemoveAll(filepath.Join(mirror, e.Name())); err != nil {
				return "", err
			}
		}
	}
	err = filepath.WalkDir(grammar, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(grammar, path)
		if err != nil || rel == "." {
			return err
		}
		if mirrorSkipped[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return entryCopy(path, filepath.Join(mirror, rel), d)
	})
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(mirror, ".git")); errors.Is(err, fs.ErrNotExist) {
		if _, err := gitRun(mirror, "init", "-q"); err != nil {
			return "", err
		}
	}
	if _, err := gitRun(mirror, "add", "-A"); err != nil {
		return "", err
	}
	if _, err := gitRun(mirror, "diff", "--cached", "--quiet"); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return "", err
		}
		msg := "sync " + time.Now().UTC().Format("2006-01-02T15:04:05Z")
		if _, err := gitRun(mirror, "-c", "user.name=volt-sync", "-c", "user.email=sync@localhost", "commit", "-q", "-m", msg); err != nil {
			return "", err
		}
	}
	return gitRun(mirror, "rev-parse", "HEAD")
}

func entryCopy(src, dst string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	switch {
	case d.IsDir():
		return os.MkdirAll(dst, info.Mode().Perm())
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	default:
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, info.Mode().Perm())
	}
}

var grammarsBlock = regexp.MustCompile(`(\[grammars\.volt\]\n)repository = "[^"]*"\ncommit = "[^"]*"`)

func tomlRewrite(path, url, sha string) error {
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if n := len(grammarsBlock.FindAllIndex(text, -1)); n != 1 {
		return fmt.Errorf("could not rewrite [grammars.volt] in %s (matched %d blocks)", path, n)
	}
	repl := []byte(fmt.Sprintf("${1}repository = %q\ncommit = %q", url, sha))
	return os.WriteFile(path, grammarsBlock.ReplaceAll(text, repl), 0o644)
}

func checkoutPreflight(root, url, sha string) error {
	checkout := filepath.Join(root, "zed-extension", "grammars", "volt")
	if _, err := os.Stat(checkout); err == nil {
		current, err := gitRun("", "--git-dir", filepath.Join(checkout, ".git"), "remote", "get-url", "origin")
		if err != nil {
			current = "not a git repo"
		}
		if current != url {
			fmt.Printf("Clearing stale grammar checkout (pointed at: %s).\n", current)
			if err := os.RemoveAll(filepath.Join(root, "zed-extension", "grammars")); err != nil {
				return err
			}
		}
	}
	tmp, err := os.MkdirTemp("", "volt-grammar-preflight")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err := gitRun(tmp, "init", "-q"); err != nil {
		return err
	}
	if _, err := gitRun(tmp, "remote", "add", "origin", url); err != nil {
		return err
	}
	if _, err := gitRun(tmp, "fetch", "-q", "--depth", "1", "origin", sha); err != nil {
		return fmt.Errorf("Zed-style 'git fetch --depth 1 origin %s' failed against the mirror: %w\n       If a git config rewrites URLs (url.*.insteadOf), exempt file:// URLs", sha, err)
	}
	if _, err := gitRun(tmp, "checkout", "-q", sha); err != nil {
		return err
	}
	parser, err := os.ReadFile(filepath.Join(tmp, "src", "parser.c"))
	if err != nil {
		return errors.New("mirror lacks src/parser.c")
	}
	if !bytes.Contains(parser, []byte("tree_sitter_volt")) {
		return errors.New("src/parser.c does not export tree_sitter_volt; regenerate it:\n       cd grammar && npm ci && ./node_modules/.bin/tree-sitter generate")
	}
	return nil
}

func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
