# grsai Native Images Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `product-dev-suite:subagent-driven-development` (recommended) or `product-dev-suite:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first-phase `platform=grsai` native image-generation gateway at `POST /v1/api/generate`, with per-model price snapshots and durable, idempotent settlement.

**Architecture:** A dedicated `GrsaiGatewayHandler` authenticates and schedules only grsai groups, then calls a native grsai client without translating model-specific parameters. A database-backed settlement record is created before upstream submission; terminal success is charged through the existing atomic usage-billing repository, while a separate recovery runtime polls known running tasks and retries pending settlement.

**Tech Stack:** Go, Gin, Ent, PostgreSQL SQL migrations, Google Wire, Vue 3, TypeScript, Vitest, Go unit/integration tests.

**Spec:** `docs/superpowers/specs/2026-09-18-grsai-native-images-design.md`

## Global Constraints

- The only new public route is `POST /v1/api/generate`; do not add a root alias or public task-result route.
- Accept only JSON `replyType`; normalize omission to `json`, and reject `stream=true` or `async=true` locally.
- Require a direct `platform=grsai` group in phase 1; composite support remains Roadmap work.
- Preserve model-specific request fields unchanged and do not add per-model parameter profiles in this phase.
- Charge only after an upstream task is confirmed `succeeded`; upstream HTTP errors, `failed`, `violation`, malformed responses, and initial `running` responses do not immediately charge.
- Do not store upstream API keys, prompt text, reference-image bytes, Base64, or unredacted request JSON in settlement records or logs.
- Use a durable database scanner for settlement recovery. Do not depend on an in-memory queue for billing recovery.
- Before any migration or production deployment change, locate and read `docs/RELEASE_DEPLOYMENT.md` as required by `AGENTS.md`. The current checkout does not contain that file; implementation must stop before migration work until its authoritative location is restored or confirmed by the repository owner.
- Keep Roadmap items out of this implementation: composite groups, public task lookup, public async/streaming modes, OpenAI-compatible grsai image routes, parameter profiles, model sync, and automatic price conversion.

---

## File Structure

| Path | Responsibility |
| --- | --- |
| `backend/internal/domain/constants.go` and `backend/internal/service/domain_constants.go` | Define `PlatformGrsai` alongside other concrete platforms. |
| `backend/ent/schema/grsai_settlement.go` | Durable settlement entity and indexes. |
| `backend/migrations/239_grsai_native_images.sql` | Add `grsai_settlements` with upgrade-safe indexes; reserve a different next number if 239 is occupied when implementation starts. |
| `backend/internal/repository/grsai_settlement_repo.go` | Ent-backed create, transition, due-scan, row-lock, and atomic settlement persistence. |
| `backend/internal/service/grsai_native.go` | Native request validation, response decoding, URL construction, and HTTP client interface. |
| `backend/internal/service/grsai_settlement.go` | State machine, price snapshots, usage-log construction, and idempotent billing orchestration. |
| `backend/internal/service/grsai_settlement_recovery.go` | Database polling, result reconciliation, retry backoff, and manual-review escalation. |
| `backend/internal/handler/grsai_gateway_handler.go` | Authenticated request flow, scheduler integration, upstream call, response passthrough, and immediate settlement trigger. |
| `backend/internal/handler/handler.go`, `backend/internal/handler/wire.go`, `backend/internal/service/wire.go`, `backend/cmd/server/wire.go`, `backend/cmd/server/wire_gen.go` | Register the handler and worker runtime through existing Wire composition. |
| `backend/internal/server/routes/gateway.go` | Register and platform-gate `/v1/api/generate`. |
| `frontend/src/types/index.ts`, `frontend/src/constants/platforms.ts`, `frontend/src/utils/platformColors.ts` | Add the concrete platform to admin and display types. |
| `frontend/src/components/account/credentialsBuilder.ts`, account/group/channel views, locale files | Configure grsai API-key/Base-URL credentials and show/select the platform. |
| Focused `*_test.go` and `*.spec.ts` files listed per task | Lock the protocol, state, billing, platform, and UI contracts. |

### Task 1: Verify the Release and Migration Preconditions

**Files:**
- Verify: `AGENTS.md`
- Verify: `docs/RELEASE_DEPLOYMENT.md`
- Verify: `backend/migrations/migrations.go`
- Verify: `backend/migrations/238_opencode_go_platform.sql`
- Create only after the prerequisite is satisfied: `backend/migrations/239_grsai_native_images.sql`

**Consumes:** The approved native-image specification.

**Produces:** A documented migration number, deployment compatibility rules, and a green light to modify database schema.

