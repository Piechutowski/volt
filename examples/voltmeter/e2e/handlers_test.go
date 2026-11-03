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
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
)

type probeLine struct {
	Screen  string            `json:"screen"`
	Run     int               `json:"run"`
	Loaded  bool              `json:"loaded"`
	Latency int               `json:"latency"`
	Fields  map[string]string `json:"fields"`
	Notice  string            `json:"notice"`
	Board   int               `json:"board"`
	Widgets int               `json:"widgets"`
	Budget  int               `json:"budget"`
	Breach  int               `json:"breached"`
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
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cmd := exec.Command(xvfb, "-displayfd", "3", "-screen", "0", "1280x800x24", "-nolisten", "tcp")
	cmd.ExtraFiles = []*os.File{w}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	w.Close()
	number := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(r).ReadString('\n')
		number <- strings.TrimSpace(line)
	}()
	var n string
	select {
	case n = <-number:
	case <-time.After(10 * time.Second):
	}
	t.Cleanup(func() { xvfbStop(cmd, n) })
	if n == "" {
		t.Fatal("Xvfb named no display")
	}
	return ":" + n
}

func xvfbStop(cmd *exec.Cmd, n string) {
	cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		<-done
	}
	if n != "" {
		os.Remove("/tmp/.X" + n + "-lock")
		os.Remove("/tmp/.X11-unix/X" + n)
	}
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

func windowOpen(t *testing.T, e *env, email string) *window {
	t.Helper()
	display := xvfbStart(t)
	x := xtestOpen(t, display)
	bin := guiBuild(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, bin, "-server", e.srv.URL, "-probe", "-email", email, "-export", t.TempDir())
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
	return w
}

func has(target string) func(probeLine) bool {
	return func(l probeLine) bool { _, ok := l.Targets[target]; return ok }
}

