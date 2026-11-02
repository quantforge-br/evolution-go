---
tags: [evolutiongo-integration]
max_turns: 15
timeout_seconds: 300
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

Write integration-plan.md for an existing Node.js/TypeScript backend that needs to send order notifications through the private Evolution Go service at evolutiongo.diogosrodrigues.com.br. Reuse an existing authorized instance. Explain the environment variables, the HTTP text request, how to check whether the account is ready, and how the worker should handle a timeout after a send. Do not call a live endpoint, create an instance, retrieve a secret, or send a message. Do not use an Evolution API v2 SDK.
