// The migration subcommand (spec §migration-tooling): new, check, sql,
// up, down and status over one data package, and the fold that gen
// runs before writing. The tool carries its own SQLite driver; the
// library and the runtime carry none.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v3"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/migrate"
	"github.com/Piechutowski/volt/nao/rt"
)

const sqliteDriver = "sqlite3"

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
				Usage:     "apply every migration to an empty SQLite, revert and reapply the reversible ones, and report the drift from the schema; exit 1 on drift or a statement SQLite refuses",
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
				Usage:     "apply every pending migration to the SQLite database at DSN (a file path or a file: URI), creating it when absent",
				ArgsUsage: "DSN [DIR]",
				Action:    migrationUpRun,
			},
			{
				Name:      "down",
				Usage:     "revert the last applied migration of the existing SQLite database at DSN",
				ArgsUsage: "DSN [DIR]",
				Action:    migrationDownRun,
			},
			{
				Name:      "status",
				Usage:     "list every migration with whether and when it was applied to the existing SQLite database at DSN, opened read-only",
				ArgsUsage: "DSN [DIR]",
				Action:    migrationStatusRun,
			},
		},
	}
}

func labelOK(label string) bool {
	if label == "" {
		return false
	}
	for _, r := range label {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func migrationNewRun(_ context.Context, c *cli.Command) error {
	if c.Args().Len() < 1 || c.Args().Len() > 2 {
		return cli.Exit("migration new: LABEL [DIR]", 2)
	}
	label := c.Args().Get(0)
	if !labelOK(label) {
		return cli.Exit(fmt.Sprintf("migration new: label %q: letters, marks, digits and underscores only (§migrations)", label), 2)
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
	if digits := len(strconv.Itoa(next)); digits > width {
		return cli.Exit(fmt.Sprintf("migration new: the next number %d does not fit the package's %d-digit run; widen every migration name to %d digits first (§migrations)", next, width, digits), 2)
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
	ms, err := migrationsLowered("migration check: ", pkg)
	if err != nil {
		return err
	}
	if err := sqliteGuard(sqliteDriver); err != nil {
		return err
	}
	drifts, err := foldCheck(ctx, pkg, ms)
	if err != nil {
		return migrationExit("migration check: ", err)
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
	ms, err := migrationsLowered("migration sql: ", pkg)
	if err != nil {
		return err
	}
	fmt.Print(migrate.SQLText(ms))
	return nil
}

func migrationUpRun(ctx context.Context, c *cli.Command) error {
	db, ms, err := migrationDatabase(ctx, c, "up")
	if err != nil {
		return err
	}
	defer db.Close()
	applied, err := rt.MigrateUp(ctx, db, ms)
	for _, name := range applied {
		fmt.Println("applied", name)
	}
	if err != nil {
		return migrationExit("migration up: ", err)
	}
	if len(applied) == 0 {
		fmt.Printf("nothing to apply: %d migration(s) applied\n", len(ms))
	}
	return nil
}

func migrationDownRun(ctx context.Context, c *cli.Command) error {
	db, ms, err := migrationDatabase(ctx, c, "down")
	if err != nil {
		return err
	}
	defer db.Close()
	name, err := rt.MigrateDown(ctx, db, ms)
	if err != nil {
		return migrationExit("migration down: ", err)
	}
	fmt.Println("reverted", name)
	return nil
}

func migrationStatusRun(ctx context.Context, c *cli.Command) error {
	db, ms, err := migrationDatabase(ctx, c, "status")
	if err != nil {
		return err
	}
	defer db.Close()
	status, err := rt.MigrateStatus(ctx, db, ms)
	if err != nil {
		return migrationExit("migration status: ", err)
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

// migrationsLowered lowers the package's migrations to their statement
// form; a migration that does not lower is an error in the migrations
// themselves, exit 1 under prefix (spec §migration-tooling rule 1).
func migrationsLowered(prefix string, pkg *lang.Package) ([]rt.Migration, error) {
	ms, err := migrate.Lower(pkg.Schema())
	if err != nil {
		return nil, cli.Exit(prefix+err.Error(), 1)
	}
	return ms, nil
}

// migrationExit maps a failure of the fold or of a database command to
// its exit code (spec §migration-tooling rule 1): 1 for a migration
// that did not run, a statement SQLite refused or a down asked of an
// irreversible one, which is an rt.MigrationError; 2 for everything
// else, which is the database or the machine.
func migrationExit(prefix string, err error) error {
	var me *rt.MigrationError
	if errors.As(err, &me) {
		return cli.Exit(prefix+err.Error(), 1)
	}
	return cli.Exit(prefix+err.Error(), 2)
}

func migrationDatabase(ctx context.Context, c *cli.Command, sub string) (*sql.DB, []rt.Migration, error) {
	if c.Args().Len() < 1 || c.Args().Len() > 2 {
		return nil, nil, cli.Exit("migration "+sub+": DSN [DIR]", 2)
	}
	dsn := c.Args().Get(0)
	if dsnIsMemory(dsn) {
		return nil, nil, cli.Exit("migration "+sub+": "+dsn+" holds nothing once the command returns; name a file", 2)
	}
	pkg, err := migrationPackage(argOr(c, 1, "."))
	if err != nil {
		return nil, nil, err
	}
	ms, err := migrationsLowered("migration "+sub+": ", pkg)
	if err != nil {
		return nil, nil, err
	}
	if err := sqliteGuard(sqliteDriver); err != nil {
		return nil, nil, err
	}
	db, err := databaseOpen(ctx, sub, dsn)
	if err != nil {
		return nil, nil, cli.Exit("migration "+sub+": "+dsn+": "+err.Error(), 2)
	}
	return db, ms, nil
}

func databaseOpen(ctx context.Context, sub, dsn string) (*sql.DB, error) {
	switch sub {
	case "status":
		db, err := sql.Open(sqliteDriver, dsnWithMode(dsn, "ro"))
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(1)
		if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	case "down":
		dsn = dsnWithMode(dsn, "rw")
	}
	if err := databasePing(ctx, dsn); err != nil {
		return nil, err
	}
	return rt.Open(sqliteDriver, dsn)
}

// databasePing opens one connection to dsn and closes it, so that a
// database that cannot be opened is reported as that, before rt.Open
// runs its pragmas on it and reports the first of them instead.
func databasePing(ctx context.Context, dsn string) error {
	db, err := sql.Open(sqliteDriver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func dsnIsMemory(dsn string) bool {
	return dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory:") || strings.Contains(dsn, "mode=memory")
}

func dsnWithMode(dsn, mode string) string {
	if strings.HasPrefix(dsn, "file:") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + "mode=" + mode
	}
	return "file:" + (&url.URL{Path: dsn}).EscapedPath() + "?mode=" + mode
}

func sqliteGuard(driver string) error {
	db, err := sql.Open(driver, ":memory:")
	if err == nil {
		defer db.Close()
		var one int
		err = db.QueryRow("SELECT 1").Scan(&one)
	}
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "CGO_ENABLED=0") {
		return cli.Exit("volt was built without cgo, so its SQLite is a stub; build volt with cgo (CGO_ENABLED=1 and a C compiler) to fold or apply migrations: "+err.Error(), 2)
	}
	return cli.Exit("volt's SQLite does not work: "+err.Error(), 2)
}

// foldCheck applies the package's migrations, lowered to ms, to one
// empty in-memory database and its schema's DDL to another, and returns
// the drift between them (spec §the-fold). A migration SQLite refuses
// stops the fold with SQLite's own message, in either direction: once
// the fold is read, the reversible migrations above the last
// irreversible one are reverted from the top and applied again
// (§the-fold rule 4).
func foldCheck(ctx context.Context, pkg *lang.Package, ms []rt.Migration) ([]migrate.Drift, error) {
	ddl, err := sqlite.Generate(pkg.Merged(), pkg.Schema(), sqlite.Options{Source: "package " + pkg.Path})
	if err != nil {
		return nil, err
	}
	foldDB, err := rt.Open(sqliteDriver, ":memory:")
	if err != nil {
		return nil, err
	}
	defer foldDB.Close()
	if _, err := rt.MigrateUp(ctx, foldDB, ms); err != nil {
		return nil, foldStopped("", err)
	}
	targetDB, err := rt.Open(sqliteDriver, ":memory:")
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
	if err := foldRoundTrip(ctx, foldDB, ms); err != nil {
		return nil, err
	}
	return migrate.Diff(target, fold), nil
}

func foldRoundTrip(ctx context.Context, db *sql.DB, ms []rt.Migration) error {
	byName := append([]rt.Migration(nil), ms...)
	sort.Slice(byName, func(i, j int) bool { return byName[i].Name < byName[j].Name })
	for i := len(byName) - 1; i >= 0 && byName[i].Down != nil; i-- {
		if _, err := rt.MigrateDown(ctx, db, ms); err != nil {
			return foldStopped(" on the way down", err)
		}
	}
	if _, err := rt.MigrateUp(ctx, db, ms); err != nil {
		return foldStopped(" on the way up again", err)
	}
	return nil
}

const viewOrTriggerHint = "a view or trigger over a rebuilt table is dropped and recreated with exec (§migrations)"

// foldStopped wraps a failure of the fold at stage with the one hint the
// statement cannot carry: SQLite refuses the rename that ends a rebuild
// when a view, or a trigger on another table, reads the table, and
// names it (spec §migrations rules 9 and 11).
func foldStopped(stage string, err error) error {
	var me *rt.MigrationError
	if errors.As(err, &me) && strings.Contains(me.SQL, " RENAME TO ") {
		if cause := me.Cause.Error(); strings.HasPrefix(cause, "error in view ") || strings.HasPrefix(cause, "error in trigger ") {
			return fmt.Errorf("the fold stopped%s: %w\n%s", stage, err, viewOrTriggerHint)
		}
	}
	return fmt.Errorf("the fold stopped%s: %w", stage, err)
}

func driftReport(w io.Writer, pkg *lang.Package, drifts []migrate.Drift) {
	fmt.Fprintf(w, "package %s: %d difference(s) between the fold of its migrations and its schema; the operations that would close them:\n", pkg.Path, len(drifts))
	for _, d := range drifts {
		fmt.Fprintf(w, "\n// %s\n%s\n", d.Reason, d.Volt)
	}
}
