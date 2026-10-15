package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type mediaCapabilitySQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type mediaCapabilityRepository struct{ sql mediaCapabilitySQLExecutor }

var _ service.MediaCapabilityRepository = (*mediaCapabilityRepository)(nil)

func NewMediaCapabilityRepository(db *sql.DB) service.MediaCapabilityRepository {
	return &mediaCapabilityRepository{sql: db}
}

const mediaCapabilitySelectColumns = `id, public_model, media_type, schema_version, revision, enabled, schema, template_id, template_version, notes, created_at, updated_at`

type mediaCapabilityScanner interface{ Scan(...any) error }

func scanMediaCapability(scanner mediaCapabilityScanner) (*service.MediaCapability, error) {
	var row service.MediaCapability
	var templateID sql.NullString
	var templateVersion sql.NullInt64
	err := scanner.Scan(&row.ID, &row.PublicModel, &row.MediaType, &row.SchemaVersion, &row.Revision, &row.Enabled, &row.Schema, &templateID, &templateVersion, &row.Notes, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return nil, err
	}
	row.TemplateID = templateID.String
	if templateVersion.Valid {
		value := int(templateVersion.Int64)
		row.TemplateVersion = &value
	}
	return &row, nil
}
func mediaCapabilityRepositoryError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrMediaCapabilityNotFound
	}
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		return service.ErrMediaCapabilityExists
	}
	return err
}
func (r *mediaCapabilityRepository) Get(ctx context.Context, id int64) (*service.MediaCapability, error) {
	row, err := scanMediaCapability(r.sql.QueryRowContext(ctx, `SELECT `+mediaCapabilitySelectColumns+` FROM media_model_capabilities WHERE id = $1 AND deleted_at IS NULL`, id))
	return row, mediaCapabilityRepositoryError(err)
}
func (r *mediaCapabilityRepository) GetByModel(ctx context.Context, publicModel, mediaType string) (*service.MediaCapability, error) {
	row, err := scanMediaCapability(r.sql.QueryRowContext(ctx, `SELECT `+mediaCapabilitySelectColumns+` FROM media_model_capabilities WHERE public_model = $1 AND media_type = $2 AND deleted_at IS NULL`, publicModel, mediaType))
	return row, mediaCapabilityRepositoryError(err)
}
func (r *mediaCapabilityRepository) List(ctx context.Context, filter service.MediaCapabilityFilter) ([]service.MediaCapability, int64, error) {
	clauses := []string{"deleted_at IS NULL"}
	args := []any{}
	if filter.Search != "" {
		args = append(args, "%"+escapeLike(filter.Search)+"%")
		clauses = append(clauses, fmt.Sprintf("public_model ILIKE $%d ESCAPE '\\'", len(args)))
	}
	if filter.MediaType != "" {
		args = append(args, filter.MediaType)
		clauses = append(clauses, fmt.Sprintf("media_type = $%d", len(args)))
	}
	if filter.Enabled != nil {
		args = append(args, *filter.Enabled)
		clauses = append(clauses, fmt.Sprintf("enabled = $%d", len(args)))
	}
	where := strings.Join(clauses, " AND ")
	var total int64
	if err := r.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_model_capabilities WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	args = append(args, filter.PageSize, (int64(filter.Page)-1)*int64(filter.PageSize))
	rows, err := r.sql.QueryContext(ctx, `SELECT `+mediaCapabilitySelectColumns+` FROM media_model_capabilities WHERE `+where+fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]service.MediaCapability, 0)
	for rows.Next() {
		row, err := scanMediaCapability(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *row)
	}
	return out, total, rows.Err()
}
func (r *mediaCapabilityRepository) Create(ctx context.Context, row *service.MediaCapability) error {
	saved, err := scanMediaCapability(r.sql.QueryRowContext(ctx, `INSERT INTO media_model_capabilities (public_model,media_type,schema_version,revision,enabled,schema,template_id,template_version,notes) VALUES ($1,$2,$3,1,$4,$5::jsonb,$6,$7,$8) RETURNING `+mediaCapabilitySelectColumns, row.PublicModel, row.MediaType, row.SchemaVersion, row.Enabled, string(row.Schema), nullableMediaTemplateID(row.TemplateID), row.TemplateVersion, row.Notes))
	if err != nil {
		return mediaCapabilityRepositoryError(err)
	}
	*row = *saved
	return nil
}
func nullableMediaTemplateID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func (r *mediaCapabilityRepository) Update(ctx context.Context, row *service.MediaCapability, revision int64) error {
	saved, err := scanMediaCapability(r.sql.QueryRowContext(ctx, `UPDATE media_model_capabilities SET public_model=$1,media_type=$2,schema_version=$3,enabled=$4,schema=$5::jsonb,template_id=$6,template_version=$7,notes=$8,revision=revision+1,updated_at=NOW() WHERE id=$9 AND revision=$10 AND deleted_at IS NULL RETURNING `+mediaCapabilitySelectColumns, row.PublicModel, row.MediaType, row.SchemaVersion, row.Enabled, string(row.Schema), nullableMediaTemplateID(row.TemplateID), row.TemplateVersion, row.Notes, row.ID, revision))
	if errors.Is(err, sql.ErrNoRows) {
		return r.classifyMediaCapabilityCAS(ctx, row.ID)
	}
	if err != nil {
		return mediaCapabilityRepositoryError(err)
	}
	*row = *saved
	return nil
}
func (r *mediaCapabilityRepository) Delete(ctx context.Context, id, revision int64) error {
	result, err := r.sql.ExecContext(ctx, `UPDATE media_model_capabilities SET deleted_at=NOW(),updated_at=NOW(),revision=revision+1 WHERE id=$1 AND revision=$2 AND deleted_at IS NULL`, id, revision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return r.classifyMediaCapabilityCAS(ctx, id)
	}
	return nil
}
func (r *mediaCapabilityRepository) classifyMediaCapabilityCAS(ctx context.Context, id int64) error {
	_, err := r.Get(ctx, id)
	if err != nil {
		return err
	}
	return service.ErrMediaCapabilityConflict
}
