# Self-hosted webphone (Browser-Phone) — audio & video testing

Our own WebRTC softphone UI, served from the dev box — replaces
tryit.jssip.net. Audio, **video**, in-call DTMF keypad, hold/transfer, call
recording, screen share. Implementation details: `webphone/README.md`
(nginx + Browser-Phone static, AGPL note).

## Open it

```
https://172.31.30.216:8443
```

Requirements on the client (Windows) machine:
- `deploy/tls/ca.crt` imported into *Trusted Root Certification Authorities*
  (already done for the WSS setup — same CA signs this page's cert).
- Microphone (and camera for video) permission for the site.

## Configure the account (gear icon → Account / Settings)

| Field | Value |
|---|---|
| Asterisk / SIP Domain | `192.168.48.143` |
| WebSocket Server | `192.168.48.143` |
| WebSocket Port | `7443` |
| WebSocket Path | `/` |
| Username (auth user) | `4201` (or `4202`, `4100`) |
| Password | see `deploy/SECRETS.md` |

Save → the phone registers (status turns green). Registration goes over
SIP-on-WSS with digest auth against the control-plane-served `a1-hash` —
same as any softphone.

## Test plan

| # | Dial | What it proves |
|---|---|---|
| 1 | `9196` | **Video echo** — you see & hear yourself mirrored by FreeSWITCH (`echo` app echoes audio AND video). Single-client full-media test. |
| 2 | `3333` | TTS IVR (Piper voice). Use the **in-call keypad** for digits: 1 → SRE submenu (1-4), 2 → Developers (1-2). Proves DTMF (RFC2833, payload 101). |
| 3 | `4202` or `4100` | User-to-user call. Open a second browser tab/device registered as `4202` for a browser↔browser **video call** (VP8 passthrough), or call `4100` to ring Zoiper. |
| 4 | `9100` / `9200` / `7000` / `8000` / `5000` | Other IVR/TTS/prompt demos from previous sessions. |

The `9196` extension is Terraform-managed: `examples/video-echo/`.

## Troubleshooting

- **Page won't open / cert warning** → CA not trusted; re-check the import,
  restart the browser. Verify: `curl --cacert deploy/tls/ca.crt
  https://172.31.30.216:8443` → 200.
- **Doesn't register** → check WSS values above (path `/`), and that 7443 is
  reachable; `fs_cli -x "sofia status profile internal reg"` on the server
  shows the registration with `transport=ws`.
- **No video in echo** → camera permission; Browser-Phone settings → enable
  video / select camera device.
- **Digits ignored in IVR** → use the keypad inside the active call window
  (not the main dialer input).
