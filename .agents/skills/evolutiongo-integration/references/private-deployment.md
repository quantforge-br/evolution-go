# Private installation and network boundaries

## Established deployment

- API: https://evolutiongo.diogosrodrigues.com.br
- Manager: https://evolutiongo.diogosrodrigues.com.br/manager/login
- Host alias: home-server
- Dokploy: https://dokploy.diogosrodrigues.com.br
- Project/environment/Compose service: evolution-go / production / evolution-go
- Origin service: evolution-go, port 8080, private modified image
- Domain setting: https=false, certificateType=none
- Cloudflare Tunnel wildcard forwards to http://dokploy-traefik:80
- Application has no host port binding. PostgreSQL is on an internal network
  with persistent volumes and no published port.

The public client still uses HTTPS. "HTTPS disabled" refers only to Dokploy's
certificate/router setting, not to disabling browser TLS. Do not enable ACME,
Let's Encrypt or a second certificate workflow in Dokploy for this domain.
Do not change Cloudflare/DNS/security rules merely to fix a client-library error.

The service uses a raw Compose definition, not Git auto-deploy. Use the sibling
dokploy-management skill for authorized deployment changes. Confirm the instance
profile and explicit service ID before mutation; the default CLI session can
point to an unrelated project. Never assume service IDs/tags are permanent.

## Credentials

The admin key is the server's GLOBAL_API_KEY in Dokploy's service environment.
In the source checkout it also exists in ignored .env.private, mode 600. Do not
print the whole environment, commit that file, copy it into a container build
context, or send it to a consumer frontend. Provision a separate instance token
for a consumer project. Dokploy's own API credential is unrelated to either key.

Do not automatically read every secret from Dokploy when only an instance token
is needed. If the consumer's credentials are absent, obtain them through the
user's approved secret channel or stop before authenticated actions.

## HTTP integration guard

Application-originated HTTP for webhooks, media/link downloads, profile/group
avatars and optional audio converters uses pkg/privacy/http.go.

- Loopback hosts are permitted by default; all other destinations need explicit
  exact entries in LOCAL_HTTP_ALLOWED_HOSTS.
- Entries are comma-separated hostnames/IPs, not URLs, ports, wildcards, CIDRs or
  suffix matches. Example: notifications-api,media-server.
- RFC1918/private LAN addresses and other containers are not implicitly trusted.
- Every redirect destination is checked before DNS/network access. Allowing an
  initial host does not allow a redirect to another host.
- Integration HTTP clients ignore environment proxies and time out after 30s.
- Host allowlisting is a trust decision, not proof of where DNS resolves or a
  port-level network boundary. Verify service ownership, routes and topology.

The current compose.dokploy.yml explicitly fixes LOCAL_HTTP_ALLOWED_HOSTS="".
For an approved local integration, update that application environment mapping
and shared networking in the actual Compose definition, then redeploy. Merely
adding LOCAL_HTTP_ALLOWED_HOSTS to the interpolation .env will not replace an
empty literal. Preserve other application settings and volumes.

An optional topology is a dedicated external bridge network attached only to
Evolution and the consuming backend, with a unique receiver alias. Keep the
PostgreSQL internal network separate and avoid attaching a whole unrelated stack
to dokploy-network just for one callback. Inspect existing networks and request
approval before creating/attaching one; do not patch a live unrelated service.

## Privacy-preserving defaults

- Remote Evolution activation, registration, heartbeat and telemetry are removed.
- WhatsApp Web version fetching is opt-in; WHATSAPP_AUTO_VERSION defaults false.
- Public/global webhooks, NATS, AMQP, MinIO and external audio converters are off.
- WEBHOOK_FILES=false; DATABASE_SAVE_MESSAGES=false; logs still need access
  controls and retention. Event envelopes may include credentials.
- CONNECT_ON_STARTUP=false; do not automatically change this setting for a
  project integration. Establish any restart/reconnect policy deliberately.
- Use multipart media upload and /send/text for plain links where possible.
- License and branding notices remain applicable. Local "active" compatibility
  status is not a legal licensing conclusion.

WhatsApp account/protocol/media communications necessarily reach WhatsApp/Meta.
Cloudflare processes traffic routed through its public hostname. Do not promise
"all traffic stays on the VPS" when using those paths. For same-host backend
calls, prefer a verified private service route over Cloudflare when feasible.

The HTTP guard does not cover direct PostgreSQL, NATS/AMQP, MinIO/S3, WhatsApp
proxy clients, browser navigation, other processes or future clients. Keep such
integrations disabled unless separately reviewed. Do not disable the guard,
replace the app with an upstream image, use a public download proxy, rewrite
stored settings, delete volumes or change the host firewall to make a test pass.

## Diagnostics

- Run scripts/probe.py for public/private-mode and optional scoped-auth checks.
- A 200 /server/ok is process health, not a WhatsApp session; inspect
  /instance/status with an instance token for Connected and LoggedIn.
- A failed callback with a blocked-host error needs a reviewed allowed-host and
  network plan, not a switch to an arbitrary public webhook collector.
- Cloudflare 403/error 1010 can vary by HTTP client fingerprint/User-Agent.
  Compare a normal curl public request and HTTP origin health through the
  existing trusted administration path. Do not disable security globally.
- /instance/qr is stateful in practice; never call it as monitoring.
- Protect Dokploy/container logs: callback URLs, bodies, account identifiers and
  some errors can contain sensitive data. Sanitize diagnostics before sharing.
