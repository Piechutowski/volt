// Nested round trips (spec §selects-over-groups): a select with a
// reverse relation nests each parent's children, in parent order and
// child primary-key order, against a real SQLite: no parent at all, a
// parent without children, and a thousand children.
package itest

import (
	"context"
	"testing"

	"github.com/Piechutowski/volt/nao/run"
)

func TestNestedResults(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()

	none, err := run.Run(ctx, db, UserCarts(404))
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("carts of no user = %v, %v; want an empty slice", none, err)
	}

	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@example.com").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("bob@example.com").NameSet("Bob")))
	if err != nil {
		t.Fatal(err)
	}
	alone, err := run.Run(ctx, db, UserCarts(bob.ID))
	if err != nil || len(alone) != 1 || alone[0].Name != "Bob" || alone[0].Orders == nil || len(alone[0].Orders) != 0 {
		t.Fatalf("carts of a user without orders = %+v, %v; want Bob with an empty slice", alone, err)
	}

	const many = 1000
	for i := 0; i < many; i++ {
		if _, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(float64(i+1)).FeeSet(1))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(bob.ID).TotalSet(7).FeeSet(1))); err != nil {
		t.Fatal(err)
	}
	all, err := run.Run(ctx, db, UserEveryone())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Name != "Ann" || all[1].Name != "Bob" {
		t.Fatalf("everyone = %d parents, want Ann then Bob", len(all))
	}
	if len(all[0].Orders) != many || len(all[1].Orders) != 1 || all[1].Orders[0].Total != 7 {
		t.Fatalf("Ann has %d orders and Bob %d; want %d and 1", len(all[0].Orders), len(all[1].Orders), many)
	}
	for i, o := range all[0].Orders {
		if o.UserID != ann.ID || o.Total != float64(i+1) {
			t.Fatalf("Ann's order %d = %+v; want her orders in primary-key order", i, o)
		}
	}
}
