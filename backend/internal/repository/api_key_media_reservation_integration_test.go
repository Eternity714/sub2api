//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepositoryReadsCurrentDurableMediaReservations(t *testing.T) {
	ctx := context.Background()
	tasks, task, _ := unifiedMediaIntegrationFixture(t)
	require.NoError(t, tasks.Create(ctx, task))
	repo := newAPIKeyRepositoryWithSQL(integrationEntClient, integrationDB)
	held, err := repo.GetReservedMediaAmount(ctx, task.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, 1.0, held)
	other, err := repo.GetReservedMediaAmount(ctx, task.APIKeyID+1000000)
	require.NoError(t, err)
	require.Zero(t, other)
	claims, err := tasks.ClaimDue(ctx, time.Now(), 100, time.Now().Add(time.Hour))
	require.NoError(t, err)
	for _, claim := range claims {
		if claim.ID == task.ID {
			for index := range claim.Units {
				claim.Units[index].State = "failed"
				claim.Units[index].Error = "synthetic_failure"
			}
			require.NoError(t, tasks.Finalize(ctx, claim))
		}
	}
	held, err = repo.GetReservedMediaAmount(ctx, task.APIKeyID)
	require.NoError(t, err)
	require.Zero(t, held)
}
