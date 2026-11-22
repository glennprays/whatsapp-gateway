I have everything I need. Here is the blueprint.

---

# Blueprint — Roadmap #2: Outbound Pacing (final phase)

## 0. Dependency preconditions (this is the LAST phase — all must already be merged)

| Depends on | What #2 consumes from it | If missing |
|---|---|---|
| **PRD-4 ops-hardening** (`HandleEvent` cases + session-status store) | `events.TemporaryBan` / `ConnectFailure` persisted per phone with `ban_expires_at`; this is the ban-aware signal. `internal/whatsapp/handler.go` already has the switch cases (PRD-4 §"Session-status tracking"). | #2 must add the `TemporaryBan` case + a `banned_until` column itself (fallback DDL in §4). |
| **PRD-3 group management** (`group_usecase.go`, `UpdateGroupParticipants`, `GROUP_ADD_PARTICIPANTS_ENABLED`) | The bulk participant-add path + its default-off gate, which #2 replaces with paced execution. | Nothing to lift; skip §3 group edit. |
| **PRD-2 two-way** (`interaction_usecase.go` `spendActionBudget`, key `action:`+phone) | The interim action cap #2 supersedes. | Nothing to replace. |
| **PRD-0 addressing** (`resolveChat`, `@g.us` requirement) | Canonical recipient JID = the per-recipient pacing key. | Recipient key would be unnormalized. |

**Design stance (ponytail):** pacing is a **pure gateway-side wrapper**. It calls **no new whatsmeow function**. It reuses the existing `ratelimiter.Limiter` interface, `Result.RetryAfter`, and the queue worker's *existing* delayed-retry machinery (`pkg/queue/worker_pool.go:181` already reschedules on `*ratelimiter.RateLimitError`). The only genuinely new code is a token-bucket backend (~40 lines) + a blocking `Pacer` wrapper (~60 lines).

---

## 1. Features / behavior changes (no new send endpoints; existing paths change observable behavior)

Pacing is applied to every **outbound** path. There are no new request DTOs for sends. Observable changes:

| Path (existing) | Old behavior | New behavior |
|---|---|---|
| `POST /message/{text,image,audio,video,document,location,poll,sticker}` — direct mode | `limiter.Allow` → **429** over budget | `pacer.Wait` **blocks up to `OUTBOUND_PACE_MAX_WAIT_SECONDS`** then sends; 429 only if the deadline is exceeded or the per-recipient hard cap trips or the account is banned |
| same — queue mode (`outgoing_message_handler.go:64`) | `Limiter.Allow` → `RateLimitError` → worker reschedules | `pacer.Wait` bounded; on deadline returns `*RateLimitError` → **same** worker reschedule (unchanged machinery) |
| `POST /message/react`, `PUT /message` (edit), `DELETE /message` — direct | react = `spendActionBudget`; **edit/delete unpaced today** (gap) | all three route through `pacer.Wait` (folds in the interim `action:` cap; closes the edit/delete gap) |
| `POST /message/read`, `POST /chat/presence` (mark-read, typing) — direct | `spendActionBudget` | `pacer.Wait` |
| `POST /group/:chat/participants` `action=add` + add-on-create | hard-gated behind `GROUP_ADD_PARTICIPANTS_ENABLED=false` | **enabled by default**, executed **paced** (spends `AllowN(len(participants))` account tokens; per-request cap `GROUP_MAX_PARTICIPANTS_PER_REQUEST` still enforced pre-call) |
| `POST /group/join` (mass-join) | hard-gated `GROUP_JOIN_VIA_LINK_ENABLED=false` | enabled by default, paced (spends 1 account token) |

**New response signals** (all paths): on a paced-out/banned reject → `429` with `Retry-After: <seconds>` header (from `RateLimitError.RetryAfter`). Optional `X-Pace-Waited-Ms` on a paced-but-successful send (diagnostic only; skip if not wanted).

**Optional admin endpoint (only if PRD-4 admin plane is present):**
- `POST /admin/pacing/:phone/resume` → clears `banned_until` early (force-resume). Req: none. Resp: `{ "phone": "...", "resumed": true }`. Gated by the `ADMIN_API_SECRET` bearer; the whole `/admin/*` surface 404s when unset. → *skip if PRD-4 admin plane not merged; add when ops asks.*

