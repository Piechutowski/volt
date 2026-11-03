package schema

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Post holds the schema definition for the Post entity.
type Post struct {
	ent.Schema
}

// Fields of the Post.
func (Post) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").Unique().Match(regexp.MustCompile(`^[a-z0-9-]{1,60}$`)),
		field.String("title").NotEmpty().Validate(maxRunes(200)),
		field.Text("body"),
		field.Enum("state").Values("draft", "published").Default("draft"),
		field.Int("author_id"),
		field.Time("published_at").Optional().Nillable(),
		field.Time("created_at").Default(now).Immutable(),
	}
}

// Edges of the Post.
func (Post) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", Member.Type).Ref("posts").Field("author_id").Unique().Required(),
		edge.To("tags", Tag.Type),
		edge.To("comments", Comment.Type),
		edge.To("audits", Audit.Type),
	}
}

// Indexes of the Post.
func (Post) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("state", "published_at"),
	}
}

func maxRunes(n int) func(string) error {
	return func(s string) error {
		if utf8.RuneCountInString(s) > n {
			return fmt.Errorf("at most %d characters", n)
		}
		return nil
	}
}
