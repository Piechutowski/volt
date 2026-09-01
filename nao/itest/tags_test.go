// Struct-tag passthrough round trip (spec §6.3 tag extension, Appendix
// A.5, decision D60): the [tag:] on users.name reaches encoding/json in
// the compiled program, on the model and its params struct alike.
package itest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTagPassthroughRenamesJSON(t *testing.T) {
	_, q := newDB(t)
	u, err := q.UserCreate(context.Background(), UserCreateParams{
		Email: "ann@example.com", Name: "Ann",
	})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), `"displayName":"Ann"`) {
		t.Errorf("model JSON did not honor [tag: 'json:\"displayName\"']: %s", doc)
	}
	if strings.Contains(string(doc), `"name"`) {
		t.Errorf("the generated json default should have been replaced, not kept: %s", doc)
	}

	var p UserCreateParams
	if err := json.Unmarshal([]byte(`{"email":"bo@example.com","displayName":"Bo"}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.Name != "Bo" {
		t.Errorf("params struct did not decode displayName: %+v", p)
	}
}