- [ ] **Step 1: Locate the required release document before editing migration-related code**

Run:

```powershell
rg --files -g 'RELEASE_DEPLOYMENT.md' -g 'AGENTS.md' . docs
```

Expected: an authoritative `docs/RELEASE_DEPLOYMENT.md` is present and can be read. If it is still absent, stop implementation and ask the repository owner to restore it or provide its authority path; do not create a migration, regenerate Ent, or start the server.

- [ ] **Step 2: Read release compatibility rules and inspect current migration ordering**

Run:

```powershell
Get-Content -Raw 'docs\RELEASE_DEPLOYMENT.md'
Get-Content -Raw 'backend\migrations\238_opencode_go_platform.sql'
```

Expected: capture the required compatibility trailer, transaction constraints, deployment order, and rollback procedure in the implementation PR description before creating the next migration.

- [ ] **Step 3: Confirm the next migration prefix is unused**

Run:

```powershell
Test-Path 'backend\migrations\239_grsai_native_images.sql'
```

Expected: `False`; if `True`, choose the next unused zero-padded number and update every reference in this plan while implementing.

- [ ] **Step 4: Leave migration creation to the persistence task**

Do not commit a migration at this point. The migration belongs to Task 3 after the Ent schema and its tests exist.

### Task 2: Add `grsai` as a Concrete Configurable Platform

**Files:**
- Modify: `backend/internal/domain/constants.go`
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/account_service.go`
- Modify: `backend/internal/service/admin_group.go`
- Modify: `backend/internal/service/scheduler_snapshot_service.go`
- Modify: `backend/internal/service/composite_platform.go`
- Modify: `backend/internal/model/error_passthrough_rule.go`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/constants/platforms.ts`
- Modify: `frontend/src/utils/platformColors.ts`
- Modify: `frontend/src/components/account/credentialsBuilder.ts`
- Modify: `frontend/src/components/account/CreateAccountModal.vue`
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`
- Test: `backend/internal/handler/admin/group_handler_platform_test.go`
- Test: `backend/internal/service/grsai_platform_test.go`
- Test: `frontend/src/constants/__tests__/platforms.spec.ts`
- Test: `frontend/src/components/account/__tests__/credentialsBuilder.spec.ts`

**Consumes:** `PlatformGrsai = "grsai"` in domain constants.

**Produces:** Valid account and group platform values, scheduler buckets, API DTOs, and admin controls that can create grsai configuration without treating it as OpenAI.

- [ ] **Step 1: Write failing backend platform-validation tests**

Add table rows that create an API-key account and a normal group using `service.PlatformGrsai`, and assert that an unsupported arbitrary string is still rejected.

```go
{
    name: "grsai platform is accepted",
    platform: service.PlatformGrsai,
    wantErr: false,
}
```

- [ ] **Step 2: Run the focused backend tests and confirm the new platform is rejected before implementation**

Run:

```powershell
go test -tags=unit ./internal/handler/admin ./internal/service -run 'GrsaiPlatform|GroupHandlerPlatform' -count=1
```

Expected: FAIL because `PlatformGrsai` and the allow-list entries do not exist.

- [ ] **Step 3: Add the platform constant and update every concrete-platform enumeration**

Define the same value in domain and service aliases:

```go
const PlatformGrsai = "grsai"
```

Add it to validation and scheduler lists that currently include `PlatformOpenCodeGo`, including the concrete account platform list in `scheduler_snapshot_service.go`. Do not add it to OpenAI-compatible endpoint switches, token refreshers, or OpenAI-specific upstream billing probes.

- [ ] **Step 4: Add failing frontend catalog and credential tests**

Assert the catalog exposes `grsai`, `GroupPlatform` and `AccountPlatform` accept it, and account credential construction accepts only an API key plus a normalized Base URL for this platform.

```ts
expect(CONCRETE_PLATFORM_OPTIONS).toContainEqual({ value: 'grsai', label: 'grsai' })
expect(buildCredentials('grsai', { apiKey: 'sk-test', baseUrl: 'https://example.test/' }))
  .toEqual({ api_key: 'sk-test', base_url: 'https://example.test' })
