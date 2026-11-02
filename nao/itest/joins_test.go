// Join round trips (spec §selects-over-groups, §relationships-ref): a
// select over a forward relation joins inner and reaches the referenced
// row's columns, one over a reverse relation joins left and reads NULL
// where no row refers back, against a real SQLite.
package itest

import (
	"context"
	"testing"

	"github.com/Piechutowski/volt/nao/run"
)

func TestJoinForwardRelationIsInner(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@example.com").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	for _, total := range []float64{5, 50} {
		if _, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(total).FeeSet(1))); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := run.Run(ctx, db, OrderPlaced(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Total != 50 || rows[0].UserName != "Ann" || rows[0].UserEmail != "ann@example.com" {
		t.Fatalf("OrderPlaced(10) = %+v, want the one order of 50 with Ann's name and email", rows)
	}
}

func TestJoinReverseRelationIsLeft(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@example.com").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("bob@example.com").NameSet("Bob"))); err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(7).FeeSet(1))); err != nil {
		t.Fatal(err)
	}
	rows, err := run.Run(ctx, db, UserBuyers())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("UserBuyers = %d rows, want Ann with her order and Bob without", len(rows))
	}
	if rows[0].Name != "Ann" || !rows[0].OrdersTotal.Valid || rows[0].OrdersTotal.V != 7 {
		t.Errorf("Ann's row = %+v, want her order's total", rows[0])
	}
	if rows[1].Name != "Bob" || rows[1].OrdersTotal.Valid {
		t.Errorf("Bob's row = %+v, want a NULL total from the left join", rows[1])
	}
}
