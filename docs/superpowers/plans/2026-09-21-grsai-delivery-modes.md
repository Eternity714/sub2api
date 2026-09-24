# GRS.AI 三种下游交付方式 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** 为 GRS.AI 原生图片接口提供 json、stream、async 三种下游交付方式，以及可恢复任务查询、额度冻结和成功后一次结算。

**Architecture:** 客户端 replyType 只决定 Sub2API 的交付方式；服务端解析后以全新对象重建上游请求，GRS.AI 上游调用始终为 replyType=stream。持久化任务状态机保存进度、上游任务 ID、价格快照和冻结状态；JSON、直连 SSE、Async Worker 与断线恢复共用该状态机。

**Tech Stack:** Go、Gin、PostgreSQL、Ent schema/SQL migration、Wire、database/sql 事务、Go net/http SSE、既有 AES SecretEncryptor、Go tests、Python 标准库 live contract probe。

**Spec:** docs/superpowers/specs/2026-09-21-grsai-delivery-modes-prd.md；docs/superpowers/specs/2026-09-21-grsai-delivery-modes-design.md

## Global Constraints

- 上游只能发送 replyType=stream；禁止上游 JSON/Async 分支。
- 下游只接受 replyType: "json" | "stream" | "async"，缺省 json；出现 stream 或 async 布尔字段即 400。
- 输入请求字节只读。解码 JSON object 后新建 map[string]json.RawMessage，逐字段复制值，最后唯一地设 replyType="stream"，再重新编码；不得原地修改或复用可变对象。
- 不提供任务取消；客户端断线不是取消，也绝不重投可能已被上游接收的任务。
- 提交时冻结额度；仅 succeeded 在幂等事务内捕获冻结并记一次用量。失败、违规、未知提交和人工关闭都释放冻结。
- Async 原始载荷须加密持久化，且不得进入日志、公开 API、测试产物或用量记录。绑定上游 ID 后删除；终态安全查询视图保存 24 小时。
- Async 跨 API Key 按用户限额：等待中 20、进行中 3。受理/领取均需数据库原子门槛；等待满额 429 且不冻结；进行中满额不得 POST，租约恢复不得重置进行中标记。
- GET /v1/api/result 同时限制 user_id 和 api_key_id；本地公开 ID/上游 ID 不存在或无权访问均返回相同 404。
- Live probe 默认不联网；只有显式 --live 才能运行；单次最多一个 nano-banana-2-lite 任务，CI 只运行离线自测。
- migration 只能新增；提交日志用中文；不改动用户现有未跟踪目录或 test/test_images_api.py。

---

## File Structure

| 文件 | 职责 |
| --- | --- |
| backend/internal/service/grsai_delivery.go | 下游交付类型、只读请求解析、独立上游请求体、公开视图 DTO。 |
| backend/internal/service/grsai_stream.go | SSE Content-Type、帧、任务 ID、进度和终态校验。 |
| backend/internal/service/grsai_task_service.go | 建任务/冻结、消费流、结算、释放和查询视图。 |
| backend/internal/service/grsai_task_runtime.go | Async 领取、绑定任务轮询、重启恢复和保留期清理。 |
| backend/internal/service/grsai_balance_hold.go | GRS.AI 专用冻结、捕获、释放命令和事务接口。 |
| backend/internal/repository/grsai_settlement_repo.go | 任务状态、双重归属查询和带围栏的状态迁移。 |
| backend/internal/repository/grsai_task_payload_repo.go | 加密 Async 载荷存取和删除。 |
| backend/ent/schema/grsai_task_payload.go | 加密载荷模型。 |
| backend/migrations/243_grsai_delivery_modes.sql | 只追加任务字段、索引、载荷表。 |
| backend/internal/handler/grsai_gateway.go | POST 三种交付与安全查询 handler。 |
| backend/internal/config/config.go and Wire files | Worker 配置、依赖注入、启动和停止。 |
| test/grsai_live_contract.py | 默认禁用的真实上游 Stream 探测。 |

## Task 1: 固定下游协议并重建上游请求体

**Files:**
- Create: backend/internal/service/grsai_delivery.go
- Create: backend/internal/service/grsai_delivery_test.go
- Modify: backend/internal/service/grsai_native.go
- Modify: backend/internal/service/grsai_native_test.go

**Interfaces:**
- type GrsaiDeliveryMode string: GrsaiDeliveryJSON, GrsaiDeliveryStream, GrsaiDeliveryAsync。
- ParseGrsaiDeliveryRequest(raw []byte) (*GrsaiDeliveryRequest, error)。
- GrsaiDeliveryRequest: Mode, OriginalBody, UpstreamBody, Model, ImageCount, ImageSize。
- ErrGrsaiInvalidReplyType and ErrGrsaiLegacyDeliveryFlag；handler 映射为安全 400。

- [ ] **Step 1: Write the failing parser tests.**

