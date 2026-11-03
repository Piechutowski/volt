package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Audit holds the schema definition for the Audit entity.
type Audit struct {
	ent.Schema
}

// Fields of the Audit.
func (Audit) Fields() []ent.Field {
	return []ent.Field{
		field.Int("post_id"),
		field.Int("member_id"),
		field.String("action").NotEmpty(),
		field.Time("at").Default(now).Immutable(),
	}
}

// Edges of the Audit.
func (Audit) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("post", Post.Type).Ref("audits").Field("post_id").Unique().Required(),
		edge.From("member", Member.Type).Ref("audits").Field("member_id").Unique().Required(),
	}
}
