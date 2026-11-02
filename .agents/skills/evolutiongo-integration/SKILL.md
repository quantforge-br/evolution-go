---
name: evolutiongo-integration
description: |
  Integrate applications, backends, CRMs, bots and automation with this private
  Evolution Go (evolutiongo) WhatsApp service. Use whenever a project needs
  Evolution Go API authentication, instance creation or pairing, WhatsApp text or
  media sending, inbound webhooks, WebSocket events, or troubleshooting at
  evolutiongo.diogosrodrigues.com.br, even if the user only says "connect my app
  to evolutiongo". Covers the actual Go API contracts, separate admin and instance
  tokens, Docker-local webhook delivery and the private HTTP allowlist. Do not
  substitute Evolution API v2 or Meta Cloud API routes. For deployment changes,
  also load dokploy-management; this skill focuses on application integration.
compatibility: |
  Any HTTP-capable backend. The optional read-only probe requires Python 3.10+
  with the standard library; its tests use only local mock HTTP servers.
---

# Evolution Go integration

Build an integration against this service's actual API, not an SDK for a
similarly named product. Keep credentials and WhatsApp account access in the
consumer project's backend.

## Load the right reference

- Read [references/api.md](references/api.md) before implementing HTTP calls,
  instance provisioning, pairing or media uploads.
- Read [references/events.md](references/events.md) before implementing inbound
  messages, webhooks, receipts or WebSocket consumers.
- Read [references/private-deployment.md](references/private-deployment.md) when
  choosing callback URLs, diagnosing blocked downloads, or changing Docker or
  Dokploy settings.

The folder is self-contained: it can be copied to another project's
`.agents/skills/evolutiongo-integration` without the Evolution Go checkout.
Paths under `pkg/` below refer to that source checkout, not the consumer project.

## Known installation

| Setting | Value |
| --- | --- |
| Public API base (no trailing /api or /v2) | https://evolutiongo.diogosrodrigues.com.br |
| Manager | https://evolutiongo.diogosrodrigues.com.br/manager/login |
| Dokploy host / project / environment / service | home-server / evolution-go / production / evolution-go |
| Origin service / port | evolution-go / 8080 |
| TLS | Public HTTPS via Cloudflare Tunnel; Dokploy HTTPS is disabled, certificateType=none |
| Build | Private self-hosted; remote Evolution activation and telemetry removed |

These are installation defaults, not immutable identifiers. Verify the intended
host and existing instance before changing anything. No credential is bundled.
Do not require Evolution registration, a license server or Meta Cloud API setup.
WhatsApp/Meta connectivity is still required for actual WhatsApp operation.

## Workflow

### 1. Inspect the consumer project and settle missing inputs

Identify its language, HTTP stack, secret management, job queue and test setup.
Ask only for decisions that block implementation:

- Reuse which existing instance, or provision a new one?
- Send-only, or also receive messages and delivery receipts?
- Which account and consented recipient are authorized for a live test?
- For callbacks, where does the receiver run and how can Evolution reach it?

Implement in the project's conventions. Do not add an unrelated framework or
provision a second instance just to test connectivity.

### 2. Separate configuration and privileges

Use consumer-side environment variables such as:

~~~dotenv
EVOLUTION_GO_BASE_URL=https://evolutiongo.diogosrodrigues.com.br
EVOLUTION_GO_INSTANCE_TOKEN=
EVOLUTION_GO_INSTANCE_ID=
# Provisioning only; omit from ordinary message workers.
EVOLUTION_GO_ADMIN_KEY=
~~~

These names belong to the consumer, not to Evolution Go's server configuration.
The server's `GLOBAL_API_KEY` is the admin key. Instance tokens are distinct,
unique random secrets chosen when creating instances. Store real values in the
project's secret store or ignored environment files; keep examples empty.

- Send `apikey: ADMIN_KEY` for admin routes like `/instance/create` and `/instance/all`.
- Send `apikey: INSTANCE_TOKEN` for `/instance/status`, `/instance/connect` and `/send/*`.
- The token selects the instance. A normal send URL contains no instance name.
- Do not use Bearer authentication or assume an admin key works on instance routes.
- Keep both secrets out of browser bundles, mobile code, URLs and logs. The
  existing Manager is a separate administrative UI, not an integration pattern.
- Treat instance listing/creation responses and event bodies as secrets: they
  can contain `token` or `instanceToken`.

### 3. Run read-only checks first

With public checks only (no key required):

~~~sh
python3 /path/to/evolutiongo-integration/scripts/probe.py
~~~

With a token already supplied securely in the environment:

~~~sh
python3 /path/to/evolutiongo-integration/scripts/probe.py --scope instance
# Admin scope is only needed to verify provisioning access.
python3 /path/to/evolutiongo-integration/scripts/probe.py --scope admin
~~~

The probe checks health and private runtime status, then optionally instance
status or admin access. It never creates/connects an instance, retrieves a QR,
sends messages, follows redirects or prints credentials/raw instance responses.
It fails on unexpected HTTP codes or a non-local runtime. Read its JSON output;
HTTP health alone does not mean WhatsApp is logged in.

