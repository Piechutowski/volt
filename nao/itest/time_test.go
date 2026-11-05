// Date/time round trips (spec §go-types): every time.Time is stored and
// bound as SQLite's own datetime text in UTC, so a value read back
// matches the rows CURRENT_TIMESTAMP wrote, text order is instant order,
// and SQLite's date functions read the column.
package itest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func usersOfOneSecond(t *testing.T, db run.DB) []User {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Pool().Exec(`INSERT INTO users (email, name) VALUES ('a@example.com', 'Ann'), ('b@example.com', 'Bob'), ('c@example.com', 'Cid')`); err != nil {
		t.Fatal(err)
	}
	users, err := run.Run(ctx, db, UserQuery(UserOrderBy(UserID.Asc())))
	if err != nil || len(users) != 3 {
		t.Fatalf("users = %+v, %v", users, err)
	}
	return users
}

func TestTimeDefaultsMatchTheirReadBack(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	users := usersOfOneSecond(t, db)
	at := users[0].CreatedAt
	if at.Location() != time.UTC {
		t.Errorf("created_at reads in %v, want UTC", at.Location())
	}
	for name, p := range map[string]rt.Pred[User]{
		"Eq":     UserCreatedAt.Eq(at),
		"Ge":     UserCreatedAt.Ge(at),
		"Le":     UserCreatedAt.Le(at),
		"In":     UserCreatedAt.In(at),
		"InList": UserCreatedAt.InList([]time.Time{at, at.Add(time.Hour)}),
	} {
		if n, err := run.Run(ctx, db, UserCount(p)); err != nil || n != 3 {
			t.Errorf("%s(read-back created_at) counts %d, %v; want all 3", name, n, err)
		}
	}
	if n, err := run.Run(ctx, db, UserCount(UserCreatedAt.Gt(at))); err != nil || n != 0 {
		t.Errorf("Gt counts %d, %v; want 0", n, err)
	}
	listed, err := run.Run(ctx, db, UserStamped([]time.Time{at}))
	if err != nil || len(listed) != 3 {
		t.Errorf("a time.Time list parameter finds %d, %v; want all 3", len(listed), err)
	}
	page, err := run.Run(ctx, db, UserQuery(UserOrderBy(UserCreatedAt.Asc(), UserID.Asc()), UserAfter(at, users[0].ID)))
	if err != nil || len(page) != 2 || page[0].ID != users[1].ID || page[1].ID != users[2].ID {
		t.Errorf("keyset after the first row = %+v, %v; want the other two rows of its second", page, err)
	}
}

func TestTimeOrdersFractionalAndWholeSeconds(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	offsets := []time.Duration{time.Second, 500 * time.Millisecond, 0, 123456789, 50 * time.Millisecond, 2 * time.Second}
	for i, d := range offsets {
		email := string(rune('a'+i)) + "@example.com"
		if _, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet(email).NameSet("Ann").CreatedAtSet(base.Add(d)))); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := run.Run(ctx, db, UserQuery(UserOrderBy(UserCreatedAt.Asc())))
	if err != nil || len(rows) != len(offsets) {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	for i := 1; i < len(rows); i++ {
		if !rows[i-1].CreatedAt.Before(rows[i].CreatedAt) {
			t.Errorf("order: %v is not before %v", rows[i-1].CreatedAt, rows[i].CreatedAt)
		}
	}
	if !rows[0].CreatedAt.Equal(base) || !rows[len(rows)-1].CreatedAt.Equal(base.Add(2*time.Second)) {
		t.Errorf("first and last = %v, %v", rows[0].CreatedAt, rows[len(rows)-1].CreatedAt)
	}
	if n, err := run.Run(ctx, db, UserCount(UserCreatedAt.Gt(base.Add(100*time.Millisecond)), UserCreatedAt.Lt(base.Add(time.Second)))); err != nil || n != 2 {
		t.Errorf("between 0.1s and 1s counts %d, %v; want 0.123456789s and 0.5s", n, err)
	}
	var text string
	if err := db.Pool().QueryRow(`SELECT group_concat(CAST(created_at AS TEXT), '|') FROM (SELECT created_at FROM users ORDER BY created_at)`).Scan(&text); err != nil {
		t.Fatal(err)
	}
	want := "2026-03-01 12:00:00|2026-03-01 12:00:00.05|2026-03-01 12:00:00.123456789|2026-03-01 12:00:00.5|2026-03-01 12:00:01|2026-03-01 12:00:02"
	if text != want {
		t.Errorf("stored text = %s\nwant          %s", text, want)
	}
}

