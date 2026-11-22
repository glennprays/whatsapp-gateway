---
status: proposed
---

# 0002 — Map whatsmeow errors via exported sentinels (`errors.Is`/`errors.As`), not substring matching

The automation-platform surface adds ~50 error mappings (group-not-member, avatar not-set vs privacy-hidden, invite revoked vs invalid, recipient AD-JID, broadcast-unsupported, …) that must become correct HTTP statuses. We will map these by matching whatsmeow's **exported error sentinels** and `*whatsmeow.IQError` codes with `errors.Is`/`errors.As`, **not** by substring-matching error text.

## Considered options

- **Substring matching** (the current `isRecipientError` pattern) — rejected despite being the "consistent with existing code" instinct: it breaks silently whenever whatsmeow changes an error string, and it **cannot be unit-tested** against real error values, which is exactly the property we need since there is no live WhatsApp in CI.

## Consequences

- Error mapping becomes unit-testable: construct the sentinel / `IQError{Code}` value, assert the domain error + HTTP status.
- Importing whatsmeow's error vars couples us to them — a major whatsmeow upgrade that renames/removes a sentinel breaks the build **loudly** (desirable) instead of silently mis-mapping.
- The recurring "needs the exact error string" open questions get resolved once against the live whatsmeow build in a manual harness, then **locked to the sentinel** so CI pins them. Do not ship substring guesses.
- The existing `isRecipientError` substring check should migrate to sentinels as these paths are touched.
