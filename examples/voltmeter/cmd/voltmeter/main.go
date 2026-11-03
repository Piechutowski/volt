// Command voltmeter is Volt's example application: serve runs the HTTP
// server on a SQLite file, migrated to the generated schema on start.
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
	"syscall"
	"time"

	"github.com/Piechutowski/volt"
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
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "voltmeter: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "voltmeter:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: voltmeter serve [-db path] [-addr host:port]")
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
