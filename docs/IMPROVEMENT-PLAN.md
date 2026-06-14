# Improvement Plan (hardening + next features)

Written 2026-06-11 after a full audit of the platform. Work through phases in
order — phases 0–2 remove real risks, the rest is quality and features.
Status legend: [ ] todo, [x] done. Update this file as items complete.

---

## Phase 0 — Git split (FIRST, everything else lands as commits)

Two private repos on GitHub under `github.com/jamalshahverdiev`:

| Repo | Contents |
|---|---|
| `freeswitch-iac-platform` | control-plane/, deploy/, webphone/, docs/, examples/, hack/, docker-compose.yml |
| `terraform-provider-freeswitch` | current provider/ moved to repo ROOT (internal/, docs/, examples/, GNUmakefile, main.go) — name is the mandatory Terraform convention |

Status: **DONE 2026-06-11.** Both repos pushed (branches
`first-step-to-fs-provider` / `first-step-to-fs-iac-platform`). Working tree
moved to /home/jamal/github/freeswitch-iac-platform; compose reuses the same
postgres volume (same project name) — data intact, api-test 89/89, tofu plans
clean. Secrets: age-encrypted (`hack/secrets.sh`, key ~/.config/age/keys.txt).

Steps:
- [x] Create both repos (private!). HANDOFF.md contains live credentials —
      decide: keep repo private and accept, or move secrets to a vault doc.
- [x] Platform: `git init`, verify .gitignore covers `.env`, `deploy/tls/`,
      `deploy/freeswitch/nginx/recordings.htpasswd`, `**/.terraform/`,
      `*.tfstate*`; initial commit; push.
- [x] Provider: move `provider/*` to repo root, rename Go module to
      `github.com/jamalshahverdiev/terraform-provider-freeswitch`, fix imports,
      rebuild, re-run `tofu -chdir=examples/... plan` smoke. Keep the
      dev_overrides binary name `terraform-provider-freeswitch` (unchanged).
- [x] Platform repo: drop provider/ dir; README cross-links both repos.
- [x] Platform control-plane module rename:
      `github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane`.
- [x] Examples reference `local/freeswitch` via dev_overrides — document that
      the public Terraform Registry (provider published from GitHub releases
      via goreleaser + GPG, registry address
      `jamalshahverdiev/freeswitch`) is the future publish path.

## Phase 1 — Backups (Postgres is now the single point of failure)

All FreeSWITCH state lives in the dev-box docker volume: freeswitch_control
(desired state), freeswitch_callcenter (runtime), freeswitch_core (core db).

Status: **DONE 2026-06-12.** backup-postgres.sh + restore-test.sh (verified
10/10 vs live), cron TEMPLATE in deploy/cron/backup.crontab (user installs
manually), recordings pulled via ssh key (installed on FS host), recovery
runbook in HANDOFF.

- [x] `hack/backup-postgres.sh`: `pg_dump -Fc` of all three DBs into
      `~/backups/freeswitch/$(date +%F)/`, keep N=14 days, prune older.
- [x] Schedule: cron (or systemd timer) on the dev box, daily.
- [x] RESTORE TEST: restore into a scratch compose project, run api-test.sh
      against it — a backup that was never restored is not a backup.
- [x] Recordings on the FS host: daily `rsync` to the dev box (or accept the
      risk explicitly and write that down here).
- [x] Document recovery runbook in HANDOFF (volume lost → restore order:
      postgres up → restore dumps → restart freeswitch → reload modules).

## Phase 2 — Boot resilience (FS <-> Postgres dependency) — SKIPPED by user decision (2026-06-12)

