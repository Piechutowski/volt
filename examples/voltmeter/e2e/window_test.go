package e2e

import (
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/collector"
)

// TestWindowShowsTheNewestRun builds the guigui client and runs it under
// a virtual X server against a served Voltmeter the collector filled
// with a real sweep at tiny sizes: scripted by -shot, the window signs
// in, opens the newest run, waits for its data and saves itself, and
// the saved picture holds the four phase lines of the run's chart, each
// in its palette color.
func TestWindowShowsTheNewestRun(t *testing.T) {
	xvfb, err := exec.LookPath("xvfb-run")
	if err != nil {
		t.Skip("the window smoke test needs xvfb-run (and Xvfb) on PATH to open a window without a display:", err)
	}
	if _, err := exec.LookPath("Xvfb"); err != nil {
		t.Skip("the window smoke test needs Xvfb on PATH:", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("the collector reads the commit with git, which is not on PATH:", err)
	}
	dir := t.TempDir()
	bin := guiBuild(t)

	e := start(t)
	repo, err := collector.RepoFind(".")
	if err != nil {
		t.Fatal(err)
	}
	got, err := collector.Collect(context.Background(), collector.Config{
		Server: e.srv.URL, User: 1, Repo: repo,
		Sizes: []int{2, 4}, Columns: 3, Reps: 1, Rev: "HEAD", Samples: 1, Tag: "window",
	}, t.Logf)
	if err != nil {
		t.Fatal(err)
	}

	shot := filepath.Join(dir, "run.png")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	run := exec.CommandContext(ctx, xvfb, "-a", "-s", "-screen 0 1280x800x24", bin,
		"-server", e.srv.URL, "-shot", shot, "-export", dir, "-timeout", "90s")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the scripted window exited with %v:\n%s", err, out)
	}
	f, err := os.Open(shot)
	if err != nil {
		t.Fatalf("no screenshot after a clean exit: %v\n%s", err, out)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("the screenshot is no PNG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() < 640 || b.Dy() < 400 {
		t.Errorf("the screenshot is %v, smaller than the window", b)
	}
	if n := colorsCount(img, 3); n < 3 {
		t.Errorf("the screenshot of run %d holds %d colors, not a drawn screen", got.Stored.Run.ID, n)
	}
	light := [][3]uint8{{0x2a, 0x78, 0xd6}, {0xeb, 0x68, 0x34}, {0x1b, 0xaf, 0x7a}, {0xed, 0xa1, 0x00}}
	dark := [][3]uint8{{0x39, 0x87, 0xe5}, {0xd9, 0x59, 0x26}, {0x19, 0x9e, 0x70}, {0xc9, 0x85, 0x00}}
	lit, dim := seriesDrawn(img, light), seriesDrawn(img, dark)
	if lit < 4 && dim < 4 {
		t.Errorf("the run screen draws %d of the four phase lines in the light palette and %d in the dark; the chart of run %d is missing", lit, dim, got.Stored.Run.ID)
	}
}

func seriesDrawn(img image.Image, palette [][3]uint8) int {
	counts := make([]int, len(palette))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			for i, c := range palette {
				if uint8(r>>8) == c[0] && uint8(g>>8) == c[1] && uint8(bl>>8) == c[2] {
					counts[i]++
				}
			}
		}
	}
	drawn := 0
	for _, n := range counts {
		if n >= 40 {
			drawn++
		}
	}
	return drawn
}

func colorsCount(img image.Image, enough int) int {
	seen := map[[4]uint32]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			seen[[4]uint32{r, g, bl, a}] = true
			if len(seen) >= enough {
				return len(seen)
			}
		}
	}
	return len(seen)
}
