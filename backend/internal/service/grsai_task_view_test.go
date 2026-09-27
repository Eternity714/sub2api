package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGrsaiTaskViewOmitsPrivateFieldsAndReportsLinkExpiry(t *testing.T) {
	id, upstreamID := "grsai-local", "upstream-secret"
	now := time.Now().UTC()
	expires := now.Add(-time.Second)
	record := &GrsaiSettlement{ID: 7, AccountID: 8, APIKeyID: 9, UserID: 10,
		LocalTaskID: &id, UpstreamTaskID: &upstreamID, PublicStatus: "succeeded", Progress: 100,
		Model: "image", ResultJSON: []byte(`{"results":[{"url":"https://local.example/a"}]}`),
		ImageObjectMetadata: []byte(`[{"object_key":"private-key"}]`), LinkExpiresAt: &expires,
		CreatedAt: now.Add(-time.Minute), ClosedAt: &now}
	view := NewGrsaiTaskView(record, now)
	encoded, err := json.Marshal(view)
	require.NoError(t, err)
	require.Equal(t, id, view.ID)
	require.True(t, view.LinkExpired)
	require.Contains(t, string(encoded), "https://local.example/a")
	require.NotContains(t, string(encoded), upstreamID)
	require.NotContains(t, string(encoded), "private-key")
	require.NotContains(t, string(encoded), "account_id")
	require.NotContains(t, string(encoded), "api_key_id")
	require.NotContains(t, string(encoded), "billable")
}

func TestGrsaiTaskViewPublicLinkHasNoExpiry(t *testing.T) {
	id := "grsai-local"
	view := NewGrsaiTaskView(&GrsaiSettlement{LocalTaskID: &id, PublicStatus: "succeeded",
		ResultJSON: []byte(`{"results":[{"url":"https://public.example/a"}]}`)}, time.Now())
	require.Nil(t, view.LinkExpiresAt)
	require.False(t, view.LinkExpired)
}
