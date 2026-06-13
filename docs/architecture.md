# Architecture & Topology

This document describes how the FreeSWITCH IaC Platform is deployed and how data
flows through it, with detailed diagrams.

The core idea: FreeSWITCH configuration (users, dialplan, gateways) lives in
**PostgreSQL**, not in on-disk XML files. A Go **control-plane** renders that
state into FreeSWITCH XML on demand. FreeSWITCH pulls it over HTTP via
`mod_xml_curl`. Runtime commands (e.g. `reloadxml`) are pushed back over ESL.

---

## 1. Deployment topology

```
        DEV BOX — WSL2 Debian (172.31.30.216)                 FREESWITCH SERVER (192.168.48.143)
 ┌──────────────────────────────────────────────┐      ┌────────────────────────────────────────┐
 │  Docker Compose                                │      │  FreeSWITCH 1.11.1  (host package)       │
 │                                                │      │                                          │
 │   ┌────────────────┐      ┌────────────────┐   │      │   ┌────────────────────────────────┐    │
 │   │  postgres:16   │◄─────│  control-plane │   │      │   │ mod_sofia        SIP  :5060/:5080│    │
 │   │  :5432         │ SQL  │  Go  :8080     │   │      │   │ mod_xml_curl     (HTTP client)   │    │
 │   │  freeswitch_   │ pgx  │                │   │      │   │ mod_event_socket ESL  :8021      │    │
 │   │  control DB    │      │                │   │      │   └────────────────────────────────┘    │
 │   └────────────────┘      └───────┬────────┘   │      │   /etc/freeswitch/autoload_configs/      │
 │                                   │            │      │     xml_curl.conf.xml                    │
 │                                   │            │      │     event_socket.conf.xml                │
 │                                   │            │      │     acl.conf.xml (+control-plane list)   │
 └───────────────────────────────────┼───────────┘      └──────────────────┬───────────────────────┘
                                      │                                     │
                                      │   ESL  (reloadxml / status)         │
                                      └──────────────────────────────────►  │ :8021
                                                                            │
                                          mod_xml_curl pulls XML            │
                                      ◄─────────────────────────────────────┘
                                       HTTP POST to 172.31.30.216:8080
```

- **PostgreSQL + control-plane** run in Docker Compose on the dev box.
- **FreeSWITCH** runs as a host package on a separate server (not a container).

---

## 2. Network paths

Independent directions, all verified reachable:

```
 ┌───────────────────────────┬──────────────────────────────┬───────────────────────────────┐
 │ Initiator                 │ Destination                  │ Purpose                       │
 ├───────────────────────────┼──────────────────────────────┼───────────────────────────────┤
 │ FreeSWITCH 192.168.48.143 │ → http://172.31.30.216:8080  │ mod_xml_curl pulls directory  │
 │ (mod_xml_curl)            │   /xml/directory, /xml/dialplan│   and dialplan XML          │
 ├───────────────────────────┼──────────────────────────────┼───────────────────────────────┤
 │ control-plane (container) │ → 192.168.48.143:8021        │ ESL: reloadxml, status        │
 ├───────────────────────────┼──────────────────────────────┼───────────────────────────────┤
 │ FreeSWITCH (unixODBC)     │ → 172.31.30.216:5432         │ runtime DBs in Postgres: core │
 │                           │   (compose Postgres)         │ db, sofia, callcenter, fifo/  │
 │                           │                              │ db/voicemail — no sqlite      │
 └───────────────────────────┴──────────────────────────────┴───────────────────────────────┘
```

**No sqlite on the server.** All FreeSWITCH runtime state lives in the platform
Postgres via ODBC (`/etc/odbc.ini`, mirrored in `deploy/freeswitch/odbc/`):
the core db (`core-db-dsn` in switch.conf.xml), all 4 sofia profiles and
mod_db/mod_fifo/mod_voicemail share db `freeswitch_core`; mod_callcenter uses
db `freeswitch_callcenter`. mod_verto is disabled (sqlite-only, unused — WebRTC
goes via sofia WSS + mod_rtc). Consequence: FreeSWITCH needs the dev-box
Postgres reachable **at boot** — if it was down, start compose first, then
`systemctl restart freeswitch`.

### SNAT gotcha

The dev box (`172.31.30.216/20`) and the FreeSWITCH server (`192.168.48.143/24`)
are on different subnets, joined by a router. Traffic from the dev box to the
server is **source-NATed by the router to `192.168.48.1`**.

