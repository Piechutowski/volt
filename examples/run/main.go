// Run regenerates one example's Go code from its .volt files with volt
// gen, then runs the example, so the generated files sit beside the
// .volt files they come from while the window shows the result:
//
//	go run ./examples/run tables
//
// With no argument it lists the examples.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	dir, err := examplesDir()
	if err != nil {
		fail(err)
	}
	names, err := examples(dir)
	if err != nil {
		fail(err)
	}
	if len(os.Args) != 2 || !contains(names, os.Args[1]) {
		fmt.Fprintf(os.Stderr, "usage: go run ./examples/run NAME\nexamples: %s\n", strings.Join(names, ", "))
		os.Exit(2)
	}
	example := filepath.Join(dir, os.Args[1])
	fmt.Printf("volt gen --sql ./... in %s\n", example)
	if err := step(example, "go", "run", "github.com/Piechutowski/volt/cmd/volt", "gen", "--sql", "./..."); err != nil {
		fail(err)
	}
	fmt.Printf("go run %s\n", example)
	if err := step(example, "go", "run", "."); err != nil {
		fail(err)
	}
}

func examplesDir() (string, error) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", "github.com/Piechutowski/volt/examples/run").Output()
	if err != nil {
		return "", fmt.Errorf("finding the examples module: %w", err)
	}
	return filepath.Dir(strings.TrimSpace(string(out))), nil
}

func examples(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "main.go")); err == nil && e.Name() != "run" {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func step(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "run:", err)
	os.Exit(1)
}
