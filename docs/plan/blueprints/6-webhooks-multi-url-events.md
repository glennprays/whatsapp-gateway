I have everything I need. Here is the blueprint.

---

# Blueprint — Phase F multi-URL/per-event webhooks + roadmap #1 generic event system

## 0. Grounding summary (what exists today)

- **Storage:** single-row `device_webhooks(jid PK → whatsmeow_device(jid) ON DELETE CASCADE, webhook_url, hmac_secret, created_at, updated_at)` — `internal/whatsapp/migration.go:21`. Repo: `internal/whatsapp/repository.go` (`SetWebhook`/`GetWebhook`/`GetWebhookByPhone`/`DeleteWebhook`).
- **Domain DTO:** `domain/whatsapp/whatsapp.go` — `Webhook{Url, HmacSecret}` (no events field).
- **Event catalog:** `domain/queue/status_webhook.go` — `StatusWebhookEvent` consts `message.queued|sent|failed|incoming`. **These 4 are the entire current vocabulary.**
- **Global event filter (not per-URL):** `config/config.go:100-101` — `WEBHOOK_STATUS_EVENTS_ENABLED` (bool) + `WEBHOOK_STATUS_EVENTS` (comma string, default `message.sent,message.failed`). Split via `strings.Split(...,",")` + `TrimSpace` at 6 dispatch sites.
- **Six dispatch sites** all do `GetWebhook(jid)` → `sender.Send(url, hmac, payload map)`:
  1. `internal/whatsapp/handler.go:148` `deliverWebhook` (direct incoming) — payload has flat `event:"message.incoming"`.
  2. `internal/queue/handlers/incoming_event_handler.go:60` (queue incoming) → publishes one `WebhookDeliveryMessage`.
  3. `internal/usecase/whatsapp/message_usecase.go:1062/1113/1158` `sendQueuedWebhook`/`sendDirectSentWebhook`/`sendDirectFailedWebhook` (direct-mode status).
  4. `internal/queue/handlers/outgoing_message_handler.go:200` `sendStatusWebhook` (queue-mode status).
  5. `internal/queue/handlers/dlq_handler.go:104` `notifyDeadLettered`.
- **Event ingress:** `client.go:93` `cli.AddEventHandler` → `HandleEvent(phoneNumber, evt)` (`handler.go:75`). The `switch` handles **only `*events.Message`**; every session/receipt event hits the implicit no-op default. `phoneNumber` (not jid) is the identifier; jid resolved via `clients.Get(phone).Store.ID`.
- **Sender:** `internal/whatsapp/webhook_sender.go` — concrete `*WebhookSender`, SSRF-guarded dialer, HMAC `X-Webhook-Signature: sha256=…`, 5s timeout. HMAC secret stored **encrypted** (`m.Cipher.Encrypt`), decrypted inside `Send`.
- **Phase D status:** the `events.LoggedOut/TemporaryBan/ConnectFailure/StreamReplaced` `HandleEvent` cases (PRD-4 feature 2) are **☐ not started**; no `session_status` table exists yet. This blueprint's #1 portion **is** the emission layer that rides on top of that switch — sequence them so one commit owns the switch.

**Design decision (events[] storage): comma-separated `TEXT` column, `''` = all events.** Rationale (ponytail): the catalog is a fixed set of dotted identifiers with no commas; the codebase already stores + splits an event list this exact way (`WEBHOOK_STATUS_EVENTS`). Postgres `text[]` needs `pq.Array` and is not SQLite-portable; JSON TEXT needs marshal/unmarshal. Comma TEXT is dialect-neutral, matches the task's "comma string" option and the existing split/trim helper. → skipped: JSON/array column, add when an event name ever needs a comma (never, catalog is fixed).

**Design decision (PK): composite `(jid, url)`, no synthetic id.** Makes the backfill a pure portable `INSERT…SELECT…ON CONFLICT(jid,url) DO NOTHING` with no non-portable UUID/serial generation. Delete/upsert key on `(jid,url)`.

---

## 1. Endpoints / features

### 1a. Rewritten `GET|POST|DELETE /api/webhook` (multi-URL, back-compat preserved)

