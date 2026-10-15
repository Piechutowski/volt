package inflect

import "testing"

func TestSingular(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"users", "user", true},
		{"posts", "post", true},
		{"comments", "comment", true},
		{"orders", "order", true},
		{"tags", "tag", true},

		{"categories", "category", true},
		{"companies", "company", true},
		{"cities", "city", true},
		{"ties", "tie", true},

		{"boxes", "box", true},
		{"churches", "church", true},
		{"dishes", "dish", true},
		{"classes", "class", true},
		{"houses", "house", true},
		{"cases", "case", true},
		{"databases", "database", true},
		{"releases", "release", true},

		{"people", "person", true},
		{"children", "child", true},
		{"statuses", "status", true},
		{"aliases", "alias", true},
		{"heroes", "hero", true},
		{"quizzes", "quiz", true},
		{"analyses", "analysis", true},
		{"addresses", "address", true},
		{"processes", "process", true},
		{"movies", "movie", true},
		{"news", "news", true},
		{"series", "series", true},

		{"status", "status", false},
		{"bonus", "bonus", false},
		{"menus", "menus", false},
		{"analysis", "analysis", false},
		{"axis", "axis", false},

		{"user", "user", true},
		{"person", "person", true},
		{"staff", "staff", true},

		{"Users", "User", true},
		{"PEOPLE", "PERSON", true},
	}
	for _, tc := range cases {
		got, ok := Singular(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Singular(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestSingularLast(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"order_items", "order_item", true},
		{"user_roles", "user_role", true},
		{"users", "user", true},
		{"blog_categories", "blog_category", true},
		{"account_statuses", "account_status", true},
		{"order_status", "order_status", false},
		{"trailing_", "trailing_", true},
	}
	for _, tc := range cases {
		got, ok := SingularLast(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("SingularLast(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
