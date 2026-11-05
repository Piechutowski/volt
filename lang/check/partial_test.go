package check_test

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

func TestPartialInjectedTwiceListsValidatorsOnce(t *testing.T) {
	f, diags := parser.ParseFile("p.volt", `table_partial stamps {
  created_at time.Time
  validators {
    StampValid(created_at)
  }
}

table posts {
  id int [pk]
  ~stamps
  ~stamps
  validators {
    StampValid(created_at)
  }
}
`)
	if diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	info, diags := check.File(f)
	if diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	if n := len(info.Tables[0].Validators); n != 2 {
		t.Errorf("posts lists %d validators, want 2: the partial's once however often it is injected, and the table's own (§tablepartial)", n)
	}
}

// TestPartialInjectionKeepsSourceOrder proves a table's effective columns
// are its injected and direct columns concatenated in source order, a
// partial injected first and one between direct columns included, and
// that a column defined twice stands where it was first defined (spec
// §partial-injection-order).
func TestPartialInjectionKeepsSourceOrder(t *testing.T) {
	f, diags := parser.ParseFile("p.volt", `table_partial keyed {
  id int [pk]
}

table_partial stamps {
  created_at time.Time
  updated_at time.Time [null]
}

table posts {
  ~keyed
  title string
  ~stamps
  body string
  created_at time.Time [null]
}
`)
	if diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	info, diags := check.File(f)
	if diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	var got []string
	for _, c := range info.Tables[0].Columns {
		got = append(got, c.Col.Name.Name())
	}
	if want := "id title created_at updated_at body"; strings.Join(got, " ") != want {
		t.Errorf("posts' columns = %v, want %s", got, want)
	}
	if c := info.Tables[0].Column("created_at"); c == nil || !c.Nullable {
		t.Errorf("created_at = %+v, want the table's own nullable definition at the partial's position", c)
	}
}
