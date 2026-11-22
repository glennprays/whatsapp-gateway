# WhatsApp Gateway: auth-console clarity, version surfacing, proxy/463 + ban polish, MCP/SDK consistency

## Context

After the last docs release the user hit several rough edges and asked for a broad
hardening pass. A 3-agent read-only audit (2026-07-30) confirmed which of the reported
problems are real and which were mis-remembered. This plan fixes the confirmed issues
across three repos, in four independent workstreams (each its own commit; the three
gateway workstreams share one branch off `dev`).

**Repos involved**
- `/Users/glennpray/projects/whatsapp-gateway` (gateway — Go)
- `/Users/glennpray/projects/mcp-whatsapp-gateway` (MCP server — Go, separate module)
- `/Users/glennpray/projects/whatsapp-gateway-sdk-go` (Go SDK — touched only if the
  `/login/status` response shape changes)

**First implementation step (do before any code):** mirror this plan file into
`docs/plan/2026-07-30-hardening-pass.md` (uncommitted — it falls under the standing
"never commit `docs/plan/**`" rule) so it stays traceable during implementation.

---

## Audit findings (ground truth)

Confirmed **real**:
- **Double "HTTP Bearer" in the API console.** `docs/openapi.yaml` `components.securitySchemes`
  (L1432-1444) defines `accountBearerAuth` (JWT, `bearerFormat: JWT`, **no description**) and
  `adminBearer` (has description). Both are `type: http, scheme: bearer`, so RapiDoc labels
  both "HTTP Bearer". Pasting the JWT into the wrong box → user endpoints go out with no
  token → 401. Per-path `security:` blocks are otherwise **correct** (account scheme on ~47
  endpoints, admin scheme only on the 3 `/admin` paths). Display problem, not routing.
- **`/login/status` naming conflation.** Handler `GetLoginStatus`
  (`internal/handler/whatsapp/whatsapp_auth_handler.go:131-149`) returns
  `{"authenticated": <bool>}` where the bool is purely whatsmeow `IsLoggedIn()` device-pairing
  state (`internal/whatsapp/client.go:313-319`), NOT JWT state. Valid JWT + unpaired WhatsApp
  = `{"authenticated": false}` = "bearer detected but not login". `openapi.yaml:1606-1624`.
- **No version source anywhere.** No ldflags, no version package/const. Only string is
  `openapi.yaml info.version: 0.25.0`, already drifted from git tags (`v0.13.0`). Docker build
  is a plain `go build` (`Dockerfile:20-21`). Docs header has no version element
  (`docs/ui/index.html:89-92`); template only gets `BasePath`, `ShowAPI`
  (`internal/router/docs_routes.go:38-47`).
- **No trusted-proxy config.** `fiber.New` (`internal/router/router.go:42-46`) sets no
  `ProxyHeader`/`EnableTrustedProxyCheck`/`TrustedProxies`, so `c.IP()` = LB socket peer. The
  register rate limiter keys on `c.IP()` (`internal/middleware/rate_limit.go:37`).
  `utils.GetIPFromFiberCtx` (`internal/utils/utils.go:5-13`) reads raw `X-Forwarded-For`
  verbatim but is **log/diagnostic only** (http_logger, trace_id middleware).
- **MCP addressing drift.** `mcp-whatsapp-gateway/internal/tools/messaging.go`: send tools
  expose `chat` (+ `to`), but `edit_message`/`delete_message`/`react_to_message` expose only
  `to` (L303-378) and `check_contact` only `msisdn` (L531-534). Functional (mapped via
  `WithChat`/`FormatMSISDN`) but inconsistent with the gateway's canonical `chat`.
- **Dead `Pacer.Drain`.** `pkg/ratelimiter/pacer.go:165-169` is documented "called on ban
  entry" but has zero callers. The ban gate alone enforces the block.

Confirmed **NOT a problem** (report to user, no work needed):
- **No `msidn` / colon-in-key / malformed-tag typos** exist in gateway DTOs, the SDK, the MCP
  server, `openapi.yaml`, or `llms.txt`. Every field is correct `msisdn`; every tag well-formed.
- **`llms.txt` is current** (547 lines, covers group/community, admin plane, webhook subs,
  pacer, idempotency). Only nit: one Python sample (`llms.txt:185`) uses `msisdn` not `chat`.
- **Ban handling already built and already uses the real expiry.** `events.TemporaryBan`
  (`internal/whatsapp/handler.go:145-154`) → `session_status.state="banned"` + `ban_expires_at`
  (`migration.go:34-51`, `repository.go:168-186`) → `session.banned` webhook (`bannedPayload`
  L207-213 emits `expires_in`) → pacer ban gate returns 429 until expiry
  (`session_inventory.go:193-211` `BanState`, `pacer.go:82-96`). 463 is a load-balancer error,
  not emitted by the gateway.

---

## Workstream A — Auth & API console (gateway)

**Commit:** `fix(docs): disambiguate auth schemes and clarify login status`

