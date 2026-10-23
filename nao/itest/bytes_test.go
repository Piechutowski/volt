// Bytes on real SQLite (spec §columns, §nullability): a []byte
// column without null is bytes and nil writes empty bytes on every bind
// path, one with null is rt.Null[[]byte] and NULL is only Valid false,
// reads never hand back nil for empty bytes, and JSON and GOB carry both
// kinds. The read tests run on mattn/go-sqlite3 and on the same driver
// behind a shim that reads empty bytes as a nil []byte, as
// modernc.org/sqlite does.
package itest

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/gob"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
)

type nilBlobDriver struct{ inner driver.Driver }

func (d nilBlobDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return nilBlobConn{c}, nil
}

type nilBlobConn struct{ inner driver.Conn }

func (c nilBlobConn) Prepare(query string) (driver.Stmt, error) {
	s, err := c.inner.Prepare(query)
	if err != nil {
		return nil, err
	}
	return nilBlobStmt{s}, nil
}

func (c nilBlobConn) Close() error { return c.inner.Close() }

func (c nilBlobConn) Begin() (driver.Tx, error) {
	return c.inner.(driver.ConnBeginTx).BeginTx(context.Background(), driver.TxOptions{})
}

func (c nilBlobConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.inner.(driver.ExecerContext).ExecContext(ctx, query, args)
}

type nilBlobStmt struct{ inner driver.Stmt }

func (s nilBlobStmt) Close() error  { return s.inner.Close() }
func (s nilBlobStmt) NumInput() int { return s.inner.NumInput() }

func (s nilBlobStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.inner.Exec(args)
}

func (s nilBlobStmt) Query(args []driver.Value) (driver.Rows, error) {
	r, err := s.inner.Query(args)
	if err != nil {
		return nil, err
	}
	return nilBlobRows{r}, nil
}

type nilBlobRows struct{ inner driver.Rows }

func (r nilBlobRows) Columns() []string { return r.inner.Columns() }
func (r nilBlobRows) Close() error      { return r.inner.Close() }

func (r nilBlobRows) Next(dest []driver.Value) error {
	if err := r.inner.Next(dest); err != nil {
		return err
	}
	for i, v := range dest {
		if b, ok := v.([]byte); ok && len(b) == 0 {
			dest[i] = []byte(nil)
		}
	}
	return nil
}

func init() {
	db, err := sql.Open("sqlite3", "")
	if err != nil {
		panic(err)
	}
	sql.Register("sqlite3_nilblob", nilBlobDriver{db.Driver()})
	db.Close()
}

var bytesDrivers = []string{"sqlite3", "sqlite3_nilblob"}

