package rt

import (
	"errors"
	"reflect"
	"testing"
)

type entry struct {
	ID    int
	Title string
	Body  Null[string]
}

var entryDesc = &Desc[entry]{
	Model:   "Post",
	Table:   `"entries"`,
	Columns: `"id", "title", "body"`,
	Fields: []FieldDesc{
		{Column: "id", Wire: "id", Key: true, Assigned: true},
		{Column: "title", Wire: "title"},
		{Column: "body", Wire: "body"},
	},
	Defaults: entry{Title: "untitled"},
	Scan: func(r Row) (entry, error) {
		var v entry
		return v, r.Scan(&v.ID, &v.Title, &v.Body)
	},
	Values: func(r entry) []any { return []any{r.ID, r.Title, r.Body} },
	Apply: func(data, vals entry, set []bool) entry {
		if set[0] {
			data.ID = vals.ID
		}
		if set[1] {
			data.Title = vals.Title
		}
		if set[2] {
			data.Body = vals.Body
		}
		return data
	},
	Cast: func(c Changeset[entry], p Params[entry], i int) (Changeset[entry], error) {
		switch i {
		case 0:
			v, ok, err := Field(p, "id", "int", false, func(r *entry) int { return r.ID })
			if err != nil || !ok {
				return c, err
			}
			return Set(c, 0, func(r *entry) { r.ID = v }), nil
		case 1:
			v, ok, err := Field(p, "title", "string", false, func(r *entry) string { return r.Title })
			if err != nil || !ok {
				return c, err
			}
			return Set(c, 1, func(r *entry) { r.Title = v }), nil
		case 2:
			v, ok, err := Field(p, "body", "string", true, func(r *entry) Null[string] { return r.Body })
			if err != nil || !ok {
				return c, err
			}
			return Set(c, 2, func(r *entry) { r.Body = v }), nil
		}
		return c, nil
	},
}

var (
	entryID    = Column[entry, int]{Name: "id"}
	entryTitle = Column[entry, string]{Name: "title"}
	entryBody  = Column[entry, string]{Name: "body"}
)

func TestChangesetValues(t *testing.T) {
	c := NewChangeset(entryDesc)
	if c.FromRow() || c.Row() != (entry{Title: "untitled"}) || !c.Valid() {
		t.Fatalf("NewChangeset = %+v", c.Row())
	}
	set := Set(c, 1, func(r *entry) { r.Title = "hello" })
	if c.Changed(entryTitle) || !set.Changed(entryTitle) || set.Changed(entryBody) {
		t.Errorf("Set marked the receiver or missed the copy: %v %v", c.Changed(entryTitle), set.Changed(entryTitle))
	}
	if set.Row().Title != "hello" || c.Row().Title != "untitled" {
		t.Errorf("Row after Set: %q, receiver %q", set.Row().Title, c.Row().Title)
	}
	sql, args := set.InsertRender()
	if sql != `INSERT INTO "entries" ("title") VALUES (?) RETURNING "id", "title", "body"` || !reflect.DeepEqual(args, []any{"hello"}) {
		t.Errorf("InsertRender = %s %v", sql, args)
	}
	if sql, args := c.InsertRender(); sql != `INSERT INTO "entries" DEFAULT VALUES RETURNING "id", "title", "body"` || args != nil {
		t.Errorf("InsertRender(nothing marked) = %s %v", sql, args)
	}
	if keys := set.Params().Keys(); !reflect.DeepEqual(keys, []string{"title"}) {
		t.Errorf("Params keys = %v", keys)
	}
}

func TestChangesetOfRow(t *testing.T) {
	row := entry{ID: 7, Title: "old", Body: Some("text")}
	c := ChangesetOf(entryDesc, row)
	if !c.FromRow() || c.Row() != row {
		t.Fatalf("ChangesetOf = %+v", c.Row())
	}
	if sql, args := c.UpdateRender(); sql != "" || args != nil {
		t.Errorf("UpdateRender(nothing marked) = %q %v", sql, args)
	}
	set := Set(c, 2, func(r *entry) { r.Body = Null[string]{} })
	sql, args := set.UpdateRender()
	if sql != `UPDATE "entries" SET "body" = ? WHERE "id" = ? RETURNING "id", "title", "body"` || !reflect.DeepEqual(args, []any{Null[string]{}, 7}) {
		t.Errorf("UpdateRender = %s %v", sql, args)
	}
	from := NewChangeset(entryDesc).From(row)
	if from.Changed(entryID) || !from.Changed(entryTitle) || !from.Changed(entryBody) || from.Row() != (entry{Title: "old", Body: Some("text")}) {
		t.Errorf("From = %+v changed id %v", from.Row(), from.Changed(entryID))
	}
}

func TestChangesetCast(t *testing.T) {
	var p Params[entry]
	if err := p.UnmarshalJSON([]byte(`{"title": 5, "body": null, "nope": 1, "id": 9}`)); err != nil {
		t.Fatal(err)
	}
	c := NewChangeset(entryDesc).Cast(p, entryTitle, entryBody, entryID)
	errs := c.Errors()
	if len(errs) != 2 || errs[0].Check != "unknown_field" || errs[0].Columns[0] != "nope" || errs[1].Check != "cast" || errs[1].Columns[0] != "title" {
		t.Errorf("Cast errors = %v", errs)
	}
	if c.Changed(entryTitle) || !c.Changed(entryBody) || !c.Changed(entryID) || c.Row().Body.Valid || c.Row().ID != 9 {
		t.Errorf("Cast marks: title %v body %v id %v (%d)", c.Changed(entryTitle), c.Changed(entryBody), c.Changed(entryID), c.Row().ID)
	}
	if keyless := NewChangeset(entryDesc).Cast(p, entryTitle, entryBody); keyless.Changed(entryID) {
		t.Errorf("Cast set the rowid alias key its allow list leaves out")
	}
	var inv *Invalid
	if err := c.Err(); !errors.As(err, &inv) || inv.Model != "Post" || len(inv.Fields) != 2 {
		t.Errorf("Err = %v", err)
	}
	custom := c.ErrorAdd(entryTitle, "taken").WithFindings([]FieldError{{Check: "required", Columns: []string{"title"}}})
	if got := custom.Errors(); len(got) != 4 || got[2].Check != "custom" || got[3].Check != "required" {
		t.Errorf("Errors after ErrorAdd and WithFindings = %v", got)
	}
	if len(c.Errors()) != 2 || custom.WithFindings(nil).Errors()[2].Check != "custom" || len(custom.WithFindings(nil).Errors()) != 3 {
		t.Errorf("ErrorAdd or WithFindings touched its receiver")
	}
}
