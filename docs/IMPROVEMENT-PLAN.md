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

- [ ] `hack/backup-postgres.sh`: `pg_dump -Fc` of all three DBs into
      `~/backups/freeswitch/$(date +%F)/`, keep N=14 days, prune older.
- [ ] Schedule: cron (or systemd timer) on the dev box, daily.
- [ ] RESTORE TEST: restore into a scratch compose project, run api-test.sh
      against it — a backup that was never restored is not a backup.
- [ ] Recordings on the FS host: daily `rsync` to the dev box (or accept the
      risk explicitly and write that down here).
- [ ] Document recovery runbook in HANDOFF (volume lost → restore order:
      postgres up → restore dumps → restart freeswitch → reload modules).

## Phase 2 — Boot resilience (FS ↔ Postgres dependency)

- [ ] systemd drop-in on the FS server for freeswitch.service:
      `Restart=on-failure`, `RestartSec=10`, plus an `ExecStartPre` script
      that waits (with timeout) for 172.31.30.216:5432 to accept connections.
- [ ] Version the drop-in in `deploy/freeswitch/systemd/`.
- [ ] Test: stop compose Postgres, reboot FS server, start Postgres → FS must
      come up on its own with no manual restart.

## Phase 3 — Tests + CI

- [ ] Renderer unit tests (highest value/effort): table-driven golden-file
      tests for directory/dialplan/callcenter/conference renderers, incl.
      a1-hash emission, not-found fallbacks, param sorting, room-extension
      synthesis + priority ordering.
- [ ] API handler tests with httptest against a store interface (or a test
      Postgres via testcontainers) — at minimum validation + error mapping.
- [ ] Provider: acceptance tests for callcenter/conference resources (the
      pattern exists in domain_resource_test.go); document why they run via
      tofu (harness can't parse OpenTofu version).
- [ ] GitHub Actions, platform repo: build + vet + golangci-lint + unit
      tests on PR; optional manual job that boots compose and runs
      deploy/api-test.sh.
- [ ] GitHub Actions, provider repo: build + vet + lint + tests +
      `tfplugindocs generate` with a "docs are up to date" diff check.

## Phase 4 — Tech debt

- [ ] hack/rec_queue_test.sh: replace the dead loopback-agent variant with
      the proven sipp recipe (sipp -sn uas -p 5070 -rtp_echo + agent contact
      sofia/internal/bot@127.0.0.1:5070); make it fully self-contained.
- [ ] Pagination (`?limit=&offset=`) on all list endpoints; provider data
      sources unaffected (they read single items).
- [ ] Audit log read API: `GET /api/v1/audit?from=&to=&resource=` — the data
      is already written; this is a free changelog of "who changed what".
- [ ] Validation: reject duplicate (context, priority) across dialplan
      extensions + conference rooms at create/update time (renderer order is
      otherwise nondeterministic).
- [ ] Secrets hygiene: move all credentials out of HANDOFF.md into
      `deploy/SECRETS.md` (gitignored) or a vault; HANDOFF references it.
- [ ] ESL hardening (optional): document the plaintext-password risk; ACL is
      the current mitigation; consider stunnel/wireguard if it ever leaves
      the lab.

## Phase 5 — Roadmap features (already agreed earlier)

- [ ] **D1 Grafana NOC dashboard**: compose service + read-only PG user +
      provisioned dashboards in deploy/grafana/ (live channels,
      registrations, queue members, agent states).
- [ ] **C1 CDR via mod_json_cdr** → control-plane `/cdr` (mTLS+Basic like
      /xml) → cdr table + `GET /api/v1/cdr` + stats; Grafana panels on top.
- [ ] **A1 AI voice agent** in the queue (Whisper STT + Piper TTS + Claude
      API) — after C1.
- [ ] Gateways from DB (Option A include+rescan) — when a SIP provider exists.

## Phase 6 — New "wow" features (pick by appetite)

- [ ] **WebSocket live events**: control-plane subscribes to ESL events and
      streams them at `/api/v1/events` (calls started/ended, agent status) —
      foundation for wallboard and bots, no polling.
- [ ] **Supervisor wallboard**: small page next to webphone reading our
      Postgres (queue depth, agent states, wait times). Demo-friendlier than
      Grafana.
- [ ] **Time-based routing**: Terraform resource for schedules (business
      hours / holidays) → day: queue, night: IVR. Real PBX pain point.
- [ ] **Phone provisioning**: `/provision/{mac}.xml` endpoint rendering
      device configs from the same DB (Yealink/Grandstream templates).
- [ ] **GitOps flow**: once repos exist — PR with IVR change → CI posts
      `tofu plan` diff → merge applies. This IS the original dream from the
      design docs.
- [ ] **Voicemail integration**: mod_voicemail already on ODBC; wire mailbox
      params into freeswitch_user, recordings into the dated tree, optional
      Telegram notification.

---

## Suggested order

Phase 0 → 1 → 2 are about risk and take roughly a day together.
Then D1 (Grafana) for visible value, Phase 3 CI in parallel with feature
work, Phase 4 piecemeal between features.
