---
type: llm
focus: trace
weight: 1
---

integration-plan.md uses http://orders-api:3000/integrations/evolutiongo/events only after verifying/adding an approved shared private Docker network and allowlisting the exact hostname orders-api; it does not assume loopback or all private IPs are allowed.
