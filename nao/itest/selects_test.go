// Select round trips (spec §selects-over-groups): the group-generated
// methods run against a float64 SQLite through a float64 driver, proving the
// checker-rendered WHERE/ORDER SQL executes and filters correctly, with
// one signature across every member of the group.
package itest

import (
	"context"
	"encoding/json"
	"github.com/Piechutowski/volt/nao/run"
	"strings"
	"testing"
)

func seedMetrics(t *testing.T, db run.DB) {
	t.Helper()
	ctx := context.Background()
	for _, r := range []PageViewChangeset{
		PageViewNew().SiteSet("alpha").DaySet(1),
		PageViewNew().SiteSet("alpha").DaySet(1),
		PageViewNew().SiteSet("alpha").DaySet(2),
		PageViewNew().SiteSet("beta").DaySet(1),
	} {
		if _, err := run.Run(ctx, db, PageViewCreate(r)); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []LinkClickChangeset{
		LinkClickNew().SiteSet("alpha").DaySet(1),
		LinkClickNew().SiteSet("beta").DaySet(2),
	} {
		if _, err := run.Run(ctx, db, LinkClickCreate(r)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSelectRowsFiltersPerMember(t *testing.T) {
	db := newDB(t)
	seedMetrics(t, db)
	ctx := context.Background()

	pv, err := run.Run(ctx, db, PageViewRows("alpha", 1))
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

	lc, err := run.Run(ctx, db, LinkClickRows("beta", 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(lc) != 1 || lc[0].Site != "beta" || lc[0].Day != 2 {
		t.Fatalf("LinkClickRows(beta, 2) = %+v, want the beta row", lc)
	}

	none, err := run.Run(ctx, db, LinkClickRows("alpha", 99))
	if err != nil || len(none) != 0 {
		t.Fatalf("no-match select = %v rows, err %v", len(none), err)
	}
}

func TestSelectOrderDescApplies(t *testing.T) {
	db := newDB(t)
	seedMetrics(t, db)

	rows, err := run.Run(context.Background(), db, PageViewRecent("nosuch", 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("Recent(since day 1) = %d rows, want all 4", len(rows))
	}

	ordered, err := run.Run(context.Background(), db, PageViewRows("alpha", 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) == 2 && ordered[0].ID > ordered[1].ID {
		t.Errorf("order (day desc, id) violated: ids %d before %d", ordered[0].ID, ordered[1].ID)
	}
}

func TestSelectProjectionSharedType(t *testing.T) {
	db := newDB(t)
	seedMetrics(t, db)
	ctx := context.Background()

	pv, err := run.Run(ctx, db, PageViewSummary("alpha", 1))
	if err != nil {
		t.Fatal(err)
	}
	lc, err := run.Run(ctx, db, LinkClickSummary("alpha", 1))
	if err != nil {
		t.Fatal(err)
	}
	rows := append(pv, lc...)
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
	db := newDB(t)
	seedMetrics(t, db)

	rows, err := run.Run(context.Background(), db, LinkClickPublic())
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
// unpacked by json_each on the float64 driver (§predicates) — a
// multi-value list, a single value, and the empty list that matches
// nothing, with one signature across both members.
func TestSelectListParam(t *testing.T) {
	db := newDB(t)
	seedMetrics(t, db)
	ctx := context.Background()

	both, err := run.Run(ctx, db, PageViewOnSites([]string{"alpha", "beta"}))
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
	beta, err := run.Run(ctx, db, LinkClickOnSites([]string{"beta"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(beta) != 1 || beta[0].Site != "beta" {
		t.Fatalf("LinkClickOnSites(beta) = %+v, want the beta row", beta)
	}
	none, err := run.Run(ctx, db, PageViewOnSites(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("PageViewOnSites(empty) = %d rows, want 0", len(none))
	}
	odd, err := run.Run(ctx, db, PageViewOnSites([]string{`al"pha`, "alpha"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(odd) != 3 {
		t.Fatalf("PageViewOnSites(quoted, alpha) = %d rows, want 3", len(odd))
	}
}