// TestWindowHandlersDriveTheState drives the real window with pointer
// clicks and key presses on a virtual X server, so the widgets' own
// handlers run, not intents sent for them: a run row opens the run and
// opens it again after the member went back, a note typed into its field
// survives a live refetch of the run and Enter saves what was typed, and
// a tag form saved by Enter shows the empty fields State holds, so a
// second Enter sends what the member sees.
func TestWindowHandlersDriveTheState(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	runUpload(t, c, sweep("ac000001", []int{2, 4}, 1))
	newest := runUpload(t, c, sweep("ac000002", []int{2, 4}, 2)).Run.ID
	w := windowOpen(t, e, "ada@voltmeter.test")
	x := w.x
	row := fmt.Sprintf("runs:%d", newest)

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
	x.textType("seen live")
	w.until("the typed note in State", func(l probeLine) bool { return l.Fields["run_note"] == "seen live" })
	resp, err := c.APISamplesRecord(bg, newest, app.SampleBatch{Samples: []app.SampleTimed{{Route: "GET /ping", Status: 200, MS: 0.2, At: 1}}})
	replyRead(t, resp, err, http.StatusCreated, nil)
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
	x.textType("nightly")
	w.click("field:tag_name")
	x.textType("nightly")
	w.until("the typed tag", func(l probeLine) bool { return l.Fields["tag_name"] == "nightly" && l.Fields["tag_slug"] == "nightly" })
	x.press("Return")
	l = w.until("the tag saved by Enter", func(l probeLine) bool { return l.Notice == "Saved tag nightly" })
	if l.Fields["tag_name"] != "" || l.Fields["tag_slug"] != "" {
		t.Errorf("State holds %q and %q after the save", l.Fields["tag_slug"], l.Fields["tag_name"])
	}
	x.textType("weekly")
	w.until("a name typed into the cleared, still focused field", func(l probeLine) bool { return l.Fields["tag_name"] != "" })
	w.click("field:tag_slug")
	x.textType("weekly")
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

// TestWindowHandlersDriveBoardsAndBudgets drives the dashboards and
// budgets screens by pointer and keys: the board list moves between a
// board with widgets and an empty one, which shows its empty notice
// rather than a blank area, each card's Remove removes its widget down
// to none, the phase picker and Add widget add one back, a budget row
// picks the budget, a ceiling typed and saved by its button is stored,
// Reset breached returns the breached budget to ok, and a budget added
// through the phase picker is stored with that phase.
func TestWindowHandlersDriveBoardsAndBudgets(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	runUpload(t, c, sweep("ad000001", []int{2}, 1))
	var tight, loose db.Budget
	e.rawJSON("POST", app.PathAPIBudgets(), `{"phase": "load", "tables": 2, "max_ms": 1}`, &tight)
	e.rawJSON("POST", app.PathAPIBudgets(), `{"phase": "vet", "tables": 2, "max_ms": 50}`, &loose)
	runUpload(t, c, sweep("ad000002", []int{2}, 2))
	var full, empty db.Dashboard
	e.rawJSON("POST", app.PathAPIMeDashboardAdd(), `{"name": "A full board"}`, &full, "X-User", "1")
	e.rawJSON("POST", app.PathAPIMeDashboardAdd(), `{"name": "B empty board"}`, &empty, "X-User", "1")
	var widgets []db.Widget
	for i, ph := range []string{"load", "vet"} {
		var wd db.Widget
		e.rawJSON("POST", app.PathAPIMeBoardWidgetAdd(full.ID), fmt.Sprintf(`{"position": %d, "phase": %q, "tables": 2}`, i, ph), &wd, "X-User", "1")
		widgets = append(widgets, wd)
	}
	if tight.ID == 0 || loose.ID == 0 || full.ID == 0 || empty.ID == 0 || widgets[1].ID == 0 {
		t.Fatalf("seed: budgets %d %d, boards %d %d, widgets %+v", tight.ID, loose.ID, full.ID, empty.ID, widgets)
	}
	w := windowOpen(t, e, "ada@voltmeter.test")
	x := w.x
	card := func(wd db.Widget) string { return fmt.Sprintf("card:%d:remove", wd.ID) }

	w.until("the runs list", func(l probeLine) bool { return l.Screen == "Runs" && has("screen:Dashboards")(l) })
	w.click("screen:Dashboards")
	w.until("the full board's cards", func(l probeLine) bool {
		return l.Screen == "Dashboards" && l.Board == full.ID && l.Widgets == 2 && has(card(widgets[0]))(l) && has(card(widgets[1]))(l)
	})
	w.click(fmt.Sprintf("boards:%d", empty.ID))
	w.until("the empty board picked, its notice laid out", func(l probeLine) bool {
		return l.Board == empty.ID && l.Widgets == 0 && has("boards:none")(l)
	})
	w.click(fmt.Sprintf("boards:%d", full.ID))
	w.until("the full board again", func(l probeLine) bool { return l.Board == full.ID && has(card(widgets[0]))(l) })
	second := w.last.Targets[card(widgets[1])]
	w.click(card(widgets[0]))
	w.until("one widget left, its card moved to the first place", func(l probeLine) bool {
		r, ok := l.Targets[card(widgets[1])]
		return l.Widgets == 1 && ok && r[0] < second[0]
	})
	w.click(card(widgets[1]))
	w.until("the last widget removed, the empty notice laid out", func(l probeLine) bool {
		return l.Widgets == 0 && has("boards:none")(l) && strings.HasPrefix(l.Notice, "Removed widget")
	})
	w.click("widget_phase:check")
	w.until("the widget phase picked", func(l probeLine) bool { return l.Fields["widget_phase"] == "check" })
	w.click("field:widget_tables")
	x.textType("2")
	w.until("the widget size typed", func(l probeLine) bool { return l.Fields["widget_tables"] == "2" })
	w.click("boards:add_widget")
	w.until("a widget added by the button", func(l probeLine) bool { return l.Widgets == 1 })
	var boards []db.DashboardWithWidgets
	e.rawJSON("GET", app.PathAPIMeDashboards(), "", &boards, "X-User", "1")
	if len(boards) != 2 || len(boards[0].Widgets) != 1 || boards[0].Widgets[0].Phase != db.PhaseCheck || boards[0].Widgets[0].Tables != 2 {
		t.Errorf("stored boards after the clicks = %+v", boards)
	}

	w.click("screen:Budgets")
	w.until("the breached budget picked first", func(l probeLine) bool {
		return l.Screen == "Budgets" && l.Budget == tight.ID && l.Breach == 1 && has(budgetRow(loose.ID))(l)
	})
	w.click(budgetRow(loose.ID))
	w.until("the loose budget picked by its row", func(l probeLine) bool { return l.Budget == loose.ID && l.Fields["ceiling"] == "50" })
	w.click("field:ceiling")
	x.press("End")
	x.press("BackSpace")
	x.press("BackSpace")
	x.textType("20")
	w.until("the typed ceiling", func(l probeLine) bool { return l.Fields["ceiling"] == "20" })
	w.click("budgets:save")
	w.until("the ceiling saved by its button", func(l probeLine) bool {
		return strings.HasPrefix(l.Notice, fmt.Sprintf("Budget %d now allows", loose.ID))
	})
	w.click("budgets:reset")
	w.until("the breached budget reset", func(l probeLine) bool { return l.Breach == 0 && strings.HasPrefix(l.Notice, "Reset 1") })
	w.click("budget_phase:check")
	w.click("field:budget_tables")
	x.textType("4")
	w.click("field:budget_max")
	x.textType("9")
	w.until("the new budget typed", func(l probeLine) bool {
		return l.Fields["budget_phase"] == "check" && l.Fields["budget_tables"] == "4" && l.Fields["budget_max"] == "9"
	})
	w.click("budgets:add")
	w.until("the budget added by its button", func(l probeLine) bool { return strings.HasPrefix(l.Notice, "Added budget") })

	var budgets []db.Budget
	e.rawJSON("GET", app.PathAPIBudgets(), "", &budgets)
	byID := map[int]db.Budget{}
	for _, b := range budgets {
		byID[b.ID] = b
	}
	added := tight.ID + 2
	if len(budgets) != 3 || byID[tight.ID].State != db.HealthOK || byID[loose.ID].MaxMs != 20 || byID[added] != (db.Budget{ID: added, Phase: db.PhaseCheck, Tables: 4, MaxMs: 9, State: db.HealthOK}) {
		t.Errorf("stored budgets after the clicks = %+v", budgets)
	}
}

func budgetRow(id int) string { return "budgets:" + strconv.Itoa(id) }
