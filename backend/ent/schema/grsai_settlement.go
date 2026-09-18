package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// GrsaiSettlement stores the sanitized, durable state required to settle a
// native GRS.AI image request. Request payloads and generated media do not
// belong in this table.
type GrsaiSettlement struct {
	ent.Schema
}

func (GrsaiSettlement) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "grsai_settlements"},
	}
}

func (GrsaiSettlement) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("group_id"),
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.String("model").MaxLen(128),
		field.Float("base_unit_price").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("group_rate_multiplier").SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),
		field.Float("account_rate_multiplier").SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),
		field.Float("billable_unit_price").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Int("requested_image_count"),
		field.String("currency").MaxLen(16).Default("USD"),
		field.String("billing_idempotency_key").MaxLen(128).Immutable(),
		field.String("upstream_task_id").MaxLen(255).Optional().Nillable(),
		field.String("upstream_status").MaxLen(32).Default("not_submitted"),
		field.String("internal_status").MaxLen(32).Default("pending_upstream"),
		field.Int("retry_count").Default(0),
		field.Int64("claim_version").Default(0),
		field.Time("next_attempt_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("last_error_summary").MaxLen(1024).Optional().Nillable(),
		field.Float("settled_amount").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("upstream_bound_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("result_updated_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("settled_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("closed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (GrsaiSettlement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("billing_idempotency_key").Unique(),
		index.Fields("account_id", "upstream_task_id").
			Unique().
			Annotations(entsql.IndexWhere("upstream_task_id IS NOT NULL AND upstream_task_id <> ''")),
		index.Fields("internal_status", "next_attempt_at").
			Annotations(entsql.IndexWhere("internal_status IN ('pending_upstream', 'pending_settlement', 'processing')")),
		index.Fields("user_id", "created_at"),
	}
}