1. **Relabel the two bearer schemes** in `docs/openapi.yaml` securitySchemes (L1432-1444):
   add a clear `description` to `accountBearerAuth` — e.g. "Per-account JWT returned by
   `POST /register`. Use this token for all messaging, session, group, contact, and webhook
   calls." Keep `adminBearer` as-is. Both fields remain, now obviously distinct in RapiDoc.
2. **Clarify `/login/status` without breaking clients.** In `GetLoginStatus`
   (`whatsapp_auth_handler.go:146-148`) keep `authenticated` (back-compat) and **add**
   `logged_in` (same value, clearly named) plus ban visibility — reuse
   `manager.BanState(phone)` (already available, see workstream C) to add
   `banned` + `ban_expires_at` when applicable. Update the usecase
   (`internal/usecase/whatsapp/auth_usecase.go:92-110`) to return the richer struct.
3. **Update `openapi.yaml`** `/login/status` response (L1606-1624): fix the `authenticated`
   description to say "WhatsApp session (device-pairing) state, not API-token validity",
   document the new `logged_in`/`banned`/`ban_expires_at` fields.
4. If the response struct changes, mirror the new fields in the SDK's login-status model
   (`whatsapp-gateway-sdk-go/types.go`) — additive only.

## Workstream B — whatsmeow bump + running-server version (gateway)

**Commit 1:** `build(deps): bump whatsmeow to latest` · **Commit 2:** `feat: surface running server version in docs header and health`

1. **whatsmeow bump:** `go get go.mau.fi/whatsmeow@latest && go mod tidy`; `go build ./...`;
   `go test ./...`; skim whatsmeow changelog for breaking changes to the event switch
   (`internal/whatsapp/handler.go`) and client calls (`internal/whatsapp/client.go`). Confirm
   `events.TemporaryBan` / `ConnectFailure` / `LoggedOut` signatures unchanged.
2. **Version var:** declare `var version = "dev"` in `cmd/api/main.go`; thread it through
   `infrastructure.Run()` → config (add `AppVersion` field) so it reaches the router/handlers.
3. **Build injection:** `Dockerfile` — add `ARG VERSION` and build with
   `-ldflags "-X main.version=$VERSION"`. `Makefile` `run`/`build` targets — inject
   `-X main.version=$(git describe --tags --always --dirty)`. Docker workflow
   (`.github/workflows/*docker*`) — pass `--build-arg VERSION=$(git describe --tags --always)`.
   Note: Docker builds on push to `main` (before `release.yml` tags), so the value is
   `v<tag>-<n>-g<sha>` — the precise running commit, acceptable and intentional.
4. **Surface it:**
   - Docs header: pass `Version` into the template (`internal/router/docs_routes.go:38-47`),
     render a small version badge next to the logo (`docs/ui/index.html:89-92` + a style rule).
   - Static site: `cmd/docs-gen` embeds the build tag at generate time (same `git describe`).
   - `GET /health` (`router.go:58+`): add `"version"` to the response body.

## Workstream C — Proxy/463 hardening + ban polish (gateway)

**Commit 1:** `fix: trust configured proxies so per-IP limits key on the real client` · **Commit 2:** `chore: drop dead pacer Drain + document 463 handling`

1. **Trusted proxy config:** add `TRUSTED_PROXIES` (comma list) + `PROXY_HEADER`
   (default `X-Forwarded-For`) to `config/config.go`. In `fiber.New`
   (`router.go:42-46`) set `ProxyHeader: cfg.ProxyHeader`,
   `EnableTrustedProxyCheck: true`, `TrustedProxies: cfg.TrustedProxies`. Then `c.IP()`
   (register limiter `rate_limit.go:37`) resolves the real client behind trusted hops and
   ignores spoofed XFF from untrusted sources. Document env vars in `.env.example`,
   `CLAUDE.md`, `wiki/Environment-Variables.md`, `llms.txt`.
2. **463 runbook:** 463 is emitted by the LB when inbound XFF exceeds its IP cap — it never
   reaches the gateway. Add a short section to `wiki/Security-Considerations.md` (or a new
   `wiki/Deployment-Proxy.md`): reset/normalize XFF at the outermost trusted proxy
   (Cloudflare/nginx) rather than appending, keep the proxy chain short, and set the gateway's
   `TRUSTED_PROXIES` to the immediate hop(s). No gateway code emits 463.
3. **Ban polish:** delete the dead `Pacer.Drain` (`pkg/ratelimiter/pacer.go:165-169`) and its
   stale "called on ban entry" doc comment (the ban gate already blocks; draining is
   redundant). Ban visibility on `/login/status` is delivered in workstream A step 2.

## Workstream D — MCP + SDK + llms.txt consistency

**Commit (MCP repo):** `fix(tools): accept canonical chat on edit/delete/react/check_contact` · **Commit (gateway):** `docs: use canonical chat in llms.txt sample`

1. **MCP** (`mcp-whatsapp-gateway/internal/tools/messaging.go`): add a `chat` input (canonical)
   to `edit_message` (L303-307), `delete_message` (L340-343), `react_to_message` (L372-378),
   and `check_contact` (L531-534), keeping `to`/`msisdn` as accepted back-compat aliases so the
   agent-facing schema matches the send tools and the gateway's documented `chat` convention.
   Update `tools_test.go` schema/manifest assertions.
