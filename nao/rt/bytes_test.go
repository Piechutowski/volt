package rt

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

type nilBlobDriver struct{}

func (nilBlobDriver) Open(string) (driver.Conn, error) { return nilBlobConn{}, nil }

type nilBlobConn struct{}

func (nilBlobConn) Prepare(string) (driver.Stmt, error) { return nilBlobStmt{}, nil }
func (nilBlobConn) Close() error                        { return nil }
func (nilBlobConn) Begin() (driver.Tx, error)           { return nil, errors.New("no transactions") }

type nilBlobStmt struct{}

func (nilBlobStmt) Close() error  { return nil }
func (nilBlobStmt) NumInput() int { return -1 }
func (nilBlobStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("no exec")
}
func (nilBlobStmt) Query([]driver.Value) (driver.Rows, error) { return &nilBlobRows{}, nil }

type nilBlobRows struct{ done bool }

func (*nilBlobRows) Columns() []string { return []string{"data"} }
func (*nilBlobRows) Close() error      { return nil }
func (r *nilBlobRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = []byte(nil)
	return nil
}

func init() { sql.Register("rt_nilblob", nilBlobDriver{}) }

func TestBytesValue(t *testing.T) {
	v, err := Bytes(nil).Value()
	if b, ok := v.([]byte); err != nil || !ok || b == nil || len(b) != 0 {
		t.Errorf("Bytes(nil).Value() = %#v, %v; want non-nil empty bytes", v, err)
	}
	v, err = Bytes("hi").Value()
	if b, ok := v.([]byte); err != nil || !ok || string(b) != "hi" {
		t.Errorf("Bytes(hi).Value() = %#v, %v", v, err)
	}
}

func TestBytesScanTypedNil(t *testing.T) {
	var b []byte
	if err := (*Bytes)(&b).Scan([]byte(nil)); err != nil || b == nil || len(b) != 0 {
		t.Errorf("Scan of a typed nil []byte = %#v, %v; want non-nil empty bytes", b, err)
	}
	src := []byte("abc")
	if err := (*Bytes)(&b).Scan(src); err != nil || string(b) != "abc" {
		t.Errorf("Scan(abc) = %q, %v", b, err)
	}
	src[0] = 'X'
	if string(b) != "abc" {
		t.Errorf("Scan shares the driver's buffer: %q", b)
	}
	if err := (*Bytes)(&b).Scan("text"); err != nil || string(b) != "text" {
		t.Errorf("Scan(text) = %q, %v", b, err)
	}
	if err := (*Bytes)(&b).Scan(nil); err == nil {
		t.Errorf("Scan(NULL) = %q, want an error: a plain []byte is never NULL", b)
	}

	var n Null[[]byte]
	if err := n.Scan([]byte(nil)); err != nil || !n.Valid || n.V == nil || len(n.V) != 0 {
		t.Errorf("Null[[]byte].Scan of a typed nil = %#v, %v; want valid non-nil empty bytes", n, err)
	}
	if err := n.Scan(nil); err != nil || n.Valid || n.V != nil {
		t.Errorf("Null[[]byte].Scan(NULL) = %#v, %v; want NULL", n, err)
	}
	if err := n.Scan([]byte("hi")); err != nil || !n.Valid || string(n.V) != "hi" {
		t.Errorf("Null[[]byte].Scan(hi) = %#v, %v", n, err)
	}
}

func TestBytesScanThroughDatabaseSQL(t *testing.T) {
	db, err := sql.Open("rt_nilblob", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var plain []byte
	if err := db.QueryRow("q").Scan(&plain); err != nil || plain != nil {
		t.Fatalf("plain scan = %#v, %v; the driver's typed nil reaches *[]byte as nil", plain, err)
	}
	var b []byte
	if err := db.QueryRow("q").Scan((*Bytes)(&b)); err != nil || b == nil || len(b) != 0 {
		t.Errorf("Bytes scan = %#v, %v; want non-nil empty bytes", b, err)
	}
	var n Null[[]byte]
	if err := db.QueryRow("q").Scan(&n); err != nil || !n.Valid || n.V == nil || len(n.V) != 0 {
		t.Errorf("Null[[]byte] scan = %#v, %v; want valid non-nil empty bytes", n, err)
	}
}

type pngBlob []byte

func (p pngBlob) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return []byte(p), nil
}

func TestNullBytesValue(t *testing.T) {
	v, err := Some([]byte(nil)).Value()
	if b, ok := v.([]byte); err != nil || !ok || b == nil || len(b) != 0 {
		t.Errorf("Some(nil []byte).Value() = %#v, %v; want non-nil empty bytes", v, err)
	}
	v, err = Some([]byte{}).Value()
	if b, ok := v.([]byte); err != nil || !ok || b == nil || len(b) != 0 {
		t.Errorf("Some([]byte{}).Value() = %#v, %v; want non-nil empty bytes", v, err)
	}
	v, err = Null[[]byte]{}.Value()
	if err != nil || v != nil {
		t.Errorf("NULL Value() = %#v, %v; want nil", v, err)
	}
	v, err = Some(pngBlob(nil)).Value()
	if err != nil || v != nil {
		t.Errorf("a declared []byte type keeps its own Valuer: got %#v, %v; want its nil", v, err)
	}
}