```
 control-plane ──┐                         router (SNAT)                 FreeSWITCH ESL
 (src 172.31.30.216)─► 172.31.16.1 ─────►  rewrites src to 192.168.48.1 ─► sees 192.168.48.1:* ─► :8021
```

Therefore the ESL ACL (`acl.conf.xml`, list `control-plane`) must allow
**`192.168.48.1/32`**, not the dev-box IP. The reverse direction
(FreeSWITCH → control-plane :8080) is direct, no NAT, so `xml_curl.conf.xml`
uses `172.31.30.216` literally.

---

## 3. Two integration mechanisms

```
   STATE CHANGES (push — we write)              CONFIG READS (pull — FreeSWITCH asks)
   ───────────────────────────────              ─────────────────────────────────────

   curl / Terraform                              FreeSWITCH (mod_xml_curl)
        │ POST /api/v1/... (Bearer token)             │ POST /xml/... (on each event)
        ▼                                             ▼
   control-plane ──► PostgreSQL  ◄──── reads ──── control-plane renders XML
        │
        │ POST /api/v1/runtime/reloadxml
        ▼
   ESL ──► FreeSWITCH (flush XML cache)
```

- **`mod_xml_curl`** — FreeSWITCH intercepts a config lookup and fetches it over
  HTTP instead of reading a file. Pull-based, happens at event time
  (registration, call setup).
- **ESL (`mod_event_socket`)** — a TCP control channel (8021). The control-plane
  connects, authenticates, and issues runtime commands. It is **not** a config
  store, only live control.

---

## 4. Control-plane internals

```
                       HTTP (chi router)
                            │
   ┌────────────────────────┼─────────────────────────────┐
   │  middleware: Bearer-token auth (/api/v1), request log │
   └────────────────────────┼─────────────────────────────┘
                            │
        ┌───────────────────┴────────────────────┐
        │                                         │
   /api/v1/*  (management, token-protected)   /xml/*  (FreeSWITCH-facing)
        │                                         │
   handlers (validate) ── store (pgx/SQL) ──► PostgreSQL
        │                       ▲                 │
   audit (redacts pw)           └──── reads ───────┤
        │                                         ▼
   runtime/esl ──► FreeSWITCH :8021         renderer (model → XML)
```

Package layout (`control-plane/internal/`):

```
 api/        router, middleware, handlers (domains, users, gateways, dialplan, xml, runtime, health)
 store/      SQL CRUD (domains, users, gateways, dialplan + directory aggregate)
 renderer/   directory / dialplan / sofia-configuration XML (encoding/xml, deterministic)
 runtime/    ESL client (connect-per-command)
 audit/      writes audit_logs, redacts password / vm-password
 db/         pgx pool + embedded SQL migrations
 models/     domain types
 config/     env vars (DATABASE_URL, API_TOKEN, ESL_ADDR, ESL_PASSWORD)
```

Endpoints:

```
 /healthz                         liveness
 /readyz                          DB ping
 /api/v1/domains|users|gateways|dialplan/extensions   CRUD (Bearer)
 /api/v1/runtime/reloadxml|health ESL commands (Bearer)
 /xml/directory                   FreeSWITCH user lookup  (network-restricted)
 /xml/dialplan                    FreeSWITCH routing      (network-restricted)
 /xml/configuration               sofia gateways (renderer ready; binding off by default)
```

---

## 5. Data model → XML pipeline

```
 PostgreSQL (normalized)                         FreeSWITCH XML (rendered)
 ────────────────────────                        ─────────────────────────
 domains(name, variables jsonb)        ┐
 users(domain_id, number, params jsonb)├─► directory_renderer ─► <document><section name="directory">
 dialplan_extensions(name, context)    ┐                          <domain><user id=..><param password=../>
 dialplan_conditions(field, expr)      ├─► dialplan_renderer  ─► <document><section name="dialplan">
 dialplan_actions(application, data)    ┘                          <context><extension><condition><action>
 gateways(profile, name, proxy ...)     ─► configuration_renderer ─► <document>... sofia.conf ...
 audit_logs(actor, action, before/after)
```

Rendering is **deterministic** (map keys are sorted) and uses `encoding/xml`, so
attribute values are auto-escaped (no XML injection from user data).

---

## 6. Flow A — SIP registration (directory)

