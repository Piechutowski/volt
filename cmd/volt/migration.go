// The migration subcommand (spec §migration-tooling): new, check, sql,
// up, down and status over one data package, and the fold that gen
// runs before writing. The tool carries its own SQLite driver; the
// library and the runtime carry none.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v3"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/migrate"
	"github.com/Piechutowski/volt/nao/rt"
)

func migrationCommand() *cli.Command {
	return &cli.Command{
		Name:  "migration",
		Usage: "write, verify, review and apply the migrations of a data package (docs/spec.md §migration-tooling)",
		Commands: []*cli.Command{
			{
				Name:      "new",
				Usage:     "write <next>__LABEL.volt holding an empty Migration into the package at DIR (default: the current directory)",
				ArgsUsage: "LABEL [DIR]",
				Action:    migrationNewRun,
			},
			{
				Name:      "check",
				Usage:     "apply every migration to an empty SQLite and report the drift from the schema; exit 1 on drift",
				ArgsUsage: "[DIR]",
				Action:    migrationCheckRun,
			},
			{
				Name:      "sql",
				Usage:     "print every migration's statements in both directions, for review",
				ArgsUsage: "[DIR]",
				Action:    migrationSQLRun,
			},
			{
				Name:      "up",
				Usage:     "apply every pending migration to the SQLite database at DSN (a file path or a file: URI)",
				ArgsUsage: "DSN [DIR]",
				Action:    migrationUpRun,
			},
			{
				Name:      "down",
				Usage:     "revert the last applied migration of the SQLite database at DSN",
				ArgsUsage: "DSN [DIR]",
				Action:    migrationDownRun,
			},
			{
				Name:      "status",
				Usage:     "list every migration with whether and when it was applied to the SQLite database at DSN",
				ArgsUsage: "DSN [DIR]",
				Action:    migrationStatusRun,
			},
		},
	}
}

var labelShape = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func migrationNewRun(_ context.Context, c *cli.Command) error {
	if c.Args().Len() < 1 || c.Args().Len() > 2 {
		return cli.Exit("migration new: LABEL [DIR]", 2)
	}
	label := c.Args().Get(0)
	if !labelShape.MatchString(label) {
		return cli.Exit(fmt.Sprintf("migration new: label %q: letters, digits and underscores only (§migrations)", label), 2)
	}
	dir := argOr(c, 1, ".")
	pkg, err := migrationPackage(dir)
	if err != nil {
		return err
	}
	next, width := 1, 4
	for _, mi := range pkg.Schema().Migrations {
		if n, err := strconv.Atoi(mi.Number); err == nil && n+1 > next {
			next = n + 1
		}
		if len(mi.Number) > width {
			width = len(mi.Number)
		}
	}
	name := fmt.Sprintf("%0*d__%s", width, next, label)
	path := filepath.Join(dir, name+".volt")
	if _, err := os.Stat(path); err == nil {
		return cli.Exit(fmt.Sprintf("migration new: %s exists; refusing to overwrite", path), 2)
	}
	src := fmt.Sprintf("package %s\n\nMigration %s {\n}\n", pkg.Name, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		return cli.Exit("migration new: "+err.Error(), 2)
	}
	fmt.Println(path)
	return nil
}

func migrationCheckRun(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() > 1 {
		return cli.Exit("migration check: [DIR]", 2)
	}
	pkg, err := migrationPackage(argOr(c, 0, "."))
	if err != nil {
		return err
	}
	drifts, err := foldCheck(ctx, pkg)
	if err != nil {
		return cli.Exit("migration check: "+err.Error(), 2)
	}
	if len(drifts) == 0 {
		fmt.Printf("package %s: the fold of %d migration(s) matches the schema\n", pkg.Path, len(pkg.Schema().Migrations))
		return nil
	}
	driftReport(os.Stdout, pkg, drifts)
	return cli.Exit("", 1)
}

func migrationSQLRun(_ context.Context, c *cli.Command) error {
	if c.Args().Len() > 1 {
		return cli.Exit("migration sql: [DIR]", 2)
	}
	pkg, err := migrationPackage(argOr(c, 0, "."))
	if err != nil {
		return err
	}
	ms, err := migrate.Lower(pkg.Schema())
	if err != nil {
		return cli.Exit("migration sql: "+err.Error(), 2)
	}
	fmt.Print(migrate.SQLText(ms))
	return nil
}

func migrationUpRun(ctx context.Context, c *cli.Command) error {
	db, ms, err := migrationDatabase(c, "up")
	if err != nil {
		return err
	}
	defer db.Close()
	applied, err := rt.MigrateUp(ctx, db, ms)
	for _, name := range applied {
		fmt.Println("applied", name)
	}
	if err != nil {
		return cli.Exit("migration up: "+err.Error(), 1)
	}
	if len(applied) == 0 {
		fmt.Printf("nothing to apply: %d migration(s) applied\n", len(ms))
	}
	return nil
}

