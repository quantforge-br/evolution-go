# Inbound events and delivery

## Choose a transport deliberately

For ordinary integration, prefer a private backend webhook on the same host.
Transport flags and subscriptions are set together using POST /instance/connect;
there is no separate /webhook/set endpoint in this build. Preserve the existing
configuration when adding/removing a transport. A successful connect response
does not prove that a callback was delivered.

The subscription names are case-sensitive:

~~~text
MESSAGE SEND_MESSAGE READ_RECEIPT CONNECTION QRCODE
PRESENCE HISTORY_SYNC CHAT_PRESENCE CALL LABEL CONTACT
GROUP NEWSLETTER BUTTON_CLICK PICTURE USER_ABOUT
~~~

Use only the set the project needs. An empty array defaults to MESSAGE; unknown
names are discarded. ["ALL"] is supported but increases data exposure.
Subscription categories are not event envelope names: MESSAGE emits Message,
SEND_MESSAGE emits SendMessage, and READ_RECEIPT emits Receipt. CONNECTION
covers Connected, PairSuccess, Disconnected, LoggedOut and other connection
events. Inspect current service dispatch for less common event types.

## Webhook setup and topology

With an approved receiver named notifications-api on a shared private Docker
network, configure LOCAL_HTTP_ALLOWED_HOSTS=notifications-api in Evolution's
application environment. Keep any previously approved hosts; do not overwrite
the list or remove the Compose restriction silently. The private Compose file
currently fixes this value to empty, so adding it only to Dokploy's interpolation
environment will not override that literal: update the service's environment
mapping deliberately, then redeploy.

Then use the instance token for POST /instance/connect with a complete payload:

~~~json
{
  "subscribe": ["MESSAGE", "SEND_MESSAGE", "READ_RECEIPT", "CONNECTION"],
  "webhookUrl": "http://notifications-api:3000/integrations/evolutiongo/events",
  "websocketEnable": "false",
  "rabbitmqEnable": "false",
  "natsEnable": "false"
}
~~~

This URL is illustrative and is not reachable until that receiver/network is
actually configured. A service name on the same machine is not automatically on
the same Docker network. Never publish the database to make callbacks work.
Do not use 127.0.0.1 unless the receiver really shares the app's network namespace.
A public Cloudflare hostname routes through a third party even if the origin is
on home-server; prefer a local route for the stated privacy requirements.

## Webhook wire format and security

The producer sends POST with Content-Type: application/json. It adds no API-key,
Authorization, HMAC signature or configurable secret header. Most event bodies
carry:

~~~json
{
  "event": "Message",
  "instanceId": "INSTANCE_UUID",
  "instanceName": "project-notifications",
  "instanceToken": "REDACTED_INSTANCE_TOKEN",
  "data": {}
}
~~~

The data shape is event-specific, mostly whatsmeow/Go serialization. A Message
commonly carries data.Info and data.Message; text may be in
Message.conversation or Message.extendedTextMessage.text. SendMessage also uses
Info and Message. Do not assume messages.upsert, data.key.remoteJid or another
product's lowercase schema. Confirm exact fields with sanitized fixtures from
the deployed build before mapping them to the project's domain model.

Restrict ingress to the trusted local sender/network. Check the expected
instanceId and instanceToken against the tenant mapping using a constant-time
secret comparison. Token-in-body checking is not HMAC and does not prevent
replays; do not describe the endpoint as signed. A custom public authenticated
receiver needs a separately designed gateway/auth scheme, not an invented header
that this producer never sends. Treat the whole raw body as sensitive until the
token, message and account data have been handled/redacted.

The producer makes up to five attempts on failure, with 30-second retry sleeps;
each HTTP attempt has a 30-second timeout. Retry work runs in memory, not a
durable delivery queue. Duplicates and loss across restarts are possible. The
producer may fan out to a global URL and an instance URL when configured, which
can create extra copies.

- Validate size and schema before accepting work.
- Persist the job/deduplication record transactionally, then respond with 2xx and
  a minimal body (for example {"ok":true}). Do expensive work asynchronously.
- Deduplicate Message/SendMessage by instance, event category and message ID.
  For receipts include receipt kind and relevant timestamp/IDs so delivered
  and read updates are not collapsed into one record. Derive keys per event.
- Ignore/quarantine unknown event types without crashing. Avoid replying to
  your own SendMessage/IsFromMe events and creating bot loops.
- Redact instanceToken before logging or writing normalized event records.
  Evolution itself logs callback URLs and response bodies; do not put secrets
  in callback query strings or echo the event back in the response.
- Treat DATABASE_SAVE_MESSAGES=false and WEBHOOK_FILES=false as defaults, not
  proof that logs or all event types contain no personal data.

## WebSocket (optional backend bridge)

This is a raw WebSocket endpoint, not Socket.IO. Its handshake is:

~~~text
wss://evolutiongo.diogosrodrigues.com.br/ws?token=ADMIN_KEY&instanceId=INSTANCE_UUID
~~~

The query token is the GLOBAL_API_KEY, not the instance token; the ordinary
apikey header is not used by this handler. Omit instanceId only for an explicitly
authorized all-instance broadcast. The instance must have websocketEnable set
to "true" or "enabled" for normal event delivery. Subscriptions still apply.

Use a backend-only consumer and an instance filter. Query tokens can appear in
proxy/access logs; redact them and avoid printing handshake URLs. The handler
also logs invalid tokens; do not probe it with a real unrelated secret. The
upgrader accepts any Origin: this is not a browser/tenant isolation boundary.
Do not hand a privileged socket URL to a browser. Prefer local webhooks when
that exposure is unnecessary.

Frame shape:

~~~json
{"queue":"instance-uuid.message","payload":"{\"event\":\"Message\",\"data\":{}}"}
~~~

Decode the frame as JSON, then decode its payload string as JSON. The payload
is not already an object. Queue is lowercased; payload event names retain case.
Validate the instance identity and handle unknown event shapes gracefully.

There is only one stored WebSocket client per instance ID. Do not attach several
workers directly and assume they all receive events; use one bridge that fans
out locally. There is no durable replay cursor or application-level ack protocol.
Reconnect with bounded backoff, but do not promise recovery of missed events.
