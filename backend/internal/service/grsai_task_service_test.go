package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grsaiTaskCreateRepoStub struct {
	GrsaiV2TaskRepository
	params CreateV2GrsaiTaskParams
	called int
}

func (r *grsaiTaskCreateRepoStub) CreateV2GrsaiTask(_ context.Context, params CreateV2GrsaiTaskParams, _ GrsaiSettlementTxFunc) (*GrsaiSettlement, error) {
	r.called++
	r.params = params
	id := params.LocalTaskID
	return &GrsaiSettlement{LocalTaskID: &id, DeliveryMode: params.DeliveryMode}, nil
}

func TestGrsaiTaskServiceCreatesLocalTaskBeforeAsyncSubmission(t *testing.T) {
	for _, mode := range []string{"json", "stream", "async"} {
		t.Run(mode, func(t *testing.T) {
			repo := &grsaiTaskCreateRepoStub{}
			group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
			svc := &GrsaiTaskService{Repo: repo, Balance: grsaiRuntimeBalanceStub{}, Pricing: &grsaiPriceStub{price: 0.25},
				Enabled: true, PayloadTTL: time.Hour, MaxWaiting: 20}
			record, err := svc.Create(context.Background(), GrsaiTaskCreateInput{
				Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey},
				APIKey:  &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group},
				Body:    []byte(`{"model":"image","n":2,"replyType":"` + mode + `"}`),
			})
			require.NoError(t, err)
			require.Equal(t, 1, repo.called)
			require.Equal(t, mode, record.DeliveryMode)
			require.Contains(t, string(repo.params.UpstreamPayload), `"replyType":"async"`)
			require.NotEmpty(t, *record.LocalTaskID)
			require.Equal(t, 20, repo.params.MaxWaiting)
			require.InDelta(t, 0.25, repo.params.BillableUnitPrice, 0.000001)
		})
	}
}

func TestGrsaiTaskServiceDisabledDoesNotCreate(t *testing.T) {
	repo := &grsaiTaskCreateRepoStub{}
	svc := &GrsaiTaskService{Repo: repo}
	_, err := svc.Create(context.Background(), GrsaiTaskCreateInput{})
	require.ErrorIs(t, err, ErrGrsaiDeliveryDisabled)
	require.Zero(t, repo.called)
}
