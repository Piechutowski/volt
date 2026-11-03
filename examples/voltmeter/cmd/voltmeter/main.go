// Command voltmeter is Volt's example application: serve runs the HTTP
// server on a SQLite file, migrated to the generated schema on start,
// and collect measures Volt and stores the numbers on a server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/collector"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/server"
	"github.com/Piechutowski/volt/nao/run"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "collect":
		err = collect(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "voltmeter: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "voltmeter:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: voltmeter serve [-db path] [-addr host:port]")
	fmt.Fprintln(os.Stderr, "       voltmeter collect [-server url] [-user id] [-sizes n,n] [-columns n] [-reps n] [-keystroke] ...")
	fmt.Fprintln(os.Stderr, "run voltmeter collect -h for every flag")
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	path := fs.String("db", "voltmeter.db", "the SQLite file, created and migrated when missing")
	addr := fs.String("addr", "localhost:8737", "the address to listen on")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	d, err := run.Open("sqlite3", *path)
	if err != nil {
		return err
	}
	defer d.Close()
	applied, err := run.MigrateUp(ctx, d, db.Migrations)
	if err != nil {
		return err
	}
	for _, name := range applied {
		slog.Info("migrated", "migration", name)
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           volt.Logger(server.Handler(d, volt.NewBroker())),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("serving", "addr", *addr, "db", *path)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shut); errors.Is(err, context.DeadlineExceeded) {
		return srv.Close()
	} else if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func collect(args []string) error {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	var cfg collector.Config
	fs.StringVar(&cfg.Server, "server", "http://localhost:8737", "the Voltmeter server to store the numbers on")
	fs.IntVar(&cfg.User, "user", 1, "the member id sent as X-User")
	fs.StringVar(&cfg.Repo, "repo", "", "the Volt checkout; the nearest go.work at or above the working directory by default")
	fs.StringVar(&cfg.Out, "out", "", "the directory the sweep writes sweep.json and profiles to; a temporary one by default")
	sizes := fs.String("sizes", "10,40,160", "table counts to sweep, ascending")
	fs.IntVar(&cfg.Columns, "columns", 20, "columns per table")
	fs.IntVar(&cfg.Reps, "reps", 3, "unprofiled timing runs per phase; the best is stored")
	fs.DurationVar(&cfg.Profile, "profile", 0, "CPU profile per phase and size, kept under -out; 0 skips profiling")
	fs.BoolVar(&cfg.Keystroke, "keystroke", true, "run the language server's keystroke benchmark at each size")
	fs.IntVar(&cfg.Count, "count", 3, "keystroke benchmark repetitions per size")
	fs.StringVar(&cfg.Rev, "rev", "HEAD", "the revision the thesis history is read from")
	fs.IntVar(&cfg.Commits, "commits", 10, "commits touching the fixtures to count lines of, newest first; 0 skips the thesis")
	fixtures := fs.String("fixtures", strings.Join(collector.FixturesDefault, ","), "fixture directories whose lines are counted")
	fs.IntVar(&cfg.Samples, "samples", 5, "timed calls per read route; 0 skips the latency samples")
	fs.StringVar(&cfg.Tag, "tag", "", "the tag of the run, added on first use")
	fs.StringVar(&cfg.Note, "note", "", "a note on the run")
	fs.IntVar(&cfg.Baseline, "baseline", 0, "the run this one is compared against")
	if err := fs.Parse(args); err != nil {
		return err
	}
	for _, s := range strings.Split(*sizes, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n <= 0 {
			return fmt.Errorf("-sizes: %q is not a table count", s)
		}
		cfg.Sizes = append(cfg.Sizes, n)
	}
	for _, f := range strings.Split(*fixtures, ",") {
		if f = strings.TrimSpace(f); f != "" {
			cfg.Fixtures = append(cfg.Fixtures, f)
		}
	}
	if cfg.Profile > 0 && cfg.Out == "" {
		return errors.New("-profile needs -out, where the profiles are kept")
	}
	if cfg.Repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		if cfg.Repo, err = collector.RepoFind(wd); err != nil {
			return err
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	got, err := collector.Collect(ctx, cfg, func(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) })
	if err != nil {
		return err
	}
	fmt.Println(cfg.Server + app.PathReportShow(got.Stored.Run.ID))
	return nil
}
