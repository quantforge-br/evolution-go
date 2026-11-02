# HTTP API contract

## Authentication and paths

Base URL: `https://evolutiongo.diogosrodrigues.com.br`. There is no `/api` prefix.
HTTP header names are case-insensitive; use `apikey`. Bearer auth is not read.

| Method/path | Credential | Notes |
| --- | --- | --- |
| GET /server/ok | None | Returns {"status":"ok"} |
| GET /license/status | None | Local compatibility status, not WhatsApp connection status |
| POST /instance/create | Admin | Create using name and caller-selected token |
| GET /instance/all | Admin | data is an array; rows include secret tokens |
| GET /instance/info/{instanceId} | Admin | Instance ID, not display name; response is sensitive |
| DELETE /instance/delete/{instanceId} | Admin | Destructive; do not use for diagnosis |
| POST /instance/connect | Instance | Starts client asynchronously and replaces event configuration |
| GET /instance/status | Instance | Capitalized data.Connected, data.LoggedIn, data.Name |
| GET /instance/qr | Instance | Can start client; not a read-only probe |
| POST /instance/pair | Instance | Phone-code pairing; human participation |
| POST /instance/reconnect | Instance | Lifecycle side effect |
| POST /instance/disconnect | Instance | Stops a connection; not equivalent to deleting an account |
| DELETE /instance/logout | Instance | Invalidates session; requires deliberate authorization |
| GET/PUT /instance/{instanceId}/advanced-settings | Instance | Settings have side effects; see warning below |
| POST /send/text | Instance | JSON |
| POST /send/media | Instance | Multipart file upload or JSON URL |
| POST /send/link | Instance | May fetch preview URLs; guard applies |
| POST /message/downloadmedia | Instance | Inspect MessageHandler.DownloadMedia before implementing |
| POST /user/check | Instance | Inspect CheckUser request/response before implementing |
| GET /group/list | Instance | Retrieve groups rather than inventing a group JID |

The advanced-settings handlers look up the path instanceId directly. Do not
assume the path is constrained to the authenticated instance by middleware.
Enforce tenant ownership in your backend and match the path ID to its stored
instance mapping. Do not expose arbitrary instanceId passthrough endpoints.

Most handlers return `{"message":"success","data":...}` on 200, or
`{"error":"..."}` on failure. Unwrap only after checking the status.
Never log complete creation, listing or info responses.

The private local status contains `status:"active"`, `mode:"local"` and
`telemetry_enabled:false`. "Active" is a Manager compatibility flag, not a
commercial license grant or a logged-in WhatsApp session.

## Create an instance (admin)

Request:

~~~http
POST /instance/create
apikey: ADMIN_KEY
Content-Type: application/json
~~~

~~~json
{
  "name": "project-notifications",
  "token": "REPLACE_WITH_A_NEW_RANDOM_SECRET"
}
~~~

Both fields are required. Generate a unique high-entropy token using the host
language's cryptographic random generator; do not derive it from a project name.
Omit `instanceId` to get a generated UUID; persist the returned `data.id`.
An explicit instanceId should be a valid UUID. The response includes `data.token`.
Name uniqueness is checked; a duplicate currently results in an error, not an
idempotent create. Read/reuse the existing instance rather than rotating its
secret or deleting it implicitly.

Optional `advancedSettings` fields are `alwaysOnline`, `rejectCall`,
`msgRejectCall`, `readMessages`, `ignoreGroups` and `ignoreStatus`.
Do not enable auto-read, call rejection, presence or filtering without a reason.
Omit `proxy` for this private deployment.

## Connect and configure events (instance)

Send all desired fields. This replaces subscriptions, webhook URL and event
transport flags, including on a running instance; it is not a partial patch.

~~~json
{
  "subscribe": ["MESSAGE", "SEND_MESSAGE", "READ_RECEIPT", "CONNECTION"],
  "webhookUrl": "",
  "websocketEnable": "false",
  "rabbitmqEnable": "false",
  "natsEnable": "false"
}
~~~

Use `POST /instance/connect` with `apikey: INSTANCE_TOKEN`. Flags are strings,
not booleans; enabled transports accept "true" or "enabled". Only enable the
chosen transport. Empty subscribe defaults to MESSAGE; unknown names are
silently discarded. `["ALL"]` expands the event set, but prefer least necessary
subscriptions. See events.md before setting a nonempty webhookUrl.

Response data contains `jid`, `webhookUrl` and `eventString`. It may have an
empty jid until pairing completes. Do not interpret 200 as "account ready".

Check `GET /instance/status`, for example:

~~~json
{
  "message": "success",
  "data": {"Connected": true, "LoggedIn": true, "Name": "Account display name"}
}
~~~

