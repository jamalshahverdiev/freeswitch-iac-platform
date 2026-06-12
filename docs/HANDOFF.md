# HANDOFF / Agent context

Read this first when resuming work in a fresh context. It captures the full
state, how to operate the system, every credential/address, the gotchas that
already bit us, and the concrete next milestones. Pair it with the saved memory
files and with `docs/architecture.md` + `docs/api.md`.

Date of last update: 2026-06-11.

**WORKING TREE: `/home/jamal/github/freeswitch-iac-platform`** (git ->
github.com/jamalshahverdiev/freeswitch-iac-platform). The provider lives in
`/home/jamal/github/terraform-provider-freeswitch`. The old FS-TF tree is
deprecated. After a fresh clone: `hack/secrets.sh decrypt`, then
`docker compose up -d`. Env needed by scripts/examples: FS_SSH_PASS,
TF_VAR_sip_password, TF_VAR_webrtc_password (values in deploy/SECRETS.md).

---

## 1. What this project is

**FreeSWITCH IaC Platform** — manage FreeSWITCH config as data in PostgreSQL
instead of on-disk XML. A Go **control-plane** renders the DB into FreeSWITCH XML
on demand; FreeSWITCH pulls it via `mod_xml_curl`. Runtime commands (reloadxml,
status) are pushed over ESL. A Terraform provider (`provider/`, DONE) drives the API.

Goal taken from `../freeswitch-iac-docs/` (14 design docs). The MVP resources are
`freeswitch_domain`, `freeswitch_user`, `freeswitch_gateway`,
`freeswitch_dialplan_extension`.

---

## 2. Two machines & how to reach them

| Role | Address | Access |
|---|---|---|
| Dev box (control-plane + PostgreSQL, in Docker) | WSL2 Debian, `172.31.30.216` | local (here) |
| FreeSWITCH server (host package 1.11.1) | `192.168.48.143` | SSH root |

SSH to FreeSWITCH (password auth, use sshpass):

```bash
sshpass -p "$FS_SSH_PASS" ssh -o StrictHostKeyChecking=no root@192.168.48.143 '<cmd>'
sshpass -p "$FS_SSH_PASS" scp -o StrictHostKeyChecking=no <file> root@192.168.48.143:<path>
```

### Network topology (verified)

```
 control-plane (172.31.30.216) ──ESL──► 192.168.48.143:8021   (runtime cmds)
 FreeSWITCH (192.168.48.143) ──HTTP──► 172.31.30.216:8080      (mod_xml_curl pulls XML)
```

**SNAT gotcha:** dev box → FS server traffic is source-NATed by the router to
`192.168.48.1`. So the ESL ACL on the server allows `192.168.48.1`, NOT the WSL2
IP. The reverse (FS → dev box :8080) is direct, uses `172.31.30.216`.

---

## 3. All credentials / secrets

All live credentials moved to **`deploy/SECRETS.md`** — present in the repo
ONLY encrypted (`deploy/SECRETS.md.age`). After a fresh clone run
`hack/secrets.sh decrypt` (age key: `~/.config/age/keys.txt`, BACK IT UP).
The same script decrypts `.env`, the two FreeSWITCH configs that embed
passwords (event_socket / xml_curl), the recordings htpasswd and the TLS
bundle (`deploy/tls.tar.age` — contains our CA!).

Hack/deploy scripts read `FS_SSH_PASS` (server SSH) and Terraform examples
read `TF_VAR_webrtc_password` / `TF_VAR_sip_password` from the environment —
values in SECRETS.md.

Control-plane base URL is **`https://localhost:8080`**. Use
`--cacert deploy/tls/ca.crt`; `/xml/*` also needs `--cert deploy/tls/client.crt
--key deploy/tls/client.key`.

`.env` is gitignored. `.env.example` has placeholders.

---

## 4. Repo layout (`freeswitch-iac-platform/`)

```
control-plane/                Go 1.25 service (module github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane)
  cmd/api/main.go             wiring + graceful shutdown
  internal/config/            env config (incl. XML_*, ESL_*)
  internal/db/                pgx pool + embedded migrations (internal/db/migrations/*.sql)
  internal/models/            domain types
  internal/store/             SQL CRUD (domains, users, gateways, dialplan, directory aggregate)
  internal/renderer/          directory.go / dialplan.go / configuration.go (+ xml.go helpers)
  internal/runtime/esl.go     minimal ESL client (connect-per-command)
  internal/audit/             audit_logs writer, redacts password/vm-password
  internal/api/               server.go (router+mw+xmlGuard), errors.go, handlers: domains/users/gateways/dialplan/xml/runtime/health
  Dockerfile                  multi-stage, distroless
docker-compose.yml            postgres + control-plane (FreeSWITCH is external)
.env / .env.example
deploy/freeswitch/            server configs: xml_curl.conf.xml, event_socket.conf.xml, acl.conf.xml, modules.conf.xml (version-controlled copies)
deploy/seed.sh                demo: domain 192.168.48.143 + users 2001/2002 + IVR 5000
deploy/seed-ivr.sh            nested IVR: 7000 main + 7100 submenu
deploy/server-apply.sh        runs ON server: enable mod_xml_curl + merge ESL ACL
deploy/api-test.sh            full API regression (52 assertions, self-cleaning)
hack/sip_register_test.py     minimal SIP REGISTER+digest tester (proves a1-hash auth end-to-end)
hack/gen-tls.sh               generate dev CA + server/client certs into deploy/tls/
deploy/tls/                   generated TLS material (gitignored): ca/server/client {crt,key}
provider/                     Terraform provider (Plugin Framework) — built & e2e-verified
  internal/provider/          provider.go, client.go, *_resource.go (+ *_test.go), helpers.go
  examples/, GNUmakefile, README.md
hack/e2e/main.tf              OpenTofu e2e config (dev_overrides; throwaway tf-e2e.local)
examples/basic-pbx/           Terraform example (target state)
docs/architecture.md          topology + flows
docs/api.md                   full API reference + IVR pattern
docs/HANDOFF.md               this file
```

---

## 5. How to operate

### Bring up / rebuild control-plane (local)

```bash
cd freeswitch-iac-platform
bash hack/gen-tls.sh                   # once: generate deploy/tls/ (gitignored)
docker compose up -d --build          # rebuild after Go changes (HTTPS + mTLS)
until curl -s --cacert deploy/tls/ca.crt https://localhost:8080/readyz | grep -q '"database":"ok"'; do sleep 1; done
```

