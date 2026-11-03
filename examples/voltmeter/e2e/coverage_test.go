package e2e

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type covered struct {
	anchor   string
	tests    []string
	excluded string
}

var coverage = []covered{
	{anchor: "values-and-effects", tests: []string{"app.TestHandlersDescribeTheirEffects", "app.TestControllersDeterministic", "db.TestAllKeepsResultOrder", "gui/state.TestUpdateLeavesItsInputAlone"}},

	{anchor: "table-of-contents", excluded: "navigation of the spec itself; it states no rule"},
	{anchor: "notation", excluded: "how the spec writes its grammar; no application behavior follows from it"},
	{anchor: "ebnf", excluded: "the grammar's metalanguage; lang/ebnf executes it against the parser"},
	{anchor: "character-notation-uxxxx", excluded: "how the spec names characters; no application behavior follows from it"},
	{anchor: "unicode-general-categories", excluded: "Voltmeter's names are ASCII; letters beyond ASCII are the conformance corpus's"},
	{anchor: "case-sensitivity-and-terminology", excluded: "Voltmeter spells every keyword the one way it has; the miscased refusal is the conformance corpus's"},
	{anchor: "source-text", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "lexical-structure", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "tokenization", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "line-structure-and-whitespace", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "comments", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "identifiers", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "keywords", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestJoins", "e2e.TestUploadsAndRuns"}},
	{anchor: "string-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "multi-line-string-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestGeneratedFilesCurrent"}},
	{anchor: "escape-sequences", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestGeneratedFilesCurrent"}},
	{anchor: "numeric-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestProjections"}},
	{anchor: "boolean-and-null-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestPredicateForms"}},
	{anchor: "expression-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestMigrationsBuildTheSchema", "db.TestRefusedWritesMapToStatuses"}},
	{anchor: "operators-and-punctuation", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "common-syntactic-forms", tests: []string{"e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "names-and-qualified-names", tests: []string{"db.TestJoins", "app.TestGeneratedFilesCurrent"}},
	{anchor: "settings-lists", tests: []string{"db.TestMigrationsBuildTheSchema", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "program-structure", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "element-definitions", tests: []string{"db.TestMigrationsBuildTheSchema"}},
	{anchor: "table", tests: []string{"db.TestMigrationsBuildTheSchema", "db.TestGeneratedFilesCurrent"}},
	{anchor: "columns", tests: []string{"db.TestCastAllowsOnlyTheListedColumns", "db.TestRefusedWritesMapToStatuses", "db.TestAutoincrementNeverReusesARunID"}},
	{anchor: "default-values", tests: []string{"db.TestCastAllowsOnlyTheListedColumns", "db.TestMigrationsBuildTheSchema"}},
	{anchor: "indexes", tests: []string{"db.TestRefusedWritesMapToStatuses", "db.TestMigrationsBuildTheSchema"}},
	{anchor: "relationships-ref", tests: []string{"db.TestRefActions", "db.TestJoins"}},
	{anchor: "types", tests: []string{"units.TestCommitNormalizesCase", "db.TestPredicateForms", "db.TestCastAllowsOnlyTheListedColumns"}},
	{anchor: "tablepartial", tests: []string{"db.TestFromAndValidate", "db.TestMigrationsBuildTheSchema"}},
	{anchor: "records-sample-data", tests: []string{"db.TestMigrationsBuildTheSchema"}},
	{anchor: "notes", tests: []string{"db.TestGeneratedFilesCurrent"}},
	{anchor: "static-semantics", tests: []string{"e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "namespaces-and-uniqueness", excluded: "Voltmeter's names are unique, so it shows only the accepting side; the refusals are the conformance corpus's"},
	{anchor: "reference-resolution", tests: []string{"db.TestRefActions", "db.TestJoins"}},
	{anchor: "partial-injection-order", tests: []string{"db.TestGeneratedFilesCurrent", "db.TestMigrationsBuildTheSchema"}},
	{anchor: "nullability-and-cardinality", tests: []string{"db.TestJoins", "db.TestRefActions", "db.TestRefusedWritesMapToStatuses"}},
	{anchor: "collected-grammar-part-i", excluded: "repeats the grammar of the sections above, which lang/ebnf checks against them"},

	{anchor: "layers-files-and-the-superset-rule", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "data-and-network-layers", tests: []string{"e2e.TestGeneratedFilesKeepTheLayers"}},
	{anchor: "projects-and-packages", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestCLIMigrations"}},
	{anchor: "imports", tests: []string{"app.TestGeneratedFilesCurrent", "e2e.TestGeneratedFilesKeepTheLayers"}},
	{anchor: "pipelines", tests: []string{"e2e.TestPagesAndPlugs"}},
	{anchor: "scopes-and-routes", tests: []string{"e2e.TestPagesAndPlugs", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "route-paths", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestPagesAndPlugs"}},
	{anchor: "routes", tests: []string{"e2e.TestPagesAndPlugs", "e2e.TestReports", "e2e.TestUploadsAndRuns"}},
	{anchor: "handler-references", tests: []string{"app.TestControllersDeterministic", "app.TestHandlersDescribeTheirEffects", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "requests-and-responses", tests: []string{"app.TestHandlersDescribeTheirEffects", "app.TestHandlersRefuseBeforeAnyEffect", "e2e.TestReports"}},
	{anchor: "request-bodies", tests: []string{"e2e.TestSignInAndScopeLoads", "e2e.TestUploadsAndRuns", "e2e.TestBudgetsBreachesAndTransactions"}},
	{anchor: "route-loads", tests: []string{"e2e.TestReports"}},
	{anchor: "scope-loads", tests: []string{"e2e.TestSignInAndScopeLoads", "e2e.TestDashboardsUnderNestedLoads"}},
	{anchor: "query-routes", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestThesis"}},
	{anchor: "formats", tests: []string{"e2e.TestTagsByDefaultHandlers", "e2e.TestSignInAndScopeLoads", "e2e.TestLimits", "db.TestRefusedWritesMapToStatuses"}},
	{anchor: "generated-client", tests: []string{"e2e.TestCollectStoresWhatTheSweepMeasured", "e2e.TestGUISignInBrowseAndOpen"}},
	{anchor: "event-routes", tests: []string{"e2e.TestEvents", "e2e.TestGUIBudgetsLive"}},
	{anchor: "scopes", tests: []string{"e2e.TestReports", "e2e.TestPagesAndPlugs"}},
	{anchor: "route-names-and-reverse-urls", tests: []string{"collector.TestReadCallsCoverTheSafeRoutes", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "route-conflicts", tests: []string{"e2e.TestUploadsAndRuns"}},
	{anchor: "resources", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestTagsByDefaultHandlers"}},
	{anchor: "declaration", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "the-action-table", tests: []string{"e2e.TestTagsByDefaultHandlers", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "settings", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestDashboardsUnderNestedLoads", "e2e.TestBudgetsBreachesAndTransactions"}},
	{anchor: "table-resolution", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestUploadsAndRuns"}},
	{anchor: "default-handlers", tests: []string{"e2e.TestTagsByDefaultHandlers"}},
	{anchor: "datasets", tests: []string{"e2e.TestUploadsAndRuns"}},
	{anchor: "dataset-declaration", tests: []string{"e2e.TestUploadsAndRuns"}},
	{anchor: "dataset-settings", tests: []string{"e2e.TestUploadsAndRuns"}},
	{anchor: "settings-whitelists", excluded: "a table of refusals; Voltmeter writes only listed settings, so the refusals are the conformance corpus's"},
	{anchor: "generation-contract-informative", tests: []string{"e2e.TestGeneratedFilesKeepTheLayers", "db.TestGeneratedFilesCurrent", "app.TestGeneratedFilesCurrent"}},
	{anchor: "groups", tests: []string{"db.TestSelectsOverThePhaseGroup", "db.TestProjections"}},
	{anchor: "predicates", tests: []string{"db.TestPredicateForms", "db.TestKeysetPagingNeverRepeatsOrSkips"}},
	{anchor: "selects-over-groups", tests: []string{"db.TestSelectsOverThePhaseGroup", "db.TestProjections", "db.TestAggregates", "db.TestJoins", "db.TestWithNestsChildren"}},
	{anchor: "validators", tests: []string{"db.TestFromAndValidate", "e2e.TestBudgetsBreachesAndTransactions"}},
	{anchor: "changesets", tests: []string{"db.TestCastAllowsOnlyTheListedColumns", "db.TestFromAndValidate", "db.TestRefusedWritesMapToStatuses", "db.TestUpdateAndDeleteByKey"}},
	{anchor: "migrations", tests: []string{"db.TestMigrationsBuildTheSchema", "db.TestMigrateDownStopsAtTheIrreversibleRebuild", "db.TestRebuildCopiesThroughFrom"}},
	{anchor: "the-fold", tests: []string{"db.TestMigrationsBuildTheSchema", "e2e.TestCLIMigrations"}},
	{anchor: "migration-ledger", tests: []string{"db.TestMigrateDownStopsAtTheIrreversibleRebuild", "e2e.TestCLIMigrations"}},
	{anchor: "migration-tooling", tests: []string{"e2e.TestCLIMigrations"}},
	{anchor: "collected-grammar-part-ii", excluded: "repeats the grammar of the sections above, which lang/ebnf checks against them"},
	{anchor: "conformance-and-the-proof-chain", excluded: "the repository's own chain, run by go test at the root; this map is the link Voltmeter adds"},
}

var (
	headingRE = regexp.MustCompile(`^(#{1,3}) (.+)$`)
	testRE    = regexp.MustCompile(`^Test[A-Z]`)
)

func anchorOf(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == ' ', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), "-")
}

func specAnchors(t *testing.T) (all, parts map[string]bool) {
	t.Helper()
	repo := filepath.Join(moduleDir(t), "..", "..")
	b, err := os.ReadFile(filepath.Join(repo, "docs", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	all, parts = map[string]bool{}, map[string]bool{}
	in, fence := false, ""
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		switch {
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			fence = trimmed[:3]
			continue
		}
		m := headingRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		title := strings.TrimSpace(m[2])
		if m[1] == "#" {
			in = strings.HasPrefix(title, "Part I")
			continue
		}
		all[anchorOf(title)] = true
		if in {
			parts[anchorOf(title)] = true
		}
	}
	if len(parts) < 60 {
		t.Fatalf("docs/spec.md looks truncated: %d headings in Part I and Part II", len(parts))
	}
	return all, parts
}

func moduleTests(t *testing.T) map[string]bool {
	t.Helper()
	mod := moduleDir(t)
	tests := map[string]bool{}
	err := filepath.WalkDir(mod, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		dir, _ := filepath.Rel(mod, filepath.Dir(p))
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !testRE.MatchString(fn.Name.Name) || fn.Type.Params.NumFields() != 1 {
				continue
			}
			if star, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr); ok {
				if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "T" {
					tests[filepath.ToSlash(dir)+"."+fn.Name.Name] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tests
}

// TestCoverageMap is the executable feature table of Voltmeter: every
// heading of Part I and Part II of docs/spec.md is either mapped to the
// Voltmeter tests that prove it or excluded with a one-line reason, every
// mapped anchor names a heading the spec has, and every named test is a
// test function of this module (spec §conformance-and-the-proof-chain).
func TestCoverageMap(t *testing.T) {
	all, parts := specAnchors(t)
	tests := moduleTests(t)
	seen := map[string]bool{}
	for _, c := range coverage {
		if seen[c.anchor] {
			t.Errorf("§%s is listed twice", c.anchor)
		}
		seen[c.anchor] = true
		if !all[c.anchor] {
			t.Errorf("§%s names no heading of docs/spec.md", c.anchor)
		}
		switch {
		case len(c.tests) > 0 && c.excluded != "":
			t.Errorf("§%s is both mapped and excluded", c.anchor)
		case len(c.tests) == 0 && strings.TrimSpace(c.excluded) == "":
			t.Errorf("§%s is neither mapped nor excluded with a reason", c.anchor)
		case strings.Contains(c.excluded, "\n"):
			t.Errorf("§%s: the reason for excluding it is more than one line", c.anchor)
		}
		for _, name := range c.tests {
			if !tests[name] {
				t.Errorf("§%s names %s, which is no test function of the module", c.anchor, name)
			}
		}
	}
	for a := range parts {
		if !seen[a] {
			t.Errorf("§%s of Part I or Part II is neither mapped nor excluded", a)
		}
	}
}