func migrationDownRun(ctx context.Context, c *cli.Command) error {
	db, ms, err := migrationDatabase(c, "down")
	if err != nil {
		return err
	}
	defer db.Close()
	name, err := rt.MigrateDown(ctx, db, ms)
	if err != nil {
		return cli.Exit("migration down: "+err.Error(), 1)
	}
	fmt.Println("reverted", name)
	return nil
}

func migrationStatusRun(ctx context.Context, c *cli.Command) error {
	db, ms, err := migrationDatabase(c, "status")
	if err != nil {
		return err
	}
	defer db.Close()
	status, err := rt.MigrateStatus(ctx, db, ms)
	if err != nil {
		return cli.Exit("migration status: "+err.Error(), 1)
	}
	width := 0
	for _, s := range status {
		if len(s.Name) > width {
			width = len(s.Name)
		}
	}
	for _, s := range status {
		state := "pending"
		if s.Applied {
			state = "applied " + s.AppliedAt
		}
		if s.OutOfOrder {
			state += " (out of order)"
		}
		if s.Note != "" {
			state += " (" + s.Note + ")"
		}
		fmt.Printf("%-*s  %s\n", width, s.Name, state)
	}
	return nil
}

func argOr(c *cli.Command, i int, def string) string {
	if c.Args().Len() > i {
		return c.Args().Get(i)
	}
	return def
}

// migrationPackage loads and checks the package at dir and insists it
// declares tables, since a migration is an element of the package whose
// tables it migrates (spec §migrations rule 1).
func migrationPackage(dir string) (*lang.Package, error) {
	root, err := lang.FindRoot(dir)
	if err != nil {
		return nil, cli.Exit(err.Error(), 2)
	}
	pr, err := lang.LoadDirs(root, []string{dir}, nil)
	if err != nil {
		return nil, cli.Exit(err.Error(), 2)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		for _, d := range diags {
			fmt.Println(d)
		}
		return nil, cli.Exit("migration: package has errors; fix them first (see 'volt check')", 2)
	}
	pkg := pr.PackageAt(dir)
	if pkg == nil || pkg.Schema() == nil || len(pkg.Schema().Tables) == 0 {
		return nil, cli.Exit(fmt.Sprintf("migration: %s declares no tables; a migration is an element of the package whose tables it migrates (§migrations)", dir), 2)
	}
	return pkg, nil
}

func migrationDatabase(c *cli.Command, sub string) (*sql.DB, []rt.Migration, error) {
	if c.Args().Len() < 1 || c.Args().Len() > 2 {
		return nil, nil, cli.Exit("migration "+sub+": DSN [DIR]", 2)
	}
	pkg, err := migrationPackage(argOr(c, 1, "."))
	if err != nil {
		return nil, nil, err
	}
	ms, err := migrate.Lower(pkg.Schema())
	if err != nil {
		return nil, nil, cli.Exit("migration "+sub+": "+err.Error(), 2)
	}
	db, err := rt.Open("sqlite3", c.Args().Get(0))
	if err != nil {
		return nil, nil, cli.Exit("migration "+sub+": "+err.Error(), 1)
	}
	return db, ms, nil
}

// foldCheck applies the package's migrations to one empty in-memory
// database and its schema's DDL to another, and returns the drift
// between them (spec §the-fold). A migration SQLite refuses stops the
// fold with SQLite's own message.
func foldCheck(ctx context.Context, pkg *lang.Package) ([]migrate.Drift, error) {
	ms, err := migrate.Lower(pkg.Schema())
	if err != nil {
		return nil, err
	}
	ddl, err := sqlite.Generate(pkg.Merged(), pkg.Schema(), sqlite.Options{Source: "package " + pkg.Path})
	if err != nil {
		return nil, err
	}
	foldDB, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	defer foldDB.Close()
	if _, err := rt.MigrateUp(ctx, foldDB, ms); err != nil {
		return nil, fmt.Errorf("the fold stopped: %w", err)
	}
	targetDB, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	defer targetDB.Close()
	if _, err := targetDB.ExecContext(ctx, string(ddl)); err != nil {
		return nil, fmt.Errorf("the schema's DDL: %w", err)
	}
	fold, err := migrate.Read(ctx, foldDB)
	if err != nil {
		return nil, err
	}
	target, err := migrate.Read(ctx, targetDB)
	if err != nil {
		return nil, err
	}
	return migrate.Diff(target, fold), nil
}

func driftReport(w io.Writer, pkg *lang.Package, drifts []migrate.Drift) {
	fmt.Fprintf(w, "package %s: %d difference(s) between the fold of its migrations and its schema; the operations that would close them:\n", pkg.Path, len(drifts))
	for _, d := range drifts {
		fmt.Fprintf(w, "\n// %s\n%s\n", d.Reason, d.Volt)
	}
}
