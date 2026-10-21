package check_test

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

const migrationsSrc = `Table users {
  id   int    [pk, increment]
  name string
}

Migration 0010__users_name {
  alter table users {
    add name string [default: '']
  }
}

Migration 0002__exec_only [irreversible] {
  exec 'PRAGMA user_version = 2'
}

Migration 0001__create_users {
  create table users {
    id int [pk, increment]
  }
}

Migration 0003__up_only {
  up {
    create index users (name)
  }
}

Migration 0004__up_down {
  up {
    exec 'SELECT 1'
  }
  down {
  }
}
`

func TestMigrationsCollectedInNameOrder(t *testing.T) {
	f, diags := parser.ParseFile("db.volt", migrationsSrc)
	info, semDiags := check.File(f)
	if diags = append(diags, semDiags...); diag.HasErrors(diags) {
		t.Fatalf("unexpected errors: %v", diags)
	}
	var names, numbers, labels []string
	var reversible []bool
	for _, mi := range info.Migrations {
		names = append(names, mi.Name)
		numbers = append(numbers, mi.Number)
		labels = append(labels, mi.Label)
		reversible = append(reversible, mi.Reversible)
	}
	if got, want := strings.Join(names, " "), "0001__create_users 0002__exec_only 0003__up_only 0004__up_down 0010__users_name"; got != want {
		t.Fatalf("names = %s, want %s", got, want)
	}
	if got, want := strings.Join(numbers, " "), "0001 0002 0003 0004 0010"; got != want {
		t.Errorf("numbers = %s, want %s", got, want)
	}
	if got, want := strings.Join(labels, " "), "create_users exec_only up_only up_down users_name"; got != want {
		t.Errorf("labels = %s, want %s", got, want)
	}
	if want := []bool{true, false, false, true, true}; !equalBools(reversible, want) {
		t.Errorf("reversible = %v, want %v", reversible, want)
	}
}

func equalBools(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMigrationRebuildSourcesFollowRenames(t *testing.T) {
	src := `Table people {
  id   int    [pk]
  name string
}

Migration 0001__init {
  create table users {
    id        int    [pk]
    full_name string [null]
  }
}

Migration 0002__people [irreversible] {
  rename table users to people
  alter table people {
    rename full_name to name
  }
  rebuild table people {
    id   int    [pk]
    name string
  }
}

Migration 0003__unknown_table [irreversible] {
  rebuild table audit {
    id int [pk]
    at string
  }
}
`
	f, diags := parser.ParseFile("db.volt", src)
	_, semDiags := check.File(f)
	if diags = append(diags, semDiags...); diag.HasErrors(diags) {
		t.Fatalf("a renamed column is a source, an unknown table goes unchecked; got %v", diags)
	}
}