~~~go
func TestParseGrsaiDeliveryRequestRebuildsIndependentStreamBody(t *testing.T) {
    raw := []byte("{\"model\":\"nano-banana-2-lite\",\"replyType\":\"async\",\"size\":{\"width\":1024},\"extra\":{\"x\":[1,true]}}")
    before := append([]byte(nil), raw...)
    got, err := ParseGrsaiDeliveryRequest(raw)
    require.NoError(t, err)
    require.Equal(t, GrsaiDeliveryAsync, got.Mode)
    require.Equal(t, before, raw)

    var upstream map[string]json.RawMessage
    require.NoError(t, json.Unmarshal(got.UpstreamBody, &upstream))
    require.JSONEq(t, "\"nano-banana-2-lite\"", string(upstream["model"]))
    require.JSONEq(t, "{\"width\":1024}", string(upstream["size"]))
    require.JSONEq(t, "{\"x\":[1,true]}", string(upstream["extra"]))
    require.JSONEq(t, "\"stream\"", string(upstream["replyType"]))
}

func TestParseGrsaiDeliveryRequestRejectsLegacyFlagsAndBadReplyType(t *testing.T) {
    for _, raw := range [][]byte{
        []byte("{\"model\":\"nano-banana-2-lite\",\"stream\":true}"),
        []byte("{\"model\":\"nano-banana-2-lite\",\"async\":true}"),
        []byte("{\"model\":\"nano-banana-2-lite\",\"replyType\":true}"),
        []byte("{\"model\":\"nano-banana-2-lite\",\"replyType\":\"provider_async\"}"),
    } {
        _, err := ParseGrsaiDeliveryRequest(raw)
        require.Error(t, err)
    }
}
~~~

- [ ] **Step 2: Run it and verify it fails.**

Run: go test ./internal/service -run 'TestParseGrsaiDeliveryRequest' -count=1

Expected: FAIL because parser and types do not exist.

- [ ] **Step 3: Implement parsing with a fresh object.**

~~~go
func ParseGrsaiDeliveryRequest(raw []byte) (*GrsaiDeliveryRequest, error) {
    original := append([]byte(nil), raw...)
    var input map[string]json.RawMessage
    if err := json.Unmarshal(original, &input); err != nil || input == nil {
        return nil, ErrGrsaiInvalidReplyType
    }
    if _, ok := input["stream"]; ok { return nil, ErrGrsaiLegacyDeliveryFlag }
    if _, ok := input["async"]; ok { return nil, ErrGrsaiLegacyDeliveryFlag }

    mode, err := parseGrsaiDeliveryMode(input["replyType"])
    if err != nil { return nil, err }
    upstream := make(map[string]json.RawMessage, len(input))
    for key, value := range input {
        if key != "replyType" {
            upstream[key] = append(json.RawMessage(nil), value...)
        }
    }
    upstream["replyType"] = json.RawMessage("\"stream\"")
    body, err := json.Marshal(upstream)
    if err != nil { return nil, err }
    model, count, size := parseGrsaiGenerateRequest(original)
    return &GrsaiDeliveryRequest{Mode: mode, OriginalBody: original, UpstreamBody: body, Model: model, ImageCount: count, ImageSize: size}, nil
}
~~~

Missing replyType maps to JSON; only a JSON string in the three-value enum is valid. Preserve every non-control, model-specific field. Replace every upstream use of PrepareGrsaiGenerateBody so callers only receive UpstreamBody.

- [ ] **Step 4: Add client assertion and run focused tests.**

~~~go
func TestGrsaiNativeClientAlwaysPostsRebuiltStreamBody(t *testing.T) {
    // httptest server decodes body and requires replyType == "stream".
    // It also requires Accept to include text/event-stream.
}
~~~

Run: go test ./internal/service -run 'Test(ParseGrsaiDeliveryRequest|GrsaiNativeClient)' -count=1

Expected: PASS for missing/json/stream/async downstream modes and legacy flag rejection.

- [ ] **Step 5: Commit.**

~~~powershell
git add backend/internal/service/grsai_delivery.go backend/internal/service/grsai_delivery_test.go backend/internal/service/grsai_native.go backend/internal/service/grsai_native_test.go
git commit -m "feat: 固定 GRS.AI 上游流式请求"
~~~

## Task 2: 持久化公开任务和加密 Async 载荷

**Files:**
- Create: backend/ent/schema/grsai_task_payload.go
- Create: backend/internal/repository/grsai_task_payload_repo.go
- Create: backend/internal/repository/grsai_task_payload_repo_test.go
- Modify: backend/ent/schema/grsai_settlement.go
- Modify: backend/internal/service/grsai_settlement.go
- Modify: backend/internal/repository/grsai_settlement_repo.go
- Modify: backend/internal/repository/grsai_settlement_repo_test.go
- Create: backend/migrations/243_grsai_delivery_modes.sql
- Regenerate: backend/ent/

**Interfaces:**
- GrsaiSettlement adds PublicTaskID, DeliveryMode, Progress, ResultURLs, HoldAmount, HoldState, PayloadDeleteAfter, ExpiresAt.
- GetOwnedByPublicOrUpstreamID(ctx, userID, apiKeyID int64, id string) (*GrsaiSettlement, error)。
- GrsaiTaskPayloadRepository: PutEncrypted, GetEncrypted, DeleteBySettlementID, DeleteExpired。
- Public statuses: queued, submitting, running, pending_settlement, settled, closed_no_charge, upstream_unknown, manual_review.

