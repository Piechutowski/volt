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
	v, err := Bytes("").Value()
	if b, ok := v.([]byte); err != nil || !ok || b == nil || len(b) != 0 {
		t.Errorf(`Bytes("").Value() = %#v, %v; want non-nil empty bytes`, v, err)
	}
	v, err = Bytes("hi").Value()
	if b, ok := v.([]byte); err != nil || !ok || string(b) != "hi" {
		t.Errorf("Bytes(hi).Value() = %#v, %v", v, err)
	}
}

func TestBytesCopies(t *testing.T) {
	in := []byte("hi")
	b := Bytes(in)
	in[0] = 'X'
	out := b.Bytes()
	out[1] = 'Y'
	if b != "hi" {
		t.Errorf("Bytes shares a slice it was made from or handed out: %q", b)
	}
	if Bytes("").Bytes() == nil {
		t.Error("Bytes() of empty bytes is nil; want non-nil empty bytes")
	}
}

func TestBytesScan(t *testing.T) {
	var b Bytes
	if err := b.Scan([]byte(nil)); err != nil || b != "" {
		t.Errorf("Scan of a typed nil []byte = %q, %v; want empty bytes", b, err)
	}
	src := []byte("abc")
	if err := b.Scan(src); err != nil || b != "abc" {
		t.Errorf("Scan(abc) = %q, %v", b, err)
	}
	src[0] = 'X'
	if b != "abc" {
		t.Errorf("Scan shares the driver's buffer: %q", b)
	}
	if err := b.Scan("text"); err != nil || b != "text" {
		t.Errorf("Scan(text) = %q, %v", b, err)
	}
	if err := b.Scan(nil); err == nil {
		t.Errorf("Scan(NULL) = %q, want an error: plain bytes are never NULL", b)
	}

	var n Null[Bytes]
	if err := n.Scan([]byte(nil)); err != nil || !n.Valid || n.V != "" {
		t.Errorf("Null[Bytes].Scan of a typed nil = %#v, %v; want valid empty bytes", n, err)
	}
	if err := n.Scan(nil); err != nil || n.Valid || n.V != "" {
		t.Errorf("Null[Bytes].Scan(NULL) = %#v, %v; want NULL", n, err)
	}
	if err := n.Scan([]byte("hi")); err != nil || !n.Valid || n.V != "hi" {
		t.Errorf("Null[Bytes].Scan(hi) = %#v, %v", n, err)
	}
}

