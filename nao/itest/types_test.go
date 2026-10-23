// Declared type round trips (spec §types, §predicates): a package-
// qualified Type reached through a from path is written, scanned back,
// compared with = and a literal, and bound as a list through its
// driver value.
package itest

import (
	"context"
	"testing"

	"github.com/Piechutowski/volt/nao/itest/pay"
	"github.com/Piechutowski/volt/nao/rt"
)

func TestDeclaredTypeRoundTrip(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")

	paid, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(9.99).FeeSet(pay.Cents(250)))
	if err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}
	free, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(1.00).FeeSet(0))
	if err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}
	got, err := q.OrderGet(ctx, paid.ID)
	if err != nil || got.Fee != 250 {
		t.Fatalf("OrderGet = %+v, %v; want fee 250 scanned through pay.Cents", got, err)
	}

	byFee, err := q.OrderByFee(ctx, 250)
	if err != nil {
		t.Fatal(err)
	}
	if len(byFee) != 1 || byFee[0].ID != paid.ID {
		t.Errorf("OrderByFee(250) = %+v, want the paid order", byFee)
	}

	fees, err := q.OrderFees(ctx, []pay.Cents{250, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(fees) != 2 || fees[0].ID != paid.ID || fees[1].ID != free.ID {
		t.Errorf("OrderFees(250, 0) = %+v, want both orders in id order", fees)
	}
	none, err := q.OrderFees(ctx, nil)
	if err != nil || len(none) != 0 {
		t.Errorf("OrderFees(empty) = %+v, %v; want no rows", none, err)
	}

	zero, err := q.OrderFree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(zero) != 1 || zero[0].ID != free.ID {
		t.Errorf("OrderFree = %+v, want the free order", zero)
	}
}

// TestKindedTypeRoundTrip proves a declared type whose Go declaration
// spells its storage type (spec §types) on a real SQLite: no
// Scanner or Valuer, written and scanned back by kind, compared through
// its handle, and bound as a list of its values in one JSON array.
func TestKindedTypeRoundTrip(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")

	pending, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(2.00).FeeSet(0))
	if err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}
	shipped, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(3.00).FeeSet(0).StatusSet(OrderStateShipped))
	if err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}
	if _, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(4.00).FeeSet(0).StatusSet(OrderStateDelivered)); err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}
	if got, err := q.OrderGet(ctx, shipped.ID); err != nil || got.Status != OrderStateShipped {
		t.Fatalf("OrderGet = %+v, %v; want status scanned back as OrderStateShipped", got, err)
	}

	open, err := q.OrderByState(ctx, []OrderState{OrderStatePending, OrderStateShipped})
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 2 || open[0].ID != pending.ID || open[1].ID != shipped.ID {
		t.Errorf("OrderByState(pending, shipped) = %+v, want those two orders in id order", open)
	}
	if none, err := q.OrderByState(ctx, nil); err != nil || len(none) != 0 {
		t.Errorf("OrderByState(empty) = %+v, %v; want no rows", none, err)
	}
	if n, err := q.OrderCount(ctx, OrderStatus.In(OrderStateDelivered)); err != nil || n != 1 {
		t.Errorf("delivered count = %d, %v; want 1", n, err)
	}
}

// TestKindedBoolAndFloatRoundTrip proves kinded types stored as bool and
// float64 (spec §types), plain and nullable, on a real SQLite:
// SQLite hands a bool back as an integer, which database/sql converts
// into a bool but not into a named bool by itself.
func TestKindedBoolAndFloatRoundTrip(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)

	full, err := q.SiteFlagCreate(ctx, SiteFlagNew().FeaturedSet(true).PinnedSet(rt.Some(Featured(true))).RatioSet(0.5).WeightSet(rt.Some(Ratio(2.5))))
	if err != nil {
		t.Fatalf("SiteFlagCreate: %v", err)
	}
	empty, err := q.SiteFlagCreate(ctx, SiteFlagNew().FeaturedSet(false).PinnedSet(rt.Null[Featured]{}).RatioSet(0).WeightSet(rt.Null[Ratio]{}))
	if err != nil {
		t.Fatalf("SiteFlagCreate: %v", err)
	}
	got, err := q.SiteFlagGet(ctx, full.ID)
	if err != nil {
		t.Fatalf("SiteFlagGet: %v", err)
	}
	if !bool(got.Featured) || got.Pinned != rt.Some(Featured(true)) || got.Ratio != 0.5 || got.Weight != rt.Some(Ratio(2.5)) {
		t.Errorf("SiteFlagGet = %+v, want featured, pinned, ratio 0.5, weight 2.5", got)
	}
	got, err = q.SiteFlagGet(ctx, empty.ID)
	if err != nil {
		t.Fatalf("SiteFlagGet: %v", err)
	}
	if bool(got.Featured) || got.Pinned.Valid || got.Ratio != 0 || got.Weight.Valid {
		t.Errorf("SiteFlagGet = %+v, want not featured, ratio 0, pinned and weight NULL", got)
	}
	all, err := q.SiteFlagList(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("SiteFlagList = %+v, %v; want both rows", all, err)
	}
}

// TestScannerBoolKindRoundTrip proves that a package-qualified type of Go
// kind bool that implements sql.Scanner is scanned by its own Scan, plain
// and nullable (spec §types): rt.Null converts a bool kind itself only for
// a type without one, so "Y" written by Value reads back as true.
func TestScannerBoolKindRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, q := newDB(t)

	yes, err := q.PostFlagCreate(ctx, PostFlagNew().ReviewedSet(true).ApprovedSet(rt.Some(pay.YesNo(true))))
	if err != nil {
		t.Fatalf("PostFlagCreate: %v", err)
	}
	none, err := q.PostFlagCreate(ctx, PostFlagNew().ReviewedSet(false))
	if err != nil {
		t.Fatalf("PostFlagCreate: %v", err)
	}
	var reviewed, approved string
	if err := db.QueryRowContext(ctx, `SELECT reviewed, approved FROM post_flags WHERE id = ?`, yes.ID).Scan(&reviewed, &approved); err != nil {
		t.Fatal(err)
	}
	if reviewed != "Y" || approved != "Y" {
		t.Errorf("stored reviewed=%q approved=%q, want Y and Y written by YesNo.Value", reviewed, approved)
	}
	got, err := q.PostFlagGet(ctx, yes.ID)
	if err != nil {
		t.Fatalf("PostFlagGet: %v", err)
	}
	if !bool(got.Reviewed) || !got.Approved.Valid || !bool(got.Approved.V) {
		t.Errorf("PostFlagGet = %+v, want reviewed and approved true, read through YesNo.Scan", got)
	}
	got, err = q.PostFlagGet(ctx, none.ID)
	if err != nil {
		t.Fatalf("PostFlagGet: %v", err)
	}
	if bool(got.Reviewed) || got.Approved.Valid {
		t.Errorf("PostFlagGet = %+v, want reviewed false and approved NULL", got)
	}
}
