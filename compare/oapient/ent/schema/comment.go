package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Comment holds the schema definition for the Comment entity.
type Comment struct {
	ent.Schema
}

// Fields of the Comment.
func (Comment) Fields() []ent.Field {
	return []ent.Field{
		field.Int("post_id"),
		field.Int("member_id"),
		field.Text("body").NotEmpty().Validate(maxRunes(1000)),
		field.Time("created_at").Default(now).Immutable(),
	}
}

// Edges of the Comment.
func (Comment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("post", Post.Type).Ref("comments").Field("post_id").Unique().Required(),
		edge.From("member", Member.Type).Ref("comments").Field("member_id").Unique().Required(),
	}
}

// Indexes of the Comment.
func (Comment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("post_id", "member_id", "created_at"),
	}
}
