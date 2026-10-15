//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type mediaCapabilityMemoryRepository struct {
	rows   map[int64]*MediaCapability
	writes int
}

func (r *mediaCapabilityMemoryRepository) List(context.Context, MediaCapabilityFilter) ([]MediaCapability, int64, error) {
	out := make([]MediaCapability, 0, len(r.rows))
	for _, row := range r.rows {
		out = append(out, *row)
	}
	return out, int64(len(out)), nil
}
func (r *mediaCapabilityMemoryRepository) Get(_ context.Context, id int64) (*MediaCapability, error) {
	if row := r.rows[id]; row != nil {
		cp := *row
		return &cp, nil
	}
	return nil, ErrMediaCapabilityNotFound
}
func (r *mediaCapabilityMemoryRepository) GetByModel(_ context.Context, model, media string) (*MediaCapability, error) {
	for _, row := range r.rows {
		if row.PublicModel == model && row.MediaType == media {
			cp := *row
			return &cp, nil
		}
	}
	return nil, ErrMediaCapabilityNotFound
}
func (r *mediaCapabilityMemoryRepository) Create(_ context.Context, row *MediaCapability) error {
	for _, old := range r.rows {
		if old.PublicModel == row.PublicModel && old.MediaType == row.MediaType {
			return ErrMediaCapabilityExists
		}
	}
	r.writes++
	row.ID = int64(len(r.rows) + 1)
	row.Revision = 1
	cp := *row
	r.rows[row.ID] = &cp
	return nil
}
func (r *mediaCapabilityMemoryRepository) Update(_ context.Context, row *MediaCapability, revision int64) error {
	old := r.rows[row.ID]
	if old == nil {
		return ErrMediaCapabilityNotFound
	}
	if old.Revision != revision {
		return ErrMediaCapabilityConflict
	}
	r.writes++
	row.Revision = revision + 1
	cp := *row
	r.rows[row.ID] = &cp
	return nil
}
func (r *mediaCapabilityMemoryRepository) Delete(_ context.Context, id, revision int64) error {
	old := r.rows[id]
	if old == nil {
		return ErrMediaCapabilityNotFound
	}
	if old.Revision != revision {
		return ErrMediaCapabilityConflict
	}
	r.writes++
	delete(r.rows, id)
	return nil
}
func validMediaCapabilityInput() MediaCapabilityInput {
	return MediaCapabilityInput{PublicModel: "example", MediaType: "image", Schema: json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true},"n":{"type":"integer","minimum":1,"maximum":8,"default":1},"metadata":{"type":"object","properties":{"resolution":{"type":"string","enum":["1K","2K"]}}}}}`)}
}
func TestMediaCapabilityServiceRejectsInvalidSchemaBeforeWrite(t *testing.T) {
	repo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{}}
	svc := NewMediaCapabilityService(repo)
	input := validMediaCapabilityInput()
	input.Schema = json.RawMessage(`{"version":1,"fields":{"duration":{"type":"number"}}}`)
	_, err := svc.Create(context.Background(), input)
	require.Error(t, err)
	require.Equal(t, 400, infraerrors.Code(err))
	require.Zero(t, repo.writes)
}
// AC-016 (configuration scope): the same public model can persist independent image and video capabilities.
func TestMediaCapabilityServiceDualMediaAndCAS(t *testing.T) {
	repo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{}}
	svc := NewMediaCapabilityService(repo)
	ctx := context.Background()
	input := validMediaCapabilityInput()
	image, err := svc.Create(ctx, input)
	require.NoError(t, err)
	require.False(t, image.Enabled)
	require.EqualValues(t, 1, image.Revision)
	input.MediaType = "video"
	input.Schema = json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string"}}}`)
	video, err := svc.Create(ctx, input)
	require.NoError(t, err)
	require.NotEqual(t, image.ID, video.ID)
	_, err = svc.Create(ctx, input)
	require.ErrorIs(t, err, ErrMediaCapabilityExists)
	input = validMediaCapabilityInput()
	input.Notes = "first edit"
	updated, err := svc.Update(ctx, image.ID, 1, input)
	require.NoError(t, err)
	require.EqualValues(t, 2, updated.Revision)
	_, err = svc.Update(ctx, image.ID, 1, input)
	require.ErrorIs(t, err, ErrMediaCapabilityConflict)
	err = svc.Delete(ctx, image.ID, 1)
	require.ErrorIs(t, err, ErrMediaCapabilityConflict)
}
func TestMediaCapabilityServiceDryRunNeverWritesAndNormalizes(t *testing.T) {
	repo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{}}
	svc := NewMediaCapabilityService(repo)
	out, err := svc.Validate(context.Background(), validMediaCapabilityInput(), `{"model":"example","prompt":"test","metadata":{"resolution":"1K"}}`)
	require.NoError(t, err)
	require.True(t, out.Valid)
	require.NotNil(t, out.Normalized)
	require.Equal(t, 1, out.Normalized.RequestedCount)
	require.Zero(t, repo.writes)
	bad, err := svc.Validate(context.Background(), validMediaCapabilityInput(), `{"model":"example","prompt":"test","duration":5}`)
	require.NoError(t, err)
	require.False(t, bad.Valid)
	require.NotEmpty(t, bad.Errors)
	require.Nil(t, bad.Normalized)
	require.Zero(t, repo.writes)
}
func TestMediaCapabilityTemplatesAreCompileableCopies(t *testing.T) {
	svc := NewMediaCapabilityService(&mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{}})
	templates := svc.Templates()
	require.GreaterOrEqual(t, len(templates), 2)
	require.Equal(t, "basic-image-v1", templates[0].ID)
	require.Equal(t, "basic-video-v1", templates[1].ID)
	for _, tpl := range templates {
		require.False(t, tpl.Enabled, tpl.ID)
		_, err := svc.Validate(context.Background(), MediaCapabilityInput{PublicModel: "template-model", MediaType: tpl.MediaType, Schema: tpl.Schema}, `{"model":"template-model","prompt":"a scene"}`)
		require.NoError(t, err, tpl.ID)
	}
	templates[0].Schema[0] = 'x'
	require.Equal(t, byte('{'), svc.Templates()[0].Schema[0])
}

// AC-017 (service scope): a disabled capability is rejected before active-request validation succeeds.
func TestMediaCapabilityServiceActiveSnapshotIsIndependent(t *testing.T) {
	repo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{}}
	svc := NewMediaCapabilityService(repo)
	ctx := context.Background()
	input := validMediaCapabilityInput()
	row, err := svc.Create(ctx, input)
	require.NoError(t, err)
	_, err = svc.ValidateActive(ctx, "example", "image", []byte(`{"model":"example","prompt":"test"}`))
	require.Equal(t, 400, infraerrors.Code(err))
	input.Enabled = true
	row, err = svc.Update(ctx, row.ID, row.Revision, input)
	require.NoError(t, err)
	out, err := svc.ValidateActive(ctx, "example", "image", []byte(`{"model":"example","prompt":"test"}`))
	require.NoError(t, err)
	require.EqualValues(t, 2, out.Capability.Revision)
	require.Len(t, out.Capability.SchemaHash, 64)
	original := string(out.Capability.Schema)
	input.Schema = json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string"}}}`)
	_, err = svc.Update(ctx, row.ID, row.Revision, input)
	require.NoError(t, err)
	require.Equal(t, original, string(out.Capability.Schema))
	require.EqualValues(t, 2, out.Capability.Revision)
}
