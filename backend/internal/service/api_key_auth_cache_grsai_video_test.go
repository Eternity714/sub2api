package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotPreservesGrsaiVideoPrices(t *testing.T) {
	t.Parallel()
	prices := map[string]map[string]float64{
		"minimax-h3": {"480p": 0.10, "768p": 0.14, "1080p": 0.30},
	}
	svc := &APIKeyService{}

	t.Run("encode", func(t *testing.T) {
		snapshot := svc.snapshotFromAPIKey(context.Background(), &APIKey{
			User: &User{ID: 1},
			Group: &Group{
				ID: 1, Platform: PlatformGrsai, VideoModelPrices: prices,
			},
		})
		require.NotNil(t, snapshot)
		require.NotNil(t, snapshot.Group)
		require.Equal(t, prices, snapshot.Group.VideoModelPrices)
	})

	t.Run("decode", func(t *testing.T) {
		key := svc.snapshotToAPIKey("unit-test-key", &APIKeyAuthSnapshot{
			User: APIKeyAuthUserSnapshot{ID: 1},
			Group: &APIKeyAuthGroupSnapshot{
				ID: 1, Platform: PlatformGrsai, VideoModelPrices: prices,
			},
		})
		require.NotNil(t, key)
		require.NotNil(t, key.Group)
		require.Equal(t, prices, key.Group.VideoModelPrices)
	})
}
