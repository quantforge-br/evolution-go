# Private self-hosting

This is a locally modified build of Evolution Go for private use. It uses software
from Evolution Foundation and whatsmeow; it is not an official release or an
endorsement. The original LICENSE, NOTICE and trademark policy are retained.
Removing a technical activation check does not remove or replace those legal
terms. The bundled Manager's branding and copyright have not been changed.

## What changed

- Removed remote registration, activation, heartbeat and deactivation code.
- Removed the legacy telemetry package and hardware-derived instance identifiers.
- Preserved /license/status, /license/register and /license/activate as read-only
  local compatibility endpoints for the bundled Manager. They return
  status=active, mode=local and telemetry_enabled=false without credentials,
  registration URLs or outbound requests. Active is a compatibility flag, not a
  commercial license grant. Existing runtime_configs records are not read,
  migrated or deleted by this build.
- Preserved admin and instance API authentication. GLOBAL_API_KEY remains required.
- Restricted application HTTP integrations (webhooks, media/link downloads, avatar
  updates and audio converters) to loopback by default. Every redirect is checked
  before DNS or HTTP connection. These requests ignore environment HTTP proxies.
- Disabled the optional WhatsApp Web version lookup by default. The bundled
  whatsmeow version is used unless all three WHATSAPP_VERSION_* values are set.
  WHATSAPP_AUTO_VERSION=true explicitly opts back into the web.whatsapp.com fetch.
- Removed the public webhook.site destination from .env.example and disabled
  optional event/storage integrations in the private deployment profile.
- Prevented an empty NATS_URL from opening the library's default NATS connection.

## Start on your VPS

Requires Docker with the Compose plugin and openssl. From this checkout:

~~~sh
./scripts/setup-private.sh
docker compose --env-file .env.private -f compose.private.yml up -d --build
~~~

The setup script creates unique API/database secrets in .env.private, mode 600.
It does not print secrets and does not overwrite an existing setup. The file is
excluded from Git and the Docker build context. Keep it on the VPS; use a secure
editor to retrieve the GLOBAL_API_KEY for login. Do not post it in logs or issues.

The API is bound to 127.0.0.1:8080 only. PostgreSQL is on an internal Docker
network with no published ports. Use an SSH tunnel from your own computer:

~~~sh
ssh -N -L 8080:127.0.0.1:8080 user@your-vps
~~~

Open http://127.0.0.1:8080/manager/login, keep that same API URL, and supply your
GLOBAL_API_KEY. No Evolution registration, email or first-time online activation
is required. Connect your own WhatsApp account from the Manager when ready.

Stop without deleting data:

~~~sh
docker compose --env-file .env.private -f compose.private.yml down
~~~

Do not add -v unless you intentionally want to delete the databases and sessions.
Back up PostgreSQL, the application volumes and .env.private yourself.

## Dokploy on home-server

The private image is deployed in Dokploy at https://dokploy.diogosrodrigues.com.br
under project evolution-go, environment production, service evolution-go.
The service uses compose.dokploy.yml, not an upstream image or Git auto-deploy.

Access the Manager at https://evolutiongo.diogosrodrigues.com.br/manager/login.
Use https://evolutiongo.diogosrodrigues.com.br as the API URL and the
GLOBAL_API_KEY stored in the Dokploy service environment. The same key is in
the local ignored .env.private file. Do not publish that file.

In Dokploy the domain points to service evolution-go, port 8080, with HTTPS
disabled and certificateType=none. Cloudflare Tunnel terminates public HTTPS
and forwards to the Traefik web entrypoint over HTTP. No app or database host
ports are published. The database uses a separate internal Docker network.

PASSKEY_PUBLIC_URL points to the public HTTPS hostname for optional WebAuthn
pairing. This does not enable HTTPS or certificate management in Dokploy.

For updates, build a new private image, assign an immutable tag, transfer it
to home-server using docker save piped over SSH into docker load, update
EVOLUTION_IMAGE in the Dokploy environment, and deploy the Compose service.
The application image uses pull_policy=never: do not prune its tagged image
from home-server before redeploying. Never replace it with an upstream image.

All private changes are committed on the local private branch; it is the
only source for these images. Build them with scripts/build-private-image.sh:
it refuses to run on another branch or with uncommitted tracked changes, uses
git archive HEAD as the build context (so untracked files and .env files never
reach the image), labels the image with the source commit and can stream it
to the host:

    scripts/build-private-image.sh --dry-run private-<tag>
    scripts/build-private-image.sh --load diogosrodrigues private-<tag>

