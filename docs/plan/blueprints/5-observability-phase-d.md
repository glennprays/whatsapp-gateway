I have everything needed. Note: idempotency, health-split, and graceful-shutdown from PRD-4 are already shipped; my feature group (observability parts a–d) is not. Here is the blueprint.

---

# Phase D — Operator Visibility: Implementation Blueprint

Scope: (a) session-status event tracking + dead-client eviction, (b) `GET /admin/sessions[/:phone]`, (c) Prometheus `/metrics`, (d) direct-mode webhook retry parity. Idempotency / health-split / `DisconnectAll` are already shipped — out of scope here.

Key repo facts that shape the design:
- `internal/whatsapp` is a single package; the in-memory client set is the package global `var clients = NewClientStore()` (manager.go:65) — reachable from `handler.go`, `client.go`, a new `session_inventory.go`, etc. without plumbing.
- The `*sqlstore.Container` lives only on the `client` impl struct (`client.container`, client.go:27); the `Manager` does **not** hold it. `client.findDeviceByPhone` already calls `container.GetAllDevices` — so inventory belongs on the `client` impl, delegated through `Manager`.
- Events arrive via `cli.AddEventHandler(func(evt any){ eventHandler(phoneNumber, evt) })` (client.go:93) → `handler.HandleEvent(phoneNumber, evt)` (handler.go:75), a plain `switch v := evt.(type)`. Only `*events.Message` is handled today. `handler` already holds `repository WhatsAppRepository` (handler.go:20).
- `prometheus/client_golang` is **NOT** in go.mod (confirmed) → must `go get`. Fiber↔net/http bridge `github.com/gofiber/fiber/v2/middleware/adaptor` (`HTTPHandler`, adaptor.go:23) IS already vendored — no new dep for mounting.
- Direct-mode status webhooks fire **synchronously inline** from the usecase (`sendDirectSentWebhook`/`sendDirectFailedWebhook`, message_usecase.go:1112/1157) via `uc.webhookSender.Send` — single attempt, blocks the HTTP response up to 5s. The incoming path is already async (`asyncDeliverWebhook`, handler.go:136) but single-attempt.

---

## 1. Endpoints / features + DTOs

### (b) `GET /admin/sessions` — root path, admin bearer
Response `200`:
```
{
  "instance": "gw-pod-7c9f",          // os.Hostname()
  "count": 2,
  "sessions": [
    {
      "phone_masked": "62819104xxxx",  // MaskedPhoneNumber
      "state": "connected",            // connected|disconnected|never_paired|logged_out|banned
      "source": "in-memory",           // in-memory | store
      "reason": "",                    // optional: stream_replaced|connect_failure:405|...
      "last_seen": "2026-07-14T09:12:03Z",
      "ban_expires_at": null           // RFC3339 when state=banned & Expire>0
    }
  ]
}
```
Empty node → `200` with `"count":0,"sessions":[]` (never an error).

### `GET /admin/sessions/:phone` — one account
- `:phone` is a bare number (no normalization — repo convention). `200` single-object `{instance, session:{...}}`; `404 {"error":"not found"}` if the phone matches neither in-memory client, device row, nor status row.

### (c) `GET /metrics` — root path, admin bearer, gated by `METRICS_ENABLED`
Prometheus text exposition. Series (NEVER a phone label):
- `whatsapp_gateway_messages_total{type,mode,result}` — counter. `type`=text|image|audio|video|document|location|poll|sticker|react|edit|delete; `mode`=direct|queue; `result`=success|failure.
- `whatsapp_gateway_webhook_deliveries_total{result,mode,event}` — counter. `result`=success|failure; `mode`=direct|queue; `event`=message.incoming|message.queued|message.sent|message.failed.
- `whatsapp_gateway_sessions{state}` — gauge, computed on scrape from `SessionInventory` (states above).
- Plus default Go/process collectors (free).

### (a) + (d) have no new HTTP surface
(a) mutates internal state on whatsmeow events; (d) changes the transport of existing direct-mode status webhooks (async + retry). No route changes.