func TestTimeNonUTCIsStoredCanonical(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	at := time.Date(2026, 3, 1, 14, 30, 0, 0, time.FixedZone("plus2", 2*3600))
	u, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("a@example.com").NameSet("Ann").CreatedAtSet(at)))
	if err != nil {
		t.Fatal(err)
	}
	if !u.CreatedAt.Equal(at) || u.CreatedAt.Location() != time.UTC {
		t.Errorf("read back %v, want %v in UTC", u.CreatedAt, at)
	}
	if _, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(u.ID).TotalSet(1).FeeSet(1).PlacedAtSet(rt.Some(at)))); err != nil {
		t.Fatal(err)
	}
	var created, placed string
	if err := db.Pool().QueryRow(`SELECT CAST(u.created_at AS TEXT), CAST(o.placed_at AS TEXT) FROM users u JOIN orders o ON o.user_id = u.id`).Scan(&created, &placed); err != nil {
		t.Fatal(err)
	}
	if created != "2026-03-01 12:30:00" || placed != "2026-03-01 12:30:00" {
		t.Errorf("stored %q and %q, want 2026-03-01 12:30:00 for both", created, placed)
	}
	if n, err := run.Run(ctx, db, UserCount(UserCreatedAt.Eq(at.UTC()), UserCreatedAt.Eq(at))); err != nil || n != 1 {
		t.Errorf("Eq in either location counts %d, %v; want 1", n, err)
	}
	if _, err := run.Run(ctx, db, UserCount(UserCreatedAt.Eq(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)))); err == nil {
		t.Error("a year past 9999 bound without an error")
	}
}

func TestTimeMinMaxAndDateFunctions(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	early := time.Date(2026, 3, 1, 23, 59, 59, 900_000_000, time.UTC)
	late := time.Date(2026, 3, 2, 0, 0, 0, 0, time.FixedZone("minus5", -5*3600))
	for i, at := range []time.Time{late, early} {
		email := string(rune('a'+i)) + "@example.com"
		if _, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet(email).NameSet("Ann").CreatedAtSet(at))); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := run.Run(ctx, db, UserJoined())
	if err != nil || len(rows) != 1 {
		t.Fatalf("UserJoined = %+v, %v", rows, err)
	}
	if !rows[0].First.Equal(early) || !rows[0].Last.Equal(late) || rows[0].First.Location() != time.UTC {
		t.Errorf("min and max = %v and %v, want %v and %v in UTC", rows[0].First, rows[0].Last, early, late.UTC())
	}
	if n, err := run.Run(ctx, db, UserCount(rt.Raw[User](`date("created_at") = ?`, "2026-03-01"))); err != nil || n != 1 {
		t.Errorf("date(created_at) = 2026-03-01 counts %d, %v; want 1", n, err)
	}
	if n, err := run.Run(ctx, db, UserCount(rt.Raw[User](`"created_at" < datetime(?, '+1 day')`, early))); err != nil || n != 2 {
		t.Errorf("created_at < datetime(early, +1 day) counts %d, %v; want 2", n, err)
	}
}

