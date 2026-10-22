package check_test

import (
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

func TestPartialInjectedTwiceListsValidatorsOnce(t *testing.T) {
	f, diags := parser.ParseFile("p.volt", `TablePartial stamps {
  created_at time.Time
  Validators {
    StampValid(created_at)
  }
}

Table posts {
  id int [pk]
  ~stamps
  ~stamps
  Validators {
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
