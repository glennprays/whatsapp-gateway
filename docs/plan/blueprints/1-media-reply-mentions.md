I have everything needed. No new routes, no migration, no config, and the queue job DTO already carries the reply/mention fields (added for text). Here is the blueprint.

---

# Blueprint — Media reply/mentions (extend `reply_to_*` + `mentions` to the 7 media/other sends)

**Scope:** image, audio, video, document, location, poll, sticker. Text already ships this (commit `8e25a7d`). This is a pure extension of existing endpoints — **no new routes, no migration, no config keys, no queue-DTO change** (see §4/§5). The `OutgoingMessageJob` struct **already** has `ReplyToID/ReplyToSender/ReplyToText/Mentions` (`domain/queue/message_queue.go:63-67`); only the media job-populate + worker-thread are missing.

**Ship as 7 commits, one per send** (per instruction). Each commit is the same 6-file vertical slice + docs. Order suggestion: image → audio → video → document → location → poll → sticker (poll last since it's the odd one).

---

## 1. Endpoints / features (behavior + DTO field deltas)

No paths change. Each existing send gains 4 optional fields, identical semantics to text (`domain/whatsapp/message.go:31-34`):

| Field | Type | Meaning |
|---|---|---|
| `reply_to_id` | string | Quoted message id → turns the send into a reply (`ContextInfo.StanzaID`). |
| `reply_to_sender` | string (number/JID) | Author of quoted msg (`ContextInfo.Participant`); **required for group replies** to attribute the quote. Resolved via `resolveChat`. |
| `reply_to_text` | string | Caller-supplied quoted preview (storeless, decision #5) → `ContextInfo.QuotedMessage.Conversation`. |
| `mentions` | []string (numbers/JIDs) | Each resolved to a user JID → `ContextInfo.MentionedJID`. |

**Encoding per send** (matches how the endpoint already parses its body):
- **Multipart (form tags):** image, audio, video, document, sticker → `form:"reply_to_id"`, `form:"reply_to_sender"`, `form:"reply_to_text"`, and **`form:"mentions"` on `[]string`** (repeated `mentions` form keys — verified below).
- **JSON (json tags):** location, poll → `json:"reply_to_id,omitempty"` … `json:"mentions,omitempty"`.

Responses are unchanged (`SendMessageResponse` / `SendMessageQueuedResponse`).

**Multipart `[]string` binding is confirmed working** (no explicit handler read needed): Fiber v2.52.12 `BodyParser` for `multipart/form-data` copies `multipartForm.Value` (a `map[string][]string`, all repeated values preserved) into its data map and binds via `parseToStruct` (`ctx.go:418-445`). Repeated `mentions=A&mentions=B` → `["A","B"]`. Caveat: `EnableSplittingOnParsers` defaults **off**, so mentions must be sent as **repeated keys**, not a comma-joined string — document this.

---

## 2. Exact whatsmeow calls + struct fields (quoted from module source)

Module: `go.mau.fi/whatsmeow@v0.0.0-20260516102357-8d3700152a69`. Real structs live in `proto/waE2E/WAWebProtobufsE2E.pb.go` (the `binary/proto` import alias, `waE2E`, re-exports them — `binary/proto/doc.go`).

**All 7 message structs expose a `ContextInfo *ContextInfo` field** — verbatim field lines (file: `proto/waE2E/WAWebProtobufsE2E.pb.go`):

```
ImageMessage    (type @8736)  L8751:  ContextInfo *ContextInfo `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
AudioMessage    (type @15304) L15316: ContextInfo *ContextInfo `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
VideoMessage    (type @7824)  L7841:  ContextInfo *ContextInfo `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
DocumentMessage (type @15468) L15486: ContextInfo *ContextInfo `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
LocationMessage (type @15860) L15873: ContextInfo *ContextInfo `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
StickerMessage  (type @13318) L13334: ContextInfo *ContextInfo `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
PollCreationMessage (type @12376) L12382: ContextInfo *ContextInfo `protobuf:"bytes,5,opt,name=contextInfo" json:"contextInfo,omitempty"`
```

The existing `buildContextInfo(msgCtx *waDomain.MessageContext) *waE2E.ContextInfo` (`internal/whatsapp/client.go:585-602`) is reused unchanged — it returns `nil` when `msgCtx.IsEmpty()`, so assigning its result to any `*ContextInfo` field is always safe (nil = omitted).

**Poll is special — built by a helper, not a literal.** Real signature (file:line):
```
msgsecret.go:326: func (cli *Client) BuildPollCreation(name string, optionNames []string, selectableOptionCount int) *waE2E.Message
```
Its body returns `&waE2E.Message{ PollCreationMessage: {...}, MessageContextInfo: {MessageSecret: ...} }`. Attach reply/mentions to **`msg.PollCreationMessage.ContextInfo`** — **never** touch `msg.MessageContextInfo` (that carries the 32-byte poll `MessageSecret`; overwriting it breaks vote decryption).

**Send/upload calls are unchanged** — the media methods already call `cli.Upload(ctx, bytes, whatsmeow.Media*)` and `cli.SendMessage(ctx, toJID, msg)`; only the `msg` construction gains a `ContextInfo`.

**Error sentinels / HTTP mapping:** no new error surface. Reply/mentions add no whatsmeow error sources beyond the existing send path. Existing mapping in `mapWhatsmeowErr` (`client.go:147-174`) still applies: `store.ErrDeviceDeleted`→409; recipient errors→400; else 500. A malformed `reply_to_sender`/`mentions` is caught **in the usecase** (`buildMessageContext`→`resolveChat`) and returns `ErrBadRequest` (400) before reaching whatsmeow.

---

## 3. Ordered edit list (per file)

Same 6 code files per send. Below shows the shape for **image**; the other 6 are mechanically identical (only the media-message struct / poll helper differs — see §2).

### (a) `domain/whatsapp/message.go` — DTO fields
Add to each request struct. Multipart group (image/audio/video/document/sticker):
```go
ReplyToID     string   `form:"reply_to_id" binding:"omitempty"`
ReplyToSender string   `form:"reply_to_sender" binding:"omitempty"`
ReplyToText   string   `form:"reply_to_text" binding:"omitempty"`
Mentions      []string `form:"mentions" binding:"omitempty"`
```
JSON group (location/poll):
```go
ReplyToID     string   `json:"reply_to_id,omitempty"`
ReplyToSender string   `json:"reply_to_sender,omitempty"`
ReplyToText   string   `json:"reply_to_text,omitempty"`
Mentions      []string `json:"mentions,omitempty"`
```

### (b) `internal/whatsapp/client.go` — interface + impl signature, attach ContextInfo
Interface (`client.go:51-57`) — append `msgCtx *waDomain.MessageContext` to all 7:
```go
SendImageMessage(ctx context.Context, traceID, phoneNumber, to string, imageBytes []byte, mimeType, caption string, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
SendAudioMessage(..., isPTT, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
SendVideoMessage(..., caption string, isGif, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
SendDocumentMessage(..., fileName, caption string, msgCtx *waDomain.MessageContext) (string, error)
SendLocationMessage(..., name, address string, msgCtx *waDomain.MessageContext) (string, error)
SendPollMessage(..., options []string, selectableCount int, msgCtx *waDomain.MessageContext) (string, error)
SendStickerMessage(..., mimeType string, msgCtx *waDomain.MessageContext) (string, error)
```
Impl attach points (set on the **inner** media struct, before any ViewOnce/FutureProof wrap so it survives the wrap):
- Image `client.go:727-736`: `imageMsg.ContextInfo = buildContextInfo(msgCtx)` (before the `if isViewOnce` wrap at L743).
- Audio `L799-809`: `audioMsg.ContextInfo = buildContextInfo(msgCtx)` (before L812 wrap).
- Video `L852-862`: `videoMsg.ContextInfo = buildContextInfo(msgCtx)` (before L868 wrap).
- Document `L908-918`: `docMsg.ContextInfo = buildContextInfo(msgCtx)` (before the `DocumentWithCaptionMessage` wrap at L927).
- Location `L1059-1068`: `locationMsg.ContextInfo = buildContextInfo(msgCtx)` (after Name/Address).
- Sticker `L1136-1146`: add `ContextInfo: buildContextInfo(msgCtx),` to the `StickerMessage{...}` literal.
- Poll `L1101`: after `msg := cli.BuildPollCreation(...)`, add `msg.PollCreationMessage.ContextInfo = buildContextInfo(msgCtx)`.

### (c) `internal/whatsapp/manager.go` — interface + thin wrapper signature
Interface (`manager.go:41-47`) and the 7 wrapper impls (`L252-432`): append `msgCtx *waDomain.MessageContext` to signature and pass it straight through to `m.Client.Send*Message(...)`. Logging bodies unchanged.

### (d) `internal/usecase/whatsapp/message_usecase.go` — build + thread (direct + job)
In each of the 7 send usecases, insert **right after `resolveChat` + length validation, before the queue branch** (so an invalid mention/sender 400s without a live client — see §8):
```go
msgCtx, err := uc.buildMessageContext(req.ReplyToID, req.ReplyToSender, req.ReplyToText, req.Mentions)
if err != nil {
    return nil, nil, err
}
```
Then:
- **Job path** (`OutgoingMessageJob{...}` literal): add `ReplyToID: msgCtx.ReplyToID, ReplyToSender: msgCtx.ReplyToSender, ReplyToText: msgCtx.ReplyToText, Mentions: msgCtx.Mentions,` (fields already exist on the struct).
- **Direct path** (`uc.whatsappManager.Send*Message(...)` call): append `msgCtx` as the last arg.

Locations: image `L300-407`, audio `L411-493`, video `L496-577`, document `L580-665`, location `L668-731`, poll `L734-804`, sticker `L807-881`.

`buildMessageContext` (`interaction_usecase.go:86`) is reused unchanged.

### (e) `internal/queue/handlers/outgoing_message_handler.go` — worker cases
For the 7 media/other cases (`L106-144`, `location` L134-135, `poll` L136-137, `sticker` L138-144), pass reply/mention context into the manager call, mirroring the existing `text` case (`L100-105`):
```go
&waDomain.MessageContext{
    ReplyToID:     job.ReplyToID,
    ReplyToSender: job.ReplyToSender,
    ReplyToText:   job.ReplyToText,
    Mentions:      job.Mentions,
}
```
Appended as the last arg to `SendImageMessage/SendAudioMessage/.../SendPollMessage/SendStickerMessage`. `react`/`delete`/`edit` cases unchanged.

### (f) Handlers — **no change**
`internal/handler/whatsapp/whatsapp_message_handler.go`: every media handler already calls `c.BodyParser(&req)` (multipart string + `[]string` bind) and every JSON handler already calls `c.BodyParser(&req)` — the new tagged fields populate automatically. Manual boolean parsing (`is_ptt`, `is_gif`, `is_view_once`) stays as-is. **No route change** (`message_routes.go` untouched).

**No Wire regen** — no new provider/constructor; signatures widen on existing interfaces only.

---

## 4. Migration DDL
**None.** Replies are storeless (decision #5); mentions are caller-supplied JIDs. No table, no persisted state. No `runXMigrations` addition.

---

## 5. New config keys
**None.** Reply/mentions are per-message metadata, not ban-risky bulk ops, so no enable-flag/gate and no new budget. `config/config.go` and `.env.example` are untouched by this feature.

---

## 6. openapi.yaml delta

Bump `version` (line 4, currently `0.18.0`) once per commit (patch bumps, e.g. `0.18.1` … `0.19.0`). For each of the 7 request schemas add the 4 properties (JSON style for the two below, form-field style for the five media ones — OpenAPI represents both the same way, but for the multipart ones `mentions` is a repeated field):

Schemas + line anchors: `SendMessageImageRequest` (L162), `SendMessageStickerRequest` (L265), `SendMessageAudioRequest` (L283), `SendMessageVideoRequest` (L312), `SendMessageDocumentRequest` (L342), `SendMessageLocationRequest` (L203), `SendMessagePollRequest` (L234). Append (copying the wording already used on `SendMessageTextRequest` L129-146):

```yaml
        reply_to_id:
          type: string
          description: "Quote this message id (turns the send into a reply)."
          example: "3EB0C767D26A1D8A2C1B"
        reply_to_sender:
          type: string
          description: "Author of the quoted message (number/JID). Recommended with reply_to_id, required for group replies."
          example: "6289999999999@s.whatsapp.net"
        reply_to_text:
          type: string
          description: "Optional quoted preview text (storeless replies)."
          example: "the original message"
        mentions:
          type: array
          items:
            type: string
          description: "Numbers/JIDs to @-mention. For multipart endpoints send repeated `mentions` form fields. Include the matching @<number> token in the caption/text for it to render visibly."
          example: ["6281111111111"]
```

For the multipart schemas (`type: multipart/form-data`) add a note on `mentions` that it is a repeated form field. Sticker/location/poll have no free-text caption — note that a mention still attaches (fires a notification) but has nothing to render inline.

---

## 7. Edge cases, partial-failure, gating

- **No gating / no default-off flags.** Reply+mentions are message metadata, not ban-risky bulk ops (contrast participant-add / mass-join). Sends already meter under `uc.limiter` (text/location/poll/sticker) or unmetered (image/audio/video/document today) — **do not** add `spendActionBudget` here; that cap is for conversation actions (react/mark-read/typing).
- **Invalid `reply_to_sender` or any `mention`** → `buildMessageContext`→`resolveChat` returns `ErrBadRequest` (400) before the queue/limiter/client is touched. Blank mentions are skipped (existing `buildMessageContext` behavior).
- **Group replies** need `reply_to_sender` for correct quote attribution; not enforced (best-effort, identical to text). Without it a group reply may not visibly quote.
- **ViewOnce + reply/mention:** `ContextInfo` is set on the inner `ImageMessage/AudioMessage/VideoMessage/DocumentMessage` **before** the `ViewOnceMessage`/`DocumentWithCaptionMessage` `FutureProofMessage` wrap, so it survives. Verify by code review (not unit-testable without a live client).
- **Poll:** attach only to `msg.PollCreationMessage.ContextInfo`; leaving `msg.MessageContextInfo.MessageSecret` intact is load-bearing (votes undecryptable otherwise).
- **Mention rendering:** gateway only populates `MentionedJID`; the caller must include the `@<number>` token in the caption/text for it to render. Sticker/location/poll (no free-text caption) attach the mention (notification fires) but show no inline `@` — document honestly.
- **Partial failure: none.** Each send is a single `SendMessage` call — it either sends with context or returns an error; no multi-step/partial state. Queue path is unchanged (job already round-trips the fields; dedup/retry semantics untouched).
- **Multipart `mentions` must be repeated keys** (not comma-joined) unless `EnableSplittingOnParsers` is enabled (it isn't).

---

## 8. Unit-test seams (usecase/resolver level, no live client)

- **Already covered (reused as-is):** `buildMessageContext` — `TestBuildMessageContext_ResolvesJIDs/_EmptyIsEmpty/_InvalidMention` (`interaction_usecase_test.go:58-95`). All 7 media usecases funnel reply/mentions through this same function, so JID resolution + blank-skip + invalid-mention are covered centrally.
- **Add one guard test per media send** (mirrors `TestMarkRead_GroupRequiresSender` pattern: `uc := &WhatsappMessageUsecase{}` with nil manager/queue/limiter). Because `buildMessageContext` runs **before** the queue/limiter branch, an invalid mention or `reply_to_sender` returns `ErrBadRequest` without dereferencing the nil client — e.g.:
  ```go
  func TestSendPoll_InvalidMention400(t *testing.T) {
      uc := &WhatsappMessageUsecase{}
      _, _, err := uc.SendPollMessage(context.Background(), "trace", "628111",
          waDomain.SendPollMessageRequest{
              Chat: "6282222222222", Question: "q", Options: []string{"a","b"},
              Mentions: []string{"@@@bad@@@"},
          })
      // expect errDomain.Error with ServiceError() == ErrBadRequest
  }
  ```
  This is the highest seam that runs without a live whatsmeow client. **Critical:** it only stays panic-free if the `buildMessageContext` call is placed before the queue/limiter branch (see §3d) — the test doubles as a guard on that ordering.
- **Optional client-level unit** (`internal/whatsapp`): a direct `buildContextInfo` assertion (maps `ReplyToID→StanzaID`, `ReplyToSender→Participant`, `Mentions→MentionedJID`, empty→nil). It's package-private and already exercised indirectly by text; a direct table test is cheap insurance for the shared helper.
- **Not unit-testable without a live client** (flag for code review instead): which struct field each media type sets `ContextInfo` on, the ViewOnce-wrap survival, and the poll `PollCreationMessage.ContextInfo` vs `MessageContextInfo` distinction.

---

**Files touched per commit (7 commits):** `domain/whatsapp/message.go`, `internal/whatsapp/client.go`, `internal/whatsapp/manager.go`, `internal/usecase/whatsapp/message_usecase.go`, `internal/queue/handlers/outgoing_message_handler.go`, `docs/openapi.yaml` (+version), plus docs (`llms.txt`, `wiki/`, `docs/ui/assets/docs/`, `CHANGELOG.md`) and a usecase test. **Not touched:** handlers, routes, Wire, config, `.env.example`, migrations, `domain/queue/message_queue.go` (job already carries the fields).