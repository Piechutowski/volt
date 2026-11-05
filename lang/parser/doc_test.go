package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/parser"
)

func docsOf(f *ast.File) string {
	var b strings.Builder
	for _, d := range f.Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			if doc := ast.DocOf(n); doc != nil {
				fmt.Fprintf(&b, "%d %T %q\n", n.Pos().Line(), n, doc.Text())
			}
			return true
		})
		part2Docs(&b, d)
		if g, ok := d.(*ast.Group); ok {
			for _, term := range g.Terms {
				if term.Doc != nil {
					fmt.Fprintf(&b, "%d member %q\n", term.Names[0].Pos().Line(), term.Doc.Text())
				}
			}
		}
	}
	return b.String()
}

func part2Docs(b *strings.Builder, n ast.Node) {
	line := func(n ast.Node) {
		if doc := ast.DocOf(n); doc != nil {
			fmt.Fprintf(b, "%d %T %q\n", n.Pos().Line(), n, doc.Text())
		}
	}
	switch x := n.(type) {
	case *ast.ImportDecl:
		for _, s := range x.Specs {
			line(s)
		}
	case *ast.Pipeline:
		for _, p := range x.Plugs {
			line(p)
		}
	case *ast.Scope:
		for _, item := range x.Items {
			if s, ok := item.(*ast.Scope); ok {
				line(s)
				part2Docs(b, s)
				continue
			}
			line(item)
			if r, ok := item.(*ast.Route); ok {
				for _, ld := range r.Loads {
					line(ld)
				}
			}
		}
	}
}

func TestDocCommentsAttach(t *testing.T) {
	src := `// File header: the blank line below keeps it apart.

// The blog.
//
// Posts and their writers.
package db

import (
	// the metrics package
	metrics/store
)

// A blog post.
table posts {
  // Shown in lists.
  title string // a trailing comment documents nothing
  id int [pk]

  // free comment, separated by a blank line

  author_id int
  ~stamps
  // indexes of posts
  indexes {
    // by author
    (author_id, id)
  }
  // validators of posts
  validators {
    // a title is never blank
    TitleShape(title)
  }
  // sample posts
  records (id, title, author_id) {
    // the first post
    1, "Hello", 1
  }
  // a group above the closing brace documents nothing
}

// Fields every table stamps.
table_partial stamps {
  // set on insert
  created_at time.Time
}

// written once, then shadowed
// by this second line of the same group
type Slug string

// Who wrote it.
ref: posts.author_id > posts.id

// What readers see.
group content {
  // posts are content
  posts
}

// Published posts.
pred published { id > 0 }

// A post's list entry.
select summary (id, title) for content

// Sample data.
records posts (id, title, author_id) {
  2, "Two", 1
}

// The first migration.
migration 0001__init {
  // create the posts
  create table posts {
    // the key
    id int [pk]
  }
}
`
	f, diags := parser.ParseFile("db/schema.volt", src)
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	want := `6 *ast.PackageClause "The blog.\n\nPosts and their writers."
10 *ast.ImportSpec "the metrics package"
14 *ast.Table "A blog post."
16 *ast.Column "Shown in lists."
24 *ast.IndexesBlock "indexes of posts"
26 *ast.Index "by author"
29 *ast.ValidatorsBlock "validators of posts"
31 *ast.Validator "a title is never blank"
34 *ast.Records "sample posts"
36 *ast.RecordRow "the first post"
42 *ast.TablePartial "Fields every table stamps."
44 *ast.Column "set on insert"
49 *ast.TypeDecl "written once, then shadowed\nby this second line of the same group"
52 *ast.Ref "Who wrote it."
55 *ast.Group "What readers see."
57 member "posts are content"
61 *ast.Pred "Published posts."
64 *ast.Select "A post's list entry."
67 *ast.Records "Sample data."
72 *ast.Migration "The first migration."
74 *ast.CreateTable "create the posts"
76 *ast.Column "the key"
`
	if got := docsOf(f); got != want {
		t.Errorf("docs:\n%s\nwant:\n%s", got, want)
	}
}

func TestDocCommentsAttachInRoutes(t *testing.T) {
	src := `package app

// The browser stack.
pipeline browser {
  // reads the session
  use Session
}

// The site.
scope / {
  // the front page
  get / Pages.Home {
    // the posts it lists
    posts db.PostList()
  }
  // posts, the whole set
  resources Post
  // the current user
  load user CurrentUser
  // every query of browse
  dataset db.browse
  // the admin area
  scope /admin {
    get / Admin.Home
  }
}
`
	f, diags := parser.ParseFile("app/routes.volt", src)
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	want := `4 *ast.Pipeline "The browser stack."
6 *ast.Plug "reads the session"
10 *ast.Scope "The site."
12 *ast.Route "the front page"
14 *ast.Load "the posts it lists"
17 *ast.Resources "posts, the whole set"
19 *ast.ScopeLoad "the current user"
21 *ast.Dataset "every query of browse"
23 *ast.Scope "the admin area"
`
	if got := docsOf(f); got != want {
		t.Errorf("docs:\n%s\nwant:\n%s", got, want)
	}
}

func TestDocCommentText(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"// one\ntable t {\n  id int\n}\n", "one"},
		{"//no space\ntable t {\n  id int\n}\n", "no space"},
		{"//  two spaces keep one\ntable t {\n  id int\n}\n", " two spaces keep one"},
		{"//\n// a\n//\n//\n// b\n//\ntable t {\n  id int\n}\n", "a\n\nb"},
		{"// trailing   \t\ntable t {\n  id int\n}\n", "trailing"},
		{"// a \\n and \"quotes\" stay\ntable t {\n  id int\n}\n", `a \n and "quotes" stay`},
		{"\t// indented\ntable t {\n  id int\n}\n", "indented"},
		{"// crlf line  \r\n// ends\r\ntable t {\r\n  id int\r\n}\r\n", "crlf line\nends"},
	} {
		f, diags := parser.ParseFile("t.volt", tc.src)
		if len(diags) != 0 {
			t.Fatalf("%q: diagnostics: %v", tc.src, diags)
		}
		if got := f.Decls[0].(*ast.Table).Doc.Text(); got != tc.want {
			t.Errorf("%q: text %q, want %q", tc.src, got, tc.want)
		}
	}
}
