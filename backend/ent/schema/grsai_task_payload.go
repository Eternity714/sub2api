package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// GrsaiTaskPayload stores only the encrypted original request for an async
// GRS.AI task. Its primary key is also the owning settlement foreign key.
type GrsaiTaskPayload struct {
	ent.Schema
}

var grsaiTaskPayloadIncremental = false

func (GrsaiTaskPayload) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "grsai_task_payloads"},
	}
}

func (GrsaiTaskPayload) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			StorageKey("settlement_id").
			Immutable().
			Annotations(entsql.Annotation{Incremental: &grsaiTaskPayloadIncremental}),
		field.String("ciphertext"),
		field.Time("expires_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (GrsaiTaskPayload) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("settlement", GrsaiSettlement.Type).
			Ref("task_payload").
			Required().
			Unique().
			Immutable(),
	}
}

func (GrsaiTaskPayload) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("expires_at"),
	}
}