func bytesDB(t *testing.T, driverName string) (*sql.DB, *Queries) {
	t.Helper()
	db, err := rt.Open(driverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ddl, err := os.ReadFile("nao_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(ddl)); err != nil {
		t.Fatalf("generated DDL rejected by SQLite: %v", err)
	}
	return db, New(db)
}

func stored(t *testing.T, db *sql.DB, table, col, where string, args ...any) (string, int) {
	t.Helper()
	var typ string
	var n sql.NullInt64
	q := "SELECT typeof(" + col + "), length(" + col + ") FROM " + table + " WHERE " + where
	if err := db.QueryRow(q, args...).Scan(&typ, &n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return typ, int(n.Int64)
}

func emptyBytes(t *testing.T, what string, b []byte) {
	t.Helper()
	if b == nil || len(b) != 0 {
		t.Errorf("%s = %#v; want non-nil empty bytes", what, b)
	}
}

func emptyBlob(t *testing.T, db *sql.DB, table, col, where string, args ...any) {
	t.Helper()
	if typ, n := stored(t, db, table, col, where, args...); typ != "blob" || n != 0 {
		t.Errorf("%s.%s where %s stored as %s length %d; want blob length 0", table, col, where, typ, n)
	}
}

func TestBytesNotNullNilWritesEmptyBytes(t *testing.T) {
	for _, drv := range bytesDrivers {
		t.Run(drv, func(t *testing.T) {
			ctx := context.Background()
			db, q := bytesDB(t, drv)

			a, err := q.AttachmentCreate(ctx, AttachmentNew().NameSet("a").BodySet(nil).DigestSet([]byte("d")))
			if err != nil {
				t.Fatalf("Create with a nil NOT NULL []byte: %v", err)
			}
			emptyBlob(t, db, "attachments", "body", "id = ?", a.ID)
			emptyBytes(t, "Create's returned body", a.Body)
			var raw []byte
			if err := db.QueryRow("SELECT body FROM attachments WHERE id = ?", a.ID).Scan(&raw); err != nil || (raw == nil) != (drv == "sqlite3_nilblob") {
				t.Fatalf("%s hands X'' to a plain *[]byte as %#v (%v); the shim reads nil, mattn non-nil", drv, raw, err)
			}

			got, err := q.AttachmentGet(ctx, a.ID)
			if err != nil {
				t.Fatal(err)
			}
			emptyBytes(t, "Get body", got.Body)
			list, err := q.AttachmentList(ctx)
			if err != nil || len(list) != 1 {
				t.Fatalf("List = %v, %v", list, err)
			}
			emptyBytes(t, "List body", list[0].Body)
			named, err := q.AttachmentNamed(ctx, "a")
			if err != nil || len(named) != 1 {
				t.Fatalf("AttachmentNamed = %v, %v", named, err)
			}
			emptyBytes(t, "select body", named[0].Body)
			slim, err := q.AttachmentSlim(ctx, "a")
			if err != nil || len(slim) != 1 {
				t.Fatalf("AttachmentSlim = %v, %v", slim, err)
			}
			emptyBytes(t, "projected select body", slim[0].Body)
			heavy, err := q.AttachmentHeavy(ctx, "a")
			if err != nil || len(heavy) != 1 || string(heavy[0].Digest) != "d" {
				t.Fatalf("AttachmentHeavy = %v, %v", heavy, err)
			}
			emptyBytes(t, "derived select body", heavy[0].Body)
			dyn, err := q.AttachmentQuery(ctx, AttachmentID.Eq(a.ID))
			if err != nil || len(dyn) != 1 {
				t.Fatalf("Query = %v, %v", dyn, err)
			}
			emptyBytes(t, "Query body", dyn[0].Body)

			again, err := q.AttachmentUpdate(ctx, got.Changeset().From(got))
			if err != nil {
				t.Fatalf("writing back the row read is refused: %v", err)
			}
			emptyBytes(t, "Update of the row read", again.Body)

			upd, err := q.AttachmentUpdate(ctx, a.Changeset().BodySet([]byte("x")))
			if err != nil || string(upd.Body) != "x" {
				t.Fatalf("Update body x = %+v, %v", upd, err)
			}
			upd, err = q.AttachmentUpdate(ctx, upd.Changeset().BodySet(nil))
			if err != nil {
				t.Fatalf("Update with a nil NOT NULL []byte: %v", err)
			}
			emptyBlob(t, db, "attachments", "body", "id = ?", a.ID)
			emptyBytes(t, "Update's returned body", upd.Body)

			from, err := q.AttachmentCreate(ctx, AttachmentNew().From(Attachment{Name: "b", Digest: []byte("d")}))
			if err != nil {
				t.Fatalf("From a row whose body is nil: %v", err)
			}
			emptyBlob(t, db, "attachments", "body", "id = ?", from.ID)

			_, err = q.AttachmentCreate(ctx, AttachmentNew().NameSet("c").DigestSet([]byte("d")))
			invalidOf(t, err, "not_null", "body")
		})
	}
}

func TestBytesNullableRoundTrip(t *testing.T) {
	for _, drv := range bytesDrivers {
		t.Run(drv, func(t *testing.T) {
			ctx := context.Background()
			db, q := bytesDB(t, drv)
			base := AttachmentNew().BodySet([]byte("b")).DigestSet([]byte("d"))

			cases := []struct {
				name  string
				cs    AttachmentChangeset
				typ   string
				valid bool
				data  string
			}{
				{"unset", base.NameSet("unset"), "null", false, ""},
				{"null", base.NameSet("null").ThumbSet(rt.Null[[]byte]{}), "null", false, ""},
				{"valid nil", base.NameSet("valid nil").ThumbSet(rt.Some([]byte(nil))), "blob", true, ""},
				{"valid empty", base.NameSet("valid empty").ThumbSet(rt.Some([]byte{})), "blob", true, ""},
				{"data", base.NameSet("data").ThumbSet(rt.Some([]byte("png"))), "blob", true, "png"},
			}
			for _, c := range cases {
				a, err := q.AttachmentCreate(ctx, c.cs)
				if err != nil {
					t.Fatalf("%s: Create: %v", c.name, err)
				}
				if typ, n := stored(t, db, "attachments", "thumb", "id = ?", a.ID); typ != c.typ || n != len(c.data) {
					t.Errorf("%s: thumb stored as %s length %d; want %s length %d", c.name, typ, n, c.typ, len(c.data))
				}
				got, err := q.AttachmentGet(ctx, a.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.Thumb.Valid != c.valid || string(got.Thumb.V) != c.data {
					t.Errorf("%s: read back %#v", c.name, got.Thumb)
				}
				if c.valid && got.Thumb.V == nil {
					t.Errorf("%s: a valid thumb read back as nil; want non-nil bytes", c.name)
				}
				slim, err := q.AttachmentSlim(ctx, c.name)
				if err != nil || len(slim) != 1 {
					t.Fatalf("%s: AttachmentSlim = %v, %v", c.name, slim, err)
				}
				heavy, err := q.AttachmentHeavy(ctx, c.name)
				if err != nil || len(heavy) != 1 {
					t.Fatalf("%s: AttachmentHeavy = %v, %v", c.name, heavy, err)
				}
				for _, sel := range []struct {
					what  string
					thumb rt.Null[[]byte]
				}{{"projected", slim[0].Thumb}, {"derived", heavy[0].Thumb}} {
					if sel.thumb.Valid != c.valid || string(sel.thumb.V) != c.data || (c.valid && sel.thumb.V == nil) {
						t.Errorf("%s: %s select thumb = %#v", c.name, sel.what, sel.thumb)
					}
				}
			}

			a, err := q.AttachmentGetByName(ctx, "data")
			if err != nil {
				t.Fatal(err)
			}
			n, err := q.AttachmentUpdateWhere(ctx, AttachmentSet(AttachmentThumb.Set(nil)), AttachmentID.Eq(a.ID))
			if err != nil || n != 1 {
				t.Fatalf("UpdateWhere thumb Set(nil) = %d, %v", n, err)
			}
			emptyBlob(t, db, "attachments", "thumb", "id = ?", a.ID)
			n, err = q.AttachmentUpdateWhere(ctx, AttachmentSet(AttachmentThumb.SetNull()), AttachmentID.Eq(a.ID))
			if err != nil || n != 1 {
				t.Fatalf("UpdateWhere thumb SetNull = %d, %v", n, err)
			}
			if typ, _ := stored(t, db, "attachments", "thumb", "id = ?", a.ID); typ != "null" {
				t.Errorf("SetNull stored %s; want null", typ)
			}
		})
	}
}

type hashKey []byte

func (h hashKey) Value() (driver.Value, error) { return []byte(h), nil }

func TestBytesKeyPaths(t *testing.T) {
	ctx := context.Background()
	db, q := bytesDB(t, "sqlite3")

	if _, err := q.BlobCreate(ctx, BlobNew().HashSet(nil).TagSet(nil)); err != nil {
		t.Fatalf("Create with nil []byte key and unique column: %v", err)
	}
	if _, err := q.BlobCreate(ctx, BlobNew().HashSet([]byte("h")).TagSet([]byte("t")).SizeSet(1)); err != nil {
		t.Fatal(err)
	}
	emptyBlob(t, db, "blobs", "hash", "size = 0")
	emptyBlob(t, db, "blobs", "tag", "size = 0")

	for _, key := range [][]byte{nil, {}} {
		if b, err := q.BlobGet(ctx, key); err != nil || b.Size != 0 {
			t.Errorf("Get(%#v) = %+v, %v; want the row keyed by empty bytes", key, b, err)
		}
		if b, err := q.BlobGetByTag(ctx, key); err != nil || b.Size != 0 {
			t.Errorf("GetByTag(%#v) = %+v, %v; want the row tagged by empty bytes", key, b, err)
		}
	}
	many, err := q.BlobGetMany(ctx, [][]byte{nil, []byte("h")})
	if err != nil || len(many) != 2 {
		t.Errorf("GetMany(nil, h) = %+v, %v; want both rows", many, err)
	}

	if b, err := q.BlobUpdate(ctx, Blob{}.Changeset().SizeSet(7)); err != nil || b.Size != 7 {
		t.Errorf("Update keyed by a nil hash = %+v, %v; want the row keyed by empty bytes", b, err)
	}

	rows, err := q.BlobQuery(ctx, BlobHash.Eq(nil))
	if err != nil || len(rows) != 1 || rows[0].Size != 7 {
		t.Errorf("Query Eq(nil) = %+v, %v", rows, err)
	}
	if n, err := q.BlobCount(ctx, BlobTag.In(nil, []byte("t"))); err != nil || n != 2 {
		t.Errorf("Count In(nil, t) = %d, %v; want 2", n, err)
	}
	for _, list := range [][][]byte{{nil, []byte("t")}, {{}, []byte("t")}} {
		if n, err := q.BlobCount(ctx, BlobTag.InList(list)); err != nil || n != 2 {
			t.Errorf("Count InList(%#v) = %d, %v; want 2, as In matches", list, n, err)
		}
	}
	if n, err := q.BlobCount(ctx, BlobTag.InList([][]byte{[]byte("t")})); err != nil || n != 1 {
		t.Errorf("Count InList(t) = %d, %v; want 1", n, err)
	}
	hash := rt.Column[Blob, hashKey]{Name: "hash"}
	if n, err := q.BlobCount(ctx, hash.InList([]hashKey{hashKey("h")})); err != nil || n != 1 {
		t.Errorf("Count InList of a Valuer binding bytes = %d, %v; want 1", n, err)
	}
	if n, err := q.BlobCount(ctx, rt.Raw[Blob]("hash = ?", []byte(nil))); err != nil || n != 1 {
		t.Errorf("Count Raw with a nil []byte arg = %d, %v; want 1", n, err)
	}
	after, err := q.BlobQuery(ctx, BlobOrderBy(BlobHash.Asc()), BlobAfter([]byte(nil)))
	if err != nil || len(after) != 1 || string(after[0].Hash) != "h" {
		t.Errorf("Query After(nil) = %+v, %v; want the one row after empty bytes", after, err)
	}
	if err := q.BlobDelete(ctx, nil); err != nil {
		t.Errorf("Delete(nil) = %v; want the row keyed by empty bytes deleted", err)
	}
	if n, err := q.BlobCount(ctx); err != nil || n != 1 {
		t.Errorf("Count after Delete(nil) = %d, %v; want 1", n, err)
	}
	if n, err := q.BlobUpdateWhere(ctx, BlobSet(BlobTag.Set(nil)), BlobHash.Eq([]byte("h"))); err != nil || n != 1 {
		t.Errorf("UpdateWhere tag Set(nil) = %d, %v; want one row set to empty bytes", n, err)
	}
	emptyBlob(t, db, "blobs", "tag", "hash = ?", []byte("h"))
}

func TestBytesJSON(t *testing.T) {
	for _, drv := range bytesDrivers {
		t.Run(drv, func(t *testing.T) {
			ctx := context.Background()
			db, q := bytesDB(t, drv)

			empty, err := q.AttachmentCreate(ctx, AttachmentNew().NameSet("empty").BodySet(nil).ThumbSet(rt.Some([]byte(nil))).DigestSet([]byte("d")))
			if err != nil {
				t.Fatal(err)
			}
			nul, err := q.AttachmentCreate(ctx, AttachmentNew().NameSet("nul").BodySet([]byte("hi")).DigestSet([]byte("d")))
			if err != nil {
				t.Fatal(err)
			}
			rows, err := q.AttachmentGetMany(ctx, []int{empty.ID, nul.ID})
			if err != nil || len(rows) != 2 {
				t.Fatalf("GetMany = %v, %v", rows, err)
			}
			doc, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			want := `[{"id":1,"name":"empty","body":"","thumb":"","digest":"ZA=="},{"id":2,"name":"nul","body":"aGk=","thumb":null,"digest":"ZA=="}]`
			if string(doc) != want {
				t.Errorf("rows as JSON:\n got %s\nwant %s", doc, want)
			}

			cs := AttachmentNew().NameSet("wire").BodySet(nil).ThumbSet(rt.Some([]byte(nil))).DigestSet([]byte("d"))
			body, err := json.Marshal(cs.Params())
			if err != nil {
				t.Fatal(err)
			}
			if want := `{"body":"","digest":"ZA==","name":"wire","thumb":""}`; string(body) != want {
				t.Errorf("Params JSON = %s; want %s", body, want)
			}
			var p AttachmentParams
			if err := json.Unmarshal(body, &p); err != nil {
				t.Fatal(err)
			}
			a, err := q.AttachmentCreate(ctx, AttachmentNew().Cast(p, AttachmentName, AttachmentBody, AttachmentThumb, AttachmentDigest))
			if err != nil {
				t.Fatalf("Create from the JSON body: %v", err)
			}
			emptyBlob(t, db, "attachments", "body", "id = ?", a.ID)
			emptyBlob(t, db, "attachments", "thumb", "id = ?", a.ID)

			if err := json.Unmarshal([]byte(`{"name": "nulled", "body": "aGk=", "thumb": null, "digest": "ZA=="}`), &p); err != nil {
				t.Fatal(err)
			}
			a, err = q.AttachmentCreate(ctx, AttachmentNew().Cast(p, AttachmentName, AttachmentBody, AttachmentThumb, AttachmentDigest))
			if err != nil {
				t.Fatal(err)
			}
			if typ, _ := stored(t, db, "attachments", "thumb", "id = ?", a.ID); typ != "null" {
				t.Errorf("thumb null stored as %s; want null", typ)
			}

			if err := json.Unmarshal([]byte(`{"body": null}`), &p); err != nil {
				t.Fatal(err)
			}
			refused := AttachmentNew().Cast(p, AttachmentBody)
			errs := refused.Errors()
			if refused.Changed(AttachmentBody) || len(errs) != 1 || errs[0].Check != "cast" || errs[0].Message != "body: null is not allowed ([]byte)" {
				t.Errorf("Cast of a JSON null on a NOT NULL []byte: changed %v, errors %+v", refused.Changed(AttachmentBody), errs)
			}
		})
	}
}

func TestBytesGob(t *testing.T) {
	ctx := context.Background()
	db, q := bytesDB(t, "sqlite3")

	cases := []struct {
		name      string
		cs        AttachmentChangeset
		thumbType string
	}{
		{"empty", AttachmentNew().BodySet([]byte{}).ThumbSet(rt.Some([]byte{})), "blob"},
		{"nil", AttachmentNew().BodySet(nil).ThumbSet(rt.Some([]byte(nil))), "blob"},
		{"null", AttachmentNew().BodySet([]byte{}).ThumbSet(rt.Null[[]byte]{}), "null"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(c.cs.NameSet(c.name).DigestSet([]byte("d")).Params()); err != nil {
			t.Fatal(err)
		}
		var p AttachmentParams
		if err := gob.NewDecoder(&buf).Decode(&p); err != nil {
			t.Fatal(err)
		}
		cast := AttachmentNew().Cast(p, AttachmentName, AttachmentBody, AttachmentThumb, AttachmentDigest)
		if row := cast.Row(); row.Body != nil || row.Thumb.V != nil {
			t.Fatalf("%s: GOB was expected to carry empty bytes as nil, got %#v", c.name, row)
		}
		a, err := q.AttachmentCreate(ctx, cast)
		if err != nil {
			t.Fatalf("%s: Create from the GOB body: %v", c.name, err)
		}
		emptyBlob(t, db, "attachments", "body", "id = ?", a.ID)
		if typ, n := stored(t, db, "attachments", "thumb", "id = ?", a.ID); typ != c.thumbType || n != 0 {
			t.Errorf("%s: thumb stored as %s length %d; want %s", c.name, typ, n, c.thumbType)
		}
	}
}

func TestBytesRequiredRefusesEmptyOnly(t *testing.T) {
	ctx := context.Background()
	_, q := bytesDB(t, "sqlite3")
	base := AttachmentNew().NameSet("r").BodySet(nil)

	for _, digest := range [][]byte{nil, {}} {
		cs := base.DigestSet(digest).Validate()
		errs := cs.Errors()
		if len(errs) != 1 || errs[0].Check != "required" || strings.Join(errs[0].Columns, ",") != "digest" {
			t.Errorf("digest %#v: errors %+v; want one required on digest", digest, errs)
		}
		_, err := q.AttachmentCreate(ctx, base.DigestSet(digest))
		invalidOf(t, err, "required", "digest")
	}
	for _, digest := range [][]byte{{0}, []byte("d")} {
		if errs := base.DigestSet(digest).Validate().Errors(); len(errs) != 0 {
			t.Errorf("digest %#v: errors %+v; want none, a nil body included", digest, errs)
		}
	}
	if n, err := q.AttachmentCount(ctx); err != nil || n != 0 {
		t.Errorf("Count = %d, %v; required refuses before the database", n, err)
	}
}
