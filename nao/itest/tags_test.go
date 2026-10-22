// Struct-tag passthrough round trip (spec §columns tag extension,
// §struct-tags, §changesets): the [tag:] on users.name reaches
// encoding/json in the compiled program, and the same json key is the
// wire name a changeset casts from and sends.
package itest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTagPassthroughRenamesJSON(t *testing.T) {
	_, q := newDB(t)
	u, err := q.UserCreate(context.Background(), UserNew().EmailSet("ann@example.com").NameSet("Ann"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), `"displayName":"Ann"`) {
		t.Errorf(`model JSON did not honor [tag: "json:\"displayName\""]: %s`, doc)
	}
	if strings.Contains(string(doc), `"name"`) {
		t.Errorf("the generated json default should have been replaced, not kept: %s", doc)
	}

	var p UserParams
	if err := json.Unmarshal([]byte(`{"email":"bo@example.com","displayName":"Bo"}`), &p); err != nil {
		t.Fatal(err)
	}
	cs := UserNew().Cast(p, UserEmail, UserName)
	if !cs.Valid() || !cs.Changed(UserName) || cs.Row().Name != "Bo" {
		t.Errorf("Cast did not read displayName into Name: %+v, %+v", cs.Row(), cs.Errors())
	}

	sent, err := json.Marshal(UserNew().NameSet("Bo").Params())
	if err != nil || string(sent) != `{"displayName":"Bo"}` {
		t.Errorf("Params sends the json key, got %s, %v", sent, err)
	}
}