Data persists in the `postgres_data` volume across restarts. `docker compose
down -v` wipes it (then re-run seeds).

### Build / vet Go (without Docker)

Go is via gvm; call the binary by full path to avoid PATH issues:

```bash
GO=/home/jamal/.gvm/gos/go1.25.7/bin/go
$GO -C control-plane build ./...
$GO -C control-plane vet ./...
```

### Seed demo data & test

```bash
API=http://localhost:8080 TOKEN=dev-token ./deploy/seed.sh        # base demo
API=http://localhost:8080 TOKEN=dev-token ./deploy/seed-ivr.sh    # nested IVR 7000/7100
API=http://localhost:8080 TOKEN=dev-token bash deploy/api-test.sh # expect 52 passed, 0 failed
```

### Verify on FreeSWITCH

```bash
sshpass -p "$FS_SSH_PASS" ssh -o StrictHostKeyChecking=no root@192.168.48.143 '
  fs_cli -x "xml_locate dialplan context name company"        # shows our extensions
  fs_cli -x "originate loopback/7000/company &park()"          # exercises the IVR (plays prompt)
  fs_cli -x "module_exists mod_xml_curl"'                      # true
# log evidence:
grep "done playing file" /var/log/freeswitch/freeswitch.log
```

### What is installed on the FS server

- `mod_xml_curl` enabled in modules.conf.xml + loaded.
- `/etc/freeswitch/autoload_configs/`: our `xml_curl.conf.xml` (binds directory+dialplan
  to `http://172.31.30.216:8080`, with `gateway-credentials`), `event_socket.conf.xml`
  (listen 0.0.0.0:8021, acl `control-plane`), `acl.conf.xml` (merged `control-plane` list:
  127.0.0.1, ::1, 192.168.48.1). Backups: `*.bak.2026-05-31-173140`.
- `/etc/fs_cli.conf` written with the new ESL password.
- `$${domain}` = `192.168.48.143`; internal sofia profile context=`public`,
  `force-register-domain=$${domain}` → managed users live under domain
  `192.168.48.143` and use variable `user_context=company` to enter our dialplan.

---

## 6. GOTCHAS (these already cost time)

1. **`reload mod_xml_curl` ≠ `reloadxml`.** Changing `gateway-url` or
   `gateway-credentials` in xml_curl.conf.xml only takes effect after
   `fs_cli -x "reload mod_xml_curl"`. `reloadxml` alone keeps the old binding →
   FreeSWITCH gets 401 and silently falls back.
2. **Changing the ESL password:** edit event_socket.conf.xml, then reload with
   the OLD password: `fs_cli -p <oldpass> -x "reload mod_event_socket"`. The
   module restarts on the new password. Update `/etc/fs_cli.conf` so plain
   `fs_cli` keeps working, and `.env` ESL_PASSWORD + recreate control-plane.
3. **`reload mod_event_socket` is async** (returns a Job-UUID); the socket
   bounces for ~1s — an immediate `fs_cli` may print "Error Connecting". Wait.
4. **ESL ACL must allow loopback** (`127.0.0.1`, `::1`) or local `fs_cli` on the
   server gets locked out. Plus `192.168.48.1` (the SNAT source, see §2).
5. **`/xml/*` is form-encoded.** `mod_xml_curl` POSTs
   `application/x-www-form-urlencoded`; Go `ParseForm()` only reads the body with
   that content type. When testing with curl use `--data` (NOT JSON), and send
   Basic auth `-u freeswitch:<pass>`.
6. **Safe-binding fallback:** `/xml/directory` returns a specific user only if we
   manage it, else `<result status="not found"/>` so FreeSWITCH falls back to its
   static files (default users 1000-1019 keep working). `/xml/dialplan` returns
   `not found` for contexts we don't manage (default/public). DO NOT break this.
7. **Shell noise:** the zsh `_encode/_decode: command not found` lines are gvm
   init noise — harmless, ignore. Avoid `cd` in compound Bash (permission
   prompt); use `$GO -C <dir>` and absolute paths.

---

## 7. Current status (DONE & verified)

- Control-plane: full CRUD for domains/users/gateways/dialplan, XML renderers,
  ESL client, audit, migrations. `go build`/`vet` clean. `api-test.sh` = 52/52.
- FreeSWITCH integration live: pulls directory + dialplan from control-plane.
- Live nested IVR from DB: 7000 main (1→submenu 7100, 2→call 2001, 3→echo),
  7100 submenu (1→call 2002, 2→echo, 0→back). Both menus play prompts.
- Security done (Milestone A): `/api/v1/*` Bearer over TLS; `/xml/*` = HTTPS +
  mTLS (client cert) + Basic auth (+ optional XML_ALLOW_CIDRS) + not-found
  fallback; directory sends `a1-hash` not plaintext password (REGISTER 200/403
  verified); ESL non-default password + ACL; audit redacts passwords.
- `deploy/api-test.sh` = 53/53 over HTTPS+mTLS.
- Terraform provider (Milestone B) DONE: `provider/` builds, e2e verified with
  OpenTofu (apply/plan-noop/import/destroy). See §9.
- IVR/TTS DONE: `mod_flite` enabled; IVR audio via Terraform either runtime TTS
  (`speak` `flite|<voice>|<text>`) or pre-recorded `.wav` (`playback`). Live:
  `examples/universal-ivr/` = user 4100 (password: SECRETS.md) + nested IVR 3333 (voice `rms`).
  Docs `docs/ivr-audio.md`, `docs/howto-custom-prompt.md`.
- **Piper (neural TTS) DONE:** `/opt/piper/` + voice `en_US-ryan-medium`, wired
  via `mod_tts_commandline` (module quotes ${text} itself — no extra quotes in
  the template). `examples/universal-ivr` speaks via
  `tts_commandline|en_US-ryan-medium|<text>`; flite stays as fallback.
