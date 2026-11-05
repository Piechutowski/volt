package itest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

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

// TestParallelInsideAtomic proves a Parallel inside an Atomic or a
// Transaction runs its steps in order on the one transaction (spec
// §generated-query-surface, the runner): every step's writes commit
// together, and a step that fails among many rolls back every other
// step's writes, round after round, where steps run at once on the one
// connection let a sibling go on in autocommit after the failure
// interrupted the transaction.
func TestParallelInsideAtomic(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	out, err := run.Run(ctx, db, rt.Atomic(rt.All(userTagsCreate("max@example.com", "go"), userTagsCreate("ned@example.com", "sql"))))
	if err != nil || len(out) != 2 || tagsCount(t, db) != 2 {
		t.Fatalf("a Parallel inside an Atomic = %v, %v; want both parents and their tags", out, err)
	}
	seedUser(t, db, "taken@example.com")
	for _, wrap := range []struct {
		name string
		e    func(...rt.AnyEffect) rt.AnyEffect
	}{
		{"an Atomic", func(steps ...rt.AnyEffect) rt.AnyEffect { return rt.Atomic(rt.All(steps...)) }},
		{"a Transaction", func(steps ...rt.AnyEffect) rt.AnyEffect { return rt.Tx(rt.All(steps...)) }},
	} {
		for round := range 50 {
			steps := make([]rt.AnyEffect, 16)
			for i := range steps {
				steps[i] = userTagsCreate(fmt.Sprintf("r%d-%d@example.com", round, i), "go", "sql")
			}
			steps[9] = userTagsCreate(fmt.Sprintf("r%d-dup@example.com", round), "go", "go")
			steps[12] = UserCreate(UserNew().EmailSet("taken@example.com").NameSet("Dup"))
			_, err := run.Perform(ctx, db, wrap.e(steps...))
			invalidOf(t, err, "unique", "user_id", "tag")
			if n, err := run.Run(ctx, db, UserCount()); err != nil || n != 3 || tagsCount(t, db) != 2 {
				t.Fatalf("round %d: a failed Parallel inside %s left %d users and %d tags, %v; want the 3 and 2 from before", round, wrap.name, n, tagsCount(t, db), err)
			}
		}
	}
}

// TestAtomicFailRollsBackWhatItWrote proves rt.Fail and rt.Pure in a
// next (spec §generated-query-surface): a next that refuses what it
// read rolls back what the Atomic wrote and fails with its own error,
// unwrapped, and a next with nothing left to perform commits.
func TestAtomicFailRollsBackWhatItWrote(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	refused := errors.New("too many tags")
	tagged := func(email string, most int64) rt.Effect[User] {
		return rt.Atomic(rt.Then(UserCreate(UserNew().EmailSet(email).NameSet("Ann")), func(u User) rt.Effect[User] {
			return rt.Then(UserTagCreate(UserTagNew().UserIDSet(u.ID).TagSet("go")), func(UserTag) rt.Effect[User] {
				return rt.Then(UserTagCount(UserTagUserID.Eq(u.ID)), func(n int64) rt.Effect[User] {
					if n > most {
						return rt.Fail[User](refused)
					}
					return rt.Pure(u)
				})
			})
		}))
	}
	if _, err := run.Run(ctx, db, tagged("refused@example.com", 0)); err != refused {
		t.Fatalf("an Atomic whose next refuses = %v; want its error unchanged", err)
	}
	if n := usersCount(t, db, "refused@example.com"); n != 0 || tagsCount(t, db) != 0 {
		t.Errorf("a refused Atomic left %d users and %d tags; want none", n, tagsCount(t, db))
	}
	u, err := run.Run(ctx, db, tagged("kept@example.com", 1))
	if err != nil || u.Email != "kept@example.com" || usersCount(t, db, "kept@example.com") != 1 || tagsCount(t, db) != 1 {
		t.Errorf("an Atomic whose next yields its row = %+v, %v; want the user and its tag committed", u, err)
	}
}

func firstOf(ids []int) int { return ids[0] }