// TestTimeReadsOtherForms proves Volt reads a row older code stored with
// an offset, and unix seconds or milliseconds, as their instants in UTC,
// a column and min of it alike, but compares only the canonical text
// (spec §go-types).
func TestTimeReadsOtherForms(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	if _, err := db.Pool().Exec(`INSERT INTO users (email, name, created_at) VALUES ('a@example.com', 'Ann', '2026-03-01 14:00:00+02:00'), ('b@example.com', 'Bob', 1772366400)`); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	users, err := run.Run(ctx, db, UserQuery(UserOrderBy(UserID.Asc())))
	if err != nil || len(users) != 2 {
		t.Fatalf("users = %+v, %v", users, err)
	}
	for _, u := range users {
		if !u.CreatedAt.Equal(want) || u.CreatedAt.Location() != time.UTC {
			t.Errorf("%s reads %v, want %v in UTC", u.Name, u.CreatedAt, want)
		}
	}
	if n, err := run.Run(ctx, db, UserCount(UserCreatedAt.Eq(want))); err != nil || n != 0 {
		t.Errorf("Eq counts %d, %v; an offset row and an integer row are not canonical text, so neither matches", n, err)
	}
	rows, err := run.Run(ctx, db, UserJoined())
	if err != nil || len(rows) != 2 {
		t.Fatalf("UserJoined = %+v, %v", rows, err)
	}
	for _, r := range rows {
		if !r.First.Equal(want) {
			t.Errorf("min of %s = %v, want %v", r.Name, r.First, want)
		}
	}
	if _, err := db.Pool().Exec(`INSERT INTO users (email, name, created_at) VALUES ('c@example.com', 'Cid', 1772366400123)`); err != nil {
		t.Fatal(err)
	}
	cid, err := run.Run(ctx, db, UserGetByEmail("c@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if ms := want.Add(123 * time.Millisecond); !cid.CreatedAt.Equal(ms) {
		t.Errorf("a column of unix milliseconds reads %v, want %v", cid.CreatedAt, ms)
	}
	rows, err = run.Run(ctx, db, UserJoined())
	if err != nil || len(rows) != 3 || !rows[2].First.Equal(cid.CreatedAt) {
		t.Errorf("min of unix milliseconds = %+v, %v; want %v, as the column reads it", rows, err, cid.CreatedAt)
	}
	if _, err := db.Pool().Exec(`UPDATE users SET created_at = 'yesterday' WHERE name = 'Ann'`); err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, UserJoined()); err == nil {
		t.Error("min of unparsable text read without an error")
	}
}

