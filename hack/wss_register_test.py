#!/usr/bin/env python3
"""SIP REGISTER over WSS (RFC 7118) — verifies the exact signaling path a
browser WebRTC client uses: TLS (validated against our CA) -> WebSocket
upgrade (Sec-WebSocket-Protocol: sip) -> SIP digest registration.

Usage:
  hack/wss_register_test.py [user] [password] [domain] [host] [port] [ca]
  defaults: 4201 $SIP_TEST_PASS 192.168.48.143 192.168.48.143 7443 deploy/tls/ca.crt
"""
import base64
import hashlib
import os
import re
import socket
import ssl
import sys

USER = sys.argv[1] if len(sys.argv) > 1 else "4201"
PASSWORD = sys.argv[2] if len(sys.argv) > 2 else os.environ.get("SIP_TEST_PASS", "")
DOMAIN = sys.argv[3] if len(sys.argv) > 3 else "192.168.48.143"
HOST = sys.argv[4] if len(sys.argv) > 4 else DOMAIN
PORT = int(sys.argv[5]) if len(sys.argv) > 5 else 7443
CA = sys.argv[6] if len(sys.argv) > 6 else os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "deploy", "tls", "ca.crt")


def md5(x: str) -> str:
    return hashlib.md5(x.encode()).hexdigest()


def ws_send_text(sock, data: bytes) -> None:
    mask = os.urandom(4)
    n = len(data)
    hdr = bytearray([0x81])  # FIN + text
    if n < 126:
        hdr.append(0x80 | n)
    elif n < 65536:
        hdr.append(0x80 | 126)
        hdr += n.to_bytes(2, "big")
    else:
        hdr.append(0x80 | 127)
        hdr += n.to_bytes(8, "big")
    hdr += mask
    sock.sendall(bytes(hdr) + bytes(b ^ mask[i % 4] for i, b in enumerate(data)))


def ws_recv_text(sock) -> bytes:
    def readn(n):
        buf = b""
        while len(buf) < n:
            chunk = sock.recv(n - len(buf))
            if not chunk:
                raise ConnectionError("websocket closed")
            buf += chunk
        return buf

    while True:
        b1, b2 = readn(2)
        opcode = b1 & 0x0F
        ln = b2 & 0x7F
        if ln == 126:
            ln = int.from_bytes(readn(2), "big")
        elif ln == 127:
            ln = int.from_bytes(readn(8), "big")
        mask = readn(4) if b2 & 0x80 else b""
        payload = readn(ln)
        if mask:
            payload = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))
        if opcode == 0x9:  # ping -> pong
            m = os.urandom(4)
            sock.sendall(bytes([0x8A, 0x80 | len(payload)]) + m +
                         bytes(b ^ m[i % 4] for i, b in enumerate(payload)))
            continue
        if opcode == 0x8:
            raise ConnectionError("websocket close frame")
        return payload


def main() -> int:
    # 1) TLS with OUR CA (same trust decision a browser makes).
    # Python 3.13+ enables VERIFY_X509_STRICT which demands keyUsage on the CA;
    # browsers/curl don't — relax it to match their behaviour.
    ctx = ssl.create_default_context(cafile=CA)
    if hasattr(ssl, "VERIFY_X509_STRICT"):
        ctx.verify_flags &= ~ssl.VERIFY_X509_STRICT
    raw = socket.create_connection((HOST, PORT), timeout=8)
    s = ctx.wrap_socket(raw, server_hostname=HOST)
    print("tls    : ok, peer=%s issuer=%s" % (
        dict(x[0] for x in s.getpeercert()["subject"])["commonName"],
        dict(x[0] for x in s.getpeercert()["issuer"])["commonName"]))

    # 2) WebSocket upgrade with SIP subprotocol
    key = base64.b64encode(os.urandom(16)).decode()
    s.sendall((
        "GET / HTTP/1.1\r\n"
        f"Host: {HOST}:{PORT}\r\n"
        "Upgrade: websocket\r\n"
        "Connection: Upgrade\r\n"
        f"Sec-WebSocket-Key: {key}\r\n"
        "Sec-WebSocket-Version: 13\r\n"
        "Sec-WebSocket-Protocol: sip\r\n\r\n").encode())
    resp = b""
    while b"\r\n\r\n" not in resp:
        resp += s.recv(1024)
    status = resp.split(b"\r\n", 1)[0].decode()
    print("upgrade:", status)
    if "101" not in status:
        print(resp.decode(errors="replace"))
        return 1

    # 3) SIP REGISTER over the websocket (RFC 7118: .invalid Via/Contact host)
    inst = "%x" % int.from_bytes(os.urandom(4), "big")
    via_host = f"{inst}.invalid"
    callid = "%x" % int.from_bytes(os.urandom(8), "big")
    tag = "%x" % int.from_bytes(os.urandom(4), "big")
    uri = f"sip:{DOMAIN}"

    def register(cseq, auth=None):
        lines = [
            f"REGISTER {uri} SIP/2.0",
            f"Via: SIP/2.0/WSS {via_host};branch=z9hG4bK{os.urandom(4).hex()}",
            "Max-Forwards: 70",
            f"From: <sip:{USER}@{DOMAIN}>;tag={tag}",
            f"To: <sip:{USER}@{DOMAIN}>",
            f"Call-ID: {callid}",
            f"CSeq: {cseq} REGISTER",
            f"Contact: <sip:{USER}@{via_host};transport=ws>",
            "Expires: 60",
        ]
        if auth:
            lines.append(auth)
        lines += ["Content-Length: 0", "", ""]
        ws_send_text(s, "\r\n".join(lines).encode())
        return ws_recv_text(s).decode(errors="replace")

    r1 = register(1)
    print("first  :", r1.splitlines()[0])
    m = re.search(r"WWW-Authenticate:\s*Digest\s*(.*)", r1, re.I)
    if not m:
        print(r1)
        return 1
    realm = re.search(r'realm="([^"]*)"', m.group(1)).group(1)
    nonce = re.search(r'nonce="([^"]*)"', m.group(1)).group(1)
    ha1 = md5(f"{USER}:{realm}:{PASSWORD}")
    resp_digest = md5(f"{ha1}:{nonce}:{md5('REGISTER:' + uri)}")
    auth = (f'Authorization: Digest username="{USER}", realm="{realm}", '
            f'nonce="{nonce}", uri="{uri}", response="{resp_digest}", algorithm=MD5')
    r2 = register(2, auth)
    print("second :", r2.splitlines()[0])
    return 0 if "200" in r2.splitlines()[0] else 2


if __name__ == "__main__":
    sys.exit(main())