```
 Softphone 2001 ──REGISTER──► FreeSWITCH :5060
                                 │ needs user 2001@192.168.48.143
                                 │ (internal profile: force-register-domain = $${domain})
                                 ▼
                    mod_xml_curl ──POST /xml/directory──► control-plane
                       body: user=2001 & domain=192.168.48.143 & action=sip_auth
                                 │
                                 ▼
              handleXMLDirectory: store.GetUser("192.168.48.143","2001")
                 ├─ found     ─► render <domain><user id="2001"><param name="password" .../>
                 └─ not found ─► <result status="not found"/>   (FreeSWITCH falls back to files)
                                 │
                                 ▼
                    FreeSWITCH checks password ──► 200 OK / REGISTER accepted
```

**Safe-binding rule.** The directory binding intercepts *every* user lookup,
including the default 1000–1019. For users we don't manage we return
`<result status="not found"/>`, which makes FreeSWITCH fall back to its on-disk
directory. So default users keep working from files; only 2001/2002 come from
the DB. The same rule applies to dialplan contexts.

---

## 7. Flow B — call into the IVR (dialplan)

```
 2001 dials 5000
   │ user 2001 has variable user_context = "company"  →  call enters context "company"
   ▼
 mod_xml_curl ──POST /xml/dialplan (Hunt-Context=company)──► control-plane
   │  context "company" has extensions  →  return XML
   │  context default/public (or empty) →  <result status="not found"/> → fall back to files
   ▼
 FreeSWITCH executes extension "ivr-menu" (^5000$):

   answer
   sleep 500
   play_and_get_digits  "1 1 3 5000 # ivr-welcome_to_freeswitch.wav <invalid> ivr_choice \d 3000"
        │  plays greeting, collects 1 digit into ${ivr_choice}
        ▼
   transfer "menu_choice_${ivr_choice} XML company"     (re-enter dialplan)
        │
        ├─ digit 1 ─► extension ^menu_choice_1$ ─► transfer 2001 ─► bridge user/2001@192.168.48.143
        ├─ digit 2 ─► extension ^menu_choice_2$ ─► transfer 2002 ─► bridge user/2002@192.168.48.143
        └─ digit 3 ─► extension ^menu_choice_3$ ─► answer + echo (echo test)
```

The entire IVR is rows in `dialplan_extensions / dialplan_conditions /
dialplan_actions`. There is no IVR XML on the server — FreeSWITCH fetches it from
the control-plane each time it needs the `company` context.

---

## 8. Flow C — apply changes (reloadxml over ESL)

```
 curl ──POST /api/v1/runtime/reloadxml (Bearer)──► control-plane
                                                     │ runtime/esl.go
                                                     ▼
                                   TCP connect 192.168.48.143:8021
                                   recv "auth/request"
                                   send "auth ClueCon"
                                   recv "+OK accepted"
                                   send "api reloadxml"
                                   recv "+OK [Success]"
                                                     │
                                                     ▼
                                   FreeSWITCH flushes its XML cache
```

For directory/dialplan this is often unnecessary (mod_xml_curl re-queries per
event), but it forces a cache flush when needed.

---

## 9. Security boundaries

```
 /api/v1/*           ── Bearer token (API_TOKEN) ───────── only we/Terraform can write state
 /xml/*              ── HTTP Basic auth (XML_USER/XML_PASSWORD) + optional IP allowlist
                        + "not found" fallback ── secrets not readable without creds; can't break defaults
 ESL :8021           ── apply-inbound-acl="control-plane" ── allow loopback (local fs_cli) + 192.168.48.1
 audit_logs          ── password / vm-password redacted to "***"
 FreeSWITCH configs  ── backed up before changes (*.bak.<timestamp>)
```

`mod_xml_curl` authenticates to `/xml/*` via
`<param name="gateway-credentials" value="freeswitch:<secret>"/>`. Rotating the
secret requires `reload mod_xml_curl` on the FS server (not just `reloadxml`).

The directory sends `a1-hash` (MD5 of `number:domain:password`), never the
plaintext SIP password — verified with a real REGISTER (200 OK on correct
password, 403 on wrong; `hack/sip_register_test.py`).

Known MVP limitations: Basic auth is clear-text over HTTP (add TLS/mTLS on an
untrusted network); SIP passwords are still stored as plaintext in PostgreSQL
(used to compute the a1-hash) and `vm-password` is emitted as-is; the
`configuration` (sofia) binding is implemented but left disabled to avoid
disturbing the working on-disk sofia profiles. See
`freeswitch-iac-docs/SECURITY_MODEL.md` for the production hardening path.
