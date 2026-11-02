---
type: llm
focus: trace
weight: 1
---

integration-plan.md treats a send timeout as an unknown outcome and uses an application outbox/deduplication boundary rather than claiming automatic retries or an idempotency-key guarantee.