### Admin-plane auth semantics (all `/admin/*` and `/metrics`)
- `ADMIN_API_SECRET` **unset** → routes never registered → Fiber default `404` (no leak).
- Set + missing/wrong `Authorization: Bearer <secret>` → `401 {"error":"unauthorized"}`, `subtle.ConstantTimeCompare`.

---

## 2. Exact whatsmeow calls + signatures (verified against module source)

These are **event structs dispatched through `AddEventHandler`**, not error sentinels — so part (a) is a `switch`/type-assert, NOT `errors.Is`. (The `errors.Is` ADR still governs the *client send* error mapping in client.go; it does not apply to the event switch.)

Event types (all in `go.mau.fi/whatsmeow/types/events/events.go`):
```go
// events.go:105
type LoggedOut struct {
    OnConnect bool                  // true = triggered by connect-failure; false = stream:error
    Reason    ConnectFailureReason  // valid only when OnConnect
}
// events.go:117
type StreamReplaced struct{}        // another client connected with same keys
// events.go:151
type TemporaryBan struct {
    Code   TempBanReason
    Expire time.Duration            // 0 = indefinite
}
func (tb *TemporaryBan) String() string   // events.go:156, human reason
// events.go:220
type ConnectFailure struct {
    Reason  ConnectFailureReason
    Message string
    Raw     *waBinary.Node
}
// events.go:164 + method events.go:200
type ConnectFailureReason int
func (cfr ConnectFailureReason) IsLoggedOut() bool  // true for 401/403/406
func (cfr ConnectFailureReason) String() string     // events.go:209, "<code>: <msg>"
```
Note: whatsmeow **also emits `LoggedOut` internally and deletes the device row** for logged-out `ConnectFailure` reasons — so `LoggedOut` is the primary case; `ConnectFailure` with `!Reason.IsLoggedOut()` is a transient/other failure worth recording as `disconnected`.

Client state / lifecycle (`go.mau.fi/whatsmeow/client.go`):
```go
func (cli *Client) IsLoggedIn() bool   // client.go:567 — connected AND authenticated
func (cli *Client) IsConnected() bool  // client.go:643 — websocket up (nil-safe)
func (cli *Client) Disconnect()        // client.go:657 — nil-safe, emits no event
```
Store / container (for the inventory merge):
```go
// store/sqlstore/container.go:156
func (c *Container) GetAllDevices(ctx context.Context) ([]*store.Device, error)
// store/sqlstore/container.go:192  (nil, nil when absent)
func (c *Container) GetDevice(ctx context.Context, jid types.JID) (*store.Device, error)
// store/store.go:217
type Device struct { ID *types.JID; PushName string; /* ... */ }  // ID.User = bare phone
```

HTTP mapping: none of these produce HTTP errors — they mutate persisted `session_status` and evict the in-memory client. The states surface later through `GET /admin/sessions` (`200`) and the gauge. No sentinel→status-code table applies to part (a).

---

## 3. Ordered edit list

### 3.1 config — `config/config.go` (+ `.env.example`)
Add (mapstructure+default tags, matching existing style):
```go
// Admin / metrics plane
AdminAPISecret string `mapstructure:"ADMIN_API_SECRET" default:""`
MetricsEnabled bool   `mapstructure:"METRICS_ENABLED" default:"true"`
// Direct-mode webhook retry parity
WebhookMaxRetries          int   `mapstructure:"WEBHOOK_MAX_RETRIES" default:"3"`
WebhookRetryBackoffSeconds int64 `mapstructure:"WEBHOOK_RETRY_BACKOFF_SECONDS" default:"2"`
```

### 3.2 Migration — `internal/whatsapp/migration.go`
Add `runSessionStatusMigrations(db)` and call it from `runMigrations` (§4). **No FK to `whatsmeow_device`** (device_webhooks FKs with ON DELETE CASCADE — we must NOT, because the row must survive whatsmeow deleting the device on LoggedOut).

### 3.3 Repository — `internal/whatsapp/repository.go`
Extend `WhatsAppRepository` interface + `whatsAppRepository` impl:
```go
UpsertSessionStatus(ctx context.Context, phone, state, reason string, banExpiresAt *time.Time) error
GetSessionStatus(ctx context.Context, phone string) (*waDomain.SessionStatus, error)   // nil,nil if absent
ListSessionStatuses(ctx context.Context) (map[string]waDomain.SessionStatus, error)     // phone → status
```
Upsert uses `INSERT ... ON CONFLICT (phone_number) DO UPDATE`, `$1..$5` placeholders (matches `SetWebhook` at repository.go:26).