func TestBytesScanThroughDatabaseSQL(t *testing.T) {
	db, err := sql.Open("rt_nilblob", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var b Bytes
	if err := db.QueryRow("q").Scan(&b); err != nil || b != "" {
		t.Errorf("Bytes scan = %q, %v; want empty bytes", b, err)
	}
	var n Null[Bytes]
	if err := db.QueryRow("q").Scan(&n); err != nil || !n.Valid || n.V != "" {
		t.Errorf("Null[Bytes] scan = %#v, %v; want valid empty bytes", n, err)
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
	v, err := Some(Bytes("")).Value()
	if b, ok := v.([]byte); err != nil || !ok || b == nil || len(b) != 0 {
		t.Errorf(`Some(Bytes("")).Value() = %#v, %v; want non-nil empty bytes`, v, err)
	}
	v, err = Null[Bytes]{}.Value()
	if err != nil || v != nil {
		t.Errorf("NULL Value() = %#v, %v; want nil", v, err)
	}
	v, err = Some(pngBlob(nil)).Value()
	if err != nil || v != nil {
		t.Errorf("a declared []byte type keeps its own Valuer: got %#v, %v; want its nil", v, err)
	}
}

func TestBytesJSON(t *testing.T) {
	cases := []struct {
		v    any
		want string
	}{
		{Bytes(""), `""`},
		{Bytes("hi"), `"aGk="`},
		{Some(Bytes("")), `""`},
		{Some(Bytes("hi")), `"aGk="`},
		{Null[Bytes]{}, `null`},
	}
	for _, c := range cases {
		got, err := json.Marshal(c.v)
		if err != nil || string(got) != c.want {
			t.Errorf("Marshal(%#v) = %s, %v; want %s", c.v, got, err, c.want)
		}
	}

	var b Bytes
	if err := json.Unmarshal([]byte(`"aGk="`), &b); err != nil || b != "hi" {
		t.Errorf(`Unmarshal("aGk=") = %q, %v; want hi`, b, err)
	}
	if err := json.Unmarshal([]byte(`null`), &b); err != nil || b != "hi" {
		t.Errorf("Unmarshal(null) = %q, %v; want the bytes left as they were", b, err)
	}
	if err := json.Unmarshal([]byte(`"not base64"`), &b); err == nil {
		t.Errorf("Unmarshal of text that is not base64 = %q; want an error", b)
	}
	var n Null[Bytes]
	if err := json.Unmarshal([]byte(`""`), &n); err != nil || !n.Valid || n.V != "" {
		t.Errorf(`Unmarshal("") = %#v, %v; want valid empty bytes`, n, err)
	}
	if err := json.Unmarshal([]byte(`null`), &n); err != nil || n.Valid {
		t.Errorf("Unmarshal(null) = %#v, %v; want NULL", n, err)
	}
}

type blobRow struct{}

func bytesIs(v any, want string) bool {
	b, ok := v.([]byte)
	return ok && string(b) == want
}

func TestDynamicLayerBindsBytes(t *testing.T) {
	data := Column[blobRow, Bytes]{Name: "data"}
	thumb := NullColumn[blobRow, Bytes]{Column[blobRow, Bytes]{Name: "thumb"}}
	png := Column[blobRow, pngBlob]{Name: "png"}

	emptyBytes := func(t *testing.T, what string, a any) {
		t.Helper()
		v, err := driver.DefaultParameterConverter.ConvertValue(a)
		if b, ok := v.([]byte); err != nil || !ok || b == nil || len(b) != 0 {
			t.Errorf("%s binds %#v (%#v, %v); want non-nil empty bytes", what, a, v, err)
		}
	}

	_, args, err := SelectRender(`"blobs"`, `"data"`, []Opt[blobRow]{
		data.Eq(""), data.In("", "x"), Raw[blobRow]("data = ?", []byte(nil)),
		OrderBy(data.Asc()), After[blobRow]([]byte(nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 5 {
		t.Fatalf("args = %#v", args)
	}
	emptyBytes(t, `Eq("")`, args[0])
	emptyBytes(t, `In("", ...)`, args[1])
	emptyBytes(t, "Raw arg nil", args[3])
	emptyBytes(t, "After(nil)", args[4])

	_, args, err = UpdateRender(`"blobs"`, []Assign[blobRow]{data.Set(""), thumb.Set(""), thumb.SetNull()}, []Pred[blobRow]{Raw[blobRow]("1 = 1")})
	if err != nil {
		t.Fatal(err)
	}
	emptyBytes(t, `Set("")`, args[0])
	emptyBytes(t, `NullColumn Set("")`, args[1])
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
		data.InList([]Bytes{""}), data.InList([]Bytes{"", "x"}),
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
	emptyBytes(t, `InList([""])`, args[0])
	emptyBytes(t, `InList(["", x])`, args[1])
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[2]); err != nil || !bytesIs(v, "x") {
		t.Errorf("InList item x binds %#v, %v", v, err)
	}
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[3]); err != nil || v != nil {
		t.Errorf("an InList item of a declared []byte type binds through its own Valuer: got %#v, %v", v, err)
	}
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[4]); err != nil || !bytesIs(v, "hi") {
		t.Errorf("InList item pngBlob(hi) binds %#v, %v", v, err)
	}
	if v, err := driver.DefaultParameterConverter.ConvertValue(args[5]); err != nil || v != "[]" {
		t.Errorf("an empty InList binds %#v; want the empty JSON array, which matches no row", args[5])
	}
}

func TestParamsJSONRendersEmptyBytes(t *testing.T) {
	type blob struct {
		Data  Bytes       `json:"data"`
		Thumb Null[Bytes] `json:"thumb"`
		Icon  Null[Bytes] `json:"icon"`
	}
	p := ParamsOf(blob{}, Mark{Wire: "data", Value: Bytes("")}, Mark{Wire: "thumb", Value: Some(Bytes(""))}, Mark{Wire: "icon", Value: Null[Bytes]{}})
	got, err := json.Marshal(p)
	if want := `{"data":"","icon":null,"thumb":""}`; err != nil || string(got) != want {
		t.Errorf("MarshalJSON = %s, %v; want %s", got, err, want)
	}
}
