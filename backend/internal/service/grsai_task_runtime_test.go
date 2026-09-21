package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grsaiTaskNativeMemory struct {
	posts   int
	results int
	result  *GrsaiUpstreamResult
}

func (u *grsaiTaskNativeMemory) Generate(context.Context, *Account, []byte) (*GrsaiUpstreamResult, error) {
	u.posts++
	return u.result, nil
}
func (u *grsaiTaskNativeMemory) Result(context.Context, *Account, string) (*GrsaiUpstreamResult, error) {
	u.results++
	return u.result, nil
}

func TestBoundDisconnectRecoveryPollsResultWithoutSecondPost(t *testing.T) {
	tasks, repo, payloads, stream := grsaiTaskFixture(t)
	_ = payloads
	claim := &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: GrsaiUpstreamStatusRunning, ClaimVersion: 1, CreatedAt: time.Now().Add(-time.Minute)}
	id := "upstream-17"
	claim.UpstreamTaskID = &id
	repo.record = claim
	native := &grsaiTaskNativeMemory{result: &GrsaiUpstreamResult{HTTPStatus: 200, TaskID: id, Status: GrsaiUpstreamStatusRunning}}
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Upstream: native, Options: GrsaiTaskRuntimeOptions{Enabled: true}}
	runtime.runClaim(context.Background(), claim)
	require.Equal(t, 0, stream.posts)
	require.Equal(t, 1, native.results)
}

func TestQueuedTaskMissingPayloadMovesToManualReview(t *testing.T) {
	tasks, repo, _, upstream := grsaiTaskFixture(t)
	claim := &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	repo.record = claim
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Options: GrsaiTaskRuntimeOptions{Enabled: true}}
	runtime.runClaim(context.Background(), claim)
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.Equal(t, 0, upstream.posts)
}
