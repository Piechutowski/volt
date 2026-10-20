package parser

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
)

const migrationSample = `package db

Migration 0001__create_users {
  create table users {
    id    int    [pk, increment]
    email string [not null, unique]

    indexes {
      email [unique]
    }
  }
  create index users (email)
}

Migration 0002__users_bio [irreversible, note: 'rebuild'] {
  up {
    alter table users {
      add bio string
      rename name to full_name
      drop legacy string [not null]
    }
    rename table users to people
    rebuild table people {
      id  int [pk, increment]
      bio string [from: ` + "`bio`" + `]
    }
    drop index people (email) [unique]
    exec 'VACUUM' [down: 'SELECT 1']
  }
  down {
    drop table people {
      id int [pk]
    }
  }
}
`

func TestMigrationSampleParses(t *testing.T) {
	f, diags := ParseFile("db.volt", migrationSample)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Decls) != 3 {
		t.Fatalf("decls = %d, want 3", len(f.Decls))
	}
	first, ok := f.Decls[1].(*ast.Migration)
	if !ok || first.Name.Name() != "0001__create_users" || first.UpDown || len(first.Up) != 2 {
		t.Fatalf("first migration = %#v", f.Decls[1])
	}
	ct, ok := first.Up[0].(*ast.CreateTable)
	if !ok || ct.Name.Name() != "users" || len(ct.Body) != 3 {
		t.Fatalf("create table = %#v", first.Up[0])
	}
	second := f.Decls[2].(*ast.Migration)
	if !second.UpDown || !second.HasDown() || len(second.Up) != 5 || len(second.Down) != 1 {
		t.Fatalf("second migration: UpDown=%v HasDown=%v up=%d down=%d", second.UpDown, second.HasDown(), len(second.Up), len(second.Down))
	}
	kinds := []string{}
	for _, op := range second.Up {
		switch op := op.(type) {
		case *ast.AlterTable:
			kinds = append(kinds, "alter")
			if len(op.Items) != 3 {
				t.Errorf("alter items = %d, want 3", len(op.Items))
			}
			if d, ok := op.Items[2].(*ast.AlterDrop); !ok || d.Type == nil || d.Settings == nil {
				t.Errorf("drop item = %#v, want a typed drop with settings", op.Items[2])
			}
		case *ast.RenameTable:
			kinds = append(kinds, "rename")
		case *ast.RebuildTable:
			kinds = append(kinds, "rebuild")
		case *ast.DropIndex:
			kinds = append(kinds, "dropindex")
			if !op.Index.Composite || op.Index.Settings == nil {
				t.Errorf("drop index = %#v", op.Index)
			}
		case *ast.Exec:
			kinds = append(kinds, "exec")
			if op.DownSQL() == nil || op.DownSQL().Tok.Val != "SELECT 1" {
				t.Errorf("exec down = %v", op.DownSQL())
			}
		}
	}
	if got, want := strings.Join(kinds, ","), "alter,rename,rebuild,dropindex,exec"; got != want {
		t.Fatalf("up operations = %s, want %s", got, want)
	}
	if dt, ok := second.Down[0].(*ast.DropTable); !ok || !dt.HasBody() {
		t.Fatalf("down operation = %#v, want drop table with a body", second.Down[0])
	}
}

func TestMigrationWordsAreIdentifiersElsewhere(t *testing.T) {
	src := "Table t {\n  up int\n  to string\n  table string\n  create string\n  drop string\n  add string\n}\n"
	f, diags := ParseFile("t.volt", src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	tbl := f.Decls[0].(*ast.Table)
	if len(tbl.Body) != 6 {
		t.Fatalf("columns = %d, want 6", len(tbl.Body))
	}
	if col := tbl.Body[0].(*ast.Column); col.Name.Name() != "up" {
		t.Fatalf("first column = %q, want up", col.Name.Name())
	}
}

func TestMigrationSyntaxErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{"Migration 0001__a {\n  down {\n  }\n}\n", "a down block follows an up block"},
		{"Migration 0001__a {\n  up {\n  }\n  create table t {\n    id int\n  }\n}\n", "expected '}' in Migration"},
		{"Migration 0001__a {\n  rename t to u\n}\n", "expected 'table' after rename"},
		{"Migration 0001__a {\n  create\n  index t id\n}\n", "expected end of line after create"},
		{"Migration 0001__a {\n  alter table t {\n    drop bio [pk]\n  }\n}\n", "expected end of line after drop column"},
		{"Migration 0001__a {\n  create table t {\n    ~base\n  }\n}\n", "not part of it"},
		{"Migration 0001__a\n[irreversible] {\n}\n", "expected '{' in Migration"},
	}
	for _, tc := range cases {
		_, diags := ParseFile("t.volt", tc.src)
		found := false
		for _, d := range diags {
			if strings.Contains(d.Msg, tc.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: no diagnostic contains %q; got %v", tc.src, tc.want, diags)
		}
	}
}
