# WebRTC accounts (browser softphone)

## TL;DR

A "WebRTC account" in FreeSWITCH is a **regular directory user** — the same
`freeswitch_user` Terraform resource. WebRTC only changes the *transport*:
the browser registers over **SIP-over-WSS** (`wss://192.168.48.143:7443`)
instead of UDP 5060, media runs over **DTLS-SRTP** with the **Opus** codec.
Nothing special is stored per-user.

Accounts created for this (see `examples/webrtc-users/`): **4201 / 4202**,
password: see `deploy/SECRETS.md`, domain `192.168.48.143`, context `company` (can dial the
`3333` IVR, `4100`, `2001`, ...).

## What is enabled on the server

| Piece | State |
|---|---|
| `ws-binding` (plain WebSocket) | `192.168.48.143:5066` |
| `wss-binding` (TLS WebSocket) | `192.168.48.143:7443` |
| `mod_opus` / `mod_rtc` / `mod_verto` | loaded |
| `wss.pem` | **our CA-signed cert** (CN/SAN `192.168.48.143`, issuer `fs-iac-ca`) |
| `dtls-srtp.pem` | FreeSWITCH default (fine — DTLS uses SDP fingerprints) |

The WSS certificate is issued by our private CA with `hack/gen-wss-cert.sh`
(outputs `deploy/tls/wss.pem` = cert+key+CA, shipped to
`/etc/freeswitch/tls/wss.pem`, then `sofia profile internal restart`).

## Browser setup (Windows)

### 1. Trust the CA (one of two ways)

- **Proper:** import `deploy/tls/ca.crt` into Windows:
  double-click `ca.crt` → *Install Certificate* → *Local Machine* →
  *Place all certificates in the following store* → **Trusted Root
  Certification Authorities**. Restart the browser.
- **Quick hack:** open `https://192.168.48.143:7443` in the browser and accept
  the security warning (creates a per-origin exception that also unblocks
  `wss://` to the same host:port).

### 2. Use a WebRTC SIP client

Easiest, no install — the JsSIP demo (browser-based):

1. Open <https://tryit.jssip.net>.
2. Settings:

   | Field | Value |
   |---|---|
   | SIP URI | `sip:4201@192.168.48.143` |
   | SIP password | see `deploy/SECRETS.md` |
   | WebSocket URI | `wss://192.168.48.143:7443` |
3. Save → it registers (green). Allow microphone access when asked.
4. Dial `3333` → the Terraform IVR answers (TTS menu); or call `4100` / `4202`.

Any SIP.js/JsSIP-based webphone works the same way; for app-style clients the
same credentials apply.

## How it flows (for understanding)

```
Browser ──wss://192.168.48.143:7443 (SIP over WebSocket, TLS via our CA)──► mod_sofia
Browser ◄──DTLS-SRTP media (Opus), ICE──► FreeSWITCH RTP (16384-32768/udp)
REGISTER auth: digest, validated against the a1-hash served by the
control-plane via mod_xml_curl — exactly like a desktop softphone.
```

## Troubleshooting

- **WSS won't connect** → CA not trusted (see step 1), or firewall blocks 7443.
  Test the cert: `openssl s_client -connect 192.168.48.143:7443` →
  `issuer=CN=fs-iac-ca`.
- **Registers but no audio** → mic permission denied, or ICE failed (check both
  ends are on the LAN; FreeSWITCH advertises its STUN-detected external IP, ICE
  host candidates cover the LAN path).
- **401/403 on register** → wrong password/domain; verify the user exists:
  `curl -s --cacert deploy/tls/ca.crt -H 'Authorization: Bearer dev-token' \
   https://localhost:8080/api/v1/users/192.168.48.143/4201`.
- Watch live: `fs_cli -x "sofia status profile internal reg"` shows the WSS
  registration with a `transport=wss` contact.
