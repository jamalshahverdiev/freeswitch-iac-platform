# Control Plane API

The control-plane exposes two groups of HTTP endpoints:

- **`/api/v1/*`** — management API (Bearer token). Used by operators / Terraform
  to write desired state. CRUD over domains, users, gateways, dialplan, plus
  runtime commands.
- **`/xml/*`** — FreeSWITCH-facing endpoints consumed by `mod_xml_curl`. No token
  (network-restricted). Return rendered FreeSWITCH XML, or a "not found"
  document so FreeSWITCH falls back to its on-disk config.

Base URL in this setup: `http://localhost:8080` (dev box). Token: `dev-token`.

All examples assume:

```bash
API=http://localhost:8080
TOKEN=dev-token
H=(-H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json")
```

A full, self-checking regression test lives in `deploy/api-test.sh`
(51 assertions, throwaway data, auto-cleanup). The IVR demo lives in
`deploy/seed-ivr.sh`.

---

## Authentication

`/api/v1/*` requires:

```
Authorization: Bearer <token>
```

Missing/invalid token → `401 unauthorized`. `/healthz`, `/readyz` and `/xml/*`
are open (the latter is meant to be network-restricted in production).

## Error format

```json
{ "error": { "code": "validation_error", "message": "domain name is required" } }
```

| HTTP | code | when |
|------|------|------|
| 400 | `validation_error` | bad body / missing required field / bad regex |
| 401 | `unauthorized` | missing or wrong token |
| 404 | `not_found` | resource (or parent domain) does not exist |
| 409 | `already_exists` | unique key already used |
| 502/503 | `runtime_error` | ESL not configured / FreeSWITCH unreachable |
| 500 | `internal_error` | unexpected server error |

---

## Health

```bash
curl -s $API/healthz   # {"status":"ok"}
curl -s $API/readyz    # {"status":"ready","database":"ok"}
```

---

## Domains

A domain groups users and dialplan. For registration to work on this server the
domain name must be the FreeSWITCH `$${domain}` (here `192.168.48.143`).

```bash
# create
curl -s $API/api/v1/domains "${H[@]}" -d '{
  "name":"192.168.48.143","description":"IaC domain","enabled":true,
  "variables":{"default_language":"en"}}'

curl -s $API/api/v1/domains "${H[@]}"                 # list
curl -s $API/api/v1/domains/192.168.48.143 "${H[@]}"  # get
curl -s -X PUT $API/api/v1/domains/192.168.48.143 "${H[@]}" -d '{"description":"x","enabled":true,"variables":{}}'
curl -s -X DELETE $API/api/v1/domains/192.168.48.143 "${H[@]}"   # cascades users + dialplan
```

| field | type | required | notes |
|---|---|---|---|
| name | string | yes | unique; used as the directory `<domain name>` |
| description | string | no | |
| enabled | bool | no | default true; disabled domains are not rendered |
| variables | map | no | emitted as `<variable>` under the domain |

---

## Users (SIP extensions)

```bash
curl -s $API/api/v1/users "${H[@]}" -d '{
  "domain":"192.168.48.143","number":"2001","enabled":true,
  "params":{"password":"2580"},
  "variables":{
    "effective_caller_id_name":"IaC User 2001",
    "effective_caller_id_number":"2001",
    "user_context":"company"
  },
  "voicemail":{"enabled":true,"password":"2001","email":"u2001@example.com","attach_file":true}}'

curl -s "$API/api/v1/users?domain=192.168.48.143" "${H[@]}"   # list (optional ?domain=)
curl -s $API/api/v1/users/192.168.48.143/2001 "${H[@]}"        # get
curl -s -X PUT $API/api/v1/users/192.168.48.143/2001 "${H[@]}" -d '{"params":{"password":"new"},"variables":{"user_context":"company"}}'
curl -s -X DELETE $API/api/v1/users/192.168.48.143/2001 "${H[@]}"
```

