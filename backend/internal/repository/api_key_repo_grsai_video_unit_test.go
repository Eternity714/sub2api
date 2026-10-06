package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityToServicePreservesGrsaiVideoPrices(t *testing.T) {
	t.Parallel()
	prices := map[string]map[string]float64{
		"minimax-h3": {"480p": 0.10, "768p": 0.14, "1080p": 0.30},
	}
	got := groupEntityToService(&dbent.Group{
		ID: 1, Platform: service.PlatformGrsai, VideoModelPrices: prices,
	})
	require.Equal(t, prices, got.VideoModelPrices)
}
