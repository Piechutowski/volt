// Aggregate round trips (spec §selects-over-groups): a grouped select
// against a real SQLite agrees with the same sums, averages and counts
// computed in Go, and an ungrouped one is a lookup of one row, NULL
// aggregates included.
package itest

import (
	"context"
	"testing"

	"github.com/Piechutowski/volt/nao/run"
)

func TestAggregatesGroupedAgreeWithGo(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	type sample struct {
		site string
		day  int
		hits int
	}
	samples := []sample{{"alpha", 1, 3}, {"alpha", 1, 5}, {"alpha", 2, 7}, {"alpha", 3, 1}, {"beta", 2, 4}, {"beta", 5, 6}}
	for _, s := range samples {
		if _, err := run.Run(ctx, db, PageViewCreate(PageViewNew().SiteSet(s.site).DaySet(s.day).HitsSet(s.hits))); err != nil {
			t.Fatal(err)
		}
	}
	type key struct {
		site   string
		bucket int
	}
	type agg struct {
		hits, n int
	}
	want := map[key]agg{}
	var order []key
	for _, s := range samples {
		k := key{s.site, s.day / 2 * 2}
		if _, seen := want[k]; !seen {
			order = append(order, k)
		}
		a := want[k]
		a.hits += s.hits
		a.n++
		want[k] = a
	}
	rows, err := run.Run(ctx, db, PageViewHourly())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("PageViewHourly = %d groups, want %d: %+v", len(rows), len(want), rows)
	}
	for _, r := range rows {
		k := key{r.Site, r.Bucket}
		a, ok := want[k]
		if !ok {
			t.Errorf("group %+v is not one Go computed", k)
			continue
		}
		if r.Total != a.hits || r.N != a.n || r.Mean != float64(a.hits)/float64(a.n) {
			t.Errorf("group %+v = total %d, n %d, mean %v; Go computed total %d, n %d", k, r.Total, r.N, r.Mean, a.hits, a.n)
		}
	}
	for i := 1; i < len(rows); i++ {
		a, b := rows[i-1], rows[i]
		if a.Site > b.Site || (a.Site == b.Site && a.Bucket >= b.Bucket) {
			t.Errorf("order (site asc, bucket asc) violated: %+v before %+v", a, b)
		}
	}
}

func TestAggregatesUngroupedIsOneRow(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	empty, err := run.Run(ctx, db, PageViewTotals())
	if err != nil {
		t.Fatal(err)
	}
	if empty.N != 0 || empty.Top.Valid || empty.Total.Valid {
		t.Fatalf("totals of no rows = %+v; want a zero count and NULL aggregates", empty)
	}
	for _, hits := range []int{2, 9, 4} {
		if _, err := run.Run(ctx, db, PageViewCreate(PageViewNew().SiteSet("alpha").DaySet(1).HitsSet(hits))); err != nil {
			t.Fatal(err)
		}
	}
	got, err := run.Run(ctx, db, PageViewTotals())
	if err != nil {
		t.Fatal(err)
	}
	if got.N != 3 || !got.Top.Valid || got.Top.V != 9 || !got.Total.Valid || got.Total.V != 15 {
		t.Errorf("totals = %+v; want 3 rows, top 9, total 15", got)
	}
}