- [ ] **Step 1: Write failing repository tests.**

~~~go
func TestGrsaiSettlementRepositoryFindsOnlyExactOwner(t *testing.T) {
    record := createGrsaiSettlement(t, repo, service.GrsaiDeliveryAsync)
    got, err := repo.GetOwnedByPublicOrUpstreamID(ctx, record.UserID, record.APIKeyID, record.PublicTaskID)
    require.NoError(t, err)
    require.Equal(t, record.ID, got.ID)

    _, err = repo.GetOwnedByPublicOrUpstreamID(ctx, record.UserID, record.APIKeyID+1, record.PublicTaskID)
    require.ErrorIs(t, err, service.ErrGrsaiSettlementNotFound)
}

func TestGrsaiTaskPayloadRepositoryNeverPersistsPlaintext(t *testing.T) {
    repo := NewGrsaiTaskPayloadRepository(db, testSecretEncryptor{})
    require.NoError(t, repo.PutEncrypted(ctx, 18, []byte("{\"prompt\":\"private\"}"), time.Now().Add(time.Hour)))
    require.NotContains(t, readPayloadCiphertext(t, db, 18), "\"prompt\":\"private\"")
    got, err := repo.GetEncrypted(ctx, 18)
    require.NoError(t, err)
    require.JSONEq(t, "{\"prompt\":\"private\"}", string(got))
}
~~~

- [ ] **Step 2: Run the tests.**

Run: go test ./internal/repository -run 'TestGrsai(SettlementRepositoryFindsOnlyExactOwner|TaskPayloadRepositoryNeverPersistsPlaintext)' -count=1

Expected: FAIL because fields/repositories are absent.

- [ ] **Step 3: Add immutable migration and Ent schema.**

~~~sql
ALTER TABLE grsai_settlements
  ADD COLUMN public_task_id varchar(64),
  ADD COLUMN delivery_mode varchar(16) NOT NULL DEFAULT 'json',
  ADD COLUMN progress integer NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
  ADD COLUMN result_urls jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN hold_amount double precision NOT NULL DEFAULT 0,
  ADD COLUMN hold_state varchar(16) NOT NULL DEFAULT 'none',
  ADD COLUMN payload_delete_after timestamptz,
  ADD COLUMN expires_at timestamptz;
CREATE UNIQUE INDEX grsai_settlements_public_task_id_uq
  ON grsai_settlements (public_task_id) WHERE public_task_id IS NOT NULL;
CREATE INDEX grsai_settlements_owner_public_lookup_idx
  ON grsai_settlements (user_id, api_key_id, public_task_id);
CREATE INDEX grsai_settlements_owner_upstream_lookup_idx
  ON grsai_settlements (user_id, api_key_id, upstream_task_id) WHERE upstream_task_id IS NOT NULL;

