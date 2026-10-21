// Declared type round trips (spec §types, §predicates): a package-
// qualified Type reached through a from path is written, scanned back,
// compared with = and a literal, and bound as a list through its
// driver value.
package itest

import (
	"context"
	"testing"

	"github.com/Piechutowski/volt/nao/itest/pay"
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
// spells its storage type (spec §types rule 3) on a real SQLite: no
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