```

- [ ] **Step 5: Implement the frontend platform catalog and account form behavior**

Extend the union types and `CONCRETE_PLATFORM_OPTIONS`; add a non-OpenAI color entry and localized `grsai` label. In `credentialsBuilder.ts`, serialize credentials as `api_key` and normalized `base_url`; make the create/edit forms expose those fields only for grsai. Reuse existing sensitive-field masking and never render the saved API key in plaintext.

- [ ] **Step 6: Run platform and UI tests**

Run:

```powershell
go test -tags=unit ./internal/handler/admin ./internal/service -run 'GrsaiPlatform|GroupHandlerPlatform' -count=1
npm --prefix frontend test -- --run src/constants/__tests__/platforms.spec.ts src/components/account/__tests__/credentialsBuilder.spec.ts
```

Expected: PASS.

- [ ] **Step 7: Commit the platform slice**

```powershell
git add -- backend/internal/domain/constants.go backend/internal/service/domain_constants.go backend/internal/service/account_service.go backend/internal/service/admin_group.go backend/internal/service/scheduler_snapshot_service.go backend/internal/service/composite_platform.go backend/internal/model/error_passthrough_rule.go backend/internal/handler/admin/group_handler_platform_test.go backend/internal/service/grsai_platform_test.go frontend/src/types/index.ts frontend/src/constants/platforms.ts frontend/src/constants/__tests__/platforms.spec.ts frontend/src/utils/platformColors.ts frontend/src/components/account/credentialsBuilder.ts frontend/src/components/account/CreateAccountModal.vue frontend/src/components/account/EditAccountModal.vue frontend/src/components/account/__tests__/credentialsBuilder.spec.ts frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/i18n/locales/en/admin/accounts.ts
git commit -m "feat: 增加 grsai 平台配置"
```

### Task 3: Create Durable Settlement Storage and Repository Operations

**Files:**
- Create: `backend/ent/schema/grsai_settlement.go`
- Create: `backend/internal/repository/grsai_settlement_repo.go`
- Create: `backend/internal/repository/grsai_settlement_repo_integration_test.go`
- Create: `backend/migrations/239_grsai_native_images.sql`
- Modify: `backend/internal/repository/wire.go`
- Modify generated: `backend/ent/...`
- Modify generated: `backend/cmd/server/wire_gen.go`
- Test: `backend/internal/repository/grsai_settlement_repo_integration_test.go`

**Consumes:** `PlatformGrsai`; the release/migration precondition from Task 1.

**Produces:** `GrsaiSettlementRepository`, row states, unique idempotency keys, due-record scanning, and a generated Ent client.

- [ ] **Step 1: Write an integration test for the state transition and uniqueness contract**

Create two records with the same `billing_idempotency_key`, then assert only one is created. Set an upstream task ID twice for the same account and assert the partial unique constraint rejects the second record. Assert the due query returns only `awaiting_result`, `settlement_pending`, and expired `submission_pending` records.

```go
created, err := repo.Create(ctx, GrsaiSettlementCreateParams{
    SettlementID: "grsai_settlement_test_1",
    BillingIdempotencyKey: "grsai:settlement:1",
    Status: service.GrsaiSettlementStatusSubmissionPending,
})
require.NoError(t, err)
require.True(t, created)
```

- [ ] **Step 2: Run the integration test and confirm it fails because the schema/repository are absent**

Run the repository integration test using the project's existing PostgreSQL test setup. Do not replace it with an in-memory mock because partial unique indexes and row locking are part of the contract.

```powershell
go test -tags=integration ./internal/repository -run 'GrsaiSettlement' -count=1
```

Expected: FAIL because `GrsaiSettlementRepository` and Ent schema do not exist.

- [ ] **Step 3: Define the Ent schema and migration**

Create `GrsaiSettlement` with these fields: internal settlement ID; status; user/API-key/group/account/channel IDs; requested, mapped, and billing model; price snapshot; request SHA-256; upstream task ID/status/HTTP status; billing idempotency key; retry count; next-retry and submission timestamps; redacted last error; manual-review reason; settled timestamp; created/updated timestamps.

Use `decimal(20,10)` for the price snapshot. Add a unique index on `billing_idempotency_key`, a partial unique index on `(account_id, upstream_task_id)` where the task ID is nonempty, and due-scan indexes on `(status, next_retry_at)` and `submission_started_at`. The SQL migration must use idempotent DDL and must not alter an existing migration file.

- [ ] **Step 4: Implement repository methods with transactional compare-and-set semantics**

Define this service-facing interface:

```go
type GrsaiSettlementRepository interface {
    Create(context.Context, GrsaiSettlementCreateParams) (*GrsaiSettlement, error)
    RecordUpstream(context.Context, GrsaiRecordUpstreamParams) (*GrsaiSettlement, error)
    ClaimDue(context.Context, time.Time, int) ([]*GrsaiSettlement, error)
    MarkClosedNoCharge(context.Context, string, GrsaiTerminalParams) error
    MarkManualReview(context.Context, string, string) error
    Settle(context.Context, string, func(context.Context, *GrsaiSettlement) error) error
}

