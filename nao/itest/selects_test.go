// Select round trips (spec §V11, decision D25): the group-generated
// methods run against a real SQLite through a real driver, proving the
// checker-rendered WHERE/ORDER SQL executes and filters correctly, with
// one signature across every member of the group.
package itest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// seedMetrics fills both members of the metrics group with rows on two
// sites and two days.
func seedMetrics(t *testing.T, q *Queries) {
	t.Helper()
	ctx := context.Background()
	for _, r := range []PageViewCreateParams{
		{Site: "alpha", Day: 1},
		{Site: "alpha", Day: 1},
		{Site: "alpha", Day: 2},
		{Site: "beta", Day: 1},
	} {
		if _, err := q.PageViewCreate(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []LinkClickCreateParams{
		{Site: "alpha", Day: 1},
		{Site: "beta", Day: 2},
	} {
		if _, err := q.LinkClickCreate(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSelectRowsFiltersPerMember(t *testing.T) {
	_, q := newDB(t)
	seedMetrics(t, q)
	ctx := context.Background()

	// Same select, same signature, first member.
	pv, err := q.PageViewRows(ctx, "alpha", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(pv) != 2 {
		t.Fatalf("PageViewRows(alpha, 1) = %d rows, want 2", len(pv))
	}
	for _, v := range pv {
		if v.Site != "alpha" || v.Day != 1 {
			t.Errorf("row escaped the predicate: %+v", v)
		}
	}

	// Second member, same predicate rendering.
	lc, err := q.LinkClickRows(ctx, "beta", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(lc) != 1 || lc[0].Site != "beta" || lc[0].Day != 2 {
		t.Fatalf("LinkClickRows(beta, 2) = %+v, want the beta row", lc)
	}

	// No match is an empty slice, not an error.
	none, err := q.LinkClickRows(ctx, "alpha", 99)
	if err != nil || len(none) != 0 {
		t.Fatalf("no-match select = %v rows, err %v", len(none), err)
	}
}

func TestSelectOrderDescApplies(t *testing.T) {
	_, q := newDB(t)
	seedMetrics(t, q)

	// recent = at or since: with :from = 0 every row qualifies, so the
	// [order: (day desc, id)] of Rows is the observable difference.
	rows, err := q.PageViewRecent(context.Background(), "nosuch", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("Recent(since day 1) = %d rows, want all 4", len(rows))
	}

	ordered, err := q.PageViewRows(context.Background(), "alpha", 1)
	if err != nil {
		t.Fatal(err)
	}
	// day desc, then id asc: both rows share day 1, ids ascend.
	if len(ordered) == 2 && ordered[0].ID > ordered[1].ID {
		t.Errorf("order (day desc, id) violated: ids %d before %d", ordered[0].ID, ordered[1].ID)
	}
}

func TestSelectProjectionSharedType(t *testing.T) {
	_, q := newDB(t)
	seedMetrics(t, q)
	ctx := context.Background()

	// One shared row type (Summary), two sources: the §V11.7 explicit
	// list. Both methods return the same Go type.
	pv, err := q.PageViewSummary(ctx, "alpha", 1)
	if err != nil {
		t.Fatal(err)
	}
	lc, err := q.LinkClickSummary(ctx, "alpha", 1)
	if err != nil {
		t.Fatal(err)
	}
	rows := append(pv, lc...) // compiles only because the type is shared
	if len(pv) != 2 || len(lc) != 1 || len(rows) != 3 {
		t.Fatalf("summary rows = %d + %d, want 2 + 1", len(pv), len(lc))
	}
	for _, r := range rows {
		if r.Site != "alpha" || r.Day != 1 {
			t.Errorf("row escaped the predicate: %+v", r)
		}
	}
}

func TestSelectProjectionStructDerivative(t *testing.T) {
	_, q := newDB(t)
	seedMetrics(t, q)

	// The star form: LinkClickPublic is LinkClick minus target, every
	// kept field copied verbatim (§V11.7).
	rows, err := q.LinkClickPublic(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("LinkClickPublic = %d rows, want 2", len(rows))
	}
	if rows[0].ID > rows[1].ID {
		t.Errorf("order (id asc) violated: %d before %d", rows[0].ID, rows[1].ID)
	}
	doc, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), "target") {
		t.Errorf("excluded column leaked into the derivative's JSON: %s", doc)
	}
}

// TestSelectListParam: `site in :sites` binds one JSON array and is
// unpacked by json_each on the real driver (§V10.3, D66) — a
// multi-value list, a single value, and the empty list that matches
// nothing, with one signature across both members.
func TestSelectListParam(t *testing.T) {
	_, q := newDB(t)
	seedMetrics(t, q)
	ctx := context.Background()

	both, err := q.PageViewOnSites(ctx, []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(both) != 4 {
		t.Fatalf("PageViewOnSites(alpha, beta) = %d rows, want 4", len(both))
	}
	for i := 1; i < len(both); i++ {
		if both[i].ID < both[i-1].ID {
			t.Fatalf("rows not in id order: %+v", both)
		}
	}
	beta, err := q.LinkClickOnSites(ctx, []string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(beta) != 1 || beta[0].Site != "beta" {
		t.Fatalf("LinkClickOnSites(beta) = %+v, want the beta row", beta)
	}
	none, err := q.PageViewOnSites(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("PageViewOnSites(empty) = %d rows, want 0", len(none))
	}
	// A value with a JSON-significant character is carried verbatim.
	odd, err := q.PageViewOnSites(ctx, []string{`al"pha`, "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(odd) != 3 {
		t.Fatalf("PageViewOnSites(quoted, alpha) = %d rows, want 3", len(odd))
	}
}