- **WebRTC DONE:** accounts = regular users; WSS :7443 was already bound, now
  serves OUR CA-signed `wss.pem` (`hack/gen-wss-cert.sh`). Users 4201/4202 via
  `examples/webrtc-users/`. Browser how-to: `docs/webrtc.md` (trust
  deploy/tls/ca.crt, tryit.jssip.net, wss://192.168.48.143:7443).
  Browser call to 3333 verified end-to-end over WSS: 407→digest→200 OK, IVR
  spoke via Piper. Fixes that were needed: profile `enable-timer=false`
  (JsSIP sends Session-Expires: 90; sofia rejected with 422 even with
  minimum-session-expires=90 — internal.xml now version-controlled in
  `deploy/freeswitch/sip_profiles/`). Open question: user's DTMF didn't arrive
  in tryit (must use the IN-CALL keypad, not the dialer input) — re-test with
  our own webphone (below).

## Self-hosted WebRTC webphone (Browser-Phone) + video — DONE ✅ (2026-06-04)

Implemented per the plan below: `webphone/` (Dockerfile nginx:1.27-alpine +
Browser-Phone fetched at build via PHONE_REF arg; nginx.conf TLS from /tls) +
compose service `webphone` (8443:443, mounts deploy/tls) — serves
**https://172.31.30.216:8443** (HTTP 200 with our CA; page = phone.js +
sip-0.20.0.min.js). Terraform `examples/video-echo/` adds **9196**
(answer+echo = audio+VIDEO mirror) in context company — verified served &
executing. User doc: `docs/webphone.md` (account settings table + test plan).
**E2E verified by the user (2026-06-04):** Browser-Phone registered from
Firefox (4201) + Chrome (4202); calls 4201↔4202 bridged both ways; **video
VP8 negotiated & flowing**; **DTMF via in-call keypad works** (4201 navigated
IVR 3333 → main_1 → SRE submenu). Added missing route `internal-4xxx`
(`^(4[0-9]{3})$` → bridge user/$1) in `examples/webrtc-users/` — 4xxx numbers
were NO_ROUTE_DESTINATION before.

Also: **server reboot test PASSED** (2026-06-04) — after a cold boot everything
came back by itself: freeswitch active, static IP, mod_xml_curl/flite/
tts_commandline/spandsp autoloaded, WSS 7443 with our cert, ESL password,
dialplan pulled from control-plane (34 extensions). Note: if the FS server is
down, provider applies still write to the DB but `freeswitch_reloadxml` fails —
re-run `tofu apply` after the server is back (it completes the reload).

(original plan kept below)

Goal: our own local WebRTC UI to test audio + VIDEO against FreeSWITCH,
replacing tryit.jssip.net.

Chosen client: **Browser-Phone** (github InnovateAsterisk/Browser-Phone) —
static SPA on SIP.js v0.20 (MIT) + jQuery; audio/video calls, in-call DTMF
keypad, transfer/hold/conference, call recording, screen share. License:
**AGPL-3.0** — fine to modify for internal/lab use; if we ever build a closed
commercial UI, write our own on SIP.js (MIT) instead.

Layout (per user request: separate top-level folder + separate compose service,
same level as control-plane/ and provider/):

```
webphone/
├── Dockerfile      # nginx:alpine + Browser-Phone static files (pin a commit/release;
│                   # download at build time via git/curl — don't vendor the whole tree)
├── nginx.conf      # HTTPS :443 inside, serve static; certs mounted from /tls
└── README.md       # what it is, how to open, license note (AGPL)
```

docker-compose.yml — new service:
```yaml
webphone:
  build: ./webphone
  ports: ["8443:443"]
  volumes: ["./deploy/tls:/tls:ro"]   # reuse server.crt/server.key (SAN: 172.31.30.216, localhost)
  restart: unless-stopped
```
→ open https://172.31.30.216:8443 from Windows (CA already trusted).

Steps:
1. Create `webphone/` (Dockerfile multi-stage: fetch Browser-Phone at pinned
   ref → copy `Phone/` static into nginx html; nginx.conf with ssl_certificate
   /tls/server.crt + /tls/server.key).
2. Add the compose service; `docker compose up -d --build webphone`.
3. Terraform: video-echo extension **9196** (`answer` + `echo`) in context
   `company` (echo mirrors audio AND video) — add to examples (new
   `examples/video-echo/` or extend universal-ivr).
4. Configure Browser-Phone in UI: WSS server `192.168.48.143`, port `7443`,
   path empty, user `4201` (password: SECRETS.md) (settings persist in localStorage).
5. Tests: register → call `9196` (see/hear yourself = video path proven) →
   video call `4201 ↔ 4202` (two tabs/devices) → IVR `3333` with the in-call
   keypad (closes the pending DTMF question).
6. Doc `docs/webphone.md` + update webrtc.md/HANDOFF/memory.

Wideband audio status (checked 2026-06-03): **G722 is ALREADY enabled
everywhere** — vars.xml `global_codec_prefs=OPUS,G722,PCMU,PCMA,...`, profiles
inherit via `$${global_codec_prefs}`, codec provided by loaded `mod_spandsp`
(listed as `G.722` in `show codecs` — mind the dot when grepping). WebRTC legs
already negotiate **Opus 48k** (better than G722). Remaining for real wideband:
1) enable G722 in Zoiper (Settings → Audio Codecs, above PCMU/PCMA);
2) ship 16 kHz versions of custom prompts into `.../ivr/16000/` (same filename;
   `sox ... -r 16000 -c 1 -b 16`); Piper TTS benefits automatically (22.05k source).
Mixed-codec bridges transcode — the worse leg caps quality.

mod_signalwire spam SILENCED (2026-06-04): unloaded + commented out in managed
modules.conf.xml (deployed).

## FUTURE PLAN — gateways from DB (deferred until a SIP provider exists)

