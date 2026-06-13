# FreeSWITCH IaC Platform — System Overview (for topology diagram)

This document is a self-contained description of everything built, written so an
LLM can render an accurate **architecture / topology diagram** from it. It lists
the zones, nodes (with host/ports/tech), the connections (edges) between them
(with protocol, direction, port, auth), and the runtime data flows. No repo
access is needed to draw the picture — everything is below.

---

## 1. One-paragraph summary

FreeSWITCH configuration (SIP users, dialplan/IVR, gateways) is stored as data in
**PostgreSQL** and exposed by a Go **Control Plane** REST API. **FreeSWITCH**
(on a separate server) pulls its directory/dialplan as XML on demand over HTTPS
via **mod_xml_curl**; the Control Plane renders that XML from the database.
Runtime commands (reloadxml, status) are pushed to FreeSWITCH over **ESL**. A
**Terraform/OpenTofu provider** drives the Control Plane API declaratively. IVR
prompts are spoken with **TTS (mod_flite)** or played from pre-recorded files.

---

## 2. Zones (draw as 3 grouped boxes + external clients)

- **Zone A — Dev/Control box** (WSL2 Debian, IP `172.31.30.216`)
  Runs Docker Compose (PostgreSQL + Control Plane) and the Terraform/OpenTofu CLI
  with the local provider plugin.
- **Zone B — FreeSWITCH server** (Debian 13, IP `192.168.48.143`)
  Runs FreeSWITCH 1.11.1 as a host package (not a container).
- **Router / NAT** between the two subnets (`172.31.x` ↔ `192.168.48.x`).
  Source-NATs Zone A → Zone B traffic to `192.168.48.1`.
- **External clients** — SIP softphones (e.g. a Windows softphone, user `4100`).

---

## 3. Nodes (boxes in the diagram)

| # | Node | Zone | Host:Port | Tech | Role |
|---|------|------|-----------|------|------|
| 1 | PostgreSQL | A | 172.31.30.216:5432 | postgres:16 (Docker) | Desired-state DB `freeswitch_control` |
| 2 | Control Plane API | A | 172.31.30.216:8080 (HTTPS) | Go (chi, pgx), Docker | REST API + XML renderer + ESL client + audit |
| 3 | Terraform / OpenTofu + provider | A | local CLI | `terraform-provider-freeswitch` (Plugin Framework) | Declarative config → API |
| 4 | FreeSWITCH | B | 192.168.48.143 | FreeSWITCH 1.11.1 | SIP switch / media / IVR |
| 4a | ├ mod_sofia (SIP) | B | 5060/udp,tcp (internal), 5080 (external) | — | SIP registrar / profiles |
| 4b | ├ mod_xml_curl | B | (HTTP client) | — | Fetches directory/dialplan XML over HTTPS |
| 4c | ├ mod_event_socket (ESL) | B | 192.168.48.143:8021/tcp | — | Runtime command channel |
| 4d | ├ mod_flite (TTS) | B | — | — | Text-to-speech for IVR `speak` |
| 4e | └ RTP media | B | 16384–32768/udp | — | Audio streams |
| 5 | SIP softphone | external | Windows client | SIP UA | Registers as user 4100, places calls |

Docker-internal: nodes 1 and 2 share a Compose bridge network; only Control
Plane publishes `:8080` to the host. Postgres `:5432` is published for local use.

---

## 4. Connections (edges in the diagram)

Direction is "initiator → target". Each edge: protocol, port, auth, purpose.

| Edge | From → To | Protocol / Port | Auth | Purpose |
|------|-----------|-----------------|------|---------|
| E1 | Control Plane (2) → PostgreSQL (1) | SQL / pgx, TCP 5432 | DB user/pass | Read/write desired state |
| E2 | Terraform provider (3) → Control Plane (2) | HTTPS, TCP 8080, path `/api/v1/*` | **Bearer token** over TLS | Declarative CRUD of domains/users/gateways/dialplan + reloadxml |
| E3 | FreeSWITCH mod_xml_curl (4b) → Control Plane (2) | HTTPS, TCP 8080, path `/xml/*` | **mTLS (client cert) + HTTP Basic auth** | Pull directory + dialplan XML on demand |
| E4 | Control Plane (2) → FreeSWITCH ESL (4c) | ESL, TCP 8021 | **ESL password + inbound ACL** | reloadxml, gateway status, registrations |
| E5 | SIP softphone (5) → FreeSWITCH mod_sofia (4a) | SIP, UDP 5060 (+ RTP 16384–32768) | SIP **digest (a1-hash)** | Register + make calls (dial the IVR) |
| E6 | FreeSWITCH (4) → SIP softphone (5) | RTP/audio, UDP | — | Media (IVR prompts, TTS) |

