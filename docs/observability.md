# Observability — Grafana NOC dashboard

A live NOC view of the platform. Because the no-sqlite milestone moved all
FreeSWITCH runtime state into our PostgreSQL, Grafana just reads the three
databases **read-only** and refreshes every 5 s — no extra code in the
control-plane, and it does NOT go through the API.

```
Grafana  ──(read-only, grafana_ro)──►  PostgreSQL
                                          ├─ freeswitch_core        (channels, sip_registrations)
                                          ├─ freeswitch_callcenter  (agents, members, tiers)
                                          └─ freeswitch_control     (users, dialplan, queues, rooms)
```

## Run

```bash
docker compose up -d grafana
# http://localhost:3000   (admin / $GRAFANA_ADMIN_PASSWORD)
```

Open dashboard **FreeSWITCH → FreeSWITCH NOC**. Panels:
- Live calls, Registered endpoints, Waiting in queue, Available agents (stats)
- Users / Dialplan extensions / Queues / Conference rooms (desired-state counts)
- Call-center agents table, live queue members, SIP registrations

## How it is wired (all version-controlled)

- `deploy/postgres-init/02-grafana-ro.sh` — creates the `grafana_ro` login
  (SELECT-only on all three DBs, password from `$GRAFANA_DB_PASSWORD`) and sets
  `ALTER DEFAULT PRIVILEGES` so tables created later (migrations, FreeSWITCH
  ODBC) stay readable.
- `deploy/grafana/provisioning/datasources/freeswitch.yml` — three Postgres
  datasources (uids `fs_control` / `fs_callcenter` / `fs_core`); password is
  injected from the `GRAFANA_DB_PASSWORD` env var, never stored in the file.
- `deploy/grafana/dashboards/noc.json` + `provisioning/dashboards/dashboards.yml`.
- compose `grafana` service on `:3000`, secrets via `.env`
  (`GRAFANA_ADMIN_PASSWORD`, `GRAFANA_DB_PASSWORD`).

## Applying the RO role to an already-running Postgres

The init script only runs on a **fresh** volume. For an existing one, run the
same script against the live container (it is idempotent):

```bash
docker exec -i -e GRAFANA_DB_PASSWORD="$GRAFANA_DB_PASSWORD" \
  -e POSTGRES_USER=freeswitch -e POSTGRES_DB=freeswitch_control \
  freeswitch-iac-platform-postgres-1 bash -s < deploy/postgres-init/02-grafana-ro.sh
```

`grafana_ro` can read every table but cannot write (verified: `CREATE TABLE`
→ "permission denied for schema public").
