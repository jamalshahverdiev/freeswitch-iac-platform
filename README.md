# FreeSWITCH IaC Platform

Manage FreeSWITCH configuration as data (PostgreSQL) instead of hand-edited XML.
FreeSWITCH pulls its directory/dialplan/config dynamically via `mod_xml_curl`
from a Control Plane API; Terraform (next milestone) drives that API.

```
Terraform ──► Control Plane API (Go) ──► PostgreSQL (desired state)
                    │  ▲
       XML renderer │  │ ESL (reloadxml / status)
                    ▼  │
            FreeSWITCH 1.11 (mod_xml_curl)
```

## Topology (this setup)

- **Control Plane + PostgreSQL** run here, on the dev box (WSL2 Debian, `172.31.30.216`) via Docker Compose.
- **FreeSWITCH** runs on a separate server (`192.168.48.143`), installed as a host package.

Verified network paths:

| From | To | Use |
|---|---|---|
| control-plane (172.31.30.216) | `192.168.48.143:8021` | ESL runtime commands |
| FreeSWITCH (192.168.48.143) | `http://172.31.30.216:8080` | mod_xml_curl pulls XML |

## Components

| Path | What |
|---|---|
| `control-plane/` | Go API: CRUD + XML renderers + ESL client + audit. PostgreSQL-backed. |
| `docker-compose.yml` | Brings up `postgres` + `control-plane` (FreeSWITCH is external). |
| `deploy/freeswitch/` | Config snippets for the FreeSWITCH server (`xml_curl`, `event_socket`, `acl`). |
| `deploy/seed.sh` | Seeds the demo (domain + 2 users + dialplan) over the REST API. |
| `examples/basic-pbx/` | Terraform example (target state; provider is the next milestone). |

## Quick start

```bash
cp .env.example .env          # adjust secrets/addresses
bash hack/gen-tls.sh          # generate dev CA + server/client certs into deploy/tls/
docker compose up -d --build  # postgres + control-plane (HTTPS + mTLS on /xml)

CA=deploy/tls/ca.crt
curl -s --cacert $CA https://localhost:8080/healthz   # {"status":"ok"}
curl -s --cacert $CA https://localhost:8080/readyz    # {"status":"ready","database":"ok"}

# Seed demo data (domain + users 2001/2002 + IVR), then the nested IVR 7000/7100:
./deploy/seed.sh
./deploy/seed-ivr.sh

# Full API regression (CRUD + auth + TLS/mTLS + ESL): expect 53 passed.
bash deploy/api-test.sh
```

The control-plane serves **HTTPS**, and `/xml/*` additionally requires a client
certificate (**mTLS**) on top of Basic auth. To run plain HTTP instead, leave the
`TLS_*` / `XML_CLIENT_CA_FILE` vars empty.

## Wire up the FreeSWITCH server

On `192.168.48.143`:

1. Copy `deploy/freeswitch/xml_curl.conf.xml` and `event_socket.conf.xml`
   (and merge `acl.conf.xml`) into `/etc/freeswitch/autoload_configs/`.
2. Copy the TLS client material (`deploy/tls/{ca.crt,client.crt,client.key}`)
   to `/etc/freeswitch/tls/` (referenced by `xml_curl.conf.xml` for mTLS).
3. Ensure `mod_xml_curl` is loaded (`modules.conf.xml`).
4. `fs_cli -x "reload mod_xml_curl" && fs_cli -x reloadacl`
   (note: changing xml_curl URL/credentials/certs needs `reload mod_xml_curl`,
   not just `reloadxml`).
5. Register SIP clients `2001` / `2002` (password `2580`, domain
   `192.168.48.143`); dial `7000` for the IVR or `2001`↔`2002` direct.

## API

`/api/v1/*` (Bearer token): `domains`, `users`, `gateways`, `dialplan/extensions`,
`runtime/reloadxml`, `runtime/health`.
`/xml/{directory,dialplan,configuration}`: FreeSWITCH-facing (network-restricted).
`/healthz`, `/readyz`: health.

## Make targets

`make up | down | logs | ps | build | vet | test | tidy | smoke`

## Security

- `/api/v1/*` — Bearer token.
- `/xml/*` — HTTPS + **mTLS** (FreeSWITCH client cert) + HTTP Basic auth, and a
  `not found` fallback so it can't break the default FreeSWITCH config.
- Directory sends **`a1-hash`**, never the plaintext SIP password (verified by a
  real REGISTER, `hack/sip_register_test.py`).
- ESL — non-default password + ACL (loopback + control-plane source only).
- Audit logs redact `password` / `vm-password`.

MVP limitation: SIP passwords are still stored plaintext in PostgreSQL (used to
compute the a1-hash). See `freeswitch-iac-docs/SECURITY_MODEL.md` for the
production path (secret_ref / Vault).
