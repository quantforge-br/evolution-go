---
type: llm
focus: trace
weight: 1
---

integration-plan.md sends a PDF through backend multipart POST /send/media with the instance apikey, number, type=document, file and filename, allowing the HTTP library to set the boundary.