type GrsaiSettlementCreateParams struct {
    SettlementID, BillingIdempotencyKey, Status string
    UserID, APIKeyID, GroupID, AccountID, ChannelID int64
    RequestedModel, MappedModel, BillingModel string
    PriceSnapshot float64
    RequestHash string
    SubmissionStartedAt time.Time
}

type GrsaiRecordUpstreamParams struct {
    SettlementID, TaskID, Status, ErrorCode, ErrorText string
    HTTPStatus int
    NextRetryAt *time.Time
}

type GrsaiTerminalParams struct {
    HTTPStatus int
    UpstreamStatus, ErrorCode, ErrorText string
}
```

`Settle` must lock one settlement row, accept only `settlement_pending`, execute the supplied billing callback in the same database transaction, and mark the row `settled` only after the callback succeeds. A second worker must observe `settled` and return without another callback.

- [ ] **Step 5: Generate Ent and Wire code, then re-run tests**

Run:

```powershell
go generate ./ent
go generate ./cmd/server
go test -tags=integration ./internal/repository -run 'GrsaiSettlement' -count=1
```

Expected: PASS, including duplicate-key and concurrent-settlement coverage.

- [ ] **Step 6: Commit the persistence slice**

```powershell
git add backend/ent backend/internal/repository backend/migrations/239_grsai_native_images.sql backend/cmd/server/wire_gen.go
git commit -m "feat: 增加 grsai 结算持久化"
```

### Task 4: Implement Native grsai Protocol Parsing and HTTP Client

**Files:**
- Create: `backend/internal/service/grsai_native.go`
- Create: `backend/internal/service/grsai_native_test.go`
- Modify: `backend/internal/service/wire.go`

**Consumes:** `service.Account` credentials (`api_key`, `base_url`) and durable settlement types from Task 3.

**Produces:** A testable `GrsaiNativeClient` that submits `/v1/api/generate` and internally retrieves `/v1/api/result` without exposing either transport to public routing.

- [ ] **Step 1: Write failing protocol tests with an `httptest.Server`**

Cover missing `model`, malformed JSON, missing/incorrect `replyType`, `stream=true`, `async=true`, trailing-slash Base URL normalization, authorization placement, raw non-2xx passthrough, and the four recognized statuses.

```go
result, err := client.Generate(ctx, account, []byte(`{"model":"nano-banana-2","prompt":"x"}`))
require.NoError(t, err)
require.JSONEq(t, `{"status":"succeeded","id":"task_1","results":[]}`, string(result.Body))
```

- [ ] **Step 2: Run the tests and confirm they fail before client implementation**

Run:

```powershell
go test -tags=unit ./internal/service -run 'GrsaiNative' -count=1
```

Expected: FAIL because `GrsaiNativeClient` is absent.

- [ ] **Step 3: Define protocol types and validation**

Use these stable interfaces:

```go
type GrsaiNativeClient interface {
    Generate(ctx context.Context, account *Account, body []byte) (*GrsaiUpstreamResult, error)
    Result(ctx context.Context, account *Account, taskID string) (*GrsaiUpstreamResult, error)
}

type GrsaiUpstreamResult struct {
    HTTPStatus int
    Body       []byte
    TaskID     string
    Status     string
    ErrorCode  string
    ErrorText  string
}
```

Validate only JSON shape, nonempty `model`, `replyType`, `stream`, and `async`. Insert `replyType:"json"` when omitted; when serialization is required for that insertion, preserve every other field's JSON value without normalization, filtering, or model-specific rewriting. Classify only `running`, `succeeded`, `failed`, and `violation` as recognized task statuses.

- [ ] **Step 4: Implement safe upstream transport**

Read `api_key` and `base_url` from the selected account; reject missing credentials before any request. Use a configured HTTP client with context cancellation, call `/v1/api/generate` or `/v1/api/result?id=<escaped-task-id>`, and retain raw response bytes for handler passthrough. Limit error snippets and never log credential values or raw prompt/image payloads.

- [ ] **Step 5: Run tests**

Run:

```powershell
go test -tags=unit ./internal/service -run 'GrsaiNative' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the protocol slice**

```powershell
git add backend/internal/service/grsai_native.go backend/internal/service/grsai_native_test.go backend/internal/service/wire.go
git commit -m "feat: 增加 grsai 原生请求客户端"
```

### Task 5: Implement Price Snapshots and Idempotent grsai Settlement