Network note for E3/E4 (important for an accurate diagram): the two boxes are on
different subnets joined by a router.
- E3 (FS → Control Plane) goes directly to `172.31.30.216:8080`.
- E4 (Control Plane → FS) is **source-NATed by the router to `192.168.48.1`**, so
  the FreeSWITCH ESL ACL allows `192.168.48.1` (not the dev-box IP).

---

## 5. Security layers (annotate the edges)

- **E2 `/api/v1/*`**: TLS + static **Bearer token** (`dev-token`). Management only.
- **E3 `/xml/*`**: TLS + **mutual TLS** (FreeSWITCH presents a client cert signed
  by a private CA) + **HTTP Basic auth** (`gateway-credentials`) + a "not found"
  fallback so it can never break FreeSWITCH's on-disk config. The directory
  response carries **`a1-hash`**, never the plaintext SIP password.
- **E4 ESL**: non-default password + `apply-inbound-acl` (loopback + `192.168.48.1`).
- **E5 SIP**: digest auth; FreeSWITCH validates against the `a1-hash` from E3.
- TLS material: a small self-signed CA → server cert (for the Control Plane,
  SAN includes `172.31.30.216`/`127.0.0.1`/`localhost`) + a client cert (for
  FreeSWITCH). Audit logs in the DB redact passwords.

---

## 6. Runtime data flows (sequence arrows for the diagram)

### Flow 1 — Declarative config change (Terraform)
```
Operator → Terraform(3) --E2 HTTPS/Bearer--> Control Plane(2) → PostgreSQL(1)
Terraform(3) --E2 POST /runtime/reloadxml--> Control Plane(2) --E4 ESL--> FreeSWITCH(4)
```

### Flow 2 — SIP registration (directory pull)
```
Softphone(5) --E5 REGISTER--> FreeSWITCH(4a)
FreeSWITCH(4b) --E3 HTTPS POST /xml/directory--> Control Plane(2) → PostgreSQL(1)
Control Plane returns <user a1-hash> → FreeSWITCH verifies digest → 200 OK
(unknown users → "not found" → FreeSWITCH falls back to local files)
```

### Flow 3 — Call into an IVR (dialplan pull + TTS)
```
Softphone(5) --E5 dial 3333--> FreeSWITCH(4a)
FreeSWITCH(4b) --E3 HTTPS POST /xml/dialplan (context=company)--> Control Plane(2) → PostgreSQL(1)
FreeSWITCH runs the extension: answer → speak (mod_flite TTS, 4d) → play_and_get_digits
→ transfer to submenu → ... (media over E6/RTP)
```

---

## 7. Logical pipeline (high-level arrow chain)

```
Terraform/OpenTofu
   → Control Plane REST API (Go)
      → PostgreSQL (desired state)
      → XML renderer ──(mod_xml_curl, HTTPS+mTLS)──> FreeSWITCH
   → ESL client ──(reloadxml/status)──> FreeSWITCH (mod_event_socket)
SIP softphones ──(SIP/RTP)──> FreeSWITCH
```

---

## 8. What is deployed right now (optional labels)

- DB entities: domain `192.168.48.143`; users `2001`, `2002`, `4100`; several
  dialplan/IVR extensions.
- IVRs (all data-driven, served via mod_xml_curl): `5000` (demo),
  `7000/7100` (nested), `8000/8100` (nested, Terraform), `3333` "Universal IVR"
  (nested, TTS, male voice `rms`): main → SRE submenu (Platform/Cloudservices/
  DevSupport/DevPortal) and Developers submenu (Golang/Kotlin).
- Test SIP user for the softphone: `4100` / password: see `deploy/SECRETS.md`, domain
  `192.168.48.143`.

---

## 9. Suggested diagram layout (hint for the image generator)

- Three rounded boxes: **"Dev / Control box (WSL2 172.31.30.216)"** containing
  *PostgreSQL* and *Control Plane API* (and a small *Terraform CLI* icon);
  **"FreeSWITCH server (192.168.48.143)"** containing *mod_sofia*, *mod_xml_curl*,
  *mod_event_socket*, *mod_flite*; and a **"Clients"** group with a *SIP
  softphone*.
- Put a **Router/NAT** element between the two server boxes; label the Zone A→B
  arrows as SNAT→`192.168.48.1`.
- Draw arrows E1–E6 with their protocol+port labels and a small lock icon on the
  authenticated ones (E2 Bearer/TLS; E3 mTLS+Basic; E4 ESL pass+ACL; E5 SIP
  digest/a1-hash).
- Use a dashed arrow for "pull" (E3, FreeSWITCH→Control Plane) and solid for
  "push" (E2, E4). Show RTP (E6) as a double-headed media arrow between the
  softphone and FreeSWITCH.
- Color suggestion: blue = control/management plane (E2, E4, API, DB),
  green = config delivery (E3 XML), orange = SIP/RTP media (E5, E6).
