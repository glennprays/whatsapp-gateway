# Development Sequence

> _Review draft — uncommitted._ The priority-ordered plan across all PRDs. Derived from the dependency analysis; honors decision #7 (automation-platform first, roadmap #1/#2 after) with the ban-risk and rework mitigations baked in.

## Principles driving the order
- **Foundation before consumers** — `chat` addressing (PRD 0) unblocks every other area, so it goes first. It's additive and backward-compatible, so it ships without a flag.
- **Low ban-risk early** — reads/queries and idempotency have negligible ban exposure; they front-load value while the risky work waits.
- **Ban-risky work gated until pacing** — bulk participant-add and mass-join sit behind enable-flags (default off) until roadmap #2 pacing lands; conversation actions use an interim per-account cap meanwhile.
- **No double migrations** — the multi-URL webhook rework overlaps roadmap #1's envelope, so it's deferred into that window rather than migrated twice.
- **Cross-repo trails core** — each SDK wave (PRD 5) trails the gateway wave it mirrors; MCP (PRD 6) trails the SDK.

## Phased sequence

### Phase A — Foundation (unblocks everything)
1. **PRD 0 — Addressing** (`resolveChat`, `chat` field on all request structs, response echo, openapi `required` relax). Additive, backward-compatible, no flag. → **ADR 0001**.
   - SDK Wave 1 (chat + reaction `SenderMsisdn` + v0.11 media backfill) can start here.

### Phase B — Early low-risk wins (parallelizable with A's tail)
2. **PRD 4 subset — Idempotency + health split + graceful shutdown.** No dependency on PRD 0. Land idempotency **early**, before automation drives send volume and clients bake in retry logic. Needs the dual-dialect migration. → **ADR 0003**.
3. **PRD 1 — Read/Query surface + shared cache/budget infra.** Depends on PRD 0. Builds the TTL cache + per-account query budget that PRD 3 reads and PRD 4 reuse. "Reads early" is the decision-#7 mitigation. → **ADR 0002** (error sentinels) applies from here on.

### Phase C — Interaction & group reads
4. **PRD 2 — Two-way primitives** (mark-read, typing, replies, mentions). Depends on PRD 0 (participant JID resolution). Ships with the **interim action cap** (reuse existing limiter, key `action:`+phone) until #2.
5. **PRD 3 read subset — group info / joined / community reads.** Reuses PRD 1's cache+budget; ships with or just after PRD 1.

### Phase D — Operator visibility
6. **PRD 4 rest — session-status event tracking → admin inventory + Prometheus metrics.** Adds the `HandleEvent` cases (LoggedOut/TemporaryBan/ConnectFailure/StreamReplaced) + status persistence + client eviction. **This IS the code roadmap #1 §3.4 needs** — one owner for the switch; #1 later only adds webhook emission on top. Metrics never label by phone.

### Phase E — Group/community mutations (gated)
7. **PRD 3 mutations** (create/leave/participants/settings/invite/community link-unlink). Depends on PRD 0 group-JID resolver + PRD 1 reads. **`GROUP_ADD_PARTICIPANTS_ENABLED` and mass-join stay off** until #2 pacing; other mutations enabled.

### Phase F — Deferred into the roadmap #1 window
8. **PRD 4 feature 8 — multi-URL / per-event webhook subscriptions.** Heaviest item (schema migration off single-row `device_webhooks`); overlaps #1's generic envelope. Defer to the #1 window to migrate once.

### Then, per decision #7
9. **Roadmap #1 — Webhook event system** (generic envelope + new event types: `session.*`, `message.delivered/read/played`, `call.offer`; builds on Phase D's event cases + Phase F's subscriptions).
10. **Roadmap #2 — Outbound pacing.** Replaces the Phase C interim action cap and un-gates the Phase E bulk paths.

### Cross-repo (trailing, per PRD 5 / PRD 6)
- SDK Waves: 1 (Phase A/B) → 3 (Phase B) → 4 (Phase C) → 5–7 (Phase E) → admin module (Phase D).
- MCP: messaging + read/query after SDK Waves 1+3; mark-read/typing after SDK Wave 4. Group mutations + admin never exposed.

## Ban-risk gate summary
| Work | Gate until pacing (#2) |
|---|---|
| Bulk participant-add / add-on-create | `GROUP_ADD_PARTICIPANTS_ENABLED=false` + per-request cap |
| Mass-join via invite link | same-style gate (add explicitly) |
| Global online presence keep-alive | default OFF, no gateway-managed heartbeat (caller-driven single-shot) |
| mark-read / typing / react | interim per-account action cap (best-effort) |
| Everything else | direct/cached, no gate |