**`POST /api/webhook`** — register/upsert one subscription.
Request DTO (`domain/whatsapp.Webhook`, extended):
| field | type | notes |
|---|---|---|
| `url` | string (required, validated) | subscription URL |
| `hmac_secret` | string (optional) | per-subscription secret |
| `events` | `[]string` (optional) | subset of the fixed catalog; **omitted/empty = all events** (matches today's single-URL-gets-everything) |

Response: `{"success": true}` (unchanged wire shape).
Errors: `400` unknown event (not in catalog) / invalid URL; `409` client not logged in (existing `manager.SetWebhookURL` gate); `401` no session.

**`GET /api/webhook`** — list subscriptions. Back-compat response:
```json
{ "url": "<first subscription url or empty>",
  "subscriptions": [ { "url": "...", "events": ["message.incoming"], "has_hmac": true } ] }
```
`url` retained (= first sub) so existing consumers reading `.url` don't break; `subscriptions[]` is additive. HMAC secret **never** returned (only `has_hmac` bool).

**`DELETE /api/webhook`** — unregister.
- No body (legacy) → delete **all** subscriptions for the account (matches today's semantics).
- Body `{"url": "..."}` → delete that one subscription.
Response `{"success": true}` (unchanged).

### 1b. Generic envelope + dispatcher (roadmap #1)

No new HTTP endpoint — outbound only. A single fan-out primitive replaces the 6 dispatch sites: for a `(jid, event, payload-map)`, load all subscriptions for `jid`, keep those whose `events` filter matches (`''` = all), and `Send` (direct) / publish N `WebhookDeliveryMessage` (queue) to each.

**Envelope:** keep the current **flat** shape (roadmap §3.3 recommendation — `message.incoming` fields stay top-level for zero-break). Every payload map already carries `event` at top level; the dispatcher just filters + fans. New event types add their own top-level fields.

### 1c. New event types (roadmap #1, emitted from the Phase D switch)

| `event` | whatsmeow source | top-level payload fields (beyond `event`,`phone_number`,`jid`,`timestamp`) |
|---|---|---|
| `session.logged_out` | `*events.LoggedOut` | `on_connect` (bool), `reason` (int code), `reason_text` (string) |
| `session.banned` | `*events.TemporaryBan` | `code` (int), `reason_text` (string), `expires_in` (seconds) |
| `session.connect_failure` | `*events.ConnectFailure` | `reason` (int), `reason_text`, `message` (string) |
| `session.connected` | `*events.Connected` | — |
| `session.disconnected` | `*events.Disconnected` | — |
| `session.replaced` | `*events.StreamReplaced` | — |

These 6 are **added to the catalog var** in the #1 sub-phase (Phase F ships the catalog with the original 4; #1's commit appends `session.*`). `message.delivered/read/played` (from `events.Receipt`) are explicitly **out of scope here** (deferred to #1's receipt→job-linkage work, §2).

---

## 2. Exact whatsmeow signatures used

This feature **receives** events (no whatsmeow function calls, no error sentinels — event structs don't return errors). HTTP mapping for the CRUD side is gateway-owned validation, not whatsmeow-error-derived. Real struct definitions, `types/events/events.go` in `whatsmeow@v0.0.0-20260516102357-8d3700152a69`:

```go
// events.go:67
type Connected struct{}

// events.go:105
type LoggedOut struct {
    OnConnect bool                  // true = triggered by connect-failure; false = stream:error
    Reason    ConnectFailureReason  // valid only when OnConnect == true
}

// events.go:117
type StreamReplaced struct{}

// events.go:151
type TemporaryBan struct {
    Code   TempBanReason   // events.go:123 (int); consts 101,102,103,104,106 (events.go:126-130)
    Expire time.Duration
}
func (tb *TemporaryBan) String() string    // events.go:156 — human text
func (tbr TempBanReason) String() string   // events.go:142 — "<code>: <message>"

// events.go:220
type ConnectFailure struct {
    Reason  ConnectFailureReason  // events.go:164 (int); consts 400-503 (events.go:166-186)
    Message string
    Raw     *waBinary.Node
}
func (cfr ConnectFailureReason) String() string     // events.go:209
func (cfr ConnectFailureReason) IsLoggedOut() bool   // events.go:200

// events.go:242
type Disconnected struct{}

// events.go:411 — deferred to #1 receipt work, NOT this phase
type Receipt struct { /* Delivered/Read/Played */ }
```

**Mapping to payload fields:**
- `TemporaryBan` → `code = int(v.Code)`, `reason_text = v.Code.String()`, `expires_in = int(v.Expire.Seconds())`.
- `LoggedOut` → `on_connect = v.OnConnect`, `reason = int(v.Reason)`, `reason_text = v.Reason.String()`. Also **evict** the client (Phase D `session.logged_out` handling; capture jid *before* eviction/FK-cascade).
- `ConnectFailure` → `reason = int(v.Reason)`, `reason_text = v.Reason.String()`, `message = v.Message`.

**HTTP mapping (CRUD endpoints, gateway-owned):**
| condition | status | via |
|---|---|---|
| `events[]` contains value not in catalog | `400` | `errDomain.NewError(errDomain.ErrBadRequest, …)` → `httperror.FromError` |
| invalid/SSRF URL | `400` | existing `utils.ValidateURL` in `manager.SetWebhookURL` |
| client not logged in | `409` (wrapping `401`) | existing `manager.SetWebhookURL`/`DeleteWebhookURL` gate |
| no session / client not found | `404`/`401` | existing client nil-guards |

---

## 3. Ordered edit list

**Phase F — multi-URL storage + CRUD (ship first):**

1. **`domain/queue/status_webhook.go`** — add the catalog as a single source of truth (start with the 4; `session.*` appended in the #1 commit):
   ```go
   var WebhookEventCatalog = map[string]struct{}{
       string(EventMessageIncoming): {}, string(EventMessageQueued): {},
       string(EventMessageSent): {}, string(EventMessageFailed): {},
   }
   func IsKnownEvent(e string) bool { _, ok := WebhookEventCatalog[e]; return ok }
   // matchesEvent: ""==all; else comma-split(events) contains event (TrimSpace each).
   func EventMatches(subEvents, event string) bool
   ```
2. **`domain/whatsapp/whatsapp.go`** — extend DTO + add subscription type:
   ```go
   type Webhook struct {
       Url        string   `json:"url" binding:"required,url"`
       HmacSecret string   `json:"hmac_secret" binding:"omitempty"`
       Events     []string `json:"events" binding:"omitempty"`   // empty = all
   }
   type WebhookSubscription struct { Url string; HmacSecret string; Events string /* comma */ }
   ```
3. **`internal/whatsapp/migration.go`** — new `runDeviceWebhookSubscriptionsMigrations(db)` (§4), called from `runMigrations` **after** `runDeviceWebhooksMigrations` (needs the source table present). Keep `device_webhooks` (data-preserving; drop in a later release).
4. **`internal/whatsapp/repository.go`** — extend interface + impl (all `$1/$2`, `ON CONFLICT (jid,url)`):
   ```go
   SetWebhookSubscription(ctx, jid, url, hmacSecret, events string) error   // upsert
   GetWebhookSubscriptions(ctx, jid string) ([]waDomain.WebhookSubscription, error)
   GetWebhookSubscriptionsByPhone(ctx, phone string) ([]waDomain.WebhookSubscription, string /*jid*/, error)
   DeleteWebhookSubscription(ctx, jid, url string) error   // one
   DeleteAllWebhookSubscriptions(ctx, jid string) error    // legacy no-body DELETE
   ```
   Keep old `Get/Set/DeleteWebhook` for one release (rollback), but stop reading them from dispatch.
5. **`internal/whatsapp/client.go`** (impl) + **`internal/whatsapp/manager.go`** (validation wrapper) — new methods `ListWebhookSubscriptions`, `SetWebhookSubscription`, `DeleteWebhookSubscription(url)`, `DeleteAllWebhookSubscriptions`. **Validate `events` ⊆ catalog in the manager** (before encrypt), reusing the existing login-status gate + `utils.ValidateURL` + `m.Cipher.Encrypt(hmac)`. Unknown event → `ErrBadRequest`.
6. **`internal/usecase/whatsapp/webhook_usecase.go`** — replace the 3 thin methods with `ListWebhookSubscriptions`, `SetWebhookSubscription(webhook *waDomain.Webhook)`, `DeleteWebhookSubscription(url string)`, `DeleteAllWebhookSubscriptions`. No cache/budget/action-cap here (config CRUD, not a WhatsApp-server read/send).
7. **`internal/handler/whatsapp/whatsapp_webhook_handler.go`** — rewrite `Get`/`Set`/`Delete` per §1a (GET returns `{url, subscriptions[]}`; DELETE branches on body `url`). Reuse existing `MustGetPhoneNumber` + `httperror.FromError`.
8. **`internal/router/webhook_routes.go`** — routes unchanged (same 3 verbs, same group); no Wire changes.

**#1 — generic dispatcher + session events (ship in same release window):**

9. **`internal/whatsapp/handler.go`** — add `func (h *handler) dispatch(ctx, traceID, phoneNumber, jid, event string, payload map[string]interface{})`: `GetWebhookSubscriptions(jid)` → for each, `if domainQueue.EventMatches(sub.Events, event)` then queue-publish `WebhookDeliveryMessage{WebhookURL:sub.Url, HmacSecret:sub.HmacSecret, Payload:payload}` (queue mode) **or** `asyncDeliverWebhook`→`sender.Send` per sub (direct mode). Refactor `deliverWebhook` to build the payload once then call `dispatch`.
10. **`internal/whatsapp/handler.go` `HandleEvent`** — add cases `*events.LoggedOut / *events.TemporaryBan / *events.ConnectFailure / *events.Connected / *events.Disconnected / *events.StreamReplaced`, each building a flat payload map and calling `dispatch(...)`. `session.logged_out` also evicts the client (Phase D). This is the switch Phase D (PRD-4 #2) owns — coordinate so it isn't written twice.
11. **`internal/queue/handlers/incoming_event_handler.go:60`**, **`outgoing_message_handler.go:200`**, **`message_usecase.go:1062/1113/1158`**, **`dlq_handler.go:104`** — swap each `GetWebhook`+single-`Send` for the subscription fan-out (call the shared repo `GetWebhookSubscriptions` + `EventMatches` filter). This folds the global `WEBHOOK_STATUS_EVENTS` filter into the per-subscription `events`.
12. **`domain/queue/status_webhook.go`** — append `session.*` consts + catalog entries (see §1c) in this commit only.

Cache/budget/action-cap: **none of this feature touches them** — webhook CRUD is DB config; dispatch is outbound push. No `queryWithBudget`/`spendReadBudget`/`spendActionBudget`.

---

## 4. Migration DDL (dual-dialect, data-preserving)

Appended to `runMigrations` after `runDeviceWebhooksMigrations`. `CREATE TABLE IF NOT EXISTS` + idempotent backfill (safe on every startup):

```sql
CREATE TABLE IF NOT EXISTS device_webhook_subscriptions (
    jid         TEXT NOT NULL REFERENCES whatsmeow_device(jid) ON DELETE CASCADE,
    url         TEXT NOT NULL,
    hmac_secret TEXT NOT NULL DEFAULT '',
    events      TEXT NOT NULL DEFAULT '',   -- comma-separated catalog subset; '' = all events
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (jid, url)
);

-- Data-preserving backfill: each existing single-row webhook -> one subscription
-- with '' (all events), matching today's "one URL receives everything". Idempotent
-- re-run via the portable ON CONFLICT DO NOTHING (SQLite 3.24+/Postgres 9.5+).
INSERT INTO device_webhook_subscriptions (jid, url, hmac_secret, events)
SELECT jid, webhook_url, hmac_secret, ''
FROM device_webhooks
ON CONFLICT (jid, url) DO NOTHING;
```

- No `?`, no `pq.Array`, no non-portable UUID/serial — DDL + backfill are byte-identical on SQLite and Postgres.
- `device_webhooks` is **not dropped** (rollback-safe; freeze writes, drop in a follow-up release).
- FK `ON DELETE CASCADE` mirrors `device_webhooks` — on re-pair/logout, whatsmeow's device-row delete cascades subscriptions (see §7 edge case).

Repo upsert (referenced by `SetWebhookSubscription`):
```sql
INSERT INTO device_webhook_subscriptions (jid, url, hmac_secret, events)
VALUES ($1, $2, $3, $4)
ON CONFLICT (jid, url)
DO UPDATE SET hmac_secret = EXCLUDED.hmac_secret, events = EXCLUDED.events, updated_at = CURRENT_TIMESTAMP;
```

---

## 5. New config keys

**None required.** The per-subscription `events` column replaces the global filter.

- **Deprecate** `WEBHOOK_STATUS_EVENTS` (`config/config.go:101`) — superseded by per-subscription `events`. Keep reading it (back-compat) but mark deprecated in `.env.example`/wiki; a subscription with `events=''` inherits "all", so the old global default no longer silently drops `message.queued`.
- **Keep** `WEBHOOK_STATUS_EVENTS_ENABLED` as a global master kill-switch for the status family (queued/sent/failed). Precedence at each status dispatch site: `WEBHOOK_STATUS_EVENTS_ENABLED == true` **AND** `EventMatches(sub.Events, event)`. `message.incoming` and `session.*` are not gated by this flag.

→ skipped: a `WEBHOOK_DEFAULT_EVENTS` knob — empty = all is self-evident. Add when someone wants a different default-per-subscription.

---

## 6. openapi.yaml delta (bump `version:` 0.18.0 → 0.19.0)

**`components/schemas/WebhookRegisterRequest`** (extend, line ~102) — add:
```yaml
        events:
          type: array
          items:
            type: string
            enum: [message.incoming, message.queued, message.sent, message.failed,
                   session.logged_out, session.banned, session.connect_failure,
                   session.connected, session.disconnected, session.replaced]
          description: >
            Event types this subscription receives. Omit or empty = all events.
            Unknown values are rejected 400.
          example: ["message.incoming", "session.logged_out"]
```

**`GET /webhook` 200** (line ~1332) — add `subscriptions[]` alongside existing `url`:
```yaml
                  subscriptions:
                    type: array
                    items:
                      type: object
                      properties:
                        url: { type: string }
                        events: { type: array, items: { type: string } }
                        has_hmac: { type: boolean }
```

**`POST /webhook`** — add `400` response ref (`{"error":"unknown event type"}`) and note the `409` not-logged-in.
**`DELETE /webhook`** — add optional request body `{ url: string }` (delete one; no body = delete all).

**New `components/schemas/WebhookEnvelope`** (documentation of the outbound wire shape) — flat `event`, `phone_number`, `jid`, `timestamp` + per-event fields, with a `WebhookSessionEvent` example block for `session.*`. Mark `message.incoming` fields as top-level/back-compat.

---

## 7. Edge cases, partial-failure, gating

- **Empty `events` = all** — a legacy `POST {url}` (no events) and the backfilled rows both mean "everything", preserving today's behavior exactly.
- **Unknown event in `events[]` → 400** (`{"error":"unknown event type"}`), validated in the manager against the catalog before any DB write (User Story #32; guards #1's future catalog expansion — Phase F only accepts the 4 until the `session.*` commit lands).
- **Partial fan-out failure** — each subscription is delivered **independently**; one URL failing (non-2xx / SSRF-blocked / timeout) is logged and does **not** stop the others. Queue mode: N independent `WebhookDeliveryMessage`, each with its own retry/DLQ. Direct mode: N independent `asyncDeliverWebhook` goroutines under the existing `webhookSem` (50) — semaphore-full drops that one event (existing behavior, now per-subscription).
- **FK cascade on logout/re-pair** — `session.logged_out` fires as whatsmeow deletes the device row, which **cascade-deletes** subscriptions. Dispatch `session.logged_out` by reading subscriptions **eagerly at event time**, capturing jid before eviction; if the row already cascaded, there are simply no subs and it's a silent no-op (acceptable). Document: webhook subscriptions must be re-registered after re-pairing (same as today's single URL).
- **Duplicate `(jid,url)`** — `ON CONFLICT` upserts (updates hmac/events), never errors; re-`POST`ing the same URL edits its filter.
- **HMAC never returned** — GET exposes only `has_hmac`; secrets stay encrypted at rest and out of read paths.
- **Gating (default-off flags):** nothing here is ban-risky, so **no default-off flag is introduced**. The only gate is `WEBHOOK_STATUS_EVENTS_ENABLED` (existing, default `true`) over the status family. `session.*` emission is inert until subscribers opt in via `events`. Ban-risky bulk ops belong to other PRDs; this feature is pure notification.

---

## 8. Unit-test seams (no live client)

Highest-seam-first, all runnable without WhatsApp:

1. **`EventMatches(subEvents, event)`** (pure) — `''`→all true; `"message.sent, message.failed"` (spaces) matches `message.sent`, not `message.queued`; unknown-in-filter never panics. One table test. **This is the load-bearing filter — the ponytail self-check.**
2. **`IsKnownEvent` / event validation** — the 4 accepted; `session.*` rejected pre-#1-commit, accepted post; garbage → `ErrBadRequest`. Assert the usecase/manager maps unknown → `httperror` 400.
3. **Repository round-trip (SQLite `:memory:`)** — `SetWebhookSubscription` twice same `(jid,url)` upserts (RowsAffected/read-back), different URLs → 2 rows; `GetWebhookSubscriptions` returns both; `DeleteWebhookSubscription(url)` removes one, `DeleteAllWebhookSubscriptions` clears; `has_hmac` derivation. Follows `internal/middleware/idempotency_test.go` in-memory pattern.
4. **Migration data-preservation** — seed a `device_webhooks` row, run `runDeviceWebhookSubscriptionsMigrations`, assert exactly one subscription with `events=''` and the copied url/hmac; run it **twice** (idempotency); execute on **both** SQLite (in-memory) and the Postgres testcontainer (per PRD Testing Decisions).
5. **GET/DELETE handler shape** — GET returns legacy `url` (= first sub) plus `subscriptions[]` with no hmac leaked; DELETE with `{url}` vs no-body routes to one-vs-all (stub usecase).
6. **Dispatcher fan-out** — with a repo stub returning 3 subs (filters `all` / `message.incoming` / `message.sent`) and a recording sender, dispatching `message.incoming` hits subs 1 & 2, not 3; a session event hits only the `all` sub. (Sender is a concrete struct today — introduce a 1-method `webhookSender` interface seam here, or test the *selection* set — `[]sub` after `EventMatches` — as a pure function and leave the HTTP `Send` to the existing SSRF tests. Prefer the pure-selection test; smaller diff.)

`HandleEvent` session cases are **not** CI-testable (no live WhatsApp, per PRD) — verify the struct→payload mapping once against a manually constructed `*events.TemporaryBan{Code:104, Expire:…}` etc. (constructing the event value and asserting the payload map, no socket), then lock the mapping to the exported sentinels/String() methods quoted in §2.

---

**Sequencing note:** Ship §3 steps 1-8 (Phase F storage/CRUD, catalog = 4 events) as one focused unit, then steps 9-12 (#1 dispatcher + `session.*`) once Phase D's `HandleEvent` switch (PRD-4 feature 2) is in — the two must not both write that switch. Per-endpoint DoD (code + highest-seam tests + openapi bump + llms.txt §"Webhook Events" catalog + `wiki/` new Webhooks catalog section + `.env.example` deprecation note + `CLAUDE.md` + `docs/ui` + `CHANGELOG` Unreleased) applies; small focused commits.

Relevant files: `/Users/glennpray/projects/whatsapp-gateway/internal/whatsapp/{repository.go,migration.go,handler.go,client.go,manager.go,webhook_sender.go}`, `/Users/glennpray/projects/whatsapp-gateway/internal/queue/handlers/{incoming_event_handler.go,outgoing_message_handler.go,dlq_handler.go}`, `/Users/glennpray/projects/whatsapp-gateway/internal/usecase/whatsapp/message_usecase.go`, `/Users/glennpray/projects/whatsapp-gateway/internal/handler/whatsapp/whatsapp_webhook_handler.go`, `/Users/glennpray/projects/whatsapp-gateway/internal/router/webhook_routes.go`, `/Users/glennpray/projects/whatsapp-gateway/domain/{whatsapp/whatsapp.go,queue/status_webhook.go}`, `/Users/glennpray/projects/whatsapp-gateway/config/config.go`, `/Users/glennpray/projects/whatsapp-gateway/docs/openapi.yaml`. whatsmeow events: `/Users/glennpray/go/pkg/mod/go.mau.fi/whatsmeow@v0.0.0-20260516102357-8d3700152a69/types/events/events.go`.