User has no external SIP provider yet; revisit when external calls are needed.
Recommended **Option A (include + rescan)**: the external profile already does
`<X-PRE-PROCESS cmd="include" data="external/*.xml"/>` — render each DB gateway
to `/etc/freeswitch/sip_profiles/external/<name>.xml` + `sofia profile external
rescan`. Needs a delivery channel (control-plane can't write the FS disk):
small pull-agent (systemd timer fetching rendered XML from a control-plane
endpoint) or CI scp push. Do NOT enable the xml_curl configuration binding for
sofia.conf with the partial renderer (would wipe profiles).

## MILESTONE — mod_callcenter via control-plane + Terraform — DONE ✅ (2026-06-04)

All 8 steps below implemented and verified END TO END, including the live
user test: Browser-Phone 4202 dialed **4444** → answered → MOH → agent 4201
rang, answered, talked 63 s → agent back to Waiting. Runtime stats
(calls_answered/no_answer_count/talk time) confirmed in OUR Postgres
`freeswitch_callcenter` DB; **no sqlite file on the server** (user hard
requirement).

What exists now:
- Migration `000002_callcenter` (cc_queues/cc_agents/cc_tiers), store, renderer
  (`RenderCallcenter` emits `odbc-dsn` param), API CRUD
  `/api/v1/callcenter/{queues,agents,tiers}`, runtime endpoints
  `/api/v1/runtime/callcenter/{reload,agents/{name}/status,queues/{name}/{agents|members|tiers}}`.
- xml.go: `key_value=callcenter.conf` served from DB; **everything else
  (incl. sofia.conf) → not-found → disk fallback**. The `configuration`
  binding is ENABLED in deployed xml_curl.conf.xml.
- Server: mod_callcenter loaded; unixodbc + odbc-postgresql installed; DSN
  `freeswitch` → 172.31.30.216:5432/freeswitch_callcenter (configs mirrored in
  `deploy/freeswitch/odbc/`). Extra DBs created by
  `deploy/postgres-init/01-extra-databases.sql` (freeswitch_callcenter +
  freeswitch_core for the next milestone).
- Provider resources: `freeswitch_callcenter_queue`, `_agent`, `_tier`,
  `_reload` (docs generated). Worked example: `examples/callcenter/` —
  queue support@192.168.48.143, agents 4201/4202, tiers, dialplan 4444.
- api-test.sh: 71 asserts incl. callcenter CRUD + xml configuration tests.

GOTCHAS learned:
- **Dialplan priority**: 4444 was first swallowed by `internal-4xxx`
  `^(4[0-9]{3})$` (priority 11) → SUBSCRIBER_ABSENT. Queue entry
  `cc-support-entry` must have priority < 11 (now 10).
- `reloadxml` does NOT apply queue/agent/tier changes — need
  `reload mod_callcenter` (the `freeswitch_callcenter_reload` resource /
  `POST /api/v1/runtime/callcenter/reload`).
- Stale runtime rows (e.g. an agent removed from config) stay in the Postgres
  runtime tables; clean via `callcenter_config tier del` + `agent del`.
- Agent no-answer puts the agent to sleep `no-answer-delay-time` (default 60 s)
  before the queue retries them.

Original plan (kept for reference):
1. **Server**: enable mod_callcenter in managed modules.conf.xml + load.
2. **Control-plane**: migration 000002 — tables `cc_queues` (name, strategy,
   moh-sound, time-base-score, max-wait-time, max-wait-time-with-no-agent,
   tier-rules-apply, discard-abandoned-after, params JSONB), `cc_agents` (name,
   contact, type=callback, status, max-no-answer, wrap-up-time, reject-delay,
   params JSONB), `cc_tiers` (queue_name, agent_name, level, position).
   API CRUD `/api/v1/callcenter/{queues,agents,tiers}`. Renderer for
   `callcenter.conf` (sections agents/tiers/queues).
3. **XML handler**: serve `key_value=callcenter.conf` from DB; **change
   sofia.conf to return not-found** (we never enabled it on the server; keep
   gateways out until Option A above). Then ENABLE the `configuration` binding
   in xml_curl.conf.xml — safe now: only callcenter.conf is served, everything
   else falls back to disk. `reload mod_xml_curl` after.
4. **Runtime (ESL)**: endpoints wrapping `callcenter_config`: agent set status,
   queue list agents/members/count, `callcenter_config queue reload <q>` (apply
   queue changes without FS restart; agents/tiers changes may need
   agent/tier reload commands too).
5. **Provider**: resources `freeswitch_callcenter_queue`, `_agent`, `_tier`
   (+ update reloadxml-style apply: a `freeswitch_callcenter_reload` or reuse
   runtime endpoint); data sources for queue/agent runtime status.
6. **Demo (Terraform)**: queue `support@192.168.48.143` (MOH default), agents
   2001/2002 (or browser 4201/4202), tiers; dialplan ext `4444` →
   `callcenter support@192.168.48.143`; optionally IVR 3333 SRE option →
   transfer to 4444. Test from webphone: 4201 calls 4444 → MOH → agent rings.
7. **NO SQLITE (user requirement)** — point mod_callcenter's runtime state at
   OUR PostgreSQL via ODBC instead of its default sqlite:
   - FS server: `apt-get install -y unixodbc odbc-postgresql`; configure
     `/etc/odbcinst.ini` (PostgreSQL driver) + `/etc/odbc.ini` DSN `freeswitch`
     → host `172.31.30.216`, port `5432`, db `freeswitch_callcenter`
     (dedicated DB in our Postgres instance — keep runtime tables `agents`/
     `tiers`/`members` isolated from the control-plane desired-state schema),
     user/pass `freeswitch`/`freeswitch`.
   - Create the extra DB in the compose Postgres (init script or one-off
     `CREATE DATABASE freeswitch_callcenter;`).
   - callcenter.conf renderer emits
     `<param name="odbc-dsn" value="freeswitch:freeswitch:freeswitch"/>`
     (format dsn:user:pass) in `<settings>`.
   - Verify FS→Postgres reachability (5432 published on the dev box; the
     FS server reaches the dev box already for xml_curl).
   - Manage these server-side ODBC config files as version-controlled copies in
     `deploy/freeswitch/odbc/` like the rest.
   - BONUS: control-plane status data sources can then read live queue/agent
     state straight from Postgres (no ESL parsing).
8. (done — see next milestone below)

## MILESTONE — FreeSWITCH core-db → Postgres — DONE ✅ (2026-06-04)

**ZERO sqlite files on the server now** (`/var/lib/freeswitch/db/` empty; the
old files are parked in `/var/lib/freeswitch/db/sqlite-backup/`, deletable).
Everything verified after a full server reboot: freeswitch active, 4 sofia
profiles RUNNING, WSS 7443 up, dialplan from control-plane (36 ext), queue
call works, api-test.sh 71/71.

What was done (all configs version-controlled in `deploy/freeswitch/`):
- `/etc/odbc.ini`: second DSN `[freeswitch-core]` → 172.31.30.216:5432
  db `freeswitch_core` (deploy/freeswitch/odbc/odbc.ini).
- `switch.conf.xml`: `core-db-dsn = freeswitch-core:freeswitch:freeswitch`
  (plain `dsn:user:pass` = ODBC) → core tables (channels/calls/registrations/
  tasks/interfaces/...) in Postgres.
- All 4 sofia profiles (internal, external, internal-ipv6, external-ipv6 in
  deploy/freeswitch/sip_profiles/): `odbc-dsn` param → same DSN → sip_*
  tables (sip_registrations, sip_dialogs, ...) in the same freeswitch_core DB
  (rows distinguished by profile_name — standard FS practice).
- `db.conf.xml` (mod_db → db_data/limit_data), `fifo.conf.xml` (mod_fifo →
  fifo_*), `voicemail.conf.xml` (→ voicemail_msgs/prefs): `odbc-dsn` → same DSN.
- **mod_verto DISABLED** in modules.conf.xml: it was the owner of `json.db`
  (table json_store), opens sqlite directly with NO odbc support, and is
  unused — our WebRTC is SIP-over-WSS via sofia + mod_rtc, not verto.

GOTCHAS:
- First boot after enabling a module's odbc-dsn logs a burst of
  `switch_odbc.c:529 ERR` (alter/select/drop on a missing table) — that is
  the normal auto-create-schema probe; tables get created right after and
  the errors never repeat. Don't panic-grep.
- BOOT DEPENDENCY (accepted): FreeSWITCH now needs the dev-box compose
  Postgres reachable at startup. If Postgres is down when FS boots, core db
  init fails — bring compose up, then `systemctl restart freeswitch`.
- Verifying "is it really in Postgres": `freeswitch_core` tables `channels`
  (live calls), `sip_registrations`; `freeswitch_callcenter` tables
  `agents`/`members`/`tiers` update live during queue calls.

---

## BACKUP & RECOVERY (Phase 1, done 2026-06-12)

- `hack/backup-postgres.sh` — pg_dump -Fc of freeswitch_control/_callcenter/
  _core from the compose container + pull of /var/lib/freeswitch/recordings
  from the FS host (ssh KEY auth — id_ed25519 installed on the server, no
  password needed). Target: ~/backups/freeswitch/<YYYY-MM-DD>/, retention 14d.
- Schedule template: `deploy/cron/backup.crontab` (NOT auto-installed; WSL2
  caveat inside). Install manually via `crontab -e`.
- `hack/restore-test.sh [date]` — restores the dumps into a scratch postgres:16
  container and compares 11 row counts against live. Verified 2026-06-12:
  10/10 (first run) and scripted version. Run it after any backup change.

RECOVERY RUNBOOK (postgres volume lost):
1. `docker compose up -d postgres` (fresh volume; init creates the 3 DBs and
   the control-plane will run migrations — but restore BEFORE starting it).
2. For each db: `docker exec -i <pg> pg_restore -U freeswitch -d <db>
   --clean --no-owner < ~/backups/freeswitch/<day>/<db>.dump`.
3. `docker compose up -d` (control-plane, webphone).
4. FS server: `systemctl restart freeswitch` (re-creates ODBC connections),
   then `fs_cli -x "reload mod_callcenter"`.
5. Verify: `bash deploy/api-test.sh` (89), browser re-registers, queue list.
Recordings restore: copy `<day>/recordings/` back to the FS host
`/var/lib/freeswitch/recordings/` (chown freeswitch:freeswitch).

## IMPROVEMENT PLAN (2026-06-11) — READ docs/IMPROVEMENT-PLAN.md FIRST

Next session starts with Phase 0 of docs/IMPROVEMENT-PLAN.md: split into two
git repos — `freeswitch-iac-platform` (this tree minus provider/) and
`terraform-provider-freeswitch` (provider/ at repo root, module renamed).
Then backups (Phase 1), FS boot resilience (Phase 2). D1/C1 follow.

## ROADMAP — next milestones (agreed with user 2026-06-04, order: V1 → R1 → D1 → C1)

Server capabilities verified: mod_conference LOADED, mod_json_cdr.so present,
`$${recordings_dir}` = /var/lib/freeswitch/recordings (dir exists).

### V1 — Video conferences via Terraform (mod_conference) — DONE ✅ (2026-06-04, USER-VERIFIED: 2 Browser-Phone tabs in 3500 saw the grid, live vid-layout switch 1x1↔grid observed, 3501 PIN 2580 worked)

Built exactly per the plan below. What exists:
- Migration 000004? no — `000003_conference` (conference_profiles, conference_rooms
  with UNIQUE(context,number), profile FK ON DELETE RESTRICT → 409 on delete-in-use).
- API CRUD `/api/v1/conference/{profiles,rooms}`; runtime
  `GET /runtime/conference/{name}` (parsed xml_list), `POST .../{kick|mute|unmute}`
  {"member":"id|all"}, `PUT .../layout` {"layout":"..."}.
- Renderer `RenderConference` (profiles only; no caller-controls — mod_conference
  installs its built-in default group). Rooms become SYNTHETIC dialplan
  extensions (renderer.ConferenceRoomExtension) merged+sorted into
  /xml/dialplan by (context, priority, name) — room priority default 5 beats
  internal-4xxx(11)/cc-entry(10). Conference dialstring: `name@profile+pin`.
- xml.go: `key_value=conference.conf` served when ≥1 profile exists, else
  not-found → disk fallback.
- Provider: `freeswitch_conference_profile`, `freeswitch_conference_room`,
  data source `freeswitch_conference_status` (running, member_count, members
  list with has_video/talking). Docs generated.
- FULL data-source parity (2026-06-04, user asked twice — keep this honest):
  12 data sources = 12 docs, all with examples. Added config DS
  `freeswitch_callcenter_queue/_agent/_tier`, `freeswitch_conference_profile/
  _room` (cross-state references); all 6 new DS verified live via
  hack/dstest2 (tofu outputs). provider/examples/ now covers ALL 11 resources
  (resource.tf + import.sh for the 9 importable; reloadxml/callcenter_reload
  are apply-triggers, no import) and ALL 12 data sources. Deliberately NO
  runtime DS for callcenter lists (ESL output is raw pipe-text; live state
  is in our Postgres — observability goes through D1/Grafana, not TF plans).
- `examples/conference/`: profile video-grid (mux), rooms standup/3500 and
  private/3501 (PIN 2580) + reloadxml. APPLIED and verified server-side:
  loopback call into 3500 → `conference(standup@video-grid)` executed, member
  visible via runtime API, rate=48000 proves OUR profile (disk default = 8000).
- api-test.sh = 85 asserts (conference CRUD + 409 profile-in-use + xml renders).
- User test PASSED: grid with both videos in 3500 (has_video=true for both
  via runtime API), live layout switch via PUT .../layout worked mid-call,
  3501 PIN prompt accepted 2580# (dialstring `name@profile+pin` confirmed).

Original plan (kept for reference): FS 1.11 does real video mux (grid layouts,
speaker focus). Same pattern as callcenter — configuration binding + ESL
runtime + provider resources.

1. Control-plane: migration 000003 — `conference_profiles` (name UNIQUE,
   canonical params: rate, interval, energy-level, comfort-noise, moh_sound,
   video params: video-mode=mux, video-layout-name (e.g. group:grid),
   video-canvas-size, video-fps, params JSONB) and `conference_rooms` (name
   UNIQUE, number, domain, context, profile FK, pin/moderator_pin nullable,
   max_members, record bool, flags). CRUD `/api/v1/conference/{profiles,rooms}`.
2. Renderer: `conference.conf` — `<profiles>` from DB. Room itself = dialplan:
   the room resource ALSO materializes a dialplan extension
   `number → answer + conference($room@$profile)` (+ `pin` via conference
   syntax `room+pin`). Decide: render that ext server-side into /xml/dialplan
   from conference_rooms (preferred — one resource, no drift) vs. composing
   freeswitch_dialplan_extension in TF.
3. xml.go: serve `key_value=conference.conf` (extend the switch; keep
   not-found fallback for the rest).
4. Runtime ESL endpoints: `conference <room> list|kick|mute|unmute|vid-layout`,
   GET participants (parse `conference xml_list`).
5. Provider: `freeswitch_conference_profile`, `freeswitch_conference_room`;
   data source `freeswitch_conference_participants`. Reload: conference.conf
   is read per-room-creation → usually NO module reload needed (verify; else
   `reload mod_conference`).
6. Example `examples/conference/`: profile video-grid (group:grid, 1280x720),
   room 3500 (+3501 with PIN). Test: 2 Browser-Phone tabs + dial 3500 with
   video → both see the grid. api-test.sh + docs/api.md section.

### R1 — Call recording, per-day folders — DONE ✅ (2026-06-04; phases 1+2)

Verified live, both sources land in the required per-day tree:
- recordings/2026/06/04/queue_18-11-44_<uuid>.wav (record_session before
  callcenter in examples/callcenter; RECORD_STEREO=true; FS auto-creates dirs)
- recordings/2026/06/04/conf_standup_18-12-36.wav (auto_record on the
  video-grid profile in examples/conference; .mp4 template would record the
  composed VIDEO canvas — documented in api.md)
- HCL gotcha: write `$${strftime(%Y/%m/%d)}` in .tf so FS receives
  `${strftime(...)}` (runtime expansion).
- Exposure: nginx-light on FS host :8088 (root /var/lib/freeswitch/recordings,
  autoindex json, Basic auth (creds: SECRETS.md), www-data added to
  freeswitch group; config + htpasswd in deploy/freeswitch/nginx/). Control-plane
  proxies: GET /api/v1/recordings[?date=YYYY-MM-DD] (list; empty list when no
  dir) + GET /api/v1/recordings/{date}/{file} (stream, name regex-validated).
  Env REC_URL/REC_USER/REC_PASSWORD (compose + .env). api-test.sh = 89.
- Phase 3 (deferred to A1): Whisper transcripts → Postgres search.
- RECORDING SOURCE switched to mod_callcenter's native `record-template`
  queue param (set via the queue resource `params` map in examples/callcenter)
  — starts when the AGENT ANSWERS, so the file is the conversation without
  MOH. Verified autonomously with a sipp bot agent (see below). The earlier
  record_session-on-caller-leg approach also worked (a real user call was
  transcribed by Whisper), but captures MOH and continues across queue phases.
- FALSE ALARM analysis (2026-06-04 evening): a user call recording looked
  "broken" — 50 s of pure digital zeros mid-file. It was NOT a bug: bridge
  happened at +4 s, speech 4-20 s recorded, then both parties were silent
  (browser noise suppression emits digital silence over Opus), goodbye at
  70-76 s ("Поехали!" transcribed by Whisper). Don't re-debug this.
- GOTCHA: reload-trigger resources must key triggers on `updated_at`, NOT
  `id` (ids = names; content-only changes like adding record-template never
  fire the reload otherwise). examples/callcenter fixed accordingly.
- GOTCHA: queue rings the CALLER's own agent if the caller is an agent of
  that queue (longest-idle picked 4202 when 4202 dialed 4444) — looks
  "broken" in demos with only browser users.
- AUTONOMOUS TESTING RECIPE (no human agents needed): mod_callcenter agents
  as `loopback/...` contacts DO NOT work (origination cancelled ~6 s even
  with loopback_bowout=false). Working bot: `sipp -sn uas -p 5070 -rtp_echo
  -bg` on the FS host + agent contact `sofia/internal/bot@127.0.0.1:5070`;
  put real agents On Break via runtime API; member = `originate
  loopback/4444/company &playback(local_stream://moh)` (MOH as the member's
  "voice"). Clean up: API deletes + `callcenter_config tier/agent del` for
  runtime rows + restore statuses. Script: hack/rec_queue_test.sh (loopback
  agent variant — needs the sipp approach merged if reused).

Original plan (kept for reference):

Recordings MUST land in nested per-day dirs: `YYYY/MM/DD/<file>.wav`,
e.g. `2026/05/04/call_record.wav` under $${recordings_dir}:
`/var/lib/freeswitch/recordings/${strftime(%Y/%m/%d)}/${uuid}_${strftime(%H-%M-%S)}.wav`
(record_session/switch creates dirs recursively — verify on first test).

1. Dialplan building block (Terraform, no new resource needed):
   `action { application=record_session, data=<dated path template> }` before
   `callcenter`/`bridge`. Add `record = true` convenience to examples
   (callcenter queue entry first: record all queue calls).
2. Also wire conference room `record` flag (V1) to
   `conference_set_auto_outcall`-style auto-record into the same dated tree.
3. Exposure (phase 2): recordings live on the FS server disk; control-plane is
   on the dev box. Add `GET /api/v1/recordings?date=YYYY-MM-DD` (list) +
   download. Delivery options (pick during impl): nginx file-server on the FS
   server (read-only, basic auth, config version-controlled in deploy/) that
   control-plane proxies; or a tiny pull-agent. NOT raw open dir.
4. Phase 3 (bridge to AI agent A1): Whisper on dev box transcribes new files
   (watcher) → transcripts table in Postgres → search API.

### D1 — Live NOC dashboard (Grafana, near-free after no-sqlite)

All runtime state is already in OUR Postgres — just visualize it.

1. compose service `grafana` (provisioned, admin pass via env) + datasources
   freeswitch_core / freeswitch_callcenter / freeswitch_control (read-only DB
   user!) + dashboards as JSON in `deploy/grafana/` (version-controlled).
2. Panels: live channels (core.channels), who is registered
   (core.sip_registrations), queue members waiting + serving agent
   (callcenter.members), agent states/calls_answered/no_answer
   (callcenter.agents), desired-state counts (control.users/extensions).
3. Refresh 5s. Later: CDR panels from C1 (calls/day, talk time, abandoned).

### C1 — CDR → Postgres through control-plane (mod_json_cdr)

Pattern-consistent with xml_curl: FS POSTs JSON CDR to control-plane over
HTTPS+mTLS+Basic — control-plane owns the schema (NO direct DB access from FS,
NO mod_cdr_sqlite).

1. Server: load mod_json_cdr (modules.conf), `json_cdr.conf.xml` → url
   `https://172.31.30.216:8080/cdr`, same client cert + gateway credentials
   as xml_curl, encode-values, retry queue on failure.
2. Control-plane: `POST /cdr` (mTLS+Basic guarded like /xml), migration —
   `cdr` table (uuid PK, caller/callee, direction, context, start/answer/end
   epochs, duration, billsec, hangup_cause, recording_path nullable, raw
   JSONB). Parse the json_cdr payload.
3. API: `GET /api/v1/cdr?from=&to=&number=&cause=` (paged) + simple stats
   endpoint (calls/answered/abandoned per day, queue SLA join with
   callcenter history).
4. Grafana panels on top (extends D1). Data source `freeswitch_cdr` ... or
   keep the table in freeswitch_control (control-plane's own DB) — decide at
   migration time (leaning: control DB, it's control-plane-owned data).

### A1 (deferred) — AI voice agent in the queue

When agents are busy/absent: bot answers (mod_audio_fork or mod_vosk STT +
existing Piper TTS + Claude API), collects the question, writes transcript to
Postgres, transfers to an agent with context. Build after R1 (recording
pipeline) and C1 (call metadata) exist.

---

## 8. MILESTONE A — security layer (a1-hash + TLS/mTLS) — DONE ✅

Both A1 (a1-hash) and A2 (TLS/mTLS) are complete and verified (details below).
**The next thing to build is MILESTONE B — the Terraform provider (§9).**

### A1. Send `a1-hash` instead of plaintext `password` in directory — DONE ✅

Implemented in `renderer/directory.go` (`directoryUserParams` / `a1Hash`):
when a user has a `password` param, the directory emits
`<param name="a1-hash" value="MD5(number:domain:password)"/>` and drops the
plaintext `password`. `vm-password` still passes through (voicemail PIN).

Verified end to end with `hack/sip_register_test.py`: the FreeSWITCH challenge
realm is the domain (`192.168.48.143`), correct password → `200 OK`, wrong
password → `403 Forbidden`. Plaintext password no longer appears in
`/xml/directory`.

Remaining (post-MVP, optional): store only the a1-hash in the DB (never
plaintext); `secret_ref` + Vault (see SECURITY_MODEL.md).

### A2. TLS / mTLS in front of the control-plane — DONE ✅

- Control-plane serves HTTPS when `TLS_CERT_FILE`+`TLS_KEY_FILE` are set
  (`config.go`, `main.go`). When `XML_CLIENT_CA_FILE` is also set, `/xml/*`
  requires a verified client cert (mTLS): `tls.Config{ClientAuth:
  VerifyClientCertIfGiven, ClientCAs: caPool}` + `xmlGuard` rejects requests with
  no verified peer cert. `/api/v1` works over TLS with just the bearer token (no
  client cert). All config-driven: leave the vars blank → plain HTTP.
- Certs: `hack/gen-tls.sh` makes a CA, a server cert (SAN
  `IP:172.31.30.216,IP:127.0.0.1,DNS:localhost`) and a client cert, into
  `deploy/tls/` (gitignored, `make tls`). Mounted into the container at `/tls`
  (compose). Cert files must be world-readable (644) — the distroless container
  runs as non-root uid 65532 and can't read 600 keys (this bit us).
- FreeSWITCH side (`deploy/freeswitch/xml_curl.conf.xml`): `gateway-url` is
  `https://172.31.30.216:8080/...` with `enable-cacert-check`, `ssl-cacert-file`,
  `ssl-cert-path`, `ssl-key-path`, `enable-ssl-verifyhost` + `gateway-credentials`
  (Basic auth still on top). TLS material copied to
  `/etc/freeswitch/tls/{ca.crt,client.crt,client.key}` (owned by freeswitch).
  Applied with `fs_cli -x "reload mod_xml_curl"`.
- Verified: `/xml` without client cert → 401, without Basic auth → 401, with both
  → 200 a1-hash; FreeSWITCH pulls directory+dialplan over HTTPS+mTLS (xml_locate
  ok, no SSL errors); real REGISTER 200/403; IVR plays. `api-test.sh` = 53/53.
- Layers now on `/xml/*`: HTTPS + mTLS + Basic auth + IP-allowlist(optional) +
  not-found fallback. ESL is left on ACL+password (TLS for ESL is non-trivial in
  FreeSWITCH; out of scope).
- Remaining optional hardening: DB still stores plaintext SIP password (secret_ref
  / Vault); cert rotation tooling; ESL over TLS/stunnel.

---

## 9. MILESTONE B — Terraform provider — DONE ✅

Built with the Terraform Plugin Framework under `provider/` (module
`github.com/jamalshahverdiev/terraform-provider-freeswitch`). Resources `freeswitch_domain`,
`freeswitch_user`, `freeswitch_gateway`, `freeswitch_dialplan_extension` — full
CRUD + ImportState, talking to `/api/v1` over TLS (provider attrs `endpoint`,
`token`, `ca_cert_file`, `insecure`; env `FREESWITCH_ENDPOINT/TOKEN/CACERT/INSECURE`).
Scaffolding extras: `GNUmakefile`, acceptance tests (`*_test.go`), `examples/`
(+`import.sh` per resource), `README.md`.

**Verified e2e with OpenTofu** (plan → apply → no-op plan → import domain+user →
destroy → API 404). `go build`/`vet` clean.

### How to run the provider (dev_overrides, no `terraform init`)

```bash
GO=/home/jamal/.gvm/gos/go1.25.7/bin/go
$GO -C provider build -o /home/jamal/.gvm/pkgsets/go1.25.7/global/bin/terraform-provider-freeswitch .
# ~/.terraformrc has dev_overrides for BOTH registry.terraform.io/local/freeswitch
# and registry.opentofu.org/local/freeswitch -> that bin dir.
export TF_CLI_CONFIG_FILE=/home/jamal/.terraformrc   # see gotcha below
tofu -chdir=hack/e2e apply -auto-approve
tofu -chdir=hack/e2e destroy -auto-approve
```

Test config lives in `hack/e2e/main.tf` (throwaway `tf-e2e.local` domain).

### Provider gotchas (cost time)

1. **Dep version lock:** `terraform-plugin-framework v1.15.1` pairs with
   `terraform-plugin-go v0.27.0` + `terraform-plugin-testing v1.13.0`. Do NOT let
   `tfplugindocs`/newer `terraform-plugin-testing` bump `terraform-plugin-go`
   (newer versions need a `GenerateResourceConfig` method the framework lacks →
   build breaks). `tfplugindocs` is therefore run via `go run ...@latest` in the
   `generate` make target, not pinned in go.mod.
2. **OpenTofu vs acceptance harness:** `terraform-plugin-testing v1.13.0`'s
   terraform-exec can't parse the `OpenTofu vX` version string, so `make testacc`
   needs a real `terraform` binary. With only `tofu`, verify via dev_overrides +
   `tofu` directly (as in `hack/e2e`).
3. **OpenTofu registry host:** under tofu, `source = "local/freeswitch"` expands
   to `registry.opentofu.org/local/freeswitch` — dev_overrides must include that
   host (not just registry.terraform.io).
4. **CLI config path:** this shell's tofu looked for `/home/freshly/.terraformrc`;
   set `TF_CLI_CONFIG_FILE=/home/jamal/.terraformrc` explicitly so dev_overrides
   are picked up.

### Provider now ALSO covers (added after initial MVP)
- Resource `freeswitch_reloadxml` (triggers → ESL reloadxml).
- Data sources: `freeswitch_domain/user/gateway/dialplan_extension` (read by key)
  + runtime `freeswitch_gateway_status` / `freeswitch_user_registration`.
- Control-plane runtime endpoints added: `GET /api/v1/runtime/gateways/{profile}/{name}`
  and `GET /api/v1/runtime/registrations/{domain}/{user}` (ESL `sofia status gateway` /
  `show registrations as json` parsing in `runtime/esl.go`).
- **IVR via Terraform proven** (`examples/ivr/`): nested 8000/8100 menu built from
  `freeswitch_dialplan_extension` + `freeswitch_reloadxml`, verified playing on the
  live FreeSWITCH. `deploy/api-test.sh` = 56/56.

All 6 data sources verified through the provider via OpenTofu (`hack/dstest`):
domain, user, user_registration, gateway, dialplan_extension (incl. nested
condition decode), gateway_status (against the host's default `example.com`
gateway → NOREG). Provider docs generated into `provider/docs/` (12 files) via
`make generate` (tfplugindocs); examples under `provider/examples/{provider,
resources/<t>/{resource.tf,import.sh}, data-sources/<t>/data-source.tf}`.

### Remaining provider polish (optional)
- Acceptance tests in CI with a real `terraform` binary.
- gateway_status returns 404 until the sofia/`configuration` xml_curl binding is
  enabled (no gateways are loaded into FreeSWITCH runtime otherwise).

(historical plan kept below)

Replaces `seed.sh`/curl with declarative `terraform apply`. The API it calls
already exists and is tested.

- New Go module under `provider/` (Go 1.25, Terraform Plugin Framework
  `github.com/hashicorp/terraform-plugin-framework`).
- Provider config: `endpoint`, `token` (env `FREESWITCH_ENDPOINT`,
  `FREESWITCH_TOKEN`). An HTTP client wrapping `/api/v1` with Bearer auth.
- Resources (CRUD + ImportState), mapping to the API and IDs:
  - `freeswitch_domain`            id = `name`             → /api/v1/domains/{name}
  - `freeswitch_user`              id = `domain/number`    → /api/v1/users/{domain}/{number}
  - `freeswitch_gateway`           id = `profile/name`     → /api/v1/gateways/{profile}/{name}
  - `freeswitch_dialplan_extension` id = `<uuid>`          → /api/v1/dialplan/extensions/{id}
  - dialplan uses nested `condition { ... action { } }` blocks (see PROVIDER_RESOURCE_SPEC.md).
- Behaviors: Read 404 → remove from state; Delete 404 → treat as success;
  Update = full PUT; mark `password`/gateway `password` sensitive.
- Build & local install:
  `~/.terraform.d/plugins/local/freeswitch/0.1.0/linux_amd64/terraform-provider-freeswitch`
- `examples/basic-pbx/` already references these resources (`source = local/freeswitch`).
- Validate end-to-end: `terraform apply` creates the same domain/users/dialplan
  that `seed.sh` does, FreeSWITCH serves them, `terraform destroy` removes them.
- Reference specs: `../freeswitch-iac-docs/PROVIDER_RESOURCE_SPEC.md`,
  `MVP_IMPLEMENTATION_PLAN.md` (phases 9-13), `API_SPEC.md`.

Suggested order: Milestone A (a1-hash → TLS) is security hygiene; Milestone B
(provider) is the headline feature. The user wanted A then B.

---

## 10. Pointers

- Design docs (read-only spec): `../freeswitch-iac-docs/` — esp. TECHNICAL_DESIGN,
  MVP_IMPLEMENTATION_PLAN, API_SPEC, PROVIDER_RESOURCE_SPEC, DATABASE_SCHEMA,
  SECURITY_MODEL.
- This platform's docs: `docs/architecture.md`, `docs/api.md`, `docs/ivr-audio.md`
  (IVR TTS + prompt files), `docs/howto-custom-prompt.md` (flite+sox runbook),
  `docs/HANDOFF.md`.
- Saved agent memory: `freeswitch-iac-project`, `freeswitch-server`,
  `postgresql-needed` (index in MEMORY.md).
- User preference: communicate in Russian; concise; no emoji; docs in English.
