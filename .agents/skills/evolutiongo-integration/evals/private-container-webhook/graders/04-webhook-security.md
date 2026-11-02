---
type: llm
focus: trace
weight: 1
---

integration-plan.md states that events are not HMAC-signed, checks expected instance identity/token, redacts instanceToken, persists/deduplicates before a minimal 2xx response, and does not invent a signature header.
