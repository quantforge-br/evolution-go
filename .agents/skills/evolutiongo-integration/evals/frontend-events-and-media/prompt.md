---
tags: [evolutiongo-integration]
max_turns: 15
timeout_seconds: 300
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

Write integration-plan.md for a React dashboard that needs live Evolution Go message events and PDF uploads. We have a backend but someone proposed putting GLOBAL_API_KEY in VITE_EVOLUTION_KEY and opening /ws directly from React, then posting a public PDF URL to /send/media. Explain the safer design for this private installation, the actual socket/frame behavior if a backend bridge is needed, and the media upload fields. Do not connect a socket, expose any real credential, alter allowed hosts or send an actual message.