Out of scope: the project's focus is the control-plane API and the Terraform
provider, not FS-server infra hardening. The dependency itself is documented
in HANDOFF ("if Postgres was down at FS boot: start compose, then restart
freeswitch"). Revisit only if the lab turns into a real deployment.

- [ ] systemd drop-in on the FS server for freeswitch.service:
      `Restart=on-failure`, `RestartSec=10`, plus an `ExecStartPre` script
      that waits (with timeout) for 172.31.30.216:5432 to accept connections.
- [ ] Version the drop-in in `deploy/freeswitch/systemd/`.
- [ ] Test: stop compose Postgres, reboot FS server, start Postgres → FS must
      come up on its own with no manual restart.

## Phase 3 — Tests + CI

- [x] Renderer unit tests (DONE 2026-06-12): control-plane/internal/renderer/
      renderer_test.go — a1-hash (no plaintext password in output!), dialplan
      grouping/filter/disabled, callcenter odbc-dsn + params merge, conference
      video/audio profiles + map override, room-extension synthesis (pin
      dialstring, max-members, action order), NotFoundDocument.
- [x] API handler tests (DONE 2026-06-12): internal/api/server_test.go —
      auth middleware (401/pass-through), public healthz, xmlGuard Basic auth,
      7 validation 400-paths (no DB needed: rejected before store), runtime
      503 without ESL, recordings proxy (503 unconfigured, bad-date/traversal
      400 with backend-untouched proof, listing proxied with Basic auth).
      DB-touching paths stay covered by deploy/api-test.sh (89 live asserts).
- [x] Provider acceptance tests (DONE 2026-06-12): callcenter_resources_test.go
      (queue+agent+tier: create/import-verify/update incl. tier id
      "queue/agent") + conference_resources_test.go (profile+room: defaults,
      pin change, drop video). Run:
      `TF_ACC=1 TF_ACC_TERRAFORM_PATH=$HOME/bin/terraform go test ./internal/provider/`
      (real terraform v1.15.4 at ~/bin/terraform; harness can't parse tofu).
      Verified self-cleaning (tfacc-* all 404 after run).
- [x] GitHub Actions, platform repo (DONE 2026-06-12, .github/workflows/
      ci.yml): build+vet+test (control-plane) + compose-config validation +
      bash -n of all scripts. golangci-lint deferred (needs a config pass).
- [x] GitHub Actions, provider repo (DONE 2026-06-12): build+vet+test job +
      docs-up-to-date job (tfplugindocs generate + git diff --exit-code,
      verified clean locally). golangci-lint deferred.

## Phase 4 — Tech debt

- [x] hack/rec_queue_test.sh (DONE 2026-06-13): rewritten to the sipp recipe;
      sipp runs as a transient systemd unit (sipp-bot) so it survives ssh —
      plain `&`/nohup/`-bg` died on session close. FS() ssh calls wrapped in
      `timeout` (a daemon's inherited stdout pipe hangs ssh otherwise). Green
      run: bot Answered, recordings 4->5, self-cleaning via EXIT trap.
- [x] Pagination (DONE 2026-06-13): optional ?limit=&offset= on every GET list
      endpoint via api/pagination.go (generic apply[T]/writeList[T]); bare-array
      body preserved + X-Total-Count header; limit capped 1000; bad params 400.
      Handler-level (xml.go still gets all rows). Provider data sources read
      single items -> unaffected. Unit + live tested.
- [x] Audit read API (DONE 2026-06-13): GET /api/v1/audit, filters
      actor/action/resource_type/resource_id + limit/offset, newest-first,
      X-Total-Count. models/audit.go + store/audit.go + api/audit.go.
- [WON'T DO] Validation duplicate (context, priority): REJECTED 2026-06-13 —
      would break legitimate cases (examples/conference has 2 rooms in context
      `company` both at default priority 5). Multiple extensions sharing a
      priority is valid in FreeSWITCH; the renderer already sorts deterministically
      by (context, priority, name), so order is stable without this constraint.
- [x] Secrets hygiene (DONE in Phase 0 git split): all creds in age-encrypted
      deploy/SECRETS.md.age; HANDOFF references it; literals scrubbed from code.
- [x] hack/secrets.sh: encrypt only changed files (DONE 2026-06-14). `encrypt`
      now decrypts each existing <f>.age and skips if the plaintext is byte-
      identical (cmp); for tls.tar.age it compares a content signature (sorted
      per-file sha256, ignoring tar mtimes). Output prints `unchanged:` vs
      `encrypted:`; idempotent (2nd run rewrites 0 files). Kills the noisy
      .age diffs.
- [ ] ESL hardening (optional): document the plaintext-password risk; ACL is
      the current mitigation; consider stunnel/wireguard if it ever leaves
      the lab.

## Phase 5 — Roadmap features (already agreed earlier)

### D1 — Grafana NOC dashboard — DONE 2026-06-13 (branch D1)

All live state already sits in our PostgreSQL (freeswitch_core.channels /
sip_registrations, freeswitch_callcenter.agents/members/tiers,
freeswitch_control.*), so this is mostly wiring + dashboard JSON.

1. [x] Read-only DB role: add to `deploy/postgres-init/` a script creating a
       `grafana_ro` login with `CONNECT` + `USAGE` + `SELECT` on all three DBs
       (no write). Password via `.env` (age) `GRAFANA_DB_PASSWORD`.
2. [x] Compose: add a `grafana` service (grafana/grafana-oss), publish e.g.
       `3000:3000`, `GF_SECURITY_ADMIN_PASSWORD` from `.env`, mount
       `deploy/grafana/provisioning/` and `deploy/grafana/dashboards/` read-only.
       Add GRAFANA_DB_PASSWORD/admin pass to `.env.example` + SECRETS.md.age.
3. [x] Provision datasources (`deploy/grafana/provisioning/datasources/*.yml`):
       three postgres datasources (control / callcenter / core) using grafana_ro,
       `sslmode=disable` (compose-internal network).
4. [x] Dashboard JSON (`deploy/grafana/dashboards/noc.json`), panels:
       - Live calls: `SELECT count(*) FROM channels` (core) — stat + timeseries
       - Registered endpoints: `sip_registrations` (core) — table (sip_user, ip)
       - Queue waiting: `members WHERE state!='Answered'` (callcenter) — stat
       - Agents: state / status / calls_answered / no_answer_count (callcenter) — table
       - Desired-state counts: users, dialplan_extensions, cc_queues,
         conference_rooms (control) — stat row
       Refresh 5s.
5. [x] Doc: `docs/observability.md` + a panel screenshot; link from README.
       Note: Grafana auto-refresh polls Postgres directly (read-only) — it does
       NOT go through the control-plane API.
6. [x] api-test/CI: nothing to assert in api-test (no API surface); just make
       sure `docker compose config` still validates with the new service.


Verified: grafana_ro reads all 3 DBs / write denied; Grafana :3000 up, 3 datasources provisioned, dashboard fs-noc loaded, live query through Grafana returned agents=2 (status 200). Docs: docs/observability.md. NOTE step 5 screenshot still TODO (user to add).

- [x] **C1 CDR via mod_json_cdr** — DONE 2026-06-14 (branch C1). POST /cdr
      (mTLS+Basic, idempotent on uuid, parses variables + callflow.caller_profile)
      -> cdr table in freeswitch_control; GET /api/v1/cdr (filters+pagination,
      X-Total-Count) + /cdr/stats (per-day). mod_json_cdr loaded on server
      (deploy/freeswitch/json_cdr.conf.xml.age, retry-spool). Grafana CDR panels
      added. api-test 103/103 + handler tests. ORIGINAL:
- [ ] (orig) **C1 CDR via mod_json_cdr** → control-plane `/cdr` (mTLS+Basic like
      /xml) → cdr table + `GET /api/v1/cdr` + stats; Grafana panels on top.
      Steps: load mod_json_cdr on the FS host (`json_cdr.conf.xml` → url
      `https://172.31.30.216:8080/cdr`, same client cert + creds as xml_curl,
      retry-on-failure queue); control-plane `POST /cdr` (mTLS+Basic guarded
      like /xml) + migration `cdr` table (uuid, caller/callee, direction,
      context, start/answer/end epochs, duration, billsec, hangup_cause,
      recording_path, raw JSONB); `GET /api/v1/cdr?from=&to=&number=&cause=`
      (paginated, reuse the page helper) + a stats endpoint; decide table
      lives in freeswitch_control (control-plane-owned). Grafana panels extend D1.
- [ ] **A1 AI voice agent** in the queue (Whisper STT + Piper TTS + Claude
      API) — after C1. When agents are absent: a bot answers (mod_audio_fork
      or mod_vosk STT + existing Piper TTS + Claude API), collects the
      question, writes a transcript to Postgres, transfers to an agent with
      context. Builds on the recording pipeline (R1) + call metadata (C1).
- [ ] Gateways from DB (Option A include+rescan) — when a SIP provider exists.
      Render each DB gateway to /etc/freeswitch/sip_profiles/external/<name>.xml
      + `sofia profile external rescan`; needs a delivery channel (pull-agent or
      CI scp) since the control-plane can't write the FS disk. Do NOT enable the
      xml_curl configuration binding for sofia.conf with a partial renderer.

## Phase 6 — New "wow" features (pick by appetite)

- [x] **SSE live events** — DONE 2026-06-14 (branch `events`). Live-verified:
      a real call streamed call.started/answered/ended over SSE. Caught & fixed a
      real bug: the logging middleware's statusWriter didn't forward Flush() so
      SSE would never flush in prod. internal/events Hub+Listener, GET
      /api/v1/events, main wiring, unit+race+httptest. Decision below kept:
      Decision: SSE, not WebSocket — we only need server->client push; SSE is
      plain HTTP, proxy-friendly, browser EventSource auto-reconnects. Plan:
      1. internal/events: `Hub` (pub/sub broadcast; Subscribe()->chan+cancel;
         Publish() non-blocking, drops to a slow subscriber so one stuck client
         can't stall the hub) + `Event{Type,Time,Data map[string]string}`.
      2. internal/events `Listener`: persistent ESL goroutine — connect+auth
         (reuse runtime.readHeaders framing), send `event plain CHANNEL_CREATE
         CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE CUSTOM callcenter::info
         conference::maintenance`, read text/event-plain frames, URL-decode the
         body's Name:value lines, normalize to events (call.started/answered/
         ended, agent.status, queue.member, conference), Publish to Hub.
         Reconnect with backoff; runs only if ESL is enabled.
      3. api: `GET /api/v1/events` SSE under bearer token — Subscribe, write
         `data: <json>\n\n` per event, ~25s heartbeat comment, unsubscribe on
         r.Context().Done().
      4. main.go: start Listener if ESL enabled; pass Hub to Server.
      5. Tests: parser (canned event-plain frames -> normalized JSON), Hub
         fan-out + slow-consumer drop, SSE handler via httptest (subscribe,
         inject event, read one frame). + manual curl stream vs live box.
      6. docs/api.md events section; note it's the base for wallboard/voicemail.
- [x] **Supervisor wallboard** — DONE 2026-06-14 (branch `wallboard`). Served
      BY the control-plane at GET /wallboard (static HTML via go:embed, no auth
      on the page; same-origin so the SSE fetch needs no CORS). Operator pastes
      the token (sessionStorage); page reads /api/v1/events via fetch()+stream
      reader (EventSource can't set Authorization). Live tiles: active calls,
      queue waiting, agents online, calls-since-open + agents table + event log.
      Event-driven (reflects activity since open; seeding in-progress calls on
      load = future). Verified: served 200, and a real call produced 6 events on
      the same stream the page consumes. Handler test added.
- [x] **Time-based routing** — DONE 2026-06-14 (branches `time-routing` in both
      repos). Condition gained optional FreeSWITCH time attrs (wday/hour/
      time-of-day/date-time/...) stored as JSONB (migration 000005); renderer
      emits them + omits empty field/expression so a pure time gate is valid;
      validation requires regex and/or time. Provider: condition `time` map,
      field/expression now Optional. examples/time-routing (6000: business
      hours→queue, else TTS "office closed"). Live-verified on a Sunday:
      Date/TimeMatch (FAIL) on wday=2-6 → falls through to the closed branch.
      FS wday numbering: 1=Sun..7=Sat (confirmed empirically). api-test 109.
- [x] **Phone provisioning** — `GET /provision/{file}` renders vendor device
      configs from a new `provisioned_devices` table (migration 000006; mac →
      number/domain/vendor). MAC normalized (lowercase, no separators); filename
      vendor convention `<mac>.cfg` (Yealink), `cfg<mac>.xml` (Grandstream),
      `<mac>.xml` (generic). The SIP **password is NOT stored** — read from the
      matching user at render time (rotate user pw ⇒ phone reprovisions). CRUD at
      `/api/v1/devices` (bearer). `/provision/*` is phone-facing: guarded by
      Basic auth (`PROVISION_USER`/`PASSWORD`) + CIDR allowlist
      (`PROVISION_ALLOW_CIDRS`); `PROVISION_SIP_SERVER`/`PORT` set the registrar.
      Terraform resource `freeswitch_device` (id=mac, ImportState by mac;
      keeps config-supplied MAC to avoid a normalization diff) + matching data
      source `freeswitch_device` (lookup by mac). Verified: api-test
      126 asserts (incl. 401 no-auth, 404 unknown mac, Grandstream `<P35>3001</P35>`
      rendered with user pw); provider acceptance create+import+update PASS.
- [ ] **GitOps flow** — now that repos are public and the provider is published:
      a separate "PBX config" repo holding tofu + remote state; GitHub Actions
      runs `tofu plan` on PR and posts the diff as a comment, `tofu apply` on
      merge to main. This realizes the original design-doc dream end to end.
      Needs: remote state backend + provider creds as repo secrets.
- **Voicemail integration** — mod_voicemail stores messages in freeswitch_core
      via ODBC. Staged:
  - [x] **#1 Declarative mailbox** — typed `voicemail{enabled,password,email,
        attach_file,email_all}` on `freeswitch_user` (migration 000007, nullable
        `voicemail` JSONB). Rendered into the directory as vm-* params (typed
        overrides freeform vm-* keys); `voicemail.password` redacted in audit &
        never in directory XML. Provider: nested `voicemail = {}` block on the
        user resource + data source. Verified: api-test 134, renderer/handler
        unit tests, provider acceptance (create+import+update+datasource), live
        directory render shows vm-enabled/vm-mailto.
  - [ ] **#2 voicemail.conf via xml_curl** — render the VM profile through
        `/xml/configuration` instead of the static host file (odbc-dsn from cfg).
  - [ ] **#3 Read API** — `GET /api/v1/voicemail/{domain}/{number}` + unread
        (MWI) count from a read-only `freeswitch_core` pool; data source.
  - [ ] **#4 (separate branch) Notifications** — ESL `MESSAGE_WAITING` → webhook
        (Telegram/email) on new voicemail; extends the events listener.

## Phase 7 — Publish the provider to the Terraform Registry

Goal: users write `source = "jamalshahverdiev/freeswitch"` in
`required_providers` — no dev_overrides. Prereqs already met: repo is PUBLIC,
named by the mandatory convention, docs/ is in tfplugindocs format (the
registry renders it), MIT license present.

1. [ ] **GPG signing key** (USER ACTION) (registry verifies release signatures):
       `gpg --full-generate-key` (RSA 4096, no expiry is fine for this),
       export: `gpg --armor --export-secret-keys <id>` -> GitHub repo secret
       `GPG_PRIVATE_KEY` + secret `PASSPHRASE`; `gpg --armor --export <id>`
       (public part) is later pasted into the registry UI.
       BACK UP the private key next to the age key.
2. [x] terraform-registry-manifest.json (DONE 2026-06-12).
3. [x] .goreleaser.yml (DONE 2026-06-12; `goreleaser check` ok; full
       `--snapshot --skip=sign` build verified locally: 14 platform zips +
       SHA256SUMS in 71s). main.go: version="dev" (ldflags-set), ServeOpts
       Address switched to registry.terraform.io/jamalshahverdiev/freeswitch
       (dev_overrides unaffected).
4. [x] Release workflow .github/workflows/release.yml (DONE 2026-06-12):
       tag v* -> import-gpg (secrets GPG_PRIVATE_KEY/PASSPHRASE) -> goreleaser.
5. [x] Tag v0.1.0 (DONE 2026-06-12): release workflow green, 14 platform
       zips + manifest + SHA256SUMS(.sig GPG-signed) attached.
6. [x] registry.terraform.io PUBLISHED (2026-06-12): live at
       registry.terraform.io/providers/jamalshahverdiev/freeswitch, v0.1.0
       confirmed via the registry API.
7. [~] OpenTofu registry: SMOKE-VERIFIED first (2026-06-13) — `terraform init`
       in a clean dir with no dev_overrides downloaded
       registry.terraform.io/jamalshahverdiev/freeswitch v0.1.1, signature
       verified (lock file has h1: hashes). OpenTofu submission = USER ACTION:
       open a "Submit Provider" issue at github.com/opentofu/registry with the
       repo URL; bot pulls the GPG key from the signed release and auto-merges.
       Pubkey fingerprint rsa4096/25A5F6EA501A7CBF, export at
       /tmp/tf-provider-pubkey.asc if the form needs it pasted.
8. [ ] After publish: update both READMEs + provider docs index
       (`source = "jamalshahverdiev/freeswitch"`, version pin example);
       keep dev_overrides documented for local development; examples/ in the
       platform repo switch `source` from `local/freeswitch` to the registry
       address (dev_overrides still wins locally when configured).
9. [ ] Versioning discipline from here on: semver tags; breaking schema
       changes -> minor bump pre-1.0; CHANGELOG.md (keep-a-changelog format).

---

## Suggested order (updated 2026-06-12)

Phase 0 + 1 done, Phase 2 skipped. Focus = control-plane API + provider:
finish Phase 3 (handler tests, provider acceptance) -> Phase 7 (Terraform
Registry publish) -> Phase 4 API debt (pagination, audit read API, priority
validation) -> Phase 5/6 features as appetite allows.
