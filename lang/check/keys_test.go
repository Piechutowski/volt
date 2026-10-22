package check_test

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

func keysInfo(t *testing.T, src string) *check.Info {
	t.Helper()
	f, diags := parser.ParseFile("k.volt", src)
	if diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	info, diags := check.File(f)
	if diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	return info
}

// TestPKInDeclarationOrder pins the order of a composite key (spec
// §columns): the pk columns in the table's column order, a column a
// partial injects counted at its injected position, whatever the order
// of anything else.
func TestPKInDeclarationOrder(t *testing.T) {
	info := keysInfo(t, `TablePartial daily {
  day int [pk]
}

Table site_days {
  site  string [pk]
  ~daily
  views int
  slot  int    [pk]
}

Table day_sites {
  ~daily
  site string [pk]
}
`)
	for _, tc := range []struct{ table, want string }{
		{"site_days", "site,day,slot"},
		{"day_sites", "day,site"},
	} {
		var got []string
		for _, cd := range info.Table(tc.table).PK() {
			got = append(got, cd.Col.Name.Name())
		}
		if strings.Join(got, ",") != tc.want {
			t.Errorf("%s: PK() = %v, want %s", tc.table, got, tc.want)
		}
	}
}

// TestColumnsUnique pins what makes columns unique (spec §indexes,
// §relationships-ref): the whole primary key in any order, a unique
// column, the key of a unique index of columns only without where:;
// never one member of a composite key, a plain index, a partial unique
// index or an expression unique index.
func TestColumnsUnique(t *testing.T) {
	info := keysInfo(t, `Table accounts {
  region string [pk]
  slot   int    [pk]
  email  string [unique]
  handle string [index]
  login  string
  team   string
  code   string

  Indexes {
    (login, team) [unique]
    code [unique, where: `+"`region != ''`"+`]
    (team, `+"`lower(login)`"+`) [unique, name: "idx_accounts_team_login_ci"]
  }
}
`)
	ti := info.Table("accounts")
	for _, tc := range []struct {
		cols []string
		want bool
	}{
		{[]string{"region", "slot"}, true},
		{[]string{"slot", "region"}, true},
		{[]string{"region"}, false},
		{[]string{"email"}, true},
		{[]string{"handle"}, false},
		{[]string{"team", "login"}, true},
		{[]string{"login"}, false},
		{[]string{"code"}, false},
		{[]string{"team"}, false},
		{nil, false},
	} {
		if got := ti.ColumnsUnique(tc.cols); got != tc.want {
			t.Errorf("ColumnsUnique(%v) = %v, want %v", tc.cols, got, tc.want)
		}
	}
}