func TestNullBytesJSON(t *testing.T) {
	cases := []struct {
		n    Null[[]byte]
		want string
	}{
		{Some([]byte(nil)), `""`},
		{Some([]byte{}), `""`},
		{Some([]byte("hi")), `"aGk="`},
		{Null[[]byte]{}, `null`},
	}
	for _, c := range cases {
		got, err := json.Marshal(c.n)
		if err != nil || string(got) != c.want {
			t.Errorf("Marshal(%#v) = %s, %v; want %s", c.n, got, err, c.want)
		}
	}

	var n Null[[]byte]
	if err := json.Unmarshal([]byte(`""`), &n); err != nil || !n.Valid || n.V == nil || len(n.V) != 0 {
		t.Errorf(`Unmarshal("") = %#v, %v; want valid empty bytes`, n, err)
	}
	if err := json.Unmarshal([]byte(`null`), &n); err != nil || n.Valid {
		t.Errorf("Unmarshal(null) = %#v, %v; want NULL", n, err)
	}

	got, err := json.Marshal(Bytes(nil))
	if err != nil || string(got) != `""` {
		t.Errorf("Marshal(Bytes(nil)) = %s, %v; want \"\"", got, err)
	}
}

type blobRow struct{}

func bytesIs(v any, want string) bool {
	b, ok := v.([]byte)
	return ok && string(b) == want
}

func TestDynamicLayerBindsBytes(t *testing.T) {
	data := Column[blobRow, []byte]{Name: "data"}
	thumb := NullColumn[blobRow, []byte]{Column[blobRow, []byte]{Name: "thumb"}}
	png := Column[blobRow, pngBlob]{Name: "png"}

	emptyBytes := func(t *testing.T, what string, a any) {
		t.Helper()
		v, err := driver.DefaultParameterConverter.ConvertValue(a)
		if b, ok := v.([]byte); err != nil || !ok || b == nil || len(b) != 0 {
			t.Errorf("%s binds %#v (%#v, %v); want non-nil empty bytes", what, a, v, err)
		}
	}

	_, args, err := SelectRender(`"blobs"`, `"data"`, []Opt[blobRow]{
		data.Eq(nil), data.In(nil, []byte("x")), Raw[blobRow]("data = ?", []byte(nil)),
		OrderBy(data.Asc()), After[blobRow]([]byte(nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 5 {
		t.Fatalf("args = %#v", args)
	}
	emptyBytes(t, "Eq(nil)", args[0])
	emptyBytes(t, "In(nil, ...)", args[1])
	emptyBytes(t, "Raw arg nil", args[3])
	emptyBytes(t, "After(nil)", args[4])

	_, args, err = UpdateRender(`"blobs"`, []Assign[blobRow]{data.Set(nil), thumb.Set(nil), thumb.SetNull()}, []Pred[blobRow]{Raw[blobRow]("1 = 1")})
	if err != nil {
		t.Fatal(err)
	}
	emptyBytes(t, "Set(nil)", args[0])
	emptyBytes(t, "NullColumn Set(nil)", args[1])
	if args[2] != nil {
		t.Errorf("SetNull binds %#v, want NULL", args[2])
	}

	_, args, err = CountRender(`"blobs"`, []Pred[blobRow]{png.Eq(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[0]); err != nil || v != nil {
		t.Errorf("a declared []byte type binds through its own Valuer: got %#v, %v", v, err)
	}

	query, args, err := CountRender(`"blobs"`, []Pred[blobRow]{
		data.InList([][]byte{nil}), data.InList([][]byte{{}, []byte("x")}),
		png.InList([]pngBlob{nil, pngBlob("hi")}), data.InList(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT count(*) FROM "blobs" WHERE "data" IN (?) AND "data" IN (?, ?) AND "png" IN (?, ?) AND "data" IN (SELECT value FROM json_each(?))`; query != want {
		t.Errorf("InList over bytes renders\n%s\nwant\n%s", query, want)
	}
	if len(args) != 6 {
		t.Fatalf("args = %#v", args)
	}
	emptyBytes(t, "InList([nil])", args[0])
	emptyBytes(t, "InList([{}, x])", args[1])
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[2]); err != nil || !bytesIs(v, "x") {
		t.Errorf("InList item x binds %#v, %v", v, err)
	}
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[3]); err != nil || v != nil {
		t.Errorf("an InList item of a declared []byte type binds through its own Valuer: got %#v, %v", v, err)
	}
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[4]); err != nil || !bytesIs(v, "hi") {
		t.Errorf("InList item pngBlob(hi) binds %#v, %v", v, err)
	}
	if args[5] != "[]" {
		t.Errorf("an empty InList binds %#v; want the empty JSON array, which matches no row", args[5])
	}
}

func TestParamsJSONRendersNilBytesEmpty(t *testing.T) {
	type blob struct {
		Data  []byte       `json:"data"`
		Thumb Null[[]byte] `json:"thumb"`
		Icon  Null[[]byte] `json:"icon"`
	}
	p := ParamsOf(blob{}, Mark{Wire: "data", Value: []byte(nil)}, Mark{Wire: "thumb", Value: Some([]byte(nil))}, Mark{Wire: "icon", Value: Null[[]byte]{}})
	got, err := json.Marshal(p)
	if want := `{"data":"","icon":null,"thumb":""}`; err != nil || string(got) != want {
		t.Errorf("MarshalJSON = %s, %v; want %s", got, err, want)
	}
}