**Files:**
- Create: `backend/internal/service/grsai_settlement.go`
- Create: `backend/internal/service/grsai_settlement_test.go`
- Modify: `backend/internal/service/model_pricing_resolver.go`
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/wire.go`

**Consumes:** `GrsaiSettlementRepository`, `ModelPricingResolver`, `UsageBillingRepository.Apply`, and `applyUsageBilling`.

**Produces:** `GrsaiSettlementService` that resolves the configured model price before submission, snapshots it, and charges at most once after confirmed success.

- [ ] **Step 1: Write failing settlement state-machine tests**

Test all terminal rules: `succeeded` moves to pending settlement and charges exactly once; `failed`, `violation`, malformed bodies, and HTTP failures close without billing; `running` remains uncharged; repeated `Settle` calls use one stable billing request ID; a transient billing error schedules the 1m/5m/15m/1h/6h sequence before `manual_review`.

```go
require.NoError(t, service.RecordSucceeded(ctx, settlementID, upstream))
require.NoError(t, service.Settle(ctx, settlementID))
require.NoError(t, service.Settle(ctx, settlementID))
require.Equal(t, 1, billing.ApplyCalls())
```

- [ ] **Step 2: Run the focused service tests and confirm they fail**

Run:

```powershell
go test -tags=unit ./internal/service -run 'GrsaiSettlement' -count=1
```

Expected: FAIL because the settlement service and price snapshot resolver are absent.

- [ ] **Step 3: Implement stable pricing and billing inputs**

Define `GrsaiPriceSnapshot` from the same channel/model pricing resolution used by normal channels, after channel mapping and before upstream submission. Persist the resolved billable model, selected account/channel IDs, per-success price, group multiplier, account multiplier, and pricing timestamp. Reject missing price before calling upstream.

Build a `UsageLog` with `RequestID` equal to `grsai_settlement:<settlement-id>`, `BillingModeImage`, `RequestTypeSync`, `ImageCount: 1`, the original public model, mapped model, selected account, group, endpoint `/v1/api/generate`, and upstream endpoint `/v1/api/generate`. Invoke `applyUsageBilling` through `UsageBillingRepository.Apply` inside the repository settlement transaction.

- [ ] **Step 4: Implement explicit transitions and retry policy**

Use these status constants consistently:

```go
const (
    GrsaiSettlementStatusSubmissionPending = "submission_pending"
    GrsaiSettlementStatusAwaitingResult    = "awaiting_result"
    GrsaiSettlementStatusSettlementPending = "settlement_pending"
    GrsaiSettlementStatusSettled           = "settled"
    GrsaiSettlementStatusClosedNoCharge    = "closed_no_charge"
    GrsaiSettlementStatusUpstreamUnknown   = "upstream_unknown"
    GrsaiSettlementStatusManualReview      = "manual_review"
)
```

Only `succeeded` can transition to `settlement_pending`. Schedule retries at 1 minute, 5 minutes, 15 minutes, 1 hour, and 6 hours. After the fifth billing failure, persist `manual_review`, retain the upstream task ID and price snapshot, and emit the existing high-severity operational error path. Do not re-submit an image request from the settlement service.

- [ ] **Step 5: Run unit and race-focused tests**

Run:

```powershell
go test -tags=unit ./internal/service -run 'GrsaiSettlement' -count=1
go test -race -tags=unit ./internal/service -run 'GrsaiSettlement.*Idempotent|GrsaiSettlement.*Concurrent' -count=1
```

Expected: PASS with one billing application per settlement ID.

- [ ] **Step 6: Commit the settlement slice**

```powershell
git add backend/internal/service/grsai_settlement.go backend/internal/service/grsai_settlement_test.go backend/internal/service/model_pricing_resolver.go backend/internal/service/usage_billing.go backend/internal/service/wire.go
git commit -m "feat: 增加 grsai 幂等结算"
```

### Task 6: Add Database-Driven Reconciliation and Recovery Runtime

**Files:**
- Create: `backend/internal/service/grsai_settlement_recovery.go`
- Create: `backend/internal/service/grsai_settlement_recovery_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire.go`
- Modify generated: `backend/cmd/server/wire_gen.go`
- Test: `backend/internal/service/grsai_settlement_recovery_test.go`

**Consumes:** Task 4 native result client and Task 5 settlement state machine.

**Produces:** A lifecycle-managed runtime that scans durable records, reconciles known task IDs, retries settlement, and quarantines unknown submissions.

- [ ] **Step 1: Write failing recovery tests with fake repository and native client**

Cover: a due `running` task becomes `settlement_pending` after an internal `succeeded` response; a `failed` response closes without charge; an expired `submission_pending` record with no task ID becomes `manual_review`; retries use their persisted `next_retry_at`; two runtime instances claiming the same row execute billing once.

```go
processed, err := recovery.RunOnce(ctx)
require.NoError(t, err)
require.Equal(t, 1, processed)
require.Equal(t, GrsaiSettlementStatusManualReview, repo.Status("unknown_1"))
```

- [ ] **Step 2: Run the recovery tests and confirm they fail**

Run:

```powershell
go test -tags=unit ./internal/service -run 'GrsaiSettlementRecovery' -count=1
```

Expected: FAIL because `GrsaiSettlementRecoveryRuntime` is absent.

- [ ] **Step 3: Add bounded runtime configuration and defaults**

Add a `grsai_settlement` config block with `enabled`, `scan_interval_seconds`, `scan_limit`, and `submission_unknown_after_seconds`. Default it disabled and use conservative values that process at most 100 records per scan. Retain the fixed five-step billing retry schedule from Task 5 rather than exposing conflicting retry-duration settings.

```go
type GrsaiSettlementConfig struct {
    Enabled                        bool `mapstructure:"enabled"`
    ScanIntervalSeconds            int  `mapstructure:"scan_interval_seconds"`
    ScanLimit                      int  `mapstructure:"scan_limit"`
    SubmissionUnknownAfterSeconds  int  `mapstructure:"submission_unknown_after_seconds"`
}
```

- [ ] **Step 4: Implement the scanner and runtime lifecycle**

Define:

```go
type GrsaiSettlementRecoveryRuntime struct {
    repo       GrsaiSettlementRepository
    settlement *GrsaiSettlementService
    native     GrsaiNativeClient
    cfg        GrsaiSettlementConfig
    mu         sync.Mutex
    cancel     context.CancelFunc
    done       chan struct{}
}
func (r *GrsaiSettlementRecoveryRuntime) Start()
func (r *GrsaiSettlementRecoveryRuntime) Stop()
func (r *GrsaiSettlementRecoveryRuntime) RunOnce(ctx context.Context) (int, error)
```

For known IDs in `awaiting_result` or `upstream_unknown`, call the internal `Result` client; for `settlement_pending`, invoke `Settle`; for expired `submission_pending` without an ID, mark `manual_review`. Start it through Wire with an application-scoped context and stop it through the existing server cleanup lifecycle. Never expose its result endpoint through Gin routes.

- [ ] **Step 5: Generate Wire code and run tests**

Run:

```powershell
go generate ./cmd/server
go test -tags=unit ./internal/config ./internal/service -run 'GrsaiSettlementRecovery|GrsaiSettlementConfig' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the recovery slice**