| field | type | required | notes |
|---|---|---|---|
| domain | string | yes | must already exist (else 404) |
| number | string | yes | unique within domain |
| params | map | no | `password`, … → `<param>` (escape hatch for arbitrary directory params) |
| variables | map | no | **set `user_context` to route the user's calls into your context** |
| voicemail | object | no | typed mailbox (see below); omit for no voicemail |

`params.password` is what the SIP phone authenticates with. `user_context`
decides which dialplan context the user's calls enter.

### Voicemail

The `voicemail` object is the typed alternative to hand-setting `vm-*` keys in
`params`. It is expanded into `mod_voicemail` directory params when the user is
rendered; if set, it overrides any matching freeform `vm-*` key.

| field | type | default | directory param |
|---|---|---|---|
| enabled | bool | `false` | `vm-enabled` |
| password | string | — | `vm-password` (PIN; redacted in the audit log) |
| email | string | — | `vm-mailto` |
| attach_file | bool | `false` | `vm-attach-file` |
| email_all | bool | `false` | `vm-email-all-messages` |

Messages are stored by FreeSWITCH in the `freeswitch_core` database (the
`voicemail.conf` profile's `odbc-dsn`). `voicemail.password` is never returned
in directory XML and is redacted (`***`) in the audit log.

---

## Gateways (SIP trunks)

Stored in the DB and rendered into `sofia.conf`. **Note:** the `configuration`
binding is disabled on the server by default, so gateways are not yet pulled by
FreeSWITCH until you enable `<binding name="configuration">` in
`xml_curl.conf.xml`.

```bash
curl -s $API/api/v1/gateways "${H[@]}" -d '{
  "name":"provider-main","profile":"external","proxy":"sip.provider.com",
  "username":"u","password":"s","realm":"sip.provider.com","register":true,
  "params":{"expire-seconds":"3600"}}'

curl -s "$API/api/v1/gateways?profile=external" "${H[@]}"
curl -s $API/api/v1/gateways/external/provider-main "${H[@]}"
curl -s -X PUT $API/api/v1/gateways/external/provider-main "${H[@]}" -d '{"proxy":"sip2.provider.com","register":false}'
curl -s -X DELETE $API/api/v1/gateways/external/provider-main "${H[@]}"
```

Required: `name`, `profile`, `proxy`. `profile`+`name` is the unique key.

---

## Dialplan extensions

An extension belongs to a domain + context and contains ordered `conditions`,
each with ordered `actions`. The renderer preserves order.

```bash
curl -s $API/api/v1/dialplan/extensions "${H[@]}" -d '{
  "name":"internal-2xxx","domain":"192.168.48.143","context":"company",
  "priority":10,"enabled":true,
  "conditions":[{
    "field":"destination_number","expression":"^(20[0-9][0-9])$",
    "actions":[{"application":"bridge","data":"user/$1@192.168.48.143"}]
  }]}'

curl -s "$API/api/v1/dialplan/extensions?context=company" "${H[@]}"   # list (?domain= / ?context=)
curl -s $API/api/v1/dialplan/extensions/<uuid> "${H[@]}"              # get
curl -s -X PUT $API/api/v1/dialplan/extensions/<uuid> "${H[@]}" -d '{ ...full object... }'
curl -s -X DELETE $API/api/v1/dialplan/extensions/<uuid> "${H[@]}"
```

Validation: domain must exist, context required, ≥1 condition, each condition
≥1 action, every `expression` must be a valid regex, action `application`
required. The `id` is a UUID returned on create (used for get/update/delete).

| field | type | required |
|---|---|---|
| name | string | yes (unique per domain+context) |
| domain | string | yes |
| context | string | yes |
| priority | int | no (default 100) |
| conditions[].field / .expression | string | yes |
| conditions[].actions[].application | string | yes |
| conditions[].actions[].data | string | no |

---

### Time-based routing

A condition may carry FreeSWITCH time-of-day attributes via a `time` object
(in addition to, or instead of, `field`/`expression`):

```json
{"field":"destination_number","expression":"^(4444)$",
 "time":{"wday":"2-6","hour":"9-17"},
 "actions":[{"application":"transfer","data":"support@192.168.48.143"}]}
```

Supported keys: `wday` (1=Sun..7=Sat), `mday`, `mon`, `mweek`, `week`, `hour`,
`minute`, `time-of-day`, `date-time`. A condition with `time` and no
`field`/`expression` is a pure time gate. Day/night routing = two extensions on
the same number ordered by priority: the higher one carries the time window;
when it's outside that window the condition fails and the call falls through to
the lower "closed" extension. Validation rejects a condition that has neither a
regex nor time attributes (400). Worked example: `examples/time-routing/`.

## Building an IVR (menus + submenu)

An IVR is just dialplan data — no special resource. The pattern:

1. A **menu** extension: `answer` → `play_and_get_digits` (stores the pressed
   digit in a channel variable) → `transfer` to a synthetic number built from
   that variable.
2. One **routing** extension per choice, matching that synthetic number.

`play_and_get_digits` data format:

```
<min> <max> <tries> <timeout> <terminators> <prompt> <invalid_prompt> <var_name> <regexp> [digit_timeout]
```

### Worked example — two levels (see `deploy/seed-ivr.sh`)

```
7000 main menu : 1 -> submenu(7100) | 2 -> call 2001 | 3 -> echo
7100 submenu   : 1 -> call 2002      | 2 -> echo       | 0 -> back to 7000
```

Main menu extension:

```json
{
  "name":"ivr-main","domain":"192.168.48.143","context":"company","priority":100,
  "conditions":[{"field":"destination_number","expression":"^(7000)$","actions":[
    {"application":"answer"},
    {"application":"sleep","data":"500"},
    {"application":"play_and_get_digits",
     "data":"1 1 3 5000 # ivr/ivr-welcome_to_freeswitch.wav ivr/ivr-that_was_an_invalid_entry.wav main_choice \\d 3000"},
    {"application":"transfer","data":"main_${main_choice} XML company"}
  ]}]
}
```

Routing for digit 1 (go to submenu), digit 2 (call a user), digit 3 (echo):

```json
{"name":"main-1","domain":"192.168.48.143","context":"company","priority":101,
 "conditions":[{"field":"destination_number","expression":"^main_1$",
  "actions":[{"application":"transfer","data":"7100 XML company"}]}]}
```

The submenu (`7100`) is the same shape with a different variable
(`sub_choice`) and a `0 → transfer 7000` option to go back. Because each level
uses a **distinct variable name**, nesting is unambiguous.

Verify on the server that FreeSWITCH pulls it:

```bash
fs_cli -x "xml_locate dialplan context name company"      # shows your extensions
fs_cli -x "originate loopback/7000/company &park()"       # exercises the menu (plays the prompt)
```

---

## Call center (`mod_callcenter`)

Queues, agents and tiers are stored in PostgreSQL and served to FreeSWITCH as
`callcenter.conf` via the configuration binding. The rendered config also
carries `<param name="odbc-dsn" .../>`, so mod_callcenter keeps its **runtime**
state (agents/tiers/members tables) in our `freeswitch_callcenter` Postgres
database — no sqlite anywhere.

```bash
# Queue (defaults: longest-idle-agent, local_stream://moh, discard-abandoned 60s)
curl -s -X POST $API/api/v1/callcenter/queues "${H[@]}" -d '{"name":"support@192.168.48.143"}'
curl -s        $API/api/v1/callcenter/queues "${H[@]}"               # list
curl -s        $API/api/v1/callcenter/queues/support@192.168.48.143 "${H[@]}"
curl -s -X PUT $API/api/v1/callcenter/queues/support@192.168.48.143 "${H[@]}" \
     -d '{"strategy":"round-robin","moh_sound":"local_stream://moh"}'
curl -s -X DELETE $API/api/v1/callcenter/queues/support@192.168.48.143 "${H[@]}"  # cascades tiers

# Agent (contact = a managed SIP user; defaults: type callback, status Available)
curl -s -X POST $API/api/v1/callcenter/agents "${H[@]}" \
     -d '{"name":"4201@192.168.48.143","contact":"user/4201@192.168.48.143"}'

# Tier (binds agent to queue; 404 if either side does not exist)
curl -s -X POST $API/api/v1/callcenter/tiers "${H[@]}" \
     -d '{"queue":"support@192.168.48.143","agent":"4201@192.168.48.143"}'
```

Callers enter a queue through a normal dialplan extension:
`action {"application":"callcenter","data":"support@192.168.48.143"}`.
Watch the priority: an earlier extension with a wider regex (e.g. `^(4[0-9]{3})$`)
will swallow the entry number.

After changing queues/agents/tiers run `POST /api/v1/runtime/callcenter/reload`
(plain `reloadxml` is not enough — mod_callcenter loads its config on module
reload).

Runtime control / observability over ESL:

```bash
curl -s -X POST $API/api/v1/runtime/callcenter/reload "${H[@]}"
curl -s -X PUT  $API/api/v1/runtime/callcenter/agents/4201@192.168.48.143/status \
     "${H[@]}" -d '{"status":"On Break"}'    # Available | On Break | Logged Out
curl -s $API/api/v1/runtime/callcenter/queues/support@192.168.48.143/agents  "${H[@]}"
curl -s $API/api/v1/runtime/callcenter/queues/support@192.168.48.143/members "${H[@]}"
curl -s $API/api/v1/runtime/callcenter/queues/support@192.168.48.143/tiers   "${H[@]}"
```

Terraform resources: `freeswitch_callcenter_queue`, `freeswitch_callcenter_agent`,
`freeswitch_callcenter_tier`, `freeswitch_callcenter_reload` — full worked
example in `examples/callcenter/`.

---

## Conferences (`mod_conference`, audio + video)

Profiles (settings groups) live in PostgreSQL and are served as
`conference.conf` via the configuration binding. A **room** materializes two
things at once: the conference itself and the dialplan extension callers dial
to enter it (rendered into `/xml/dialplan` automatically — no separate
extension resource needed). `video_mode = "mux"` gives a composed video
conference (everyone sees a grid).

```bash
# Profile (defaults: 48 kHz, group:grid, 1280x720@15fps, MOH while alone)
curl -s -X POST $API/api/v1/conference/profiles "${H[@]}" -d '{"name":"video-grid","video_mode":"mux"}'

# Rooms: dial 3500 to join "standup"; 3501 requires PIN 2580
curl -s -X POST $API/api/v1/conference/rooms "${H[@]}" \
     -d '{"name":"standup","number":"3500","domain":"192.168.48.143","context":"company","profile":"video-grid"}'
curl -s -X POST $API/api/v1/conference/rooms "${H[@]}" \
     -d '{"name":"private","number":"3501","domain":"192.168.48.143","context":"company","profile":"video-grid","pin":"2580"}'
```

Apply: only `POST /api/v1/runtime/reloadxml` (for the dialplan entry).
Profiles are read when a NEW conference starts — no module reload. A profile
used by rooms cannot be deleted (409).

Runtime control:

```bash
curl -s $API/api/v1/runtime/conference/standup "${H[@]}"   # participants (404 = not running)
curl -s -X POST $API/api/v1/runtime/conference/standup/kick   "${H[@]}" -d '{"member":"all"}'
curl -s -X POST $API/api/v1/runtime/conference/standup/mute   "${H[@]}" -d '{"member":"1"}'
curl -s -X POST $API/api/v1/runtime/conference/standup/unmute "${H[@]}" -d '{"member":"1"}'
curl -s -X PUT  $API/api/v1/runtime/conference/standup/layout "${H[@]}" -d '{"layout":"2x2"}'
```

Terraform: `freeswitch_conference_profile`, `freeswitch_conference_room`
resources + `freeswitch_conference_status` data source (live participants).
Worked example: `examples/conference/`.

---

## Call recordings

Recordings land on the FreeSWITCH host in **per-day folders**
(`/var/lib/freeswitch/recordings/YYYY/MM/DD/<file>.wav`) — FreeSWITCH creates
the nested directories itself. Two sources:

- **Queue calls**: `record_session` action before `callcenter` in the entry
  extension (see `examples/callcenter/` — `record_path` local with
  `$${strftime(%Y/%m/%d)}`; `$${...}` is the HCL escape so FreeSWITCH gets
  `${strftime(...)}` and expands it per call).
- **Conferences**: `auto_record` on `freeswitch_conference_profile`
  (see `examples/conference/`). Tip: a `.mp4` template records the composed
  VIDEO canvas, `.wav` records the audio mix.

The control-plane proxies a read-only nginx file server on the FS host
(`:8088`, Basic auth, config in `deploy/freeswitch/nginx/`):

```bash
curl -s $API/api/v1/recordings "${H[@]}"                  # today
curl -s "$API/api/v1/recordings?date=2026-06-04" "${H[@]}"
# -> {"date":"2026-06-04","recordings":[{"file":"...","size":...,"mtime":"...","url":"/api/v1/recordings/..."}]}
curl -s -OJ $API/api/v1/recordings/2026-06-04/conf_standup_18-12-36.wav "${H[@]}"
```

---

## Phone provisioning

Desk phones fetch their config from the control-plane at boot
(`GET /provision/<mac>`) instead of being configured by hand. A
`provisioned_device` maps a MAC to an extension; the SIP password is **never
stored on the device record** — it is read from the matching user
(`params.password`) at render time, so rotating the user's password
automatically reprovisions the phone.

```bash
# register a phone for extension 3001 (MAC accepts any separators/case)
curl -s $API/api/v1/devices "${H[@]}" -d '{
  "mac":"80:5e:c0:11:22:33","vendor":"yealink","model":"T46U",
  "number":"3001","domain":"demo.test","display_name":"Reception"}'

curl -s $API/api/v1/devices "${H[@]}"                       # list
curl -s $API/api/v1/devices/805ec0112233 "${H[@]}"          # get (normalized mac)
curl -s -X PUT $API/api/v1/devices/805ec0112233 "${H[@]}" -d '{"number":"3001","domain":"demo.test","vendor":"grandstream"}'
curl -s -X DELETE $API/api/v1/devices/805ec0112233 "${H[@]}"
```

| field | type | required | notes |
|---|---|---|---|
| mac | string | yes | normalized to lowercase, no separators; unique |
| number | string | yes | extension; its password comes from the matching user |
| domain | string | yes | SIP domain the user lives in |
| vendor | string | no | `yealink` (default) \| `grandstream` \| `generic` |
| model | string | no | informational |
| display_name | string | no | falls back to `number` |
| enabled | bool | no | default `true`; a disabled device returns 404 from `/provision` |

### The `/provision/<mac>` endpoint (phone-facing)

This is the only endpoint a phone talks to, so it is **not** behind the bearer
token. Because the rendered config contains the cleartext SIP password, it is
guarded by **Basic auth (`PROVISION_USER`/`PROVISION_PASSWORD`) and/or a CIDR
allowlist (`PROVISION_ALLOW_CIDRS`)**. The filename's vendor convention selects
the format — `<mac>.cfg` (Yealink), `cfg<mac>.xml` (Grandstream), `<mac>.xml`
(generic):

```bash
# what the phone fetches at boot
curl -s -u "$PROVISION_USER:$PROVISION_PASSWORD" $API/provision/805ec0112233.cfg
# -> Yealink account.1.* config with account.1.password read from user 3001
```

`PROVISION_SIP_SERVER`/`PROVISION_SIP_PORT` set the registrar the phone is
pointed at (defaults to the device's domain and `5060`). Unknown/disabled MACs
and users without a password return **404**.

---

## XML endpoints (consumed by `mod_xml_curl`)

`mod_xml_curl` POSTs `application/x-www-form-urlencoded`. Behaviour:

| endpoint | returns | fallback |
|---|---|---|
| `/xml/directory` | the requested user if managed | `<result status="not found"/>` → FreeSWITCH static files |
| `/xml/dialplan` | the requested context if managed | not found → static files |
| `/xml/configuration` | `callcenter.conf`, `conference.conf`, `voicemail.conf` (the latter a `default` profile with `odbc-dsn` → freeswitch_core) by `key_value` | any other config → not found → static files |

**Authentication (required).** These endpoints expose SIP passwords and trunk
secrets, so they are protected:

- **HTTP Basic auth** when `XML_PASSWORD` is set (`XML_USER` defaults to
  `freeswitch`). `mod_xml_curl` sends it via
  `<param name="gateway-credentials" value="freeswitch:<secret>"/>`.
- **Optional IP allowlist** via `XML_ALLOW_CIDRS` (defense in depth; behind
  Docker/NAT verify the observed source IP first).

Without valid credentials the endpoints return `401`.

```bash
XU=freeswitch; XP=<XML_PASSWORD>
# what FreeSWITCH effectively does for a registration:
curl -s -u "$XU:$XP" $API/xml/directory --data 'user=2001&domain=192.168.48.143'
# a call routing lookup:
curl -s -u "$XU:$XP" $API/xml/dialplan  --data 'context=company'
# no creds -> 401
curl -s -o /dev/null -w '%{http_code}\n' $API/xml/directory --data 'user=2001&domain=192.168.48.143'
```

> **Rotating the XML secret:** update `XML_PASSWORD` (control-plane) **and**
> `gateway-credentials` in `xml_curl.conf.xml`, then on the FS server run
> `fs_cli -x "reload mod_xml_curl"` — a plain `reloadxml` does NOT re-read the
> binding credentials.
>
> Basic auth still sends the secret in clear over HTTP. On an untrusted network
> add TLS/mTLS in front (see `SECURITY_MODEL.md`).

**SIP passwords are NOT exposed in `/xml/directory`.** The directory renderer
emits `<param name="a1-hash" value="MD5(number:domain:password)"/>` instead of
the plaintext `password`, so even an authenticated reader of `/xml/directory`
never sees the SIP password. Verified end to end with a real REGISTER
(`hack/sip_register_test.py`): correct password → `200 OK`, wrong → `403`.
Remaining: the DB still stores the plaintext password (used to compute the
hash); `vm-password` (voicemail PIN) is still emitted as-is.

---

## Live events (SSE)

`GET /api/v1/events` is a **Server-Sent Events** stream (Bearer token) of live
telephony events. A single persistent ESL listener in the control-plane
subscribes to FreeSWITCH and fans events out to all connected clients via an
in-process hub (no polling, no external broker — single-instance; a shared
broker like Redis pub/sub would be needed only if the control-plane is scaled
to multiple replicas).

```bash
curl -N --cacert deploy/tls/ca.crt -H "Authorization: Bearer $TOKEN" \
     https://localhost:8080/api/v1/events
# event: call.started
# data: {"type":"call.started","ts":1781433467,"data":{"uuid":"...","direction":"inbound","caller":"4201","destination":"4444"}}
# event: call.answered
# event: call.ended  data:{... "cause":"NORMAL_CLEARING","billsec":"42"}
```

Event types: `call.started` / `call.answered` / `call.ended`, `agent.status`
(call-center agent), `queue.member` (queue join/leave/offer), `conference`
(member add/del). Each is `{type, ts, data{...}}`. A `: ping` heartbeat is sent
every 25 s. Returns 503 if the event stream is not enabled (ESL not
configured).

> Browser note: `EventSource` cannot send an `Authorization` header, so a
> browser client (see the wallboard below) reads this stream with `fetch()` +
> a streaming body reader instead.

## Supervisor wallboard

`GET /wallboard` serves a self-contained live dashboard (static HTML, embedded
in the binary, **no auth on the page itself**). The operator pastes the API
token once (kept in `sessionStorage`); the page then opens the SSE stream
same-origin and shows, in real time: active calls, queue-waiting count, agent
statuses, calls-since-open and a rolling event log. It is event-driven —
it reflects activity from the moment it is opened (seeding current in-progress
calls on load is a future enhancement). Open `https://<control-plane>/wallboard`.

## Call detail records (CDR)

FreeSWITCH `mod_json_cdr` POSTs each call's CDR (JSON) to **`POST /cdr`**, which
sits behind the same guard as `/xml/*` (HTTPS + mTLS client cert + Basic auth).
The control-plane parses it (uuid/times/cause from `variables`, caller &
destination from `callflow[].caller_profile`) and stores it in the `cdr` table
(plus the full payload in a `raw` JSONB column). Ingest is idempotent on the
channel uuid, so `mod_json_cdr`'s retry queue can't create duplicates.

Server config: `deploy/freeswitch/json_cdr.conf.xml` (url
`https://172.31.30.216:8080/cdr`, same client cert + creds as xml_curl; failed
posts spool to disk and retry — no CDR lost while the control-plane is down).

Read it back (Bearer token):

```bash
curl -s "$API/api/v1/cdr?number=4201&limit=50" "${H[@]}"      # filters: number, cause, from, to (epoch), answered=true
#   newest first, X-Total-Count header
curl -s "$API/api/v1/cdr/stats?from=&to=" "${H[@]}"            # per-day rollups
#   -> [{"day":"2026-06-14","total","answered","abandoned","talk_time","avg_billsec"}, ...]
```

CDRs are append-only (no update/delete API). Grafana panels in the NOC
dashboard chart calls/answered/abandoned per day and talk time (see
[observability](observability.md)).

## Pagination

All `GET` list endpoints accept optional `?limit=N&offset=M`. Without them the
full set is returned (unchanged behaviour). The response body stays a bare JSON
array; the unpaged total is in the `X-Total-Count` header. `limit` is capped at
1000; a negative or non-numeric `limit`/`offset` returns `400`.

```bash
curl -s -D- "$API/api/v1/users?domain=192.168.48.143&limit=50&offset=0" "${H[@]}"
#   X-Total-Count: 137
```

## Audit log (changelog)

Every create/update/delete is recorded (with `password`/`vm-password`
redacted). Read it back as a changelog — newest first, with filters:

```bash
curl -s "$API/api/v1/audit?limit=20" "${H[@]}"
curl -s "$API/api/v1/audit?resource_type=freeswitch_callcenter_queue" "${H[@]}"
curl -s "$API/api/v1/audit?actor=terraform&action=delete" "${H[@]}"
# filters: actor, action, resource_type, resource_id; + limit/offset, X-Total-Count
# -> [{"id","actor","action","resource_type","resource_id","before","after","created_at"}, ...]
```

## Runtime (ESL)

```bash
curl -s    $API/api/v1/runtime/health    "${H[@]}"   # {"esl":"ok"}
curl -s -X POST $API/api/v1/runtime/reloadxml "${H[@]}"   # {"status":"ok","message":"+OK [Success]"}

# Runtime status (ESL):
curl -s $API/api/v1/runtime/gateways/external/provider-main "${H[@]}"
#   -> {"name","profile","status","state","attributes":{...}}  (404 if not loaded in sofia)
curl -s $API/api/v1/runtime/registrations/192.168.48.143/2001 "${H[@]}"
#   -> {"user","domain","registered":true|false,"contact","agent","network_ip","network_port","expires"}
```

`reloadxml` flushes FreeSWITCH's XML cache. Directory/dialplan are normally
re-queried per event, so it is mostly a safety step after a batch of changes.

---

## How a change reaches FreeSWITCH

```
POST/PUT/DELETE /api/v1/...  →  PostgreSQL
        │
        └─ (optional) POST /api/v1/runtime/reloadxml  →  ESL  →  flush cache
                │
                ▼
FreeSWITCH, on the next registration / call, pulls fresh XML via mod_xml_curl
```

No file edits on the server are required for users or dialplan/IVR — only the
one-time setup in `deploy/freeswitch/` (see `docs/architecture.md`).