Then set EVOLUTION_IMAGE and redeploy. With CONNECT_ON_STARTUP=true the server
resumes linked sessions at startup; a failed connection (for example DNS still
down right after a host reboot) is retried every 5 s, doubling up to 5 min,
until it succeeds. To roll back, set EVOLUTION_IMAGE back to the previous tag,
which stays loaded on the host.

## Local binary

The upstream prebuilt ./evolution-go has been removed because it still contains
remote licensing code. Do not restore it. Compile the modified source instead:

~~~sh
GOTELEMETRY=off go build -ldflags "-X main.version=local-private" -o build/evolution-go ./cmd/evolution-go
cp .env.example .env
# Edit .env: use your own API key and local PostgreSQL credentials.
./build/evolution-go -dev
~~~

Keep the working directory at the repository root so Manager assets are served.
Do not reuse upstream container images either; compose.private.yml builds this
source tree. The stock Compose examples elsewhere still describe upstream
images and are not the private deployment entry point.

## Local integrations and existing installations

Leave LOCAL_HTTP_ALLOWED_HOSTS empty for loopback-only application HTTP requests.
Containers on the same VPS can be enabled by exact service names, for example:

~~~env
LOCAL_HTTP_ALLOWED_HOSTS=media,converter
~~~

Configure their URLs to those local services and add the corresponding services
and networks yourself. Names are comma-separated hostnames or IPs, not URLs,
ports, wildcards or suffix patterns. Allowlisting a host is an explicit trust
decision: it is not proof that the host lives on your VPS. Do not allowlist public
third-party domains if you want to keep integrations local.

External media URLs and link preview downloads are blocked by default. Upload
media directly or serve it from your VPS. Per-instance webhook URLs already
saved in PostgreSQL are also subject to the HTTP allowlist. Clear unwanted stored
webhooks/settings before reusing an existing database.

Direct database, AMQP, NATS, MinIO/S3 and WhatsApp proxy connections are separate
clients, not covered by the HTTP integration guard. The private Compose profile
keeps PostgreSQL local and disables every other one. If using your own existing
.env or deployment, audit all of those endpoints and instance proxy settings;
blank out external services. This patch does not silently rewrite existing data.
Logs may contain message/account information and remain on the VPS; protect them
and configure retention. DATABASE_SAVE_MESSAGES=false is not a substitute for
log access controls.

## Network boundaries

This is not a fully offline WhatsApp implementation. Account login, messages,
media and protocol metadata necessarily go to WhatsApp/Meta and its media
infrastructure. You cannot use WhatsApp while sending absolutely nothing outside
the VPS. The restriction here is no Evolution telemetry/licensing and no
unapproved application HTTP integrations.

Building/installing uses Go module, container and operating-system package
registries. GOTELEMETRY=off disables Go toolchain telemetry during the documented
build. The browser may contact external sites if you click documentation or
community links, or enable third-party browser extensions. No changes were made
to the precompiled frontend assets.

The HTTP guard is defense in depth, not a system-wide egress firewall. It does
not restrict other VPS processes, future custom clients or DNS configuration.
For an independent network guarantee, apply a separately reviewed per-container
or per-service egress policy allowing only your services and the WhatsApp
infrastructure you actually need. Do not flush a host firewall or copy a static
Meta IP range list: that can break SSH and legitimate WhatsApp connections.

## Verification

~~~sh
GOTELEMETRY=off go test ./...
GOTELEMETRY=off go test -race ./pkg/privacy ./pkg/core ./pkg/events/nats ./pkg/events/webhook ./pkg/config ./pkg/sendMessage/service ./cmd/evolution-go
GOTELEMETRY=off go vet ./...
curl --fail http://127.0.0.1:8080/license/status
# Expected: mode=local, status=active, telemetry_enabled=false; no identifiers.
curl -i http://127.0.0.1:8080/instance/all
# Expected: HTTP 401 without an API key, not a license-related HTTP 503.
~~~

Tests cover zero remote requests from local compatibility routes, blocked
external HTTP hosts/redirects before DNS, allowed local media, external audio
converters and link previews, the empty NATS configuration, version-fetch opt-in,
and authentication remaining mandatory without activation.