### 3.4 Domain DTOs — new `domain/whatsapp/session.go`
```go
type SessionStatus struct {
    State        string     // connected|disconnected|never_paired|logged_out|banned
    Reason       string
    BanExpiresAt *time.Time
    UpdatedAt    time.Time
}
type SessionInventoryItem struct {
    PhoneMasked  string     `json:"phone_masked"`
    State        string     `json:"state"`
    Source       string     `json:"source"`             // in-memory | store
    Reason       string     `json:"reason,omitempty"`
    LastSeen     time.Time  `json:"last_seen"`
    BanExpiresAt *time.Time `json:"ban_expires_at,omitempty"`
}
type SessionInventory struct {
    Instance string                 `json:"instance"`
    Count    int                    `json:"count"`
    Sessions []SessionInventoryItem `json:"sessions"`
}
```
Handler masks phones; the item carries the **already-masked** phone so the usecase/manager can mask centrally.

### 3.5 (a) Event handling — `internal/whatsapp/handler.go`
Extend the `switch` in `HandleEvent` (handler.go:80). New cases persist status via `h.repository.UpsertSessionStatus` and evict via `clients.Delete(phoneNumber)`:
```go
case *events.LoggedOut:
    // whatsmeow has (or will) delete the device row; record before it vanishes.
    h.repository.UpsertSessionStatus(ctx, phoneNumber, "logged_out", loggedOutReason(v), nil)
    clients.Delete(phoneNumber)
case *events.TemporaryBan:
    var exp *time.Time
    if v.Expire > 0 { t := time.Now().Add(v.Expire); exp = &t }
    h.repository.UpsertSessionStatus(ctx, phoneNumber, "banned", v.String(), exp)
    clients.Delete(phoneNumber)
case *events.ConnectFailure:
    if v.Reason.IsLoggedOut() {
        h.repository.UpsertSessionStatus(ctx, phoneNumber, "logged_out", v.Reason.String(), nil)
        clients.Delete(phoneNumber)
    } else {
        // transient/other — record, but do NOT evict (auto-reconnect may recover)
        h.repository.UpsertSessionStatus(ctx, phoneNumber, "disconnected", "connect_failure:"+v.Reason.String(), nil)
    }
case *events.StreamReplaced:
    // another process took the socket; our client is dead. Evict; do not disconnect (already gone).
    h.repository.UpsertSessionStatus(ctx, phoneNumber, "disconnected", "stream_replaced", nil)
    clients.Delete(phoneNumber)
```
Use a bounded `ctx` (e.g. `context.WithTimeout(context.Background(), 5*time.Second)`) — the event has no request ctx. **No webhook emitted** (decision #7 — #1 later only adds emission here). Also increment the metric `sessions` implicitly (gauge is scrape-computed, so nothing to do). `loggedOutReason(v)` = `v.Reason.String()` when `v.OnConnect`, else `"stream_error"`.

### 3.6 (b) Inventory — new `internal/whatsapp/session_inventory.go`
Add to the **`Client`** interface + impl (it holds `container`):
```go
ListDeviceJIDs(ctx context.Context) (map[string]time.Time, error) // phone → last_seen(now); wraps container.GetAllDevices
```
Actually cleaner: put the whole merge on `client` since it alone has `container` and can read the `clients` global. Add to `Client` interface:
```go
SessionInventory(ctx context.Context) ([]waDomain.SessionInventoryItem, error)
```
Impl merges three sources into a `map[string]item` keyed by phone:
1. **Persisted status** (`repository.ListSessionStatuses`) — authoritative for `banned`/`logged_out`; source=store, last_seen=UpdatedAt, ban_expires_at.
2. **Device rows** (`container.GetAllDevices`, phone via `WhatsappDecomposeJID(device.ID.User)`) — a row without an in-memory client & no worse persisted status ⇒ `disconnected`, source=store.
3. **In-memory clients** (`clients.GetAll()`) — highest fidelity, source=in-memory:
   - `c.Store == nil || c.Store.ID == nil` ⇒ `never_paired`
   - `c.IsConnected() && c.IsLoggedIn()` ⇒ `connected`
   - else ⇒ `disconnected`
   - **Exception:** a persisted `banned` (unexpired) or `logged_out` wins over a lingering in-memory client (honest dead-session reporting).
   Phone masked here.

`Manager` gains (manager.go interface + thin wrapper delegating to `m.Client.SessionInventory`):
```go
SessionInventory(ctx context.Context, traceID string) (*waDomain.SessionInventory, error) // fills Instance=os.Hostname(), Count
GetOneSession(ctx context.Context, traceID, phone string) (*waDomain.SessionInventoryItem, error) // nil,nil if none
```

### 3.7 (b) Admin handler — new `internal/handler/admin/session_handler.go` (+ `provider.go`)
```go
type AdminHandler struct { manager whatsapp.Manager; logger *log.Logger }
func (h *AdminHandler) Sessions(c *fiber.Ctx) error    // GET /admin/sessions → 200 inventory
func (h *AdminHandler) Session(c *fiber.Ctx) error     // GET /admin/sessions/:phone → 200 | 404
func ProvideAdminHandler(m whatsapp.Manager, l *log.Logger) *AdminHandler
```
Add `AdminHandler *admin.AdminHandler` to `handler.Handler` aggregate (handler.go) + `ProvideMainHandler` param; register `admin.ProvideAdminHandler` in `wire.go`; `wire gen ./internal/infrastructure`.

### 3.8 Admin middleware — new `internal/middleware/admin.go`
```go
func NewAdminAuth(secret string) fiber.Handler // constant-time bearer; 401 JSON on mismatch
```
Uses `crypto/subtle.ConstantTimeCompare`. Only ever mounted when secret non-empty (router decides), so it never has to special-case the unset path.

### 3.9 (c) Metrics — new `internal/metrics/metrics.go`
```go
package metrics
var (
  MessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
    Namespace:"whatsapp_gateway", Name:"messages_total"}, []string{"type","mode","result"})
  WebhookDeliveriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
    Namespace:"whatsapp_gateway", Name:"webhook_deliveries_total"}, []string{"result","mode","event"})
)
func Register()                                   // MustRegister the two counters (idempotent guard)
func RegisterSessionsGauge(fn func() map[string]int) // registers a scrape-time Collector
func RecordSend(msgType, mode string, err error)  // result from err==nil
func RecordWebhook(mode, event string, err error)
func Handler() fiber.Handler                       // adaptor.HTTPHandler(promhttp.Handler())
```
`sessionsCollector` implements `prometheus.Collector`; `Collect` calls `fn()` (a closure over `manager.SessionInventory`) and emits `whatsapp_gateway_sessions{state=...}` per state. **Scrape-time computation = no gauge mutation on every event = always correct, least code.** `ponytail: gauge recomputed per scrape; if SessionInventory ever gets expensive, cache it behind READ_QUERY TTL.`

Instrumentation call-sites:
- `metrics.RecordSend(...)` in each usecase `Send*` at the direct-success, direct-fail, and enqueue points (message_usecase.go). Mode is known there (queue vs direct branch).
- `metrics.RecordWebhook("direct", event, err)` inside the three `sendDirect*Webhook`/`sendQueuedWebhook` helpers; `metrics.RecordWebhook("queue", event, err)` inside `outgoing_message_handler.sendStatusWebhook` (line ~296) and the incoming `deliverWebhook` (handler.go:172, event=message.incoming, mode=direct/queue by whether it went through the queue). `ponytail: if touching all call sites is too spread, defer webhook counter to a follow-up commit — messages_total + sessions gauge are the load-bearing two.`

### 3.10 (d) Webhook retry parity — `internal/whatsapp/webhook_sender.go` + `message_usecase.go`
Add config-driven retry to `WebhookSender`:
```go
type WebhookSender struct { /* + */ maxRetries int; backoff time.Duration; sem chan struct{} }
func NewWebhookSender(cipher, logger, cfg *config.Config) *WebhookSender // read WebhookMaxRetries/BackoffSeconds
// bounded exponential backoff around existing Send:
func (ws *WebhookSender) sendWithRetry(ctx context.Context, url, secret string, payload any) error
// fire-and-forget, DETACHED context, semaphore-bounded (parity w/ handler.webhookSem):
func (ws *WebhookSender) SendAsync(url, secret string, payload any)
```
`SendAsync` spawns a goroutine that builds its **own** `context.WithTimeout(context.Background(), ...)` — **critical**: the three direct helpers currently pass the request ctx (`c.Context()`), which is cancelled the instant the HTTP response is written; a naive `go ws.Send(ctx,...)` would fail every time. Then rewrite `sendDirectSentWebhook`/`sendDirectFailedWebhook` (and optionally `sendQueuedWebhook`) to call `uc.webhookSender.SendAsync(webhook.Url, webhook.HmacSecret, payload)` instead of the inline `uc.webhookSender.Send(ctx, ...)`. Update `ProvideWebhookSender` (provider.go:32) to pass `cfg`. Queue-mode handlers keep calling `Send` (single attempt) — they already have RabbitMQ-level retry; do NOT double-retry there.
`ponytail: reuse handler.go's maxConcurrentWebhooks=50 semaphore pattern; drop-on-full with a warn log (best-effort, documented).`

### 3.11 Router — `internal/router/router.go` + `ProvideRouter`
Add `manager whatsapp.Manager` param to `ProvideRouter` (provider.go) + `SetupRouter` (one Wire edit — no import cycle: `whatsapp` doesn't import `router`). At the end of `SetupRouter`, gated:
```go
if cfg.AdminAPISecret != "" {
    adminAuth := middleware.NewAdminAuth(cfg.AdminAPISecret)
    adminGrp := app.Group("/admin", adminAuth)           // ROOT
    adminGrp.Get("/sessions", h.AdminHandler.Sessions)
    adminGrp.Get("/sessions/:phone", h.AdminHandler.Session)
    if cfg.MetricsEnabled {
        metrics.Register()
        metrics.RegisterSessionsGauge(func() map[string]int { return tallyStates(manager) })
        app.Get("/metrics", adminAuth, metrics.Handler())  // ROOT
    }
}
```
When secret empty: nothing registered → `/admin/*` and `/metrics` 404. `tallyStates` calls `manager.SessionInventory` and buckets by state.

### 3.12 Wire — `internal/infrastructure/wire.go`
Add `admin_handler.ProvideAdminHandler`; add `AdminHandler` to `ProvideMainHandler`; `ProvideRouter` gains the already-provided `whatsapp.Manager`. Run `wire gen ./internal/infrastructure`.

---

## 4. Migration DDL (dual-dialect, `$`-less DDL, additive, data-preserving)

Append to `runMigrations` (migration.go:8) and add:
```go
func runSessionStatusMigrations(db *sql.DB) error {
    query := `
    CREATE TABLE IF NOT EXISTS session_status (
        phone_number   TEXT PRIMARY KEY,
        state          TEXT NOT NULL,
        reason         TEXT,
        ban_expires_at TIMESTAMP,
        updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
    _, err := db.ExecContext(context.Background(), query)
    return err
}
```
Notes: `CREATE TABLE IF NOT EXISTS`, dialect-neutral types (`TEXT`/`TIMESTAMP`, same as existing tables). **Deliberately no `REFERENCES whatsmeow_device(jid) ON DELETE CASCADE`** — unlike `device_webhooks` — so a banned/logged-out record persists after whatsmeow deletes the device row (the entire point of part (a)). Keyed by bare phone (repo convention: no normalization). Upsert (`UpsertSessionStatus`) is `INSERT ... ON CONFLICT (phone_number) DO UPDATE SET state=EXCLUDED.state, reason=EXCLUDED.reason, ban_expires_at=EXCLUDED.ban_expires_at, updated_at=CURRENT_TIMESTAMP` — portable across SQLite(modernc) + Postgres, `$1..$5`.

---

## 5. New config keys

| Key | Type | Default | Notes |
|---|---|---|---|
| `ADMIN_API_SECRET` | string | `""` | Empty ⇒ `/admin/*` + `/metrics` unregistered (404). Set ⇒ bearer, constant-time. |
| `METRICS_ENABLED` | bool | `true` | Toggles `/metrics` independently; still needs `ADMIN_API_SECRET` set to be reachable. |
| `WEBHOOK_MAX_RETRIES` | int | `3` | Direct-mode status-webhook retry attempts (queue mode keeps RabbitMQ retry). |
| `WEBHOOK_RETRY_BACKOFF_SECONDS` | int64 | `2` | Exponential-backoff base for direct-mode retries. |

`IDEMPOTENCY_*` and `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS` already exist. Mirror all four into `.env.example`, `wiki/Environment-Variables.md`, and `CLAUDE.md` Configuration list. **New Go dependency:** `github.com/prometheus/client_golang` (`go get`) — pulls indirect `client_model`, `common`, `procfs` (`cespare/xxhash` already present).

---

## 6. openapi.yaml delta (bump `info.version` 0.18.0 → 0.19.0)

Add `tags: [{name: Admin}, {name: Metrics}]`. Because admin/metrics live at ROOT (outside the `/api/v1` server base), document them with absolute paths and an `x-root-path: true` note (the served spec injects only the api-base server URL).

New paths:
- `GET /admin/sessions` (tag Admin, `security: [{adminBearer: []}]`): `200` → `SessionInventory` schema; `401` → `{error}`; `404` (plane disabled) noted in description.
- `GET /admin/sessions/{phone}`: param `phone` (string). `200` → `SessionInventoryItem` wrapper; `401`; `404`.
- `GET /metrics` (tag Metrics, `adminBearer`): `200` `text/plain` Prometheus exposition; `401`; `404`.

New schemas: `SessionInventory`, `SessionInventoryItem` (fields per §3.4; `state` enum `connected|disconnected|never_paired|logged_out|banned`; `source` enum `in-memory|store`).

New `securityScheme` `adminBearer` (http/bearer).

Also (part d is transparent) add a one-line note under `POST /message/*` descriptions: direct-mode status webhooks are delivered async with bounded retry (best-effort; durable delivery = enable RabbitMQ). No schema change for (d).

---

## 7. Edge cases, partial-failure, gating

- **404-vs-401 leak (security):** empty `ADMIN_API_SECRET` ⇒ routes literally not registered ⇒ Fiber default 404 — never a 401 that would confirm the plane exists. Enforced at the router `if` in §3.11, not inside the middleware.
- **Constant-time compare:** `subtle.ConstantTimeCompare` on the bearer; unequal length is inherently non-matching (subtle returns 0) — still constant-time relative to the configured secret.
- **whatsmeow deletes the device row on LoggedOut:** handled by persisting to the un-FK'd `session_status` table *before* eviction; inventory then reports `logged_out` from source=store even though `GetAllDevices` no longer lists it.
- **Ban expiry:** `banned` with `ban_expires_at` in the past is downgraded to `disconnected` at inventory-read time (don't trust a stale ban forever); the row is left as-is (lazy — no sweeper; corrected on next successful connect event or read).
- **Per-instance honesty (horizontal scaling):** a device row with no in-memory client here ⇒ `disconnected`, source=store — it may be live on *another* pod. Inventory + gauge are explicitly per-instance; response carries `instance` (os.Hostname). Documented in wiki + PRD.
- **Metrics cardinality:** phone is **never** a label — enforced by construction (label sets are fixed string slices; `sessions` gauge is bucketed by `state` only). `/admin/sessions` carries per-account (masked) detail instead.
- **StreamReplaced/ConnectFailure eviction vs auto-reconnect:** LoggedOut / TemporaryBan / logged-out ConnectFailure / StreamReplaced ⇒ evict (`clients.Delete`); a *transient* ConnectFailure (`!IsLoggedOut()`) records `disconnected` but does **not** evict, so whatsmeow's `EnableAutoReconnect` can recover the same client object.
- **(d) detached context (correctness, load-bearing):** async webhook goroutines must use `context.Background()`-derived ctx, else the request-ctx cancellation fails every retry. Semaphore-bounded, drop-on-full with a warn (best-effort; no DLQ in direct mode — documented). Retry exhaustion has **no HTTP effect** (already returned) — logged at error.
- **Gating / default-off:** whole admin plane is default-OFF (`ADMIN_API_SECRET=""`). `/metrics` additionally gated by `METRICS_ENABLED` (default on, but unreachable without the secret). No ban-risky bulk op here — nothing behind an enable-flag beyond the plane itself.
- **Fail-open on status persistence:** an `UpsertSessionStatus` DB error in `HandleEvent` is logged, not propagated (event handlers can't return errors to whatsmeow); the client is still evicted so it stops looking alive.

---

## 8. Unit-test seams (no live whatsmeow client)

- **Admin middleware (`admin_test.go`):** table-test `NewAdminAuth` — correct bearer → `Next`; wrong/missing/empty → 401 JSON; assert constant-time path exercised. Router-level: with `ADMIN_API_SECRET=""`, assert `/admin/sessions` and `/metrics` return 404 (routes unregistered).
- **SessionInventory merge (`session_inventory_test.go`) — the core derivation:** inject stub inputs (a fake `clients` set, a stub `GetAllDevices`, a stub `ListSessionStatuses`) and assert derived state for every branch: never_paired (`Store.ID==nil`), connected (`IsConnected&&IsLoggedIn`), disconnected (device row, no client), logged_out (status row, device row gone), banned (unexpired) overriding a lingering client, banned-expired → disconnected, source labelling, masking. Factor the pure merge into `func mergeInventory(inMem map[string]clientState, devices map[string]time.Time, status map[string]SessionStatus, host string) []Item` so it's testable without a real `*whatsmeow.Client` (use a small `clientState{paired,connected,loggedIn}` projection).
- **`HandleEvent` status cases (`handler_test.go`):** construct `*events.LoggedOut{}`, `*events.TemporaryBan{Code:.., Expire:..}`, `*events.ConnectFailure{Reason: ConnectFailureLoggedOut}` and `{Reason: ConnectFailureServiceUnavailable}`, `*events.StreamReplaced{}`; with a mock `WhatsAppRepository`, assert the right `UpsertSessionStatus(state, reason, banExpiresAt)` and that `clients.Delete` is/ isn't called (transient ConnectFailure = not deleted). Real live events are not CI-testable — this locks the mapping against constructed values. `ban_expires_at` = now+Expire only when Expire>0.
- **WebhookSender retry (`webhook_sender_test.go`):** point at an `httptest.Server` that fails N times then 200; assert exactly `maxRetries+1` attempts max, backoff grows, gives up after budget. Assert `SendAsync` returns immediately (non-blocking) and uses a non-cancelled ctx even when the caller's ctx is already cancelled (regression guard for the detached-context bug).
- **Metrics (`metrics_test.go`):** `RecordSend`/`RecordWebhook` increment the right label tuple (use `testutil.ToFloat64`); `sessionsCollector.Collect` with a stub state map emits one `sessions{state}` series per state and never a phone label (`testutil.CollectAndCount` + scrape a phone value never appears).
- **Repository round-trip (`repository_test.go`, SQLite in-mem + pg testcontainer):** `UpsertSessionStatus` insert then update (ON CONFLICT), `GetSessionStatus` nil-on-absent, `ListSessionStatuses` map shape, and that the row **survives** a `DELETE FROM whatsmeow_device` (proves the missing FK).

---

Doc DoD (per repo convention, all in follow-up commits): `docs/openapi.yaml` (+version bump), `llms.txt` (admin family + per-instance/metrics note), `wiki/Environment-Variables.md`, `wiki/Security-Considerations.md` (admin-404 plane), `.env.example`, `CLAUDE.md` (endpoints + config), `docs/ui/assets/docs/*`, `CHANGELOG.md`. Small focused commits: (1) migration+repo+DTO, (2) HandleEvent cases, (3) inventory+admin handler+middleware+router, (4) metrics, (5) webhook retry parity, (6) docs.

Everything cited is real; the only signature I could not find as a *sentinel* is because parts (a)/(c)/(d) are gateway-owned — no whatsmeow error sentinels are involved (contrast: the send-path `errors.Is` mapping in `client.go`, which is unchanged here).