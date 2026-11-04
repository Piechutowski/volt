// Limit round trips (spec §selects-over-groups): a select's limit: keeps
// the first rows in its order on a real SQLite, a literal on every member
// of a group, a parameter after a grouping and on the parents of a
// nesting, and a parameter below 1 is refused before any statement runs,
// since SQLite reads a negative LIMIT as no limit at all.
package itest

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

type statementCount struct{ n *atomic.Int64 }

func (h statementCount) Enabled(context.Context, slog.Level) bool { return true }

func (h statementCount) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "statement" {
		h.n.Add(1)
	}
	return nil
}

func (h statementCount) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h statementCount) WithGroup(string) slog.Handler      { return h }

func seedHits(t *testing.T, db run.DB) []PageView {
	t.Helper()
	var out []PageView
	for _, r := range []PageViewChangeset{
		PageViewNew().SiteSet("alpha").DaySet(1).HitsSet(5),
		PageViewNew().SiteSet("alpha").DaySet(2).HitsSet(9),
		PageViewNew().SiteSet("beta").DaySet(1).HitsSet(100),
		PageViewNew().SiteSet("alpha").DaySet(3).HitsSet(9),
		PageViewNew().SiteSet("alpha").DaySet(4).HitsSet(1),
		PageViewNew().SiteSet("gamma").DaySet(1).HitsSet(40),
		PageViewNew().SiteSet("alpha").DaySet(5).HitsSet(7),
	} {
		v, err := run.Run(context.Background(), db, PageViewCreate(r))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

func pageViewIDs(rows []PageView) []int {
	ids := make([]int, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

func TestSelectLimitKeepsTheFirstRowsInOrder(t *testing.T) {
	db := newDB(t)
	pv := seedHits(t, db)
	ctx := context.Background()

	top, err := run.Run(ctx, db, PageViewTopHits("alpha", 3))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pageViewIDs(top), []int{pv[1].ID, pv[3].ID, pv[6].ID}; !slices.Equal(got, want) {
		t.Fatalf("top 3 of alpha by hits desc, id asc = ids %v, want %v", got, want)
	}
	one, err := run.Run(ctx, db, PageViewTopHits("alpha", 1))
	if err != nil || len(one) != 1 || one[0].ID != pv[1].ID {
		t.Fatalf("top 1 of alpha = %+v, %v; want the first of the two rows with 9 hits", one, err)
	}
	all, err := run.Run(ctx, db, PageViewTopHits("alpha", 50))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pageViewIDs(all), []int{pv[1].ID, pv[3].ID, pv[6].ID, pv[0].ID, pv[4].ID}; !slices.Equal(got, want) {
		t.Fatalf("a limit above the row count = ids %v, want every alpha row in order %v", got, want)
	}
}

func TestSelectLimitLiteralOnEveryMember(t *testing.T) {
	db := newDB(t)
	seedMetrics(t, db)
	ctx := context.Background()

	pv, err := run.Run(ctx, db, PageViewFirstDays())
	if err != nil {
		t.Fatal(err)
	}
	if len(pv) != 2 || pv[0].Day != 1 || pv[1].Day != 1 || pv[0].ID > pv[1].ID {
		t.Fatalf("first two page views by day, id = %+v; want the two earliest of day 1", pv)
	}
	if _, err := run.Run(ctx, db, LinkClickCreate(LinkClickNew().SiteSet("gamma").DaySet(1))); err != nil {
		t.Fatal(err)
	}
	lc, err := run.Run(ctx, db, LinkClickFirstDays())
	if err != nil {
		t.Fatal(err)
	}
	if len(lc) != 2 || lc[0].Day != 1 || lc[1].Day != 1 || lc[0].Site != "alpha" || lc[1].Site != "gamma" {
		t.Fatalf("first two link clicks by day, id = %+v; want alpha and gamma of day 1", lc)
	}
}

func TestSelectLimitCountsGroups(t *testing.T) {
	db := newDB(t)
	seedHits(t, db)

	sites, err := run.Run(context.Background(), db, PageViewTopSites(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || sites[0].Site != "beta" || sites[0].Total != 100 || sites[1].Site != "gamma" || sites[1].Total != 40 {
		t.Fatalf("top two sites by total = %+v; want beta 100 and gamma 40, alpha's 31 cut", sites)
	}
}

func TestSelectLimitCountsParents(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	var users []User
	for _, email := range []string{"ann@example.com", "bob@example.com", "cy@example.com"} {
		users = append(users, seedUser(t, db, email))
	}
	for i := 0; i < 3; i++ {
		if _, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(users[1].ID).TotalSet(float64(i+1)).FeeSet(1))); err != nil {
			t.Fatal(err)
		}
	}
	newest, err := run.Run(ctx, db, UserNewest(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(newest) != 2 || newest[0].ID != users[2].ID || newest[1].ID != users[1].ID {
		t.Fatalf("two newest users = %+v; want cy then bob", newest)
	}
	if len(newest[0].Orders) != 0 || len(newest[1].Orders) != 3 {
		t.Fatalf("cy has %d orders and bob %d; want 0 and all 3, the limit counting parents only", len(newest[0].Orders), len(newest[1].Orders))
	}
}

func TestSelectLimitBelowOneRunsNoStatement(t *testing.T) {
	var n atomic.Int64
	db := newDB(t).WithLogger(slog.New(statementCount{&n}))
	seedHits(t, db)
	ctx := context.Background()

	for _, limit := range []int{0, -1} {
		before := n.Load()
		rows, err := run.Run(ctx, db, PageViewTopHits("alpha", limit))
		if !errors.Is(err, rt.ErrLimit) || rows != nil {
			t.Errorf("PageViewTopHits(alpha, %d) = %v, %v; want rt.ErrLimit and no rows", limit, rows, err)
		}
		sites, err := run.Run(ctx, db, PageViewTopSites(limit))
		if !errors.Is(err, rt.ErrLimit) || sites != nil {
			t.Errorf("PageViewTopSites(%d) = %v, %v; want rt.ErrLimit and no rows", limit, sites, err)
		}
		users, err := run.Run(ctx, db, UserNewest(limit))
		if !errors.Is(err, rt.ErrLimit) || users != nil {
			t.Errorf("UserNewest(%d) = %v, %v; want rt.ErrLimit and no rows", limit, users, err)
		}
		if ran := n.Load() - before; ran != 0 {
			t.Errorf("a limit of %d ran %d statements; want none", limit, ran)
		}
	}

	q := PageViewTopHits("alpha", -1)
	unchecked, err := run.Run(ctx, db, rt.Query[PageView]{SQL: q.SQL, Args: q.Args, Scan: q.Scan})
	if err != nil || len(unchecked) != 5 {
		t.Fatalf("the statement run past the check with -1 = %d rows, %v; SQLite reads LIMIT -1 as no limit, so want all 5 alpha rows", len(unchecked), err)
	}
}
