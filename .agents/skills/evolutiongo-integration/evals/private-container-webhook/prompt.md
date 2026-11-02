---
tags: [evolutiongo-integration]
max_turns: 15
timeout_seconds: 300
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

Write integration-plan.md for receiving WhatsApp messages in a backend container named orders-api on home-server, port 3000, route /integrations/evolutiongo/events. Evolution Go is in a different Dokploy Compose project. Its Compose definition contains LOCAL_HTTP_ALLOWED_HOSTS: "". I want the traffic to stay on the host instead of going through a public webhook collector. Explain the exact configuration plan and receiver behavior, but do not modify Dokploy, Docker networks, stored subscriptions or a live account. Keep Cloudflare Tunnel as the HTTPS terminator.