```powershell
git add -- backend/internal/config/config.go backend/internal/config/config_test.go backend/internal/service/grsai_settlement_recovery.go backend/internal/service/grsai_settlement_recovery_test.go backend/internal/service/wire.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: 增加 grsai 结算恢复任务"
```

### Task 7: Add the Native Handler and Route Without Changing OpenAI Images

**Files:**
- Create: `backend/internal/handler/grsai_gateway_handler.go`
- Create: `backend/internal/handler/grsai_gateway_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/gateway.go`
- Modify: `backend/internal/server/routes/gateway_test.go`
- Modify generated: `backend/cmd/server/wire_gen.go`

**Consumes:** Tasks 2-6 and the existing API-key middleware, group allow-list, image-generation gate, billing eligibility service, and account scheduler.

**Produces:** `GrsaiGatewayHandler.Generate`, registered only at `POST /v1/api/generate` and isolated from `/v1/images/generations`.

- [ ] **Step 1: Write handler and route tests before implementation**

Assert all of the following: grsai group + `succeeded` returns upstream status/body and creates one settlement; direct non-grsai groups get local `404`; `replyType` violation and `stream`/`async` are local `400` with zero upstream calls; upstream `failed`/`violation`/HTTP errors pass through with zero billing; `running` passes through and persists `awaiting_result`; OpenAI `/v1/images/generations` remains handled by `OpenAIGatewayHandler.Images`.

```go
req := httptest.NewRequest(http.MethodPost, "/v1/api/generate", strings.NewReader(`{"model":"nano-banana-2"}`))
req.Header.Set("Authorization", "Bearer sk-test")
router.ServeHTTP(rec, req)
require.Equal(t, http.StatusOK, rec.Code)
require.JSONEq(t, `{"id":"task_1","status":"succeeded","results":[]}`, rec.Body.String())
```

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run:

