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
	anchor      string
	tests       []string
	excluded    string
	unexercised string
}

var coverage = []covered{
	{anchor: "values-and-effects", tests: []string{"app.TestHandlersDescribeTheirEffects", "app.TestControllersDeterministic", "db.TestAllKeepsResultOrder", "gui/state.TestUpdateLeavesItsInputAlone", "db.TestChangesetsLeaveTheirReceiver", "app.TestResponsesLeaveTheirReceiver", "db.TestTxIsAllOrNothing", "db.TestStatementCacheAndLogging"}},

	{anchor: "table-of-contents", excluded: "navigation of the spec itself; it states no rule"},
	{anchor: "notation", excluded: "how the spec writes its grammar; no application behavior follows from it"},
	{anchor: "ebnf", excluded: "the grammar's metalanguage; lang/ebnf executes it against the parser"},
	{anchor: "character-notation-uxxxx", excluded: "how the spec names characters; no application behavior follows from it"},
	{anchor: "unicode-general-categories", excluded: "Voltmeter's names are ASCII; letters beyond ASCII are the conformance corpus's"},
	{anchor: "case-sensitivity-and-terminology", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "e2e.TestCLICheckVetGenRoutes"}, unexercised: "the refusal of a miscased keyword or of a name used in another case, which is the conformance corpus's"},
	{anchor: "source-text", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "lexical-structure", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "tokenization", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "line-structure-and-whitespace", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "e2e.TestCLICheckVetGenRoutes"}, unexercised: "CRLF line ends; the sources end lines with LF"},
	{anchor: "comments", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "identifiers", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "keywords", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestJoins", "e2e.TestUploadsAndRuns", "db.TestPredicateForms", "db.TestPredicateAlgebra"}, unexercised: "the refusal of a miscased or misplaced keyword, which is the conformance corpus's"},
	{anchor: "string-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "multi-line-string-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestGeneratedFilesCurrent"}},
	{anchor: "escape-sequences", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestGeneratedFilesCurrent"}, unexercised: "only the quote escape occurs; a backslash, newline or tab escape has no place in Voltmeter's strings"},
	{anchor: "numeric-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestProjections", "db.TestSelectFormsTheOthersLeaveOut"}, unexercised: "an exponent and a negative literal; no Voltmeter value is compared against one"},
	{anchor: "boolean-and-null-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestPredicateForms", "db.TestSelectFormsTheOthersLeaveOut"}, unexercised: "null as a value, such as a default or a record field; it appears only in is null and is not null"},
	{anchor: "expression-literals", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestMigrationsBuildTheSchema", "db.TestRefusedWritesMapToStatuses"}},
	{anchor: "operators-and-punctuation", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "db.TestPredicateAlgebra"}},
	{anchor: "common-syntactic-forms", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestSourcesSpeakTheLexicon"}, unexercised: "the refusals; a clean volt check proves the accepting side only"},
	{anchor: "names-and-qualified-names", tests: []string{"db.TestJoins", "app.TestGeneratedFilesCurrent"}},
	{anchor: "settings-lists", tests: []string{"db.TestMigrationsBuildTheSchema", "e2e.TestCLICheckVetGenRoutes"}, unexercised: "a repeated tag:, the one repeatable setting, which no column uses"},
	{anchor: "program-structure", tests: []string{"e2e.TestSourcesSpeakTheLexicon", "e2e.TestCLICheckVetGenRoutes"}, unexercised: "the refusals; a clean volt check proves the accepting side only"},
	{anchor: "element-definitions", tests: []string{"db.TestMigrationsBuildTheSchema"}},
	{anchor: "table", tests: []string{"db.TestMigrationsBuildTheSchema", "db.TestNotesBecomeComments", "db.TestPartialsInjectInSourceOrder"}, unexercised: "model:, the one table setting Voltmeter does not use"},
	{anchor: "columns", tests: []string{"db.TestCastAllowsOnlyTheListedColumns", "db.TestRefusedWritesMapToStatuses", "db.TestAutoincrementNeverReusesARunID"}, unexercised: "[]byte columns, tag: struct tags, model: and a nullable declared type"},
	{anchor: "default-values", tests: []string{"db.TestCastAllowsOnlyTheListedColumns", "db.TestMigrationsBuildTheSchema"}},
	{anchor: "indexes", tests: []string{"db.TestRefusedWritesMapToStatuses", "db.TestMigrationsBuildTheSchema"}},
	{anchor: "relationships-ref", tests: []string{"db.TestRefActions", "db.TestJoins"}, unexercised: "composite and one-to-one refs, and the restrict and set_default actions"},
	{anchor: "types", tests: []string{"db.TestPredicateForms", "db.TestCastAllowsOnlyTheListedColumns", "db.TestRebuildCopiesThroughFrom"}, unexercised: "a nullable declared type; ordering a numeric declared type is refused by the checker (a gap)"},
	{anchor: "tablepartial", tests: []string{"db.TestFromAndValidate", "db.TestMigrationsBuildTheSchema"}, unexercised: "a table overriding a partial's column (rule 4), which vet/shadowedcolumn warns against"},
	{anchor: "records-sample-data", tests: []string{"db.TestMigrationsBuildTheSchema"}, unexercised: "a top-level records element, a records block without a column list, and null or expression values in records"},
	{anchor: "notes", tests: []string{"db.TestNotesBecomeComments"}},
	{anchor: "static-semantics", tests: []string{"e2e.TestCLICheckVetGenRoutes", "db.TestGeneratedFilesCurrent"}, unexercised: "the refusals; a clean volt check proves the accepting side only"},
	{anchor: "namespaces-and-uniqueness", tests: []string{"e2e.TestCLICheckVetGenRoutes", "db.TestGeneratedFilesCurrent", "app.TestGeneratedFilesCurrent"}, unexercised: "the refusals; the checker refusing BudgetState, the column handle's name, as a type shaped Health, but no test pins it"},
	{anchor: "reference-resolution", tests: []string{"db.TestRefActions", "db.TestJoins"}},
	{anchor: "partial-injection-order", tests: []string{"db.TestPartialsInjectInSourceOrder"}, unexercised: "a column defined twice, so the first-definition position never applies"},
	{anchor: "nullability-and-cardinality", tests: []string{"db.TestJoins", "db.TestRefActions", "db.TestRefusedWritesMapToStatuses"}},
	{anchor: "collected-grammar-part-i", excluded: "repeats the grammar of the sections above, which lang/ebnf checks against them"},

	{anchor: "layers-files-and-the-superset-rule", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestSourcesSpeakTheLexicon"}},
	{anchor: "data-and-network-layers", tests: []string{"e2e.TestGeneratedFilesKeepTheLayers", "e2e.TestClientsLinkNoServerCode"}, unexercised: "gen removing the route files of a package that no longer routes (rule 4)"},
	{anchor: "projects-and-packages", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestCLIMigrations"}, unexercised: "volt gen -o, a package routing its own tables, and the layout invariance of a project moved between directories"},
	{anchor: "imports", tests: []string{"app.TestGeneratedFilesCurrent", "e2e.TestGeneratedFilesKeepTheLayers"}, unexercised: "an import alias and a multi-segment package path"},
	{anchor: "pipelines", tests: []string{"e2e.TestPagesAndPlugs"}},
	{anchor: "scopes-and-routes", tests: []string{"e2e.TestPagesAndPlugs", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "route-paths", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestPagesAndPlugs"}},
	{anchor: "routes", tests: []string{"e2e.TestPagesAndPlugs", "e2e.TestReports", "e2e.TestUploadsAndRuns"}},
	{anchor: "handler-references", tests: []string{"app.TestControllersDeterministic", "app.TestHandlersDescribeTheirEffects", "e2e.TestCLICheckVetGenRoutes"}, unexercised: "two routes sharing one Controller.Action (rule 3); two routes share only the query db.RunUntagged"},
	{anchor: "requests-and-responses", tests: []string{"app.TestHandlersDescribeTheirEffects", "app.TestHandlersRefuseBeforeAnyEffect", "e2e.TestReports"}},
	{anchor: "request-bodies", tests: []string{"e2e.TestSignInAndScopeLoads", "e2e.TestUploadsAndRuns", "e2e.TestBudgetsBreachesAndTransactions", "e2e.TestMalformedBodiesBeforeTheRow", "e2e.TestMeasurementsValidated"}},
	{anchor: "route-loads", tests: []string{"e2e.TestReports", "e2e.TestRouteLoadsBindFromTheQuery"}, unexercised: "a literal argument, a List load, and a timeout bounding a route's loads, since the timed routes have none"},
	{anchor: "scope-loads", tests: []string{"e2e.TestSignInAndScopeLoads", "e2e.TestDashboardsUnderNestedLoads", "e2e.TestQueryRouteUnderAScopeLoad"}, unexercised: "the zero Load, an error through the spine; CurrentUser and OwnedBoard always decide"},
	{anchor: "query-routes", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestThesis", "e2e.TestListAndPrefixParameters", "e2e.TestMalformedBodiesBeforeTheRow", "e2e.TestQueryRouteUnderAScopeLoad"}},
	{anchor: "formats", tests: []string{"e2e.TestTagsByDefaultHandlers", "e2e.TestSignInAndScopeLoads", "e2e.TestLimits", "db.TestRefusedWritesMapToStatuses", "e2e.TestWritesNegotiateFirst"}},
	{anchor: "generated-client", tests: []string{"e2e.TestCollectStoresWhatTheSweepMeasured", "e2e.TestGUISignInBrowseAndOpen", "e2e.TestListAndPrefixParameters", "e2e.TestClientsLinkNoServerCode"}},
	{anchor: "event-routes", tests: []string{"e2e.TestEvents", "e2e.TestGUIBudgetsLive", "e2e.TestWindowHandlersDriveTheState"}, unexercised: "a lagging subscriber dropped by the broker, which the root package's TestBrokerDropsStalledSubscriber proves"},
	{anchor: "scopes", tests: []string{"e2e.TestReports", "e2e.TestPagesAndPlugs"}},
	{anchor: "route-names-and-reverse-urls", tests: []string{"e2e.TestReverseURLsRoundTrip", "collector.TestReadCallsCoverTheSafeRoutes", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "route-conflicts", tests: []string{"e2e.TestUploadsAndRuns"}},
	{anchor: "resources", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestTagsByDefaultHandlers", "e2e.TestRemovedActionsAreRefused"}},
	{anchor: "declaration", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestCLICheckVetGenRoutes"}},
	{anchor: "the-action-table", tests: []string{"e2e.TestTagsByDefaultHandlers", "e2e.TestCLICheckVetGenRoutes", "e2e.TestRemovedActionsAreRefused"}, unexercised: "the new and edit actions, which serve forms an API has none of"},
	{anchor: "settings", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestDashboardsUnderNestedLoads", "e2e.TestBudgetsBreachesAndTransactions", "e2e.TestQueryRouteUnderAScopeLoad", "e2e.TestRemovedActionsAreRefused"}},
	{anchor: "table-resolution", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestUploadsAndRuns"}},
	{anchor: "default-handlers", tests: []string{"e2e.TestTagsByDefaultHandlers"}},
	{anchor: "datasets", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestListAndPrefixParameters"}},
	{anchor: "dataset-declaration", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestListAndPrefixParameters"}},
	{anchor: "dataset-settings", tests: []string{"e2e.TestUploadsAndRuns", "e2e.TestListAndPrefixParameters"}},
	{anchor: "settings-whitelists", tests: []string{"e2e.TestCLICheckVetGenRoutes", "e2e.TestRouteLoadsBindFromTheQuery", "e2e.TestListAndPrefixParameters", "e2e.TestUploadsAndRuns", "db.TestMigrationsBuildTheSchema"}, unexercised: "the refusal of a setting outside its element's list; every listed setting is used"},
	{anchor: "generation-contract-informative", tests: []string{"e2e.TestGeneratedFilesKeepTheLayers", "db.TestGeneratedFilesCurrent", "app.TestGeneratedFilesCurrent"}},
	{anchor: "groups", tests: []string{"db.TestSelectsOverThePhaseGroup", "db.TestProjections", "db.TestPredicateAlgebra"}},
	{anchor: "predicates", tests: []string{"db.TestPredicateForms", "db.TestKeysetPagingNeverRepeatsOrSkips", "db.TestPredicateAlgebra", "db.TestSelectFormsTheOthersLeaveOut", "db.TestDynamicVerbs", "e2e.TestKeysetPaging"}, unexercised: "a negative or exponent literal, a parameter or literal left of the operator, and <, <=, > or >= over a numeric declared type, which the checker refuses (a gap)"},
	{anchor: "selects-over-groups", tests: []string{"db.TestSelectsOverThePhaseGroup", "db.TestProjections", "db.TestAggregates", "db.TestJoins", "db.TestWithNestsChildren", "db.TestPredicateAlgebra", "db.TestSelectFormsTheOthersLeaveOut", "e2e.TestListAndPrefixParameters"}, unexercised: "order:, aggregates and arithmetic over a numeric declared type, which the checker refuses (a gap)"},
	{anchor: "validators", tests: []string{"db.TestFromAndValidate", "e2e.TestBudgetsBreachesAndTransactions", "e2e.TestMeasurementsValidated"}},
	{anchor: "changesets", tests: []string{"db.TestCastAllowsOnlyTheListedColumns", "db.TestFromAndValidate", "db.TestRefusedWritesMapToStatuses", "db.TestUpdateAndDeleteByKey", "db.TestChangesetsLeaveTheirReceiver", "db.TestDynamicVerbs", "e2e.TestMeasurementsValidated"}},
	{anchor: "migrations", tests: []string{"db.TestMigrationsBuildTheSchema", "db.TestMigrateDownStopsAtTheIrreversibleRebuild", "db.TestRebuildCopiesThroughFrom", "db.TestEarlyDownsRevertTheirStructure"}, unexercised: "drop table, drop index, and a rebuild that keeps autoincrement (the sqlite_sequence carry-over)"},
	{anchor: "the-fold", tests: []string{"db.TestMigrationsBuildTheSchema", "e2e.TestCLIMigrations"}},
	{anchor: "migration-ledger", tests: []string{"db.TestMigrateDownStopsAtTheIrreversibleRebuild", "e2e.TestCLIMigrations"}},
	{anchor: "migration-tooling", tests: []string{"e2e.TestCLIMigrations"}},
	{anchor: "collected-grammar-part-ii", excluded: "repeats the grammar of the sections above, which lang/ebnf checks against them"},
	{anchor: "conformance-and-the-proof-chain", excluded: "the repository's chain, whose links run at the root; Voltmeter's drift and round-trip tests are mapped under the sections they prove"},

	{anchor: "mapping-to-go", tests: []string{"db.TestGeneratedFilesCurrent", "e2e.TestGeneratedFilesKeepTheLayers", "e2e.TestClientsLinkNoServerCode"}},
	{anchor: "go-types", tests: []string{"db.TestRebuildCopiesThroughFrom", "db.TestPredicateForms", "db.TestSelectFormsTheOthersLeaveOut", "db.TestCastAllowsOnlyTheListedColumns"}, unexercised: "[]byte, which no Voltmeter column holds"},
	{anchor: "nullability", tests: []string{"db.TestJoins", "db.TestDynamicVerbs", "db.TestRefActions", "db.TestSelectFormsTheOthersLeaveOut"}, unexercised: "[]byte and rt.Null[[]byte], which no Voltmeter column holds"},
	{anchor: "names", tests: []string{"db.TestGeneratedFilesCurrent", "e2e.TestCLICheckVetGenRoutes"}, unexercised: "model: and a name beginning with a digit"},
	{anchor: "notes-become-doc-comments", tests: []string{"db.TestNotesBecomeComments"}, unexercised: "a partial column overridden by a table, which brings its own note"},
	{anchor: "struct-tags", tests: []string{"db.TestPartialsInjectInSourceOrder", "e2e.TestQueryRouteUnderAScopeLoad", "e2e.TestReverseURLsRoundTrip"}, unexercised: "tag: passthrough pairs, which no column needs"},
	{anchor: "generated-query-surface", tests: []string{"db.TestDynamicVerbs", "db.TestKeysetPagingNeverRepeatsOrSkips", "db.TestUpdateAndDeleteByKey", "db.TestTxIsAllOrNothing", "db.TestAllKeepsResultOrder", "db.TestStatementCacheAndLogging", "e2e.TestKeysetPaging"}, unexercised: "Distinct, which selects every model column and the key with them, so it cannot change a result (a gap)"},
	{anchor: "mapping-to-sqlite-ddl", tests: []string{"db.TestMigrationsBuildTheSchema", "db.TestPartialsInjectInSourceOrder", "db.TestNotesBecomeComments", "db.TestAutoincrementNeverReusesARunID", "db.TestRefActions"}},
	{anchor: "license-and-provenance", excluded: "the spec's own licence and sources; no application behavior follows from it"},
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
			in = strings.HasPrefix(title, "Part I") || title == "Appendices"
			continue
		}
		all[anchorOf(title)] = true
		if in {
			parts[anchorOf(title)] = true
		}
	}
	if len(parts) < 60 {
		t.Fatalf("docs/spec.md looks truncated: %d headings in Part I, Part II and the appendices", len(parts))
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
// heading of Part I, Part II and the appendices of docs/spec.md is
// either mapped to the Voltmeter tests that prove it or excluded with a
// one-line reason, a mapped heading names in one line the rules of it
// Voltmeter does not exercise, every anchor names a heading the spec
// has, and every named test is a test function of this module. A clean
// volt check counts as proof of the accepting side of a section and
// never of its refusals, which are the conformance corpus's. The map is
// a reviewed claim: the test proves the names exist, not that a named
// test exercises its section, which a reader judges from the test.
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
		case c.unexercised != "" && len(c.tests) == 0:
			t.Errorf("§%s lists rules it does not exercise but maps no test", c.anchor)
		case strings.Contains(c.unexercised, "\n"):
			t.Errorf("§%s: what it does not exercise is more than one line", c.anchor)
		}
		for _, name := range c.tests {
			if !tests[name] {
				t.Errorf("§%s names %s, which is no test function of the module", c.anchor, name)
			}
		}
	}
	for a := range parts {
		if !seen[a] {
			t.Errorf("§%s of Part I, Part II or the appendices is neither mapped nor excluded", a)
		}
	}
}
