package service

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGrsaiLocalTaskIDIsUniqueAndOpaque(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		id := NewGrsaiLocalTaskID()
		require.True(t, strings.HasPrefix(id, "media_"))
		_, err := uuid.Parse(strings.TrimPrefix(id, "media_"))
		require.NoError(t, err)
		_, exists := seen[id]
		require.False(t, exists)
		seen[id] = struct{}{}
	}
}