Use mocks during development. Creating instances, initiating pairing, reconnecting,
changing subscriptions, logging out or sending a message are real side effects.
Run those only within the user's authorized task. For smoke tests prefer an
explicitly approved test account and recipient; do not message arbitrary contacts.

### 4. Provision or reuse, then pair deliberately

Use the admin credential only in a provisioning boundary. Prefer an existing
instance if authorized. Save its `id` and instance token securely.

Use the instance token to call `POST /instance/connect` with the complete desired
subscription/transport configuration. This call overwrites those settings;
omitting a previous webhook or flag can disable it. See the API reference.

Pair through the Manager, QR or phone flow with human participation. Poll
`GET /instance/status` with bounded backoff until both `data.Connected` and
`data.LoggedIn` are true. Preserve the capitalization. A successful connect
response starts asynchronous work; it does not establish a logged-in session.

Do not use `GET /instance/qr` as a health check: it can start a WhatsApp client.
If passkey fields are returned instead of a QR, hand off to the human/Manager;
do not fabricate a QR, bypass authentication or store ceremony URLs in logs.

### 5. Implement a backend adapter

Centralize URL joining, `apikey` selection, timeouts, response validation and
error mapping. Keep admin provisioning separate from the instance-scoped client.
Read the Go-compatible request and response shapes in the reference.

- Send text with `POST /send/text`, not `/message/sendText/{instance}`.
- Upload media using multipart `POST /send/media`. Do not set a multipart
  Content-Type manually; let the HTTP library supply its boundary.
- Prefer direct uploads over public URLs. The private service blocks arbitrary
  outbound media/preview/webhook hosts, even on RFC1918 networks.
- Normalize phone inputs to international digits; preserve full JIDs for groups
  and other supported address types. Do not guess group IDs or alter `@lid` IDs.
- Check non-2xx responses before unwrapping `{message, data}`. Some handlers
  return 500 for validation-like failures; inspect a sanitized error as well.
- Use a durable application outbox and deduplicate business actions. A timeout
  after sending has an unknown outcome. Do not blindly retry sends, uploads or
  lifecycle changes; the service does not promise an idempotency-key contract.
- Honor Retry-After when present and use bounded backoff for safe reads and
  reconnections. Do not invent a documented rate limit or delivery guarantee.
- A send response is not proof of delivery/read. Use receipt events when needed.

### 6. Add inbound processing only if requested

Prefer a backend webhook on a verified same-host/container route. Configure the
exact receiver hostname in `LOCAL_HTTP_ALLOWED_HOSTS` and a shared Docker network
only after explicit approval to change deployment. Loopback inside Evolution's
container is not the consuming project's container.

Webhooks are not HMAC-signed and contain an instance credential. Restrict network
access, verify the expected instance identity/token, and redact that token before
persistence or logging. Do not claim body-token checking is a signature.
Persist/deduplicate work before acknowledging with 2xx and process asynchronously.
Keep response bodies minimal: Evolution logs webhook URLs and response bodies.

WebSocket requires the global admin key in a query parameter; use a protected
backend bridge only when necessary. Do not ship that key to a frontend or invent
Socket.IO paths. See the events reference for double JSON decoding and limits.

### 7. Verify and hand off

Add unit/contract tests with mock HTTP responses for correct auth scope, expected
paths/payloads, capitalized status fields, errors/timeouts and ambiguous sends.
For inbound processing test replay deduplication, unknown event types and token
redaction. Include private-host rejection and multipart upload tests if relevant.

Report the changed files, required environment names, instance mapping, test
results and unverified live steps. Clearly distinguish mocked tests, HTTP checks,
account pairing, successful sending, and confirmed message delivery.

## Source of truth and troubleshooting

When the Evolution checkout is available, verify behavior in this order:

1. `pkg/routes/routes.go` and `pkg/middleware/auth_middleware.go`.
2. Relevant handlers and services in `pkg/instance`, `pkg/sendMessage` and `pkg/message`.
3. `pkg/internal/event_types/event_types.go`, `pkg/whatsmeow/service/whatsmeow.go`,
   `pkg/events/webhook/webhook_producer.go` and `pkg/events/websocket/websocket_producer.go`.
4. `docs/swagger.json` and `/swagger/index.html` as discovery aids. Some schema names
   still mention EvolutionAPI; Swagger omits auth details and multipart behavior.

Use CodeGraph first if the checkout is indexed; otherwise read the scoped source.
Do not resolve uncertainty by borrowing another Evolution product's examples.
For 401, check admin vs instance scope; for 404, check path/base URL; for blocked
hosts, review the privacy reference rather than disabling the guard. Cloudflare
can return edge-level 403/error 1010 for certain HTTP clients; compare a normal
curl request with the origin's health/logs before blaming API authentication.
For deployment work, load the sibling dokploy-management skill and preserve
Dokploy HTTPS=false, the private image, existing volumes and unrelated services.

## Maintainer checks

Run the bundled probe tests without contacting a live account:

~~~sh
PYTHONDONTWRITEBYTECODE=1 python3 /path/to/evolutiongo-integration/scripts/test_probe.py -v
~~~

The evals/ directory contains three agent scenarios with explicit graders for
backend notifications, private-container webhooks and frontend/media boundaries.
They request plans only and forbid live side effects; they are not a record of
a completed agent benchmark. Recheck contracts against source after API changes.
