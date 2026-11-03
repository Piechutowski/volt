package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/app"
)

type probeLine struct {
	Screen  string            `json:"screen"`
	Run     int               `json:"run"`
	Loaded  bool              `json:"loaded"`
	Latency int               `json:"latency"`
	Fields  map[string]string `json:"fields"`
	Notice  string            `json:"notice"`
	Busy    bool              `json:"busy"`
	Window  [2]int            `json:"window"`
	Targets map[string][4]int `json:"targets"`
}

type window struct {
	t     *testing.T
	x     *xtest
	lines chan probeLine
	last  probeLine
}

func (w *window) until(what string, ok func(probeLine) bool) probeLine {
	w.t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		if !w.last.Busy && ok(w.last) {
			return w.last
		}
		select {
		case l, open := <-w.lines:
			if !open {
				w.t.Fatalf("the window exited waiting for %s; last state %+v", what, w.last)
			}
			w.last = l
		case <-deadline:
			w.t.Fatalf("waiting for %s; last state %+v", what, w.last)
		}
	}
}

func (w *window) click(target string) {
	w.t.Helper()
	r, ok := w.last.Targets[target]
	if !ok {
		w.t.Fatalf("no %s drawn; targets %v", target, w.last.Targets)
	}
	w.x.click(w.last.Window[0]+(r[0]+r[2])/2, w.last.Window[1]+(r[1]+r[3])/2)
}

func xvfbStart(t *testing.T) string {
	t.Helper()
	xvfb, err := exec.LookPath("Xvfb")
	if err != nil {
		t.Skip("driving the window needs Xvfb on PATH:", err)
	}
	for n := 70; n < 100; n++ {
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X11-unix/X%d", n)); err == nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X%d-lock", n)); err == nil {
			continue
		}
		display := fmt.Sprintf(":%d", n)
		cmd := exec.Command(xvfb, display, "-screen", "0", "1280x800x24", "-nolisten", "tcp")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
		for range 100 {
			if _, err := os.Stat(fmt.Sprintf("/tmp/.X11-unix/X%d", n)); err == nil {
				return display
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("Xvfb %s did not come up", display)
	}
	t.Skip("no free X display between :70 and :99")
	return ""
}

func guiBuild(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "voltmeter-gui")
	build := exec.Command("go", "build", "-o", bin, "github.com/Piechutowski/volt/examples/voltmeter/gui")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the client: %v\n%s", err, out)
	}
	return bin
}

// TestWindowHandlersDriveTheState drives the real window with pointer
// clicks and key presses on a virtual X server, so the widgets' own
// handlers run, not intents sent for them: a run row opens the run and
// opens it again after the member went back, a note typed into its field
// survives a live refetch of the run and Enter saves what was typed, and
// a tag form saved by Enter shows the empty fields State holds, so a
// second Enter sends what the member sees.
func TestWindowHandlersDriveTheState(t *testing.T) {
	display := xvfbStart(t)
	x := xtestOpen(t, display)
	bin := guiBuild(t)
	e := start(t)
	c := e.gob(0)
	upload(t, c, sweep("ac000001", []int{2, 4}, 1))
	newest := upload(t, c, sweep("ac000002", []int{2, 4}, 2)).Run.ID

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-server", e.srv.URL, "-probe", "-email", "ada@voltmeter.test", "-export", t.TempDir())
	cmd.Env = append(os.Environ(), "DISPLAY="+display)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		if t.Failed() {
			t.Logf("window stderr:\n%s", stderr.String())
		}
	})
	w := &window{t: t, x: x, lines: make(chan probeLine, 256)}
	go func() {
		defer close(w.lines)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var l probeLine
			if json.Unmarshal(sc.Bytes(), &l) == nil {
				w.lines <- l
			}
		}
	}()
	row := fmt.Sprintf("runs:%d", newest)
	has := func(target string) func(probeLine) bool {
		return func(l probeLine) bool { _, ok := l.Targets[target]; return ok }
	}

	w.until("the runs list", func(l probeLine) bool { return l.Screen == "Runs" && has(row)(l) })
	w.click(row)
	w.until("the newest run opened by its row", func(l probeLine) bool { return l.Screen == "Run" && l.Run == newest && l.Loaded })
	w.until("the run's fields drawn", has("field:run_note"))
	w.click("screen:Runs")
	w.until("the runs list again", func(l probeLine) bool { return l.Screen == "Runs" && has(row)(l) })
	w.click(row)
	w.until("the same row opening the run a second time", func(l probeLine) bool { return l.Screen == "Run" && l.Loaded })

	w.until("the note field", has("field:run_note"))
	w.click("field:run_note")
	x.typeText("seen live")
	w.until("the typed note in State", func(l probeLine) bool { return l.Fields["run_note"] == "seen live" })
	resp, err := c.APISamplesRecord(bg, newest, app.SampleBatch{Samples: []app.SampleTimed{{Route: "GET /ping", Status: 200, MS: 0.2, At: 1}}})
	read(t, resp, err, http.StatusCreated, nil)
	l := w.until("the live refetch of the run", func(l probeLine) bool { return l.Latency == 1 })
	if l.Fields["run_note"] != "seen live" {
		t.Errorf("after the live refetch the note holds %q, want what was typed", l.Fields["run_note"])
	}
	x.press("Return")
	w.until("the run saved by Enter", func(l probeLine) bool { return strings.HasPrefix(l.Notice, "Saved run") })
	runs, err := c.APIRunListed(bg)
	if err != nil || runs[0].ID != newest || runs[0].Note.V != "seen live" {
		t.Errorf("the stored note = %+v, %v", runs[0], err)
	}

	w.click("screen:Runs")
	w.until("the tag form", func(l probeLine) bool { return l.Screen == "Runs" && has("field:tag_name")(l) })
	w.click("field:tag_slug")
	x.typeText("nightly")
	w.click("field:tag_name")
	x.typeText("nightly")
	w.until("the typed tag", func(l probeLine) bool { return l.Fields["tag_name"] == "nightly" && l.Fields["tag_slug"] == "nightly" })
	x.press("Return")
	l = w.until("the tag saved by Enter", func(l probeLine) bool { return l.Notice == "Saved tag nightly" })
	if l.Fields["tag_name"] != "" || l.Fields["tag_slug"] != "" {
		t.Errorf("State holds %q and %q after the save", l.Fields["tag_slug"], l.Fields["tag_name"])
	}
	x.typeText("weekly")
	w.until("a name typed into the cleared, still focused field", func(l probeLine) bool { return l.Fields["tag_name"] != "" })
	w.click("field:tag_slug")
	x.typeText("weekly")
	w.click("field:tag_name")
	x.press("Return")
	w.until("the second tag saved by Enter", func(l probeLine) bool {
		return strings.HasPrefix(l.Notice, "Saved tag") || strings.HasPrefix(l.Notice, "4")
	})
	for slug, want := range map[string]string{"nightly": "nightly", "weekly": "weekly"} {
		if tag, err := c.APITag(bg, slug); err != nil || tag.Name != want {
			t.Errorf("tag %s = %+v, %v; want the name %q the field showed when Enter was pressed", slug, tag, err, want)
		}
	}
}