// TestTimeKeys proves a date/time key works on every key path: Get,
// GetMany, Update, Delete and a with, whose parent keys travel as one
// JSON array of canonical text (spec §go-types, §selects-over-groups).
func TestTimeKeys(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	plus2 := time.FixedZone("plus2", 2*3600)
	keys := []time.Time{
		time.Date(2026, 3, 1, 14, 0, 0, 0, plus2),
		time.Date(2026, 3, 1, 12, 0, 0, 250_000_000, time.UTC),
		time.Date(2026, 3, 1, 12, 0, 1, 0, time.UTC),
	}
	for i, k := range keys {
		if _, err := run.Run(ctx, db, BatchCreate(BatchNew().OpenedAtSet(k).LabelSet(string(rune('a'+i))))); err != nil {
			t.Fatal(err)
		}
		for j := 0; j <= i; j++ {
			if _, err := run.Run(ctx, db, BatchItemCreate(BatchItemNew().OpenedAtSet(k))); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := run.Run(ctx, db, BatchGet(keys[0]))
	if err != nil || got.Label != "a" {
		t.Errorf("Get(+02:00 key) = %+v, %v", got, err)
	}
	many, err := run.Run(ctx, db, BatchGetMany([]time.Time{keys[0].UTC(), keys[2]}))
	if err != nil || len(many) != 2 {
		t.Errorf("GetMany = %+v, %v", many, err)
	}
	second, err := run.Run(ctx, db, BatchGet(keys[1]))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, BatchUpdate(second.Changeset().LabelSet("b2"))); err != nil {
		t.Errorf("Update by a fractional key: %v", err)
	}
	trail, err := run.Run(ctx, db, BatchTrail())
	if err != nil || len(trail) != 3 {
		t.Fatalf("BatchTrail = %+v, %v", trail, err)
	}
	for i, b := range trail {
		if !b.OpenedAt.Equal(keys[i]) || len(b.BatchItems) != i+1 {
			t.Errorf("batch %d = %v with %d items, want %v with %d", i, b.OpenedAt, len(b.BatchItems), keys[i], i+1)
		}
	}
	if trail[1].Label != "b2" {
		t.Errorf("updated label = %q", trail[1].Label)
	}
	if n, err := run.Run(ctx, db, BatchItemDeleteWhere(BatchItemOpenedAt.Eq(keys[2]))); err != nil || n != 3 {
		t.Errorf("DeleteWhere items of the last key = %d, %v", n, err)
	}
	if n, err := run.Run(ctx, db, BatchDelete(keys[2])); err != nil || n != 1 {
		t.Errorf("Delete(last key) = %d, %v", n, err)
	}
}

// TestTimeOutsideTheYearsIsAnError proves a time past 9999 in UTC, which
// RFC 3339 query input such as 9999-12-31T23:00:00-05:00 parses to, fails
// its statement with an error on every bind path, list parameters, InList
// and the keys of a with included, and never panics (spec §go-types).
func TestTimeOutsideTheYearsIsAnError(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	at, err := time.Parse(time.RFC3339, "9999-12-31T23:00:00-05:00")
	if err != nil {
		t.Fatal(err)
	}
	for name, q := range map[string]rt.Query[User]{
		"In":     UserQuery(UserCreatedAt.In(at)),
		"InList": UserQuery(UserCreatedAt.InList([]time.Time{at})),
		"list":   UserStamped([]time.Time{at}),
	} {
		if _, err := run.Run(ctx, db, q); err == nil || !strings.Contains(err.Error(), "0000 to 9999") {
			t.Errorf("%s over year 10000 = %v, want the years error", name, err)
		}
	}
	if _, err := db.Pool().Exec(`INSERT INTO batches (opened_at) VALUES (253402300800)`); err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, BatchTrail()); err == nil || !strings.Contains(err.Error(), "0000 to 9999") {
		t.Errorf("a with over a stored key of year 10000 = %v, want the years error", err)
	}
}

// TestTimeOffsetKeysAreOutOfReach proves the limit §go-types states for
// rows older code stored with an offset: a non-key column is repaired by
// writing its read-back value, but the generated key paths bind the
// canonical text and do not reach a row whose key is in another form.
func TestTimeOffsetKeysAreOutOfReach(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	if _, err := db.Pool().Exec(`INSERT INTO batches (opened_at, label) VALUES ('2026-03-01 14:00:00+02:00', 'old')`); err != nil {
		t.Fatal(err)
	}
	listed, err := run.Run(ctx, db, BatchList())
	if err != nil || len(listed) != 1 {
		t.Fatalf("BatchList = %+v, %v", listed, err)
	}
	if _, err := run.Run(ctx, db, BatchGet(listed[0].OpenedAt)); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("Get(read-back offset key) = %v, want no row", err)
	}
	if _, err := db.Pool().Exec(`INSERT INTO users (email, name, created_at) VALUES ('a@example.com', 'Ann', '2026-03-01 14:00:00+02:00')`); err != nil {
		t.Fatal(err)
	}
	ann, err := run.Run(ctx, db, UserGetByEmail("a@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, UserUpdate(ann.Changeset().CreatedAtSet(ann.CreatedAt))); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.Pool().QueryRow(`SELECT CAST(created_at AS TEXT) FROM users`).Scan(&stored); err != nil || stored != "2026-03-01 12:00:00" {
		t.Errorf("a rewritten non-key column holds %q, %v; want the stored form", stored, err)
	}
}