// TestParallelPanicInsideAtomic proves on SQLite what spec
// §generated-query-surface says of a panic: a next that panics inside a
// Parallel inside an Atomic raises its panic on the goroutine that
// performs the effect, never ending the process, the transaction rolls
// back with every sibling's writes, and the one connection is free for
// the next statement; outside a transaction the panic carries the
// stack of the step's goroutine.
func TestParallelPanicInsideAtomic(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	broken := rt.Then(UserCreate(UserNew().EmailSet("broken@example.com").NameSet("Bo")), func(u User) rt.Effect[User] {
		return UserGet(firstOf(nil))
	})
	steps := []rt.AnyEffect{userTagsCreate("sibling@example.com", "go")}
	for i := range 8 {
		steps = append(steps, userTagsCreate(fmt.Sprintf("s%d@example.com", i), "go", "sql"))
	}
	steps = append(steps[:4], append([]rt.AnyEffect{broken}, steps[4:]...)...)
	raised := panicked(func() { _, _ = run.Run(ctx, db, rt.Atomic(rt.All(steps...))) })
	if _, ok := raised.(runtime.Error); !ok {
		t.Fatalf("a panicking next inside a Parallel inside an Atomic raised %v; want the runtime error itself on this goroutine", raised)
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n, err := run.Run(bounded, db, UserCount())
	if err != nil || n != 0 || tagsCount(t, db) != 0 {
		t.Errorf("after the panic the database holds %d users and %d tags, %v; want the transaction rolled back and the connection free", n, tagsCount(t, db), err)
	}
	reading := rt.Then(UserCount(), func(int64) rt.Effect[User] { return UserGet(firstOf(nil)) })
	err, ok := panicked(func() { _, _ = run.Run(ctx, db, rt.All(UserCount(), reading)) }).(error)
	var re runtime.Error
	if !ok || !errors.As(err, &re) || !strings.Contains(err.Error(), "itest.firstOf") {
		t.Errorf("a panicking next inside a Parallel outside a transaction raised %v; want the runtime error with its goroutine's stack", err)
	}
}

func panicked(f func()) (v any) {
	defer func() { v = recover() }()
	f()
	return nil
}

type rogue struct{ rt.Exec }

func (rogue) Perform(ctx context.Context, x rt.Executor) (int64, error) {
	return x.Exec(ctx, "DELETE FROM users", nil)
}

type batch struct{ rt.Transaction }

type fanout struct{ rt.Parallel }

type wrapped struct{ rt.Effect[int64] }

// TestEffectSealedAgainstEmbedding proves the seal of spec
// §generated-query-surface: a type of another package that embeds an
// effect kind, a Transaction and a Parallel among them, or an Effect is
// the effect it embeds, performing, describing and comparing as that
// one, so a Perform of its own never holds the executor; one that
// embeds a nil Effect panics, as a method of a nil interface does.
func TestEffectSealedAgainstEmbedding(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	seedUser(t, db, "stays@example.com")
	e := rogue{rt.Exec{SQL: "UPDATE users SET name = name WHERE id = 0"}}
	if n, err := run.Run(ctx, db, rt.Atomic[int64](e)); err != nil || n != 0 {
		t.Fatalf("the embedding type = %d, %v; want the embedded Exec's 0 rows", n, err)
	}
	if n := usersCount(t, db, "stays@example.com"); n != 1 {
		t.Errorf("%d users after the embedding type ran; want its own Perform never called", n)
	}
	touch := rt.Exec{SQL: "UPDATE users SET name = 'Ann' WHERE id = 1"}
	drop := rt.Exec{SQL: "DELETE FROM users WHERE id = 0"}
	for _, c := range []struct {
		name              string
		one, other, inner rt.AnyEffect
	}{
		{"an Exec", e, rogue{drop}, e.Exec},
		{"a Transaction", batch{rt.Tx(touch)}, batch{rt.Tx(drop)}, rt.Tx(touch)},
		{"a Parallel", fanout{rt.All(touch)}, fanout{rt.All(drop)}, rt.All(touch)},
		{"an Effect", wrapped{touch}, wrapped{drop}, touch},
	} {
		if rt.Describe(c.one) != rt.Describe(c.inner) || !rt.Same(c.one, c.inner) || rt.Same(c.one, c.other) {
			t.Errorf("a type embedding %s describes as %q beside %q; want it described and compared as the effect it embeds", c.name, rt.Describe(c.one), rt.Describe(c.other))
		}
	}
	_, err := run.Run(ctx, db, batch{rt.Tx(UserCreate(UserNew().EmailSet("batch@example.com").NameSet("Bat")), UserCreate(UserNew().EmailSet("stays@example.com").NameSet("Dup")))})
	invalidOf(t, err, "unique", "email")
	if n := usersCount(t, db, "batch@example.com"); n != 0 {
		t.Errorf("a type embedding a Transaction left %d rows of its failed steps; want it performed as the Transaction", n)
	}
	if n, err := run.Run(ctx, db, wrapped{UserCount()}); err != nil || n != 1 {
		t.Errorf("a type embedding an Effect = %d, %v; want the count it embeds", n, err)
	}
	if panicked(func() { _, _ = run.Run(ctx, db, wrapped{}) }) == nil || panicked(func() { rt.Describe(wrapped{}) }) == nil {
		t.Error("a type embedding a nil Effect performed or described; want the panic of a nil interface's method")
	}
}
