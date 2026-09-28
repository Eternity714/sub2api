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
				Body:    []byte(`{"model":"image","replyType":"` + mode + `"}`),
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

// The task price determines media handling without classifying the model name.
type grsaiVideoPriceStub struct{ price GrsaiTaskPrice }

func (p grsaiVideoPriceStub) GrsaiUnitPrice(context.Context, string, *Group) (float64, error) {
	return 0, ErrGrsaiSettlementPricingMissing
}
func (p grsaiVideoPriceStub) ResolveGrsaiTaskPrice(context.Context, string, *Group, string) (GrsaiTaskPrice, error) {
	return p.price, nil
}

func TestGrsaiTaskServiceVideoPricingAndRejectBeforeCreate(t *testing.T) {
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 2, VideoRateIndependent: true, VideoRateMultiplier: 1.5}
	rate := 1.2
	repo := &grsaiTaskCreateRepoStub{}
	svc := &GrsaiTaskService{Repo: repo, Balance: grsaiRuntimeBalanceStub{}, Pricing: grsaiVideoPriceStub{price: GrsaiTaskPrice{Mode: BillingModeVideo, UnitPrice: .14, Resolution: "768p"}}, Enabled: true}
	input := GrsaiTaskCreateInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey, RateMultiplier: &rate}, APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Body: []byte(`{"model":"minimax-h3","duration":5,"resolution":"768p","seed":9007199254740993}`)}
	_, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, repo.called)
	require.InDelta(t, 1.26, repo.params.BillableUnitPrice, 1e-10)
	require.Equal(t, 5, repo.params.VideoDurationSeconds)
	require.Equal(t, "768p", repo.params.VideoResolution)
	require.Equal(t, "video", repo.params.MediaKind)
	require.Equal(t, 3, repo.params.TaskVersion)
	require.Contains(t, string(repo.params.UpstreamPayload), `"seed":9007199254740993`)
	for _, body := range []string{`{"model":"minimax-h3","resolution":"768p"}`, `{"model":"minimax-h3","duration":5.0,"resolution":"768p"}`, `{"model":"minimax-h3","duration":11,"resolution":"1080p"}`} {
		repo.called = 0
		input.Body = []byte(body)
		_, err = svc.Create(context.Background(), input)
		require.Error(t, err)
		require.Zero(t, repo.called)
	}
}