2. **llms.txt** (`whatsapp-gateway/llms.txt:185`): change the Python "Pattern 2" sample from
   `'msisdn': recipient` to `'chat': recipient`.
3. **Report to user:** no typos were found anywhere; the "reaudit payloads" ask is satisfied
   by the audit (nothing to change beyond the sample line).

---

## Critical files

- `docs/openapi.yaml` — securitySchemes L1432-1444; `/login/status` L1606-1624.
- `internal/handler/whatsapp/whatsapp_auth_handler.go:131-149`;
  `internal/usecase/whatsapp/auth_usecase.go:92-110`; `internal/whatsapp/client.go:313-319`.
- `cmd/api/main.go` (version var); `internal/infrastructure/` (Run wiring);
  `config/config.go` (AppVersion, TrustedProxies, ProxyHeader);
  `internal/router/docs_routes.go:38-47`; `docs/ui/index.html:89-92`; `cmd/docs-gen/`.
- `Dockerfile:17-21`; `Makefile`; the Docker build workflow under `.github/workflows/`.
- `internal/router/router.go:42-46` (fiber.Config), `:58+` (health);
  `internal/middleware/rate_limit.go:37`.
- `pkg/ratelimiter/pacer.go:165-169`; `internal/whatsapp/session_inventory.go:193-211`.
- `go.mod:23` (whatsmeow pin).
- MCP: `mcp-whatsapp-gateway/internal/tools/messaging.go`, `tools_test.go`.
- `llms.txt:185`.

## Verification

- **Build/unit:** `go build ./...` + `go test ./...` in the gateway and MCP repos after each
  workstream. `wire gen ./internal/infrastructure` if any provider signature changes
  (AppVersion). Rebuild static docs: `./scripts/build-docs-pages.sh /tmp/waga_site`.
- **Console (manual):** `make run`, open the API tab — confirm the account Bearer field is now
  clearly labeled vs the admin one; set the JWT in the account box and confirm a user call
  succeeds. Confirm the version badge shows a real `git describe` value; `curl /health` shows
  `version`.
- **login/status:** with a valid JWT on an **unpaired** account, confirm response now carries
  `logged_in:false` (+ `authenticated:false` back-compat) and no `banned`; on a paired account
  `logged_in:true`.
- **Proxy:** send a request with a spoofed `X-Forwarded-For` from an untrusted source and
  confirm the register limiter keys on the real peer (not the spoofed IP) once
  `TRUSTED_PROXIES` is set.
- **MCP:** run the MCP tool manifest test; invoke `edit`/`react`/`check_contact` with `chat`
  and confirm they reach the gateway.
- **Live end-to-end (user-provided):** user supplies a JWT for an already-logged-in account
  and a target recipient. Send a text via `POST /api/message/text` with `chat=<target>` and
  confirm 200 + delivery; check `/login/status` reports `logged_in:true`; exercise a
  react/edit path. Run against the gateway build with all four workstreams applied.

## Deferred / not doing
- Not renaming `authenticated` (breaking) — adding `logged_in` alongside instead.
- Not reconciling `openapi.yaml info.version` into the git-tag scheme this pass (separate
  concern); the running-server version is the source of truth going forward.
- Empty/stub sibling repos `whatsapp-gateway-mcp/`, `wahtsapp-gateway-mcp/` left untouched.

---

## Implementation status (2026-07-30) — CODED, NOT PUSHED

All four workstreams implemented + verified (build + `go test`), committed locally
on branches. Nothing pushed/merged — awaiting live testing (user-provided JWT +
logged-in account + target) then explicit push/merge approval.

- Gateway branch `feat/auth-version-proxy-hardening` (off `dev`), 6 commits:
  - `9d6baef` A: disambiguate auth schemes + clarify /login/status
  - `c172f5a` B1: bump whatsmeow to 2026-07-22 (253 tests green)
  - `ecc8677` B2: running-server version in docs header + /health
  - `c23744d` C1: trusted-proxy config (TRUSTED_PROXIES/PROXY_HEADER)
  - `0f2f8b2` C2: drop dead Pacer.Drain + HTTP 463 runbook
  - `e39b3b5` D: llms.txt sample msisdn->chat
- SDK `whatsapp-gateway-sdk-go` branch `feat/login-status-fields` (`b683ec2`):
  additive logged_in/banned/ban_expires_at on LoginStatus.
- MCP `mcp-whatsapp-gateway` branch `fix/chat-first-addressing` (`44fa818`):
  chat-first edit/delete/react/check_contact.

Audit result reported to user: NO msidn/malformed-tag typos anywhere; llms.txt
already current; ban handling already built (only Drain dead-code removed).

### Remaining
1. Live end-to-end test with user JWT (send text with `chat`, verify /login/status
   logged_in, version on /health + docs header, react/edit).
2. On approval: push all three branches, open PRs, merge (gateway PR gets a
   `release:` label per CI; SDK/MCP per their own release flow).
