package perfsweep

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Keystroke is the language server's whole in-process path of one
// keystroke inside one table of a one-file project, didChange to the
// publish of its diagnostics (lsp.BenchmarkKeystroke), at one size.
type Keystroke struct {
	Tables      int     `json:"tables"`
	MSPerOp     float64 `json:"ms_per_op"` // the best of the benchmark's repetitions
	MBPerOp     float64 `json:"mb_per_op"`
	AllocsPerOp int     `json:"allocs_per_op"`
}

// Keystrokes runs the language server's keystroke benchmark once per
// size, count repetitions each, through `go test` from the repository
// root (the server is its own module), and keeps the best repetition.
func Keystrokes(repo string, sizes []int, columns, count int, log func(format string, args ...any)) ([]Keystroke, error) {
	var out []Keystroke
	for _, tables := range sizes {
		cmd := exec.Command("go", "test", "./lsp", "-run", "^$", "-bench", "^BenchmarkKeystroke$", "-benchmem", "-count", strconv.Itoa(count))
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "VOLT_KEYSTROKE_TABLES="+strconv.Itoa(tables), "VOLT_KEYSTROKE_COLUMNS="+strconv.Itoa(columns))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		raw, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("keystroke benchmark at %d tables: %w\n%s%s", tables, err, raw, stderr.String())
		}
		k := Keystroke{Tables: tables}
		found := false
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			// BenchmarkKeystroke-8  126  9683428 ns/op  6133665 B/op  35263 allocs/op
			if len(f) < 8 || !strings.HasPrefix(f[0], "BenchmarkKeystroke") || f[3] != "ns/op" {
				continue
			}
			ns, err1 := strconv.ParseFloat(f[2], 64)
			b, err2 := strconv.ParseFloat(f[4], 64)
			allocs, err3 := strconv.Atoi(f[6])
			if err1 != nil || err2 != nil || err3 != nil {
				continue
			}
			if !found || ns/1e6 < k.MSPerOp {
				k.MSPerOp, k.MBPerOp, k.AllocsPerOp = ns/1e6, b/(1<<20), allocs
			}
			found = true
		}
		if !found {
			return nil, fmt.Errorf("keystroke benchmark at %d tables printed no result:\n%s", tables, raw)
		}
		out = append(out, k)
		if log != nil {
			log("%3d tables keystroke %8.1f ms %8.2f MB alloc %7d allocs", tables, k.MSPerOp, k.MBPerOp, k.AllocsPerOp)
		}
	}
	return out, nil
}

// KeystrokeChart draws the keystroke's wall time over the table
// counts into dir and returns the file name.
func KeystrokeChart(dir string, ks []Keystroke, columns int) (string, error) {
	var xs []int
	var vals []float64
	for _, k := range ks {
		xs = append(xs, k.Tables)
		vals = append(vals, k.MSPerOp)
	}
	svg := LineChart("One keystroke, didChange to publish", "ms", fmt.Sprintf("tables (%d columns each, one file)", columns), xs, []string{"keystroke"}, map[string][]float64{"keystroke": vals}, map[string]string{"keystroke": phaseColors["load"]})
	name := "keystroke.svg"
	return name, os.WriteFile(filepath.Join(dir, name), []byte(svg), 0o644)
}