The two booleans distinguish transport connection from login. Use bounded,
cancellable polling with backoff. Private deployment sets CONNECT_ON_STARTUP=false;
a valid stored session may need an explicit authorized connect after a restart.

## QR, phone and passkey pairing

Prefer the existing Manager for a human-operated setup. For an authorized custom
pairing UI call `GET /instance/qr` only while the account needs pairing. Its
response data has `qrcode` (image representation) and `code` (raw QR data).
Treat both as credentials. Check the representation before adding a data-URL
prefix; do not repeatedly restart pairing on an already logged-in account.

For phone-code pairing, `POST /instance/pair` accepts:

~~~json
{"phone":"5511999999999","subscribe":["MESSAGE","CONNECTION"]}
~~~

Phone includes country code and digits only. The current Go response field is
`data.PairingCode`, capitalized. Do not automatically retry a pairing ceremony.

If QR data instead has `passkeyStage`, `passkeyOpenUrl` and `passkeyCode`,
hand off to the user or the Manager's passkey flow. Ceremony URLs/codes are
sensitive, short-lived account actions. PASSKEY_PUBLIC_URL is configured to the
public HTTPS base; changing it requires a reviewed deployment change.

## Text sending (instance)

~~~http
POST /send/text
apikey: INSTANCE_TOKEN
Content-Type: application/json
~~~

~~~json
{
  "number": "5511999999999",
  "text": "Your requested notification",
  "id": "APPLICATION_MESSAGE_ID"
}
~~~

The destination and text are required. The optional id feeds WhatsApp message
construction; it is not a documented request idempotency guarantee. Maintain a
separate application outbox/unique business key and preserve the returned message
identifier, normally `data.Info.ID`. A response has `data.Info`, `data.Message`
and possibly `data.MessageContextInfo`; do not assume Evolution API v2's key.id.
Do not persist the whole response unless needed: it contains account/message data.

Optional fields: `quoted:{messageId,participant}`, `mentionedJid` (string array),
`mentionAll`, `delay`, `forwardingScore` and `formatJid`. Confirm delay units
in the service before exposing a setting; do not infer them from a field name.

Use international digits for a person; full JIDs such as
`120345678901234@g.us` identify groups. Existing LID/newsletter JIDs must retain
their server suffix. JSON formatJid defaults to true for normalization; setting
it false bypasses part of validation and needs deliberate, verified handling.
Multipart requests take a different middleware path; validate inputs locally.
Use actual JIDs from authorized group/contact discovery, not these examples.

## Media sending (instance)

Prefer direct multipart uploads to avoid third-party downloads and URL exposure.
Illustrative request (not a command to execute against a real contact):

~~~http
POST /send/media
apikey: INSTANCE_TOKEN
Content-Type: multipart/form-data; boundary=BOUNDARY_GENERATED_BY_CLIENT
~~~

| Multipart field | Value |
| --- | --- |
| number | Recipient's international digits or verified JID |
| type | image, video, audio or document |
| file | Uploaded file bytes |
| caption | Optional caption |
| filename | Optional document/file name |
| id | Optional message ID; not an idempotency guarantee |

Let the HTTP library generate the boundary. The file field is named `file`,
not media or attachment. Other accepted multipart fields include delay,
mentionAll, mentionedJid, quoted.messageId and quoted.participant.

Current file validation accepts JPEG/PNG/WebP images and MP4 video. Audio uses
local conversion when API_AUDIO_CONVERTER is empty; verify ffmpeg and runtime
limits if conversion fails. Documents use type=document and should include a
filename. Test payload size against application memory and Cloudflare limits;
file uploads are read into memory. Do not invent a universal maximum.

JSON URL mode accepts `{number,type,url,caption,filename}`, but the service fetches
that URL through the private HTTP guard. Arbitrary public hosts, private LAN IPs
and unlisted service names are blocked by default. Do not disable the guard to
make a quick demo work. A base64 field is not part of MediaStruct's JSON contract.

Link previews via `/send/link` can also trigger guarded HTTP fetches. For a plain
link with no preview processing, send it as text via `/send/text` instead.

## Failure handling

- 400: validate fields and JIDs; inspect a sanitized error.
- 401: missing/wrong apikey or wrong credential scope; do not retry credentials.
- 403 at Cloudflare: inspect edge response and compare health from origin/curl.
- 404: verify base URL and actual Go route; do not add v2 paths.
- 429/5xx: use bounded retries only for reads or other proven safe operations.
- Send timeout/connection loss: outcome may be unknown. Retain pending outbox
  state and reconcile before retrying; server media sending also has retries.
- "no active session found"/"not connected": inspect status and request deliberate
  reconnection, not logout/delete.
- Blocked destination: inspect allowed hosts and Docker topology; do not expand
  to wildcards, public download proxies or external telemetry services.