```powershell
go test -tags=unit ./internal/handler ./internal/server/routes -run 'GrsaiGenerate|GrsaiRoute|OpenAIImagesRoute' -count=1
```

Expected: FAIL because the route and handler do not exist.

- [ ] **Step 3: Implement `GrsaiGatewayHandler.Generate` using existing gateway controls**

The handler must retrieve the API key and authenticated subject, read a bounded JSON body, apply the native protocol validator, call `GroupAllowsImageGeneration`, invoke security audit using a grsai-native protocol label, acquire image/user/account slots, call `CheckBillingEligibility`, resolve channel mapping and selected account, and create the pre-submission settlement record before `Generate`.

On an upstream response, persist `TaskID` and status before writing the raw body. Immediately call settlement for `succeeded`; if billing fails, log the operation error and still write the upstream success response. Release every acquired slot on all return paths. Do not add OpenAI failover after a raw upstream response, because resubmission can generate a duplicate image.

- [ ] **Step 4: Register the handler and platform gate**

Add `GrsaiGateway *GrsaiGatewayHandler` to `Handlers` and Wire providers. In `RegisterGatewayRoutes`, register `gateway.POST("/api/generate", grsaiHandler)` under the existing `/v1` middleware chain. The route gate must require `getGroupPlatform(c) == service.PlatformGrsai`; all other platforms return the existing not-supported `404` envelope and mark the local feature-gate ops reason.

- [ ] **Step 5: Run focused tests and compile the server**

Run:

```powershell
go generate ./cmd/server
go test -tags=unit ./internal/handler ./internal/server/routes -run 'GrsaiGenerate|GrsaiRoute|OpenAIImagesRoute' -count=1
go test ./cmd/server ./internal/handler ./internal/server/routes -run '^$'
```

Expected: PASS.

- [ ] **Step 6: Commit the gateway slice**

```powershell
git add -- backend/internal/handler/grsai_gateway_handler.go backend/internal/handler/grsai_gateway_handler_test.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_test.go backend/cmd/server/wire_gen.go
git commit -m "feat: 接入 grsai 原生生图网关"
```

### Task 8: Complete Admin Experience and Platform-Isolation Regression Coverage

**Files:**
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/ChannelsView.vue`
- Modify: `frontend/src/components/common/PlatformIcon.vue`
- Modify: `frontend/src/utils/keyGroupProviders.ts`
- Modify: `frontend/src/views/admin/__tests__/channelPlatformOptions.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/GroupsView.compositePlatforms.spec.ts`
- Create: `frontend/src/views/admin/__tests__/grsaiPlatform.spec.ts`
- Test: `backend/internal/service/composite_platform_test.go`
- Test: `backend/internal/server/routes/gateway_model_allowlist_test.go`

**Consumes:** Platform catalog from Task 2 and route isolation from Task 7.

**Produces:** Operators can select/filter grsai accounts, groups, and channels; composite groups and existing OpenAI routes remain outside the first-phase native handler.

- [ ] **Step 1: Write failing UI tests for direct grsai configuration**

Verify the platform is available in account, group, and channel filters; credentials display the Base URL field; and a user key associated with a grsai group is shown as a valid group option.

```ts
expect(screen.getByRole('option', { name: 'grsai' })).toBeInTheDocument()
expect(screen.getByLabelText(/Base URL/i)).toBeVisible()
```

- [ ] **Step 2: Run the UI tests and confirm they fail**

Run:

```powershell
npm --prefix frontend test -- --run src/views/admin/__tests__/grsaiPlatform.spec.ts
```

Expected: FAIL before the views consume the new platform catalog.

- [ ] **Step 3: Implement catalog-driven admin UI support**

Use `CONCRETE_PLATFORM_OPTIONS` as the single source for selectors and filters. Update icon/provider maps with a safe grsai fallback, add translations, and ensure account forms retain `api_key` masking and `base_url` normalization after edit. Do not add a grsai option to OpenAI image configuration toggles that route `/v1/images/generations`.

- [ ] **Step 4: Add backend regression tests for phase boundaries**

Add cases proving a composite group is rejected at `/v1/api/generate`, a grsai account is not selected by OpenAI image routes, and group model allow-list enforcement occurs before native account selection.

- [ ] **Step 5: Run UI and isolation tests**

Run:

```powershell
npm --prefix frontend test -- --run src/views/admin/__tests__/grsaiPlatform.spec.ts src/views/admin/__tests__/channelPlatformOptions.spec.ts
go test -tags=unit ./internal/service ./internal/server/routes -run 'Grsai|CompositePlatform|GatewayModelAllowlist' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the admin and isolation slice**

