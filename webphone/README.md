# webphone — self-hosted WebRTC softphone UI

[Browser-Phone](https://github.com/InnovateAsterisk/Browser-Phone) (static SPA
on SIP.js) served by nginx over HTTPS with our CA-signed cert. Lets us test the
full WebRTC stack against FreeSWITCH — audio, **video**, in-call DTMF keypad,
hold/transfer, call recording, screen share — without third-party sites.

## Run

Part of the main `docker-compose.yml`:

```bash
docker compose up -d --build webphone
```

Open **https://172.31.30.216:8443** (the dev box IP; CA `deploy/tls/ca.crt`
must be trusted on the client machine — see `docs/webrtc.md`).

## Account settings (gear icon → Account)

| Field | Value |
|---|---|
| Asterisk/SIP Domain | `192.168.48.143` |
| WebSocket Server | `192.168.48.143` |
| WebSocket Port | `7443` |
| WebSocket Path | `/` |
| Username / Auth user | `4201` (or `4202`, `4100`) |
| Password | see `deploy/SECRETS.md` |

Then: call `9196` (video echo — see/hear yourself), `3333` (TTS IVR; use the
in-call keypad for digits), `4202`/`4100` for user-to-user (video) calls.

## License note

Browser-Phone is **AGPL-3.0** (the bundled SIP.js engine is MIT). Modifying it
for internal/lab use is fine; offering a modified version to external users
requires publishing the modifications. For a future closed UI, build on SIP.js
directly.