---

## 2. Exact whatsmeow calls (REAL signatures, file:line) + error sentinels

The pacer itself calls none of these — it wraps the usecase call that eventually reaches them. Quoted for the integration points #2 touches:

**Send (already wrapped by `Manager.SendTextMessage` etc.)** — `send.go:185`:
```go
func (cli *Client) SendMessage(ctx context.Context, to types.JID, message *waE2E.Message, extra ...SendRequestExtra) (resp SendResponse, err error)
```

**Ban signal consumed via `HandleEvent` (PRD-4 owns the switch case)** — `types/events/events.go:151`:
```go
// TemporaryBan is emitted when there's a connection failure with the ConnectFailureTempBanned reason code.
type TemporaryBan struct {
	Code   TempBanReason   // codes 101,102,103,104,106  (events.go:126-130)
	Expire time.Duration
}
```
`TempBanReason` constants — `events.go:126-130` (`TempBanSentToTooManyPeople`=101 … `TempBanBroadcastList`=106). `String()` at `events.go:142`. Related lifecycle events already handled by PRD-4: `events.ConnectFailure` (`events.go:220`), `events.LoggedOut` (`events.go:105`), `events.StreamReplaced` (`events.go:117`).

**Bulk participant-add un-gated by #2 (PRD-3 owns the call)** — `group.go:189`:
```go
func (cli *Client) UpdateGroupParticipants(ctx context.Context, jid types.JID, participantChanges []types.JID, action ParticipantChange) ([]types.GroupParticipant, error)
```
`ParticipantChangeAdd = "add"` (`group.go:182`).

**Error sentinels & HTTP mapping (pacing layer):** the pacer produces exactly one error type — `*ratelimiter.RateLimitError` (`pkg/ratelimiter/limiter.go:16`). Mapping:

| Pacer outcome | Returned as | HTTP |
|---|---|---|
| cleared to send | `nil` | proceeds |
| account bucket empty & deadline exceeded (mode=pace) or over-budget (mode=reject) | `*RateLimitError{RetryAfter}` | **429** (`ErrTooManyRequests`) + `Retry-After` |
| per-recipient hard cap tripped | `*RateLimitError{RetryAfter}` | **429** |
| `banned_until` in the future | `*RateLimitError{RetryAfter: banned_until-now}` | **429** |

The **group-add path** keeps PRD-3/ADR-0002 whatsmeow error mapping (IQ 419 resource-limit → 429 via `mapWhatsmeowErr`); pacing only adds a pre-call `pacer.Wait`. No new sentinel handling. Confirmed: pacing introduces **no** whatsmeow error sentinels.

---

## 3. Ordered edit list

### 3.1 `pkg/ratelimiter/tokenbucket.go` (NEW) — token-bucket backend implementing the existing `Limiter` interface
```go
type TokenBucketConfig struct {
	Rate   float64       // tokens/sec refill
	Burst  float64       // bucket capacity (max tokens)
	Prefix string
}
type TokenBucketLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tbEntry
	rate    float64
	burst   float64
	prefix  string
	now     func() time.Time // injectable; defaults to time.Now (test seam)
}
type tbEntry struct{ tokens float64; last time.Time }

func NewTokenBucketLimiter(cfg TokenBucketConfig) *TokenBucketLimiter
func (t *TokenBucketLimiter) Allow(ctx context.Context, key string) (Result, error)         // = AllowN(…,1)
func (t *TokenBucketLimiter) AllowN(ctx context.Context, key string, n int64) (Result, error) // refill by elapsed*rate capped at burst; deduct n if available else Allowed:false, RetryAfter=(n-tokens)/rate
func (t *TokenBucketLimiter) Reset(ctx context.Context, key string) error                     // delete bucket (used by force-resume/ban-drain)
```
`// ponytail: in-memory, per-instance buckets (matches ReadQueryBudget precedent in query.go). Multi-instance aggregate cap is therefore per-instance/approximate — a Redis token bucket is the upgrade path if a fleet needs a global cap.`