```powershell
git add -- frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/ChannelsView.vue frontend/src/components/common/PlatformIcon.vue frontend/src/utils/keyGroupProviders.ts frontend/src/views/admin/__tests__/channelPlatformOptions.spec.ts frontend/src/views/admin/__tests__/GroupsView.compositePlatforms.spec.ts frontend/src/views/admin/__tests__/grsaiPlatform.spec.ts backend/internal/service/composite_platform_test.go backend/internal/server/routes/gateway_model_allowlist_test.go
git commit -m "feat: 完善 grsai 管理配置与隔离测试"
```

### Task 9: Execute End-to-End Verification and Release Gates

**Files:**
- Modify only when test observations reveal a defect: files owned by Tasks 2-8
- Verify: `test/test_images_api.py`
- Verify: `docs/superpowers/specs/2026-09-18-grsai-native-images-design.md`

**Consumes:** Completed, reviewed Tasks 1-8.

**Produces:** Evidence that the first-phase native protocol, durable settlement, and existing image/chat behavior are safe to release.

- [ ] **Step 1: Run generation, formatting, and package compilation checks**

Run:

```powershell
go generate ./ent
go generate ./cmd/server
gofmt -w backend/internal/domain/constants.go backend/internal/service/domain_constants.go backend/internal/service/account_service.go backend/internal/service/admin_group.go backend/internal/service/scheduler_snapshot_service.go backend/internal/service/composite_platform.go backend/internal/service/grsai_native.go backend/internal/service/grsai_native_test.go backend/internal/service/grsai_settlement.go backend/internal/service/grsai_settlement_test.go backend/internal/service/grsai_settlement_recovery.go backend/internal/service/grsai_settlement_recovery_test.go backend/internal/service/model_pricing_resolver.go backend/internal/service/usage_billing.go backend/internal/service/wire.go backend/internal/repository/grsai_settlement_repo.go backend/internal/repository/grsai_settlement_repo_integration_test.go backend/internal/handler/grsai_gateway_handler.go backend/internal/handler/grsai_gateway_handler_test.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_test.go
go test ./internal/config ./internal/service ./internal/repository ./internal/handler ./internal/server/routes -run '^$'
```

Expected: generated files are current and compilation succeeds. Review `git diff` after `gofmt`; retain only grsai-related formatting changes.

- [ ] **Step 2: Run the full focused Go regression suite**

Run:

```powershell
go test -tags=unit ./internal/service ./internal/repository ./internal/handler ./internal/server/routes -run 'Grsai|OpenAIImages|CompositePlatform|GatewayModelAllowlist|BatchImageSettlement' -count=1
go test -race -tags=unit ./internal/service ./internal/repository -run 'GrsaiSettlement' -count=1
```

Expected: all success, failure, running reconciliation, duplicate billing, and unknown-submission tests pass.

- [ ] **Step 3: Run frontend checks**

Run:

```powershell
npm --prefix frontend test -- --run src/constants/__tests__/platforms.spec.ts src/components/account/__tests__/credentialsBuilder.spec.ts src/views/admin/__tests__/grsaiPlatform.spec.ts
npm --prefix frontend run build
```

Expected: platform types, admin forms, and production build pass.

- [ ] **Step 4: Run a manual native smoke test with a disposable configured account**

Use a temporary grsai group and a non-production user API key. Submit a known low-cost model through `/v1/api/generate` with `replyType=json`; verify raw response passthrough, exactly one settlement row, exactly one usage log, and one balance deduction only after `succeeded`. Then test a deliberately invalid model parameter and verify the upstream response returns without a charge.

Do not place any real credential in source, fixtures, test output, or commit history.

- [ ] **Step 5: Run repository hygiene checks**

Run:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors; staged changes are limited to the grsai feature, generated Ent/Wire output, migration, tests, and necessary documentation. Preserve the user's existing untracked `.agents/`, `.playwright-cli/`, and `test/` content.

- [ ] **Step 6: Run the release gate after reading the required release document**

Apply the compatibility, migration, Blue-Green, approval, and rollback checks in `docs/RELEASE_DEPLOYMENT.md`. Do not deploy, tag, or modify production state from this implementation plan.

- [ ] **Step 7: Request code review with complete verification evidence**

All implementation changes are committed in the focused tasks above. Do not create a catch-all commit or stage broad directories during release verification. Request review with the test evidence, migration compatibility note, and the explicit residual risk for the upstream submit-to-task-ID uncertainty window.
