//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMediaCapabilityRepositoryIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	model := "media-test-" + uuid.NewString()
	repo := NewMediaCapabilityRepository(integrationDB)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM media_model_capabilities WHERE public_model=$1`, model)
	})
	makeRow := func(media string) *service.MediaCapability {
		return &service.MediaCapability{PublicModel: model, MediaType: media, SchemaVersion: 1, Schema: json.RawMessage(`{"version":1,"fields":{}}`)}
	}
	image := makeRow("image")
	require.NoError(t, repo.Create(ctx, image))
	require.Positive(t, image.ID)
	require.EqualValues(t, 1, image.Revision)
	require.False(t, image.Enabled)
	require.ErrorIs(t, repo.Create(ctx, makeRow("image")), service.ErrMediaCapabilityExists)
	video := makeRow("video")
	require.NoError(t, repo.Create(ctx, video))
	require.NotEqual(t, image.ID, video.ID)
	rows, total, err := repo.List(ctx, service.MediaCapabilityFilter{Search: model, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.EqualValues(t, 2, total)
	found, err := repo.GetByModel(ctx, model, "image")
	require.NoError(t, err)
	require.Equal(t, image.ID, found.ID)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); copy := *image; copy.Enabled = true; results <- repo.Update(ctx, &copy, 1) }()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrMediaCapabilityConflict):
			conflict++
		default:
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)
	require.ErrorIs(t, repo.Delete(ctx, image.ID, 1), service.ErrMediaCapabilityConflict)
	require.NoError(t, repo.Delete(ctx, image.ID, 2))
	_, err = repo.Get(ctx, image.ID)
	require.ErrorIs(t, err, service.ErrMediaCapabilityNotFound)
	replacement := makeRow("image")
	require.NoError(t, repo.Create(ctx, replacement))
	require.NotEqual(t, image.ID, replacement.ID)
}