### 3.2 `pkg/ratelimiter/pacer.go` (NEW) — blocking wrapper (reject→pace), backend-agnostic
```go
type PacerConfig struct {
	Enabled           bool
	Mode              string        // "pace" | "reject"
	AccountRate       float64       // OUTBOUND_PACE_RATE_PER_SECOND
	AccountBurst      float64       // OUTBOUND_PACE_BURST
	MaxWait           time.Duration // OUTBOUND_PACE_MAX_WAIT_SECONDS
	Jitter            time.Duration // OUTBOUND_PACE_JITTER_MS
	RecipientLimit    int64         // OUTBOUND_PACE_PER_RECIPIENT_REQUESTS
	RecipientWindow   time.Duration // OUTBOUND_PACE_PER_RECIPIENT_WINDOW_SECONDS
}
type Pacer struct {
	enabled   bool
	mode      string
	account   Limiter // TokenBucketLimiter, key "acct:"+phone
	recipient Limiter // MemoryLimiter (fixed-window reject), key "rcpt:"+phone+":"+recipient
	maxWait   time.Duration
	jitter    time.Duration
	banUntil  func(phone string) (time.Time, bool) // nil-safe; from PRD-4 status store
	sleep     func(ctx context.Context, d time.Duration) error // test seam; default select-on-timer
	now       func() time.Time
	rnd       *rand.Rand
}
func NewPacer(cfg PacerConfig, banUntil func(string) (time.Time, bool)) *Pacer

// Wait clears an outbound op or returns *RateLimitError. Order:
//  1. if !enabled → nil
//  2. ban gate: banUntil(phone) in future → *RateLimitError{RetryAfter: until-now}
//  3. per-recipient hard cap (recipient.AllowN) → if !Allowed → *RateLimitError (never paced)
//  4. account token bucket: loop AllowN(n); Allowed → nil; else mode=="reject" → *RateLimitError;
//     mode=="pace" → sleep min(res.RetryAfter+jitter, remaining-maxWait); on maxWait exhausted → *RateLimitError
func (p *Pacer) Wait(ctx context.Context, phone, recipient string, n int64) error

// Drain empties the account bucket (called on TemporaryBan entry). ponytail: optional — the
// ban gate already blocks sends; Drain only prevents a token-burst the instant a ban lifts.
func (p *Pacer) Drain(phone string)
func (p *Pacer) Resume(phone string) // force-resume: clears in-memory ban cache (admin)
```
Key design notes:
- **Per-recipient = hard reject, per-account = pace.** Rejecting repeat messages to one recipient is the *correct* behavior (repeat sends to a non-contact is the #1 ban trigger — you want a hard stop, not a block-and-eventually-send). Aggregate throughput just needs spacing → block. This sidesteps the two-blocking-buckets over-wait/double-spend problem entirely.
- `banUntil` is a plain func (no `internal/` import from `pkg/`); the adapter to PRD-4's store is built in wire (3.6).
- Unified entry: **both** direct usecase and queue worker call `pacer.Wait`; they differ only in how they map the returned `*RateLimitError` (usecase → domain `ErrTooManyRequests`; worker → existing `errors.As` retry at `worker_pool.go:182`).

### 3.3 `config/config.go` — add fields (§5) after the `ReadQuery*` block.

### 3.4 `internal/whatsapp/manager.go` + `internal/whatsapp/client.go` — add ban-state accessor
```go
// Manager interface + impl (thin passthrough to PRD-4's session-status store)
BanState(phoneNumber string) (bannedUntil time.Time, banned bool)
```
Reads the `ban_expires_at` PRD-4 persists. If PRD-4 stored it in-memory only, this reads that map; if DB-backed, a cached lookup. `// ponytail: read-through of PRD-4's store; no new state owner.`

### 3.5 `internal/usecase/whatsapp/message_usecase.go`
- Struct: add `pacer *ratelimiter.Pacer` field; constructor param `pacer *ratelimiter.Pacer` (keep `limiter` for now or drop it — see cleanup below).
- Replace each direct-mode block:
```go
res, err := uc.limiter.Allow(ctx, phoneNumber)  // remove (appears at :256, :715, :788, :865)
if !res.Allowed { … 429 … }
```
with:
```go
if err := uc.pace(ctx, phoneNumber, req.Msisdn, 1); err != nil { return nil, nil, err }
```
- Add helper (mirrors `spendReadBudget` shape):
```go
// pace clears an outbound op through the pacer, mapping a *RateLimitError to a
// 429 domain error and a limiter infra error to a 500. Supersedes spendActionBudget.
func (uc *WhatsappMessageUsecase) pace(ctx context.Context, phone, recipient string, n int64) error {
	err := uc.pacer.Wait(ctx, phone, recipient, n)
	if err == nil { return nil }
	var rl *ratelimiter.RateLimitError
	if errors.As(err, &rl) {
		return errDomain.NewError(errDomain.ErrTooManyRequests,
			fmt.Errorf("rate limited; retry after %ds", int(rl.RetryAfter.Seconds())+1))
	}
	return errDomain.NewError(errDomain.ErrInternalFailure, err)
}
```
- `EditMessage` (:952) and `DeleteMessage` (:927): add `uc.pace(ctx, phoneNumber, req.Msisdn, 1)` before the manager call — **closes the current unpaced-edit/delete gap.**
- `ReactToMessage` (:909): replace `uc.spendActionBudget(...)` → `uc.pace(ctx, phoneNumber, req.Msisdn, 1)`.

### 3.6 `internal/usecase/whatsapp/interaction_usecase.go`
- Delete `spendActionBudget` (:18-25).
- `MarkRead` (:53) and `SendChatPresence` (:76): replace `uc.spendActionBudget(ctx, phoneNumber)` → `uc.pace(ctx, phoneNumber, target, 1)`.

### 3.7 `internal/usecase/whatsapp/group_usecase.go` (PRD-3 file) — lift the gate
- Remove the `if !cfg.GroupAddParticipantsEnabled { return 403 }` guard on `action=add`/add-on-create and on `GROUP_JOIN_VIA_LINK_ENABLED`. Keep `GROUP_MAX_PARTICIPANTS_PER_REQUEST` cap (pre-call 400). Before `UpdateGroupParticipants`:
```go
if err := uc.pace(ctx, phoneNumber, groupJID, int64(len(participants))); err != nil { return err } // AllowN = one token per participant
```
- Keep `GROUP_ADD_PARTICIPANTS_ENABLED` / `GROUP_JOIN_VIA_LINK_ENABLED` as **kill-switches** but flip defaults to `true` (pacing now supplies safety). Documented as "leave off only to fully disable the op."

### 3.8 `internal/queue/handlers/outgoing_message_handler.go`
- Struct: replace `Limiter ratelimiter.Limiter` with `Pacer *ratelimiter.Pacer`.
- Replace the `res, err = h.Limiter.Allow(ctx, job.PhoneNumber)` block (:63-84) with:
```go
if err := h.Pacer.Wait(ctx, job.PhoneNumber, job.To, 1); err != nil {
	var rl *ratelimiter.RateLimitError
	if errors.As(err, &rl) {
		h.Logger.Warn(traceID, "paced/limited", …)
		return rl // worker_pool.go:181 reschedules with rl.RetryAfter delay — unchanged
	}
	return err
}
```

### 3.9 Wire / providers
- `pkg/ratelimiter/provider.go` (or new `ProvideOutboundPacer`): keep signature `func(cfg *config.Config, banUntil func(string)(time.Time,bool)) *Pacer` — **but** `pkg` can't import `internal/whatsapp`, so build the `banUntil` adapter in `internal/infrastructure`:
```go
// internal/infrastructure/pacer_provider.go (new)
func ProvideOutboundPacer(cfg *config.Config, mgr whatsapp.Manager) *ratelimiter.Pacer {
	return ratelimiter.NewPacer(pacerConfigFrom(cfg), mgr.BanState)
}
```
- `internal/infrastructure/wire.go`: add `ProvideOutboundPacer` to `wire.Build(...)`. **One shared singleton** (like the existing `limiter` at `wire_gen.go:77` shared between usecase:81 and workers:86) so direct + queue count the same buckets.
- `internal/usecase/whatsapp/provider.go`: add `pacer *ratelimiter.Pacer` param → `NewWhatsappMessageUsecase(..., pacer)`.
- `internal/infrastructure/workers_provider.go`: pass `Pacer: pacer` into `OutgoingMessageHandler`.
- Run `wire gen ./internal/infrastructure`.

**Cleanup:** `MESSAGE_RATE_LIMIT_*` + `uc.limiter` become dead once all send paths use the pacer. Lazy call: keep `MESSAGE_RATE_LIMIT_REQUESTS/DURATION` as the **fallback source for `OUTBOUND_PACE_*` defaults** and drop the raw `limiter` field, OR keep `limiter` only if the pacer is disabled (`OUTBOUND_PACE_ENABLED=false` → fall back to old `limiter.Allow` reject). Recommend the latter for a safe rollout: pacer wraps limiter behind the enable flag.

---

## 4. Migration DDL

**Preferred: none.** PRD-4's session-status store already persists `ban_expires_at` keyed by phone (PRD-4 §"Session-status tracking", edge-case row: `persist {…, banned, ban_expires_at} keyed by phone`). `Manager.BanState` reads it. #2 adds **zero** migrations.

**Fallback only if PRD-4 did NOT persist ban state (in-memory only)** — add restart-safe persistence in `internal/whatsapp/migration.go` (`runMigrations → runPacingMigrations`, dual-dialect, `CREATE TABLE IF NOT EXISTS`, data-preserving):
```sql
CREATE TABLE IF NOT EXISTS session_pacing_state (
    phone_number TEXT    PRIMARY KEY,
    banned_until BIGINT  NOT NULL DEFAULT 0,   -- unix seconds; 0 = not banned
    ban_reason   INTEGER NOT NULL DEFAULT 0,   -- events.TempBanReason
    updated_at   BIGINT  NOT NULL DEFAULT 0
);
```
Upsert (portable, `$1/$2` — lib/pq rejects `?`):
```sql
INSERT INTO session_pacing_state (phone_number, banned_until, ban_reason, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (phone_number) DO UPDATE
SET banned_until = EXCLUDED.banned_until, ban_reason = EXCLUDED.ban_reason, updated_at = EXCLUDED.updated_at;
```
Read: `SELECT banned_until, ban_reason FROM session_pacing_state WHERE phone_number = $1`. `TEXT/BIGINT/INTEGER` are dialect-neutral (SQLite + Postgres).

---

## 5. New config keys (`config/config.go` + `.env.example`)

| Field / env | Type | Default | Purpose |
|---|---|---|---|
| `OutboundPaceEnabled` / `OUTBOUND_PACE_ENABLED` | bool | `true` | Master switch; `false` → fall back to legacy `limiter.Allow` reject |
| `OutboundPaceMode` / `OUTBOUND_PACE_MODE` | string | `pace` | `pace` (block ≤ MaxWait) or `reject` (immediate 429 / immediate requeue) |
| `OutboundPaceRatePerSecond` / `OUTBOUND_PACE_RATE_PER_SECOND` | float64 | `1` | Account bucket steady refill (tokens/sec) |
| `OutboundPaceBurst` / `OUTBOUND_PACE_BURST` | float64 | `5` | Account bucket capacity (allowed short burst) |
| `OutboundPaceMaxWaitSeconds` / `OUTBOUND_PACE_MAX_WAIT_SECONDS` | int64 | `30` | Block deadline; exceeded → 429 (direct) / requeue (queue) |
| `OutboundPaceJitterMs` / `OUTBOUND_PACE_JITTER_MS` | int64 | `250` | Random extra gap added to each paced sleep (traffic non-periodic) |
| `OutboundPacePerRecipientRequests` / `OUTBOUND_PACE_PER_RECIPIENT_REQUESTS` | int64 | `10` | Hard per-`(account,recipient)` cap (reject, not paced) |
| `OutboundPacePerRecipientWindowSeconds` / `OUTBOUND_PACE_PER_RECIPIENT_WINDOW_SECONDS` | int64 | `60` | Window for the per-recipient cap |
| `GroupAddParticipantsEnabled` / `GROUP_ADD_PARTICIPANTS_ENABLED` | bool | **`true`** (was `false`) | Kill-switch only; pacing now supplies ban-safety |
| `GroupJoinViaLinkEnabled` / `GROUP_JOIN_VIA_LINK_ENABLED` | bool | **`true`** (was `false`) | Kill-switch only |

Add a `GetOutboundPaceMaxWait()`/similar helper if you prefer (matches `GetRateLimitDuration()` at config.go:232). Clamp in `normalize()`: rate>0, burst≥1, maxWait≥0.

---

## 6. `docs/openapi.yaml` delta (bump version)

- **No new paths/request schemas** for sends (bodies unchanged).
- Add to every send/action/group-mutation path a **429** response referencing a shared component:
```yaml
components:
  responses:
    RateLimited:
      description: Outbound pacing budget exhausted, per-recipient cap hit, or account temporarily banned.
      headers:
        Retry-After: { schema: { type: integer }, description: Seconds until retry }
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
```
- On `POST /group/:chat/participants`: drop the "403 when `GROUP_ADD_PARTICIPANTS_ENABLED=false`" note; document add is enabled + paced; keep the 400 `GROUP_MAX_PARTICIPANTS_PER_REQUEST` response.
- (Optional) add `POST /admin/pacing/:phone/resume` under the admin tag with `security: [adminBearer]`, response `{phone, resumed}` — only if the admin endpoint is built.
- Prose note: sends may now exhibit **added latency** (paced block) instead of a fast 429.

---

## 7. Edge cases, partial-failure semantics, gating

- **Ban entry (`TemporaryBan`):** PRD-4's `HandleEvent` case persists `banned_until = now + Expire`. `#2` optionally calls `pacer.Drain(phone)` there. Every subsequent `Wait` returns 429 with `Retry-After = banned_until-now` until expiry; auto-resumes (no timer needed — gate is checked lazily on each send). Restart-safe via the persisted column.
- **`Expire == 0`** (`events.go:157` — unknown ban duration): treat as a conservative default hold (e.g. `OUTBOUND_PACE_MAX_WAIT_SECONDS` × N or a `BAN_DEFAULT_HOLD_SECONDS`); do **not** treat 0 as "no ban."
- **Bulk group-add partial failure:** `UpdateGroupParticipants` returns per-participant `[]types.GroupParticipant` (some ok, some failed). Pacing spends `AllowN(len)` **up front**; the per-participant result array is assembled by PRD-3's usecase unchanged. If `AllowN` can't grant `len` tokens and mode=pace, it blocks until the bucket can cover them (or 429 on deadline) — it does **not** partially send.
- **Queue mode blocking a worker:** `Wait` is bounded by `MaxWait`; with `WORKER_OUTGOING_MESSAGES=3` and ~1s waits this is fine. On `MaxWait` exceeded it returns `*RateLimitError` → the worker reschedules (delivery ack'd, re-published with delay) rather than holding the AMQP delivery indefinitely.
- **Direct-mode context deadline:** `Wait` honors `ctx` (Fiber request ctx). If the HTTP client hangs up mid-block, `ctx.Done()` → return promptly (map to 499/normal cancel), don't leak a goroutine.
- **Double-spend under contention:** none — per-recipient is reject-only, per-account is the single blocking bucket. No two-bucket over-wait.
- **Gating / defaults-flip:** `GROUP_ADD_PARTICIPANTS_ENABLED` and `GROUP_JOIN_VIA_LINK_ENABLED` flip to `true` (pacing lifts the Phase-E hard gates); they remain as pure off-switches. `GROUP_MAX_PARTICIPANTS_PER_REQUEST` cap stays (pre-call 400). `OUTBOUND_PACE_ENABLED=false` is the escape hatch back to legacy reject behavior.
- **Noop/disabled:** `OUTBOUND_PACE_ENABLED=false` → `Wait` returns `nil` immediately; callers keep working (fall back to legacy `limiter` if you kept it).

---

## 8. Unit-test seams (highest seam, no live client)

**`pkg/ratelimiter/tokenbucket_test.go`** (fake clock via injected `now`):
- Refill math: burst=5,rate=1/s → 5 immediate `Allow`s pass, 6th denies with `RetryAfter≈1s`; advance clock 3s → 3 tokens back.
- `AllowN(3)` deducts 3; denies when <3 with `RetryAfter=(3-tokens)/rate`.
- `Reset` empties a bucket.

**`pkg/ratelimiter/pacer_test.go`** (inject `now`, `sleep`, and a `banUntil` stub; `sleep` records calls instead of real sleeping):
- **pace mode:** account bucket empty → `Wait` calls `sleep(≈RetryAfter+jitter)` then returns `nil` (assert it eventually clears within `MaxWait`).
- **deadline:** total waited > `MaxWait` → returns `*RateLimitError` (assert via `errors.As`).
- **reject mode:** over-budget → immediate `*RateLimitError`, `sleep` never called.
- **per-recipient hard cap:** N+1 sends to same recipient → `*RateLimitError`, and it is **not** paced (sleep not called for the recipient dimension).
- **ban gate:** `banUntil` returns future → `*RateLimitError{RetryAfter≈until-now}`; returns past/`false` → clears.
- **jitter bounded:** every sleep ≤ `RetryAfter + Jitter`.
- **disabled:** `enabled=false` → `nil`, zero interactions.

**`internal/usecase/whatsapp/*_test.go`** (build `WhatsappMessageUsecase{pacer: NewPacer(...)}` like `newTestUsecase` in `query_test.go`, with a fake `Manager`):
- `pace` helper maps `*RateLimitError` → `errDomain.ErrTooManyRequests` (assert `errors.As` + `ServiceError()`), infra error → `ErrInternalFailure`.
- `ReactToMessage` / `MarkRead` / `SendChatPresence` / `EditMessage` / `DeleteMessage` all spend a token (exhaust a burst-1 bucket → 2nd call 429) — proves the interim `action:` cap and the edit/delete gap are gone.
- Group-add: `action=add` with `> GROUP_MAX_PARTICIPANTS_PER_REQUEST` still 400 before pacing; within cap spends `AllowN(len)`.

**No live-WhatsApp tests** — like PRD-4 notes, the `TemporaryBan` mapping isn't CI-testable; the `banUntil` callback boundary is unit-tested with a stub, and the whatsmeow event → `banned_until` write is PRD-4's (already locked to `errors.Is`/`As` sentinels per ADR-0002).

---

## DoD surfaces to update (per repo convention)
Code + tests (pacer/token-bucket unit + usecase seams) · `docs/openapi.yaml` (version bump + `RateLimited` response) · `llms.txt` · `wiki/Environment-Variables.md` + `wiki/Security-Considerations.md` (replace "interim best-effort action cap" note with the real pacer; document per-instance ceiling + ban auto-resume) · `.env.example` (8 `OUTBOUND_PACE_*` + 2 flag default flips) · `CLAUDE.md` (Configuration + the "conversation actions run under spendActionBudget" line → "under the outbound pacer") · `docs/ui/assets/docs/configuration/environment-variables.md` · `CHANGELOG.md`. Small focused commits: (1) token bucket + pacer + tests; (2) config keys; (3) wire/provider singleton; (4) usecase send/action swap; (5) queue worker swap; (6) group-gate lift; (7) docs.

**Files (absolute):** `/Users/glennpray/projects/whatsapp-gateway/pkg/ratelimiter/{tokenbucket.go,pacer.go,provider.go}` · `/Users/glennpray/projects/whatsapp-gateway/config/config.go` · `/Users/glennpray/projects/whatsapp-gateway/internal/usecase/whatsapp/{message_usecase.go,interaction_usecase.go,group_usecase.go,provider.go}` · `/Users/glennpray/projects/whatsapp-gateway/internal/queue/handlers/outgoing_message_handler.go` · `/Users/glennpray/projects/whatsapp-gateway/internal/whatsapp/{manager.go,client.go,handler.go,migration.go}` · `/Users/glennpray/projects/whatsapp-gateway/internal/infrastructure/{wire.go,workers_provider.go,pacer_provider.go}`.

**One flagged simplification:** token buckets are in-memory/per-instance (matches the existing `ReadQueryBudget` in `query.go`); a horizontally-scaled fleet paces per-node, so the aggregate account cap is approximate. Redis token bucket is the upgrade path — add only if a multi-instance global cap is proven necessary.