CREATE TABLE grsai_task_payloads (
  settlement_id bigint PRIMARY KEY REFERENCES grsai_settlements(id) ON DELETE CASCADE,
  ciphertext text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX grsai_task_payloads_expires_at_idx ON grsai_task_payloads (expires_at);
~~~

Create a non-sequential UUID public ID. Add/select/scan all columns in the raw SQL repository. Owner lookup must be:

~~~sql
WHERE user_id = $1 AND api_key_id = $2
  AND (public_task_id = $3 OR upstream_task_id = $3)
~~~

Encrypt before INSERT, decrypt only in Worker, and never log plaintext or ciphertext.

- [ ] **Step 4: Run Ent/repository verification and commit.**

Run: go generate ./ent; go test ./ent/... ./internal/repository -run 'TestGrsai' -count=1

Expected: PASS; first-phase records remain readable, wrong owner is not found, expired/deleted payload cannot be retrieved.

~~~powershell
git add backend/ent backend/migrations/243_grsai_delivery_modes.sql backend/internal/service/grsai_settlement.go backend/internal/repository/grsai_settlement_repo.go backend/internal/repository/grsai_settlement_repo_test.go backend/internal/repository/grsai_task_payload_repo.go backend/internal/repository/grsai_task_payload_repo_test.go
git commit -m "feat: 持久化 GRS.AI 交付任务"
~~~

## Task 3: 实现冻结、释放和成功时原子结算

**Files:**
- Create: backend/internal/service/grsai_balance_hold.go
- Create: backend/internal/service/grsai_balance_hold_test.go
- Modify: backend/internal/service/usage_billing.go
- Modify: backend/internal/service/grsai_settlement.go
- Modify: backend/internal/repository/usage_billing_repo.go
- Modify: backend/internal/repository/usage_billing_repo_test.go
- Modify: backend/internal/repository/grsai_settlement_repo.go

**Interfaces:**
- GrsaiBalanceHoldCommand has SettlementID, UserID, APIKeyID, Amount, IdempotencyKey.
- GrsaiHoldBillingRepository has ReserveGrsaiBalance, CaptureGrsaiBalanceTx, ReleaseGrsaiBalanceTx.
- Settlement transaction callback captures hold and applies UsageBillingRepository.ApplyTx through the same sql.Tx.
- CloseNoChargeWithRelease and MarkManualReviewWithRelease perform an atomic release/terminal transition.

- [ ] **Step 1: Write failing hold/settlement tests.**

~~~go
func TestGrsaiPrepareReservesBeforeSubmission(t *testing.T) {
    svc, holds := newGrsaiSettlementServiceForTest(t)
    task, err := svc.Prepare(ctx, validGrsaiPrepareInput())
    require.NoError(t, err)
    require.Equal(t, "held", task.HoldState)
    require.Equal(t, task.ID, holds.Reserve[0].SettlementID)
}

func TestGrsaiSucceededCapturesHoldAndChargesOnceInOneTransaction(t *testing.T) {
    applied, err := svc.SettleAt(ctx, task.ID, task.ClaimVersion, time.Now().Add(time.Minute))
    require.NoError(t, err)
    require.True(t, applied)
    require.Equal(t, []string{"capture", "usage", "mark_settled"}, events)
    applied, err = svc.SettleAt(ctx, task.ID, task.ClaimVersion, time.Now().Add(time.Minute))
    require.NoError(t, err)
    require.False(t, applied)
}

func TestGrsaiFailureReleasesHoldAndNeverCreatesUsage(t *testing.T) {
    outcome := svc.Finish(ctx, task, &GrsaiUpstreamResult{Status: "failed"}, nil)
    require.NoError(t, outcome.SettlementError)
    require.Equal(t, "released", reload(t, task.ID).HoldState)
    require.Empty(t, billing.ApplyCalls)
}
~~~

- [ ] **Step 2: Run tests and verify failure.**

Run: go test ./internal/service ./internal/repository -run 'TestGrsai(PrepareReserves|SucceededCaptures|FailureReleases)' -count=1

Expected: FAIL because no GRS.AI hold contract exists.

- [ ] **Step 3: Implement domain-specific holds.**

Use idempotency key grsai_hold:<settlement-id>. Prepare performs price snapshot → task row → reserve → hold_state=held → submission claim. A reserve failure closes the unsubmitted row as no-charge/none, returns the established insufficient-balance error, and does not contact upstream.

Add GRS.AI-specific public methods in usage_billing_repo.go. Private SQL helpers may be shared, but do not repurpose ReserveBatchImageBalance or its cancellation semantics. Reserve transfers available balance to a hold; capture consumes the hold; release restores availability. Every operation is idempotent by settlement ID and operation.

- [ ] **Step 4: Make terminal transitions transactional.**

~~~go
_, err := s.Repo.Settle(ctx, id, claimVersion, amount, func(txCtx context.Context, tx *sql.Tx, record *GrsaiSettlement) error {
    hold := grsaiHoldCommand(record)
    if err := s.Holds.CaptureGrsaiBalanceTx(txCtx, tx, hold); err != nil {
        return err
    }
    _, err := s.Billing.ApplyTx(txCtx, tx, grsaiBillingCommand(record))
    return err
})
~~~

Use the same locked callback pattern for CloseNoChargeWithRelease and MarkManualReviewWithRelease: release a held balance, set hold_state=released, then terminal state. Capture/usage failure leaves hold active, marks pending_settlement, and uses existing backoff.

- [ ] **Step 5: Run billing regression and commit.**

Run: go test ./internal/service ./internal/repository -run '(TestGrsai|TestBatchImage)' -count=1

Expected: PASS; exactly one capture/use on success, exactly one release/no usage on failed/violation/manual-review, existing Batch Image tests stay green.

~~~powershell
git add backend/internal/service/grsai_balance_hold.go backend/internal/service/grsai_balance_hold_test.go backend/internal/service/grsai_settlement.go backend/internal/service/usage_billing.go backend/internal/repository/usage_billing_repo.go backend/internal/repository/usage_billing_repo_test.go backend/internal/repository/grsai_settlement_repo.go
git commit -m "feat: 增加 GRS.AI 冻结结算状态机"
~~~

## Task 4: 建立受验证的上游 SSE 客户端和共享消费器

**Files:**
- Create: backend/internal/service/grsai_stream.go
- Create: backend/internal/service/grsai_stream_test.go
- Modify: backend/internal/service/grsai_native.go
- Modify: backend/internal/service/grsai_native_test.go
- Modify: backend/internal/service/grsai_settlement.go
- Modify: backend/internal/repository/grsai_settlement_repo.go

**Interfaces:**
- OpenGenerateStream(ctx, account, upstreamBody) returns GrsaiUpstreamStream with StatusCode, ContentType and Body io.ReadCloser.
- ParseGrsaiSSE(r io.Reader, callback func(GrsaiStreamEvent) error) returns final GrsaiUpstreamResult.
- GrsaiStreamEvent has RawData, TaskID, Status, Progress, ResultURLs, Terminal.
- RecordStreamEvent persists the frame before a handler callback emits it.

- [ ] **Step 1: Write failing stream tests.**

~~~go
func TestParseGrsaiSSERequiresStableIDAndMonotonicProgress(t *testing.T) {
    body := strings.NewReader("data: {\\\"id\\\":\\\"task-1\\\",\\\"status\\\":\\\"running\\\",\\\"progress\\\":10}\\n\\n" +
        "data: {\\\"id\\\":\\\"task-1\\\",\\\"status\\\":\\\"succeeded\\\",\\\"progress\\\":100,\\\"results\\\":[{\\\"url\\\":\\\"https://example.invalid/a.png\\\"}]}\\n\\n")
    final, err := ParseGrsaiSSE(body, nil)
    require.NoError(t, err)
    require.Equal(t, GrsaiUpstreamStatusSucceeded, final.Status)

    _, err = ParseGrsaiSSE(strings.NewReader("data: {\\\"id\\\":\\\"a\\\",\\\"progress\\\":20}\\n\\ndata: {\\\"id\\\":\\\"b\\\",\\\"progress\\\":10}\\n\\n"), nil)
    require.Error(t, err)
}

func TestOpenGenerateStreamRejectsNonSSE(t *testing.T) {
    // httptest returns application/json; client returns typed error and closes body.
}
~~~

- [ ] **Step 2: Run and verify failure.**

Run: go test ./internal/service -run 'Test(ParseGrsaiSSE|OpenGenerateStream)' -count=1

Expected: FAIL because stream API/parser are absent.

- [ ] **Step 3: Implement strict upstream Stream handling.**

Set Accept: text/event-stream; only 2xx plus Content-Type containing text/event-stream exposes its body. Other responses read a bounded error body, redact credentials with existing helper, then close body.

Use bufio.Scanner with explicit maximum token size. Accept complete data: JSON frames and ignore comments/empty keepalives. First event needs ID; later IDs match; progress remains 0..100 and never falls. Only succeeded, failed, violation are terminal; success needs result URL. Protocol inconsistency is a fixed internal protocol error, never a retry POST.

- [ ] **Step 4: Persist then notify.**

~~~go
final, err := ParseGrsaiSSE(stream.Body, func(event GrsaiStreamEvent) error {
    if err := tasks.RecordStreamEvent(ctx, claim, event); err != nil {
        return err
    }
    if onPersistedEvent != nil {
        return onPersistedEvent(event)
    }
    return nil
})
~~~

First ID calls fenced BindUpstreamTask; every event updates status/progress/results. Read error before bind becomes manual_review plus release. Read error after bind becomes upstream_unknown with held balance for polling, never re-POST.

- [ ] **Step 5: Run focused suite and commit.**

Run: go test ./internal/service -run 'Test(Grsai|ParseGrsaiSSE|OpenGenerateStream)' -count=1

Expected: PASS for invalid content type, changed ID, regressive progress, result-less success, and pre/post-bind interruption.

~~~powershell
git add backend/internal/service/grsai_stream.go backend/internal/service/grsai_stream_test.go backend/internal/service/grsai_native.go backend/internal/service/grsai_native_test.go backend/internal/service/grsai_settlement.go backend/internal/repository/grsai_settlement_repo.go
git commit -m "feat: 解析并持久化 GRS.AI 上游流"
~~~

## Task 5: 实现任务服务、Async Worker、恢复和公开视图

**补充验收（2026-09-23）：** 受理在创建/冻结前原子检查每用户 20 个等待名额；Worker 首次领取原子检查每用户 3 个进行中名额，并持久标记首次领取。已领取任务恢复不再消耗第二个名额；终态及仅结算重试释放名额。先以真库并发、跨 API Key、租约恢复和 20/21、3/4 边界测试验证，不能以 `BatchLimit` 代替容量门槛。

**Files:**
- Create: backend/internal/service/grsai_task_service.go
- Create: backend/internal/service/grsai_task_service_test.go
- Create: backend/internal/service/grsai_task_runtime.go
- Create: backend/internal/service/grsai_task_runtime_test.go
- Modify: backend/internal/service/grsai_settlement.go
- Modify: backend/internal/repository/grsai_settlement_repo.go

**Interfaces:**
- CreateGrsaiTask(ctx, input GrsaiTaskInput) returns GrsaiSettlement.
- RunGrsaiTask(ctx, claim, onPersistedEvent) returns final GrsaiUpstreamResult.
- GrsaiTaskRuntime exposes Start, Stop, RunOnce.
- GrsaiTaskView contains only ID, UpstreamTaskID, Status, Progress, Model, CreatedAt, UpdatedAt, Results, ErrorCode, ErrorSummary.

- [ ] **Step 1: Write failing recovery tests.**

~~~go
func TestAsyncTaskPersistsEncryptedPayloadThenWorkerConsumesOnce(t *testing.T) {
    task, err := tasks.CreateGrsaiTask(ctx, asyncInput([]byte("{\"model\":\"nano-banana-2-lite\",\"replyType\":\"async\"}")))
    require.NoError(t, err)
    require.Equal(t, "queued", task.InternalStatus)
    require.True(t, payloads.Exists(t, task.ID))

    runtime.RunOnce(ctx)
    require.Equal(t, 1, upstream.PostCalls)
    require.False(t, payloads.Exists(t, task.ID))
    require.Equal(t, "settled", reload(t, task.ID).InternalStatus)
}

func TestBoundDisconnectPollsResultWithoutSecondPost(t *testing.T) {
    task := boundRunningTask(t)
    runtime.RunOnce(ctx)
    require.Equal(t, 0, upstream.PostCalls)
    require.Equal(t, 1, upstream.ResultCalls)
}

func TestPublicTaskViewDoesNotExposePrivateFields(t *testing.T) {
    got, err := tasks.GetPublicTaskView(ctx, task.UserID, task.APIKeyID, task.PublicTaskID)
    require.NoError(t, err)
    encoded, _ := json.Marshal(got)
    require.NotContains(t, string(encoded), "account_id")
    require.NotContains(t, string(encoded), "billable_unit_price")
    require.NotContains(t, string(encoded), "ciphertext")
}
~~~

- [ ] **Step 2: Run and verify failure.**

Run: go test ./internal/service -run 'Test(AsyncTask|BoundDisconnect|PublicTaskView)' -count=1

Expected: FAIL because service/runtime do not exist.

- [ ] **Step 3: Implement the shared state machine.**

CreateGrsaiTask validates, snapshots price, freezes and gives each task a public UUID. Only async encrypts/saves OriginalBody with configured TTL. JSON/Stream never persist request payload. Async starts queued; worker claims submitting, decrypts, reparses through Task 1, and only posts the newly built stream body.

RunGrsaiTask may POST only an active claim without UpstreamTaskID. After Task 4 binds ID, delete payload. A pre-bind uncertain submission becomes manual_review plus release: never guess ID or retry POST.

- [ ] **Step 4: Implement deterministic recovery.**

RunOnce claim-version-fences due records and processes:

1. queued: only valid decryptable payload can POST; missing/corrupt payload becomes manual review plus release. Nominal payload TTL must not expire a still-queued or newly claimed, unsubmitted task.
2. bound running/upstream_unknown/expired processing: only GET /v1/api/result?id=<upstream-id>, then settle/release from final state.
3. pending_settlement: only retry Task 3 capture/use transaction.
4. unbound submitting beyond submission_unknown_timeout_seconds: manual review plus release.

No recovery branch re-POSTs a bound record. Public view is constructed from owner-scoped repository data. Map sanitized failures only to upstream_failed, policy_violation or manual_review and cap summary length.

- [ ] **Step 5: Run recovery/concurrency tests and commit.**

Run: go test ./internal/service ./internal/repository -run 'Test(AsyncTask|BoundDisconnect|PublicTaskView|Grsai.*(Claim|Recover|Settle))' -count=1

Expected: PASS; two runtimes yield one claimant, bound work makes zero second POSTs, payload disappears when bound/terminal or expired outside the unsubmitted queue.

~~~powershell
git add backend/internal/service/grsai_task_service.go backend/internal/service/grsai_task_service_test.go backend/internal/service/grsai_task_runtime.go backend/internal/service/grsai_task_runtime_test.go backend/internal/service/grsai_settlement.go backend/internal/repository/grsai_settlement_repo.go
git commit -m "feat: 增加 GRS.AI 异步任务恢复"
~~~

## Task 6: 提供三种 POST 交付和安全结果查询

**Files:**
- Modify: backend/internal/handler/grsai_gateway.go
- Modify: backend/internal/handler/grsai_gateway_test.go
- Create: backend/internal/handler/grsai_gateway_delivery_test.go
- Modify: backend/internal/handler/wire.go
- Modify: backend/internal/server/routes/gateway.go
- Modify: backend/internal/server/routes/gateway_test.go

**Interfaces:**
- Generate uses Task 1 parser and Task 5 task service.
- Result serves GET /v1/api/result?id=<local-or-upstream-id>.
- Async response: 202 with id, status, model, created_at.
- Stream response: text/event-stream. JSON response: verified terminal JSON.

- [ ] **Step 1: Write failing handler tests.**

~~~go
func TestGrsaiGenerateAsyncReturnsPublicTaskID(t *testing.T) {
    c, rec := newGrsaiGatewayTestContext(t)
    c.Request = httptest.NewRequest(http.MethodPost, "/v1/api/generate",
        strings.NewReader("{\"model\":\"nano-banana-2-lite\",\"replyType\":\"async\"}"))
    attachOwnedGrsaiAPIKey(c)
    handler.Generate(c)
    require.Equal(t, http.StatusAccepted, rec.Code)
    require.Contains(t, rec.Body.String(), "\"status\":\"queued\"")
}

func TestGrsaiResultRequiresSameUserAndAPIKey(t *testing.T) {
    // Owner receives 200 by public ID and upstream ID; modified API key receives exactly 404.
}

func TestGrsaiStreamPersistsBeforeWritingEvent(t *testing.T) {
    // Fake service captures persist-running before data-frame write.
}
~~~

- [ ] **Step 2: Run and verify failure.**

Run: go test ./internal/handler ./internal/server/routes -run 'TestGrsai(GenerateAsync|ResultRequires|StreamPersists)' -count=1

Expected: FAIL because delivery branches/query handler are absent.

- [ ] **Step 3: Refactor Generate by delivery mode.**

Preserve existing auth, group platform, model allowlist, content audit, user/account concurrency, selection and pricing precheck. Audit OriginalBody; upstream receives only UpstreamBody.

JSON creates/freezes/claims then consumes stream before writing response, returning verified terminal JSON. Stream consumes/persists first event before setting SSE headers; each persisted event writes data: <RawData> followed by double newline and Flush. On client write error/context cancellation stop only output; context.WithoutCancel hands durable work to runtime. Async creates/freezes/encrypts and immediately returns 202; handler makes no upstream request.

- [ ] **Step 4: Register Result route and restrict output.**

Register in existing authenticated v1 group:

~~~go
v1.GET("/api/result", handlers.GrsaiGateway.Result)
~~~

Handler additionally requires GRS.AI group platform. Empty ID and every non-owner lookup use errorResponse(404, "not_found", "task not found"). Marshal only GrsaiTaskView; never raw settlement/upstream body. Create no cancellation route.

- [ ] **Step 5: Run API regression and commit.**

Run: go test ./internal/handler ./internal/server/routes -run 'TestGrsai' -count=1

Expected: PASS; JSON/Stream/Async assert fixed upstream stream, SSE disconnect has no duplicate POST, dual ownership is enforced.

~~~powershell
git add backend/internal/handler/grsai_gateway.go backend/internal/handler/grsai_gateway_test.go backend/internal/handler/grsai_gateway_delivery_test.go backend/internal/handler/wire.go backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_test.go
git commit -m "feat: 提供 GRS.AI 多模式交付接口"
~~~

## Task 7: 配置、Wire 生命周期和清理

实现与独立复审记录：`docs/superpowers/reviews/2026-09-23-grsai-task7-integration.md`。代码与测试已完成；当前工作树包含此前未提交的 Task 5/6 混合改动，Task 7 的提交步骤暂不单独执行。

**Files:**
- Modify: backend/internal/config/config.go
- Modify: backend/internal/config/config_test.go
- Modify: backend/internal/service/wire.go
- Modify: backend/internal/repository/wire.go
- Modify: backend/cmd/server/wire.go
- Regenerate: backend/cmd/server/wire_gen.go
- Modify: backend/cmd/server/wire_gen_test.go

**Interfaces:**
- GrsaiDeliveryConfig under grsai_delivery: enabled, scan_interval_seconds, batch_limit, payload_ttl_seconds, result_retention_hours.
- Runtime cleanup deletes bound/terminal/expired ciphertext and expires terminal safe-task records.

- [x] **Step 1: Write failing config/lifecycle test.**

~~~go
func TestGrsaiDeliveryConfigDefaultsAndValidation(t *testing.T) {
    cfg := loadTestConfig(t, "")
    require.False(t, cfg.GrsaiDelivery.Enabled)
    require.Equal(t, 5, cfg.GrsaiDelivery.ScanIntervalSeconds)
    require.Equal(t, 20, cfg.GrsaiDelivery.BatchLimit)
    require.Equal(t, 900, cfg.GrsaiDelivery.PayloadTTLSeconds)
    require.Equal(t, 24, cfg.GrsaiDelivery.ResultRetentionHours)
    require.Error(t, validateTestConfig(t, "grsai_delivery.batch_limit: 0"))
}
~~~

- [x] **Step 2: Run and verify failure.**

Run: go test ./internal/config ./cmd/server -run 'TestGrsaiDelivery' -count=1

Expected: FAIL because config/runtime are not injected.

- [x] **Step 3: Add conservative defaults and validation.**

~~~go
viper.SetDefault("grsai_delivery.enabled", false)
viper.SetDefault("grsai_delivery.scan_interval_seconds", 5)
viper.SetDefault("grsai_delivery.batch_limit", 20)
viper.SetDefault("grsai_delivery.payload_ttl_seconds", 900)
viper.SetDefault("grsai_delivery.result_retention_hours", 24)
~~~

Validate scan 1..300 seconds, batch 1..100, payload TTL 60..3600 seconds, result retention 1..168 hours. Keep disabled by default. Existing grsai_settlement_recovery retains its existing meaning.

- [x] **Step 4: Wire runtime and cleanup.**

Inject SecretEncryptor, payload repo, holds, native client and task service. Append GrsaiTaskRuntime.Stop to provideCleanup; Stop cancels ticker/work scheduling without deleting data. Regenerate Wire; never manually edit wire_gen.go.

Each RunOnce deletes payload once upstream ID binds, task turns terminal, or payload expires outside the unsubmitted queue. Delete only terminal task records after the configured retention measured from closed_at, not the create-time expires_at. Retain pending_settlement, upstream_unknown and actionable manual_review.

- [ ] **Step 5: Run verification and commit.**

Run: go test ./internal/config ./internal/service ./internal/repository ./cmd/server -run 'Test(GrsaiDelivery|GrsaiTaskRuntime|Wire)' -count=1

Expected: PASS; default Worker is off, Stop succeeds, 24-hour view expires to 404, pending/manual work remains.

~~~powershell
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/internal/service/wire.go backend/internal/repository/wire.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go backend/cmd/server/wire_gen_test.go
git commit -m "feat: 接入 GRS.AI 任务恢复运行时"
~~~

## Task 8: 真实上游契约探测与发布验证

**Files:**
- Create: test/grsai_live_contract.py
- Create: test/README.grsai-live-contract.md
- Modify: an existing Python-test selector only if it exists, to explicitly exclude the live probe

**Interfaces:**
- CLI requires exactly --live; reads GRSAI_BASE and GRSAI_KEY; timeout default 180 seconds.
- One POST uses nano-banana-2-lite and replyType=stream; subsequent GET calls use /v1/api/result.
- Output contains only masked task-ID suffix, content type, terminal status and progress.

- [ ] **Step 1: Write offline self-tests in the script.**

~~~python
def test_live_gate_requires_explicit_flags():
    code, output = run_probe([])
    assert code == 2
    assert "--live" in output

def test_scrubber_removes_key_prompt_and_result_url():
    text = scrub("Bearer secret-key prompt=private https://images.example/a.png")
    assert "secret-key" not in text
    assert "private" not in text
    assert "https://images.example/a.png" not in text
~~~

Use only unittest and standard library; do not modify existing user test script.

- [ ] **Step 2: Run network safety gate.**

Run: python test/grsai_live_contract.py --self-test; python test/grsai_live_contract.py

Expected: self-test PASS; second command exits code 2, asks for --live, opens no network connection.

- [ ] **Step 3: Implement a single scrubbed live call.**

Use urllib.request, fixed minimal in-memory prompt and one POST guard post_started. Require SSE Content-Type; parse data frames for stable ID, monotonic progress and terminal state; poll /v1/api/result?id=<id> until matching state or timeout. Upstream credits/cost are not a release gate; never save response, prompt, key or image URL.

- [ ] **Step 4: Run local full verification without spending credits.**

Run: go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./internal/config ./cmd/server -count=1; python test/grsai_live_contract.py --self-test; python test/grsai_live_contract.py

Expected: Go tests/self-test pass; no-flags probe safely exits. Do not run --live in this task.

- [ ] **Step 5: Document gates and commit.**

Document the live command as python test/grsai_live_contract.py --live; rollout is JSON → Async → Stream. Stream remains disabled until live probe repeatedly passes, bound disconnect recovers by query, no duplicate settlement occurs, and abnormal manual_review has no backlog.

~~~powershell
git add test/grsai_live_contract.py test/README.grsai-live-contract.md
git commit -m "test: 增加 GRS.AI 上游流式契约探测"
~~~

## Final Verification Checklist

- [ ] go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./internal/config ./cmd/server -count=1 passes.
- [ ] go vet ./internal/service ./internal/repository ./internal/handler ./internal/server/routes passes.
- [ ] go generate ./ent then git diff --exit-code -- backend/ent passes.
- [ ] python test/grsai_live_contract.py --self-test passes; no-flags invocation has no network request.
- [ ] Explicit live run uses --live, creates one nano-banana-2-lite task and prints no secret/prompt/URL/raw SSE.
- [ ] Production uses gray-status.sh, immutable sha-<commit> to a 0% candidate, validates JSON then Async then Stream, and uses only gray scripts for traffic. On billing/auth/5xx anomaly return traffic to stable and preserve candidate evidence.

## Self-Review

| 设计要求 | 覆盖任务 |
| --- | --- |
| 单一 replyType、全新对象、上游 Stream | Tasks 1, 4, 6 |
| JSON/Stream/Async | Tasks 5, 6 |
| 绑定 ID 后断线恢复且不重投 | Tasks 4, 5, 6 |
| 双重归属和安全查询视图 | Tasks 2, 5, 6 |
| 加密载荷和删除 | Tasks 2, 5, 7 |
| 冻结/捕获/释放和重试 | Tasks 3, 5 |
| 无取消 | Global Constraints, Tasks 5, 6 |
| nano-banana-2-lite、单任务、显式 live 探测 | Task 8 |
| JSON → Async → Stream 灰度门 | Task 8 and Final Verification |

Placeholder scan completed: each task supplies files, interfaces, tests, commands, expected result and commit. Type review completed: Task 1 produces request types; Task 2 persistence; Task 3 hold contract; Task 4 stream contract; Task 5 task service; Tasks 6--7 consume them.

## Execution Handoff

Plan complete and saved to docs/superpowers/plans/2026-09-21-grsai-delivery-modes.md. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task and review between tasks.
2. **Inline Execution** — execute tasks in this session using executing-plans, in batches with checkpoints.

Which approach?

