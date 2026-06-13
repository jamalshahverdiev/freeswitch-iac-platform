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
- [ ] ESL hardening (optional): document the plaintext-password risk; ACL is
      the current mitigation; consider stunnel/wireguard if it ever leaves
      the lab.

## Phase 5 — Roadmap features (already agreed earlier)

### D1 — Grafana NOC dashboard (NEXT; near-free after the no-sqlite milestone)

All live state already sits in our PostgreSQL (freeswitch_core.channels /
sip_registrations, freeswitch_callcenter.agents/members/tiers,
freeswitch_control.*), so this is mostly wiring + dashboard JSON.

1. [ ] Read-only DB role: add to `deploy/postgres-init/` a script creating a
       `grafana_ro` login with `CONNECT` + `USAGE` + `SELECT` on all three DBs
       (no write). Password via `.env` (age) `GRAFANA_DB_PASSWORD`.
2. [ ] Compose: add a `grafana` service (grafana/grafana-oss), publish e.g.
       `3000:3000`, `GF_SECURITY_ADMIN_PASSWORD` from `.env`, mount
       `deploy/grafana/provisioning/` and `deploy/grafana/dashboards/` read-only.
       Add GRAFANA_DB_PASSWORD/admin pass to `.env.example` + SECRETS.md.age.
3. [ ] Provision datasources (`deploy/grafana/provisioning/datasources/*.yml`):
       three postgres datasources (control / callcenter / core) using grafana_ro,
       `sslmode=disable` (compose-internal network).
4. [ ] Dashboard JSON (`deploy/grafana/dashboards/noc.json`), panels:
       - Live calls: `SELECT count(*) FROM channels` (core) — stat + timeseries
       - Registered endpoints: `sip_registrations` (core) — table (sip_user, ip)
       - Queue waiting: `members WHERE state!='Answered'` (callcenter) — stat
       - Agents: state / status / calls_answered / no_answer_count (callcenter) — table
       - Desired-state counts: users, dialplan_extensions, cc_queues,
         conference_rooms (control) — stat row
       Refresh 5s.
5. [ ] Doc: `docs/observability.md` + a panel screenshot; link from README.
       Note: Grafana auto-refresh polls Postgres directly (read-only) — it does
       NOT go through the control-plane API.
6. [ ] api-test/CI: nothing to assert in api-test (no API surface); just make
       sure `docker compose config` still validates with the new service.

- [ ] **C1 CDR via mod_json_cdr** → control-plane `/cdr` (mTLS+Basic like
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

- [ ] **WebSocket / SSE live events** — foundation for wallboard + bots.
      1. Add a PERSISTENT ESL listener to the control-plane (current ESL client
         is connect-per-command; add a long-lived goroutine that connects, sends
         `event plain CHANNEL_CREATE CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE
         CUSTOM callcenter::info conference::maintenance`, reconnects on drop).
      2. Internal pub/sub hub (fan-out to subscribers); parse the events we care
         about into small JSON (call started/ended, agent status, queue join/leave,
         conference join/leave).
      3. Expose `GET /api/v1/events` as SSE (simpler than WS for one-way) behind
         the bearer token; heartbeat + last-event-id optional.
      4. Tests: feed canned ESL event frames into the parser (unit); a manual
         curl stream check against the live box.
- [ ] **Supervisor wallboard** — a small static page (like webphone) or a
      control-plane HTML route that consumes `/api/v1/events` (Phase 6 #1) and
      shows queue depth, agent states, current calls, wait times in real time.
      Serve from compose (or reuse the webphone nginx). Demo-friendlier than
      Grafana because it is push/live and call-center focused.
- [ ] **Time-based routing** — FreeSWITCH dialplan conditions support time
      fields (`wday`, `time-of-day`, `mday`, `date-time`). Extend the dialplan
      renderer + `freeswitch_dialplan_extension` condition schema with optional
      time attributes, then add a `freeswitch_schedule`-style helper/example:
      business hours → queue 4444, off-hours → an IVR "call back later". Keep it
      pure dialplan (no new runtime moving parts).
- [ ] **Phone provisioning** — `GET /provision/{mac}.xml` (or .cfg) endpoint
      rendering vendor device configs from a new `provisioned_devices` table
      (mac → user/line/server/codecs). Templates per vendor (Yealink,
      Grandstream). Served over HTTP on the LAN (devices fetch on boot); guard
      by MAC + network ACL. Terraform resource `freeswitch_device`.
- [ ] **GitOps flow** — now that repos are public and the provider is published:
      a separate "PBX config" repo holding tofu + remote state; GitHub Actions
      runs `tofu plan` on PR and posts the diff as a comment, `tofu apply` on
      merge to main. This realizes the original design-doc dream end to end.
      Needs: remote state backend + provider creds as repo secrets.
- [ ] **Voicemail integration** — mod_voicemail already stores in
      freeswitch_core via ODBC. Wire mailbox params into `freeswitch_user`
      (vm-password already passes through; add enable/greeting/email options),
      drop voicemail recordings into the dated tree, and add an optional
      notification (control-plane webhook → Telegram/email) on new voicemail
      via an ESL `MESSAGE_WAITING` / vm event subscription (ties into Phase 6 #1).

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
