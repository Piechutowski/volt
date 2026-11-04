package itest

import (
	"context"
	"errors"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func userTagsCreate(email string, tags ...string) rt.Effect[[]any] {
	return rt.Then(UserCreate(UserNew().EmailSet(email).NameSet("Ann")), func(u User) rt.Effect[[]any] {
		writes := make([]rt.AnyEffect, len(tags))
		for i, tag := range tags {
			writes[i] = UserTagCreate(UserTagNew().UserIDSet(u.ID).TagSet(tag))
		}
		return rt.Tx(writes...)
	})
}

func usersCount(t *testing.T, db run.DB, email string) int64 {
	t.Helper()
	n, err := run.Run(context.Background(), db, UserCount(UserEmail.Eq(email)))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func tagsCount(t *testing.T, db run.DB) int64 {
	t.Helper()
	n, err := run.Run(context.Background(), db, UserTagCount())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestAtomicWritesAParentAndItsChildrenTogether proves an Atomic over a
// Then (spec §generated-query-surface): the children's inserts carry
// the key the parent's insert assigned, and all of them commit in one
// transaction.
func TestAtomicWritesAParentAndItsChildrenTogether(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	seedUser(t, db, "first@example.com")
	out, err := run.Run(ctx, db, rt.Atomic(userTagsCreate("ann@example.com", "go", "sql")))
	if err != nil || len(out) != 2 {
		t.Fatalf("Atomic = %v, %v; want the two tags", out, err)
	}
	u, err := run.Run(ctx, db, UserGetByEmail("ann@example.com"))
	if err != nil || u.ID != 2 {
		t.Fatalf("the parent = %+v, %v; want it stored with the second key", u, err)
	}
	for i, tag := range []string{"go", "sql"} {
		got, ok := out[i].(UserTag)
		if !ok || got.UserID != u.ID || got.Tag != tag {
			t.Errorf("tag %d = %#v; want %q on user %d", i, out[i], tag, u.ID)
		}
	}
	if n := tagsCount(t, db); n != 2 {
		t.Errorf("%d tags stored, want 2", n)
	}
}

// TestAtomicChildFailureLeavesNoParent proves the rollback of an Atomic
// (spec §generated-query-surface): a child refused as a duplicate rolls
// the parent back with it, and the refusal comes back as the
// *rt.Invalid the error handler answers with 409.
func TestAtomicChildFailureLeavesNoParent(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	_, err := run.Run(ctx, db, rt.Atomic(userTagsCreate("bob@example.com", "go", "go")))
	inv := invalidOf(t, err, "unique", "user_id", "tag")
	if inv.Model != "UserTag" {
		t.Errorf("the refusal names %q; want UserTag", inv.Model)
	}
	if n := usersCount(t, db, "bob@example.com"); n != 0 {
		t.Errorf("%d parents stored after the child failed; want 0", n)
	}
	if n := tagsCount(t, db); n != 0 {
		t.Errorf("%d tags stored after the child failed; want 0", n)
	}
	if _, err := run.Run(ctx, db, rt.Atomic(rt.Then(UserGet(999), func(u User) rt.Effect[User] { return UserCreate(UserNew().EmailSet("never@example.com").NameSet("No")) }))); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("an Atomic over a miss = %v; want rt.ErrNotFound unwrapped", err)
	}
}

// TestThenOutsideAtomicIsNotAtomic proves what spec
// §generated-query-surface says of a Then on its own: its steps run one
// after another, each on its own, so the first step's write stays when
// the second fails.
func TestThenOutsideAtomicIsNotAtomic(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	_, err := run.Run(ctx, db, userTagsCreate("cy@example.com", "go", "go"))
	invalidOf(t, err, "unique", "user_id", "tag")
	if n := usersCount(t, db, "cy@example.com"); n != 1 {
		t.Errorf("%d parents stored after a Then outside an Atomic failed; want the first step's write left", n)
	}
	if n := tagsCount(t, db); n != 0 {
		t.Errorf("%d tags stored; want the Transaction next returned rolled back as a whole", n)
	}
	seedUser(t, db, "taken@example.com")
	_, err = run.Run(ctx, db, rt.Then(UserCreate(UserNew().EmailSet("dee@example.com").NameSet("Dee")), func(User) rt.Effect[User] {
		return UserCreate(UserNew().EmailSet("taken@example.com").NameSet("Dup"))
	}))
	invalidOf(t, err, "unique", "email")
	if n := usersCount(t, db, "dee@example.com"); n != 1 {
		t.Errorf("%d rows for the first step; want its write left", n)
	}
}

// TestThenInsideTransactionJoinsIt proves a Then that is a step of a
// Transaction runs every step on that transaction (spec
// §generated-query-surface): a failure inside it rolls back the steps
// before it, and a step after it sees what it wrote.
func TestThenInsideTransactionJoinsIt(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	_, err := run.Run(ctx, db, rt.Tx(
		UserCreate(UserNew().EmailSet("eve@example.com").NameSet("Eve")),
		userTagsCreate("fay@example.com", "go", "go"),
	))
	invalidOf(t, err, "unique", "user_id", "tag")
	if n := usersCount(t, db, "eve@example.com") + usersCount(t, db, "fay@example.com"); n != 0 {
		t.Errorf("%d users stored after the Then inside the Transaction failed; want 0", n)
	}
	out, err := run.Run(ctx, db, rt.Tx(userTagsCreate("gil@example.com", "go"), UserTagCount()))
	if err != nil || len(out) != 2 || out[1] != int64(1) {
		t.Fatalf("Tx with a Then step = %v, %v; want the tag counted inside the transaction", out, err)
	}
	_, err = run.Run(ctx, db, rt.Tx(userTagsCreate("hal@example.com", "go"), UserCreate(UserNew().EmailSet("gil@example.com").NameSet("Dup"))))
	invalidOf(t, err, "unique", "email")
	if n := usersCount(t, db, "hal@example.com"); n != 0 || tagsCount(t, db) != 1 {
		t.Errorf("a later step's failure left %d of the Then's parents and %d tags; want 0 and the earlier 1", n, tagsCount(t, db))
	}
}

// TestMapYieldsTheResultThroughF proves Map (spec
// §generated-query-surface): f of the inner effect's result, an error
// unchanged, and the parent row handed back after its children's writes.
func TestMapYieldsTheResultThroughF(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	id, err := run.Run(ctx, db, rt.Map(UserCreate(UserNew().EmailSet("ivy@example.com").NameSet("Ivy")), func(u User) int { return u.ID }))
	if err != nil || id != 1 {
		t.Fatalf("Map over a create = %d, %v; want the stored key", id, err)
	}
	some, err := run.Run(ctx, db, rt.Map(UserCount(), func(n int64) bool { return n > 0 }))
	if err != nil || !some {
		t.Errorf("Map over a count = %v, %v", some, err)
	}
	_, err = run.Run(ctx, db, rt.Map(UserCreate(UserNew().EmailSet("ivy@example.com").NameSet("Dup")), func(u User) int { return u.ID }))
	invalidOf(t, err, "unique", "email")
	parent, err := run.Run(ctx, db, rt.Atomic(rt.Then(UserCreate(UserNew().EmailSet("jo@example.com").NameSet("Jo")), func(u User) rt.Effect[User] {
		tags := []rt.AnyEffect{
			UserTagCreate(UserTagNew().UserIDSet(u.ID).TagSet("go")),
			UserTagCreate(UserTagNew().UserIDSet(u.ID).TagSet("sql")),
		}
		return rt.Map(rt.Tx(tags...), func([]any) User { return u })
	})))
	if err != nil || parent.Email != "jo@example.com" || parent.ID != 2 || tagsCount(t, db) != 2 {
		t.Errorf("the parent after its children = %+v, %v; want the stored parent with its two tags", parent, err)
	}
}

// TestNestedAtomicJoinsTheOuter proves an Atomic inside another opens
// no transaction of its own (spec §generated-query-surface): what it
// wrote rolls back when the outer one fails after it, and commits with
// the outer one otherwise.
func TestNestedAtomicJoinsTheOuter(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	seedUser(t, db, "taken@example.com")
	_, err := run.Run(ctx, db, rt.Atomic(rt.Then(rt.Atomic(UserCreate(UserNew().EmailSet("kim@example.com").NameSet("Kim"))), func(User) rt.Effect[User] {
		return UserCreate(UserNew().EmailSet("taken@example.com").NameSet("Dup"))
	})))
	invalidOf(t, err, "unique", "email")
	if n := usersCount(t, db, "kim@example.com"); n != 0 {
		t.Errorf("the inner Atomic's write survived the outer failure: %d rows", n)
	}
	if _, err := run.Run(ctx, db, rt.Atomic(rt.Atomic(userTagsCreate("lee@example.com", "go")))); err != nil {
		t.Fatal(err)
	}
	if n := usersCount(t, db, "lee@example.com"); n != 1 || tagsCount(t, db) != 1 {
		t.Errorf("nested Atomics committed %d parents and %d tags; want 1 and 1", n, tagsCount(t, db))
	}
}

// TestParallelInsideAtomic proves a Parallel inside an Atomic does what
// it does inside a Transaction (spec §generated-query-surface): its
// steps run on the one transaction, and the first error rolls back
// every step's writes.
func TestParallelInsideAtomic(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	out, err := run.Run(ctx, db, rt.Atomic(rt.All(userTagsCreate("max@example.com", "go"), userTagsCreate("ned@example.com", "sql"))))
	if err != nil || len(out) != 2 || tagsCount(t, db) != 2 {
		t.Fatalf("a Parallel inside an Atomic = %v, %v; want both parents and their tags", out, err)
	}
	_, err = run.Run(ctx, db, rt.Atomic(rt.All(userTagsCreate("oli@example.com", "go"), userTagsCreate("pat@example.com", "go", "go"))))
	invalidOf(t, err, "unique", "user_id", "tag")
	if n := usersCount(t, db, "oli@example.com") + usersCount(t, db, "pat@example.com"); n != 0 || tagsCount(t, db) != 2 {
		t.Errorf("a failed Parallel inside an Atomic left %d parents and %d tags; want 0 and the earlier 2", n, tagsCount(t, db))
	}
}
