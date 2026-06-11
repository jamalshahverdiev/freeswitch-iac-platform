#!/usr/bin/env python3
"""Minimal SIP REGISTER with MD5 digest auth — proves a user can register.

Used to verify the control-plane directory (incl. a1-hash auth) end to end
without a softphone. Sends REGISTER, expects a 401 challenge, answers with the
digest, expects 200 OK.

Usage:
  hack/sip_register_test.py [user] [password] [domain] [host] [port]
  defaults: 2001 2580 192.168.48.143 192.168.48.143 5060
"""
import hashlib
import random
import re
import socket
import sys

USER = sys.argv[1] if len(sys.argv) > 1 else "2001"
PASSWORD = sys.argv[2] if len(sys.argv) > 2 else "2580"
DOMAIN = sys.argv[3] if len(sys.argv) > 3 else "192.168.48.143"
HOST = sys.argv[4] if len(sys.argv) > 4 else DOMAIN
PORT = int(sys.argv[5]) if len(sys.argv) > 5 else 5060


def md5(x: str) -> str:
    return hashlib.md5(x.encode()).hexdigest()


def main() -> int:
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.settimeout(5)
    s.connect((HOST, PORT))
    localip, lport = s.getsockname()

    callid = "%x" % random.getrandbits(64)
    fromtag = "%x" % random.getrandbits(32)
    branch = "z9hG4bK%x" % random.getrandbits(32)
    uri = "sip:%s" % DOMAIN

    def register(cseq, auth=None):
        lines = [
            "REGISTER %s SIP/2.0" % uri,
            "Via: SIP/2.0/UDP %s:%d;branch=%s%d;rport" % (localip, lport, branch, cseq),
            "Max-Forwards: 70",
            "From: <sip:%s@%s>;tag=%s" % (USER, DOMAIN, fromtag),
            "To: <sip:%s@%s>" % (USER, DOMAIN),
            "Call-ID: %s" % callid,
            "CSeq: %d REGISTER" % cseq,
            "Contact: <sip:%s@%s:%d>" % (USER, localip, lport),
            "Expires: 60",
        ]
        if auth:
            lines.append(auth)
        lines += ["Content-Length: 0", "", ""]
        s.send("\r\n".join(lines).encode())
        return s.recv(4096).decode(errors="replace")

    r1 = register(1)
    print("first  :", r1.splitlines()[0])
    m = re.search(r"WWW-Authenticate:\s*Digest\s*(.*)", r1, re.I)
    if not m:
        print("no challenge received; full response:\n", r1)
        return 1
    chal = m.group(1)
    realm = re.search(r'realm="([^"]*)"', chal).group(1)
    nonce = re.search(r'nonce="([^"]*)"', chal).group(1)
    print("challenge: realm=%r nonce=%r" % (realm, nonce))

    ha1 = md5("%s:%s:%s" % (USER, realm, PASSWORD))
    ha2 = md5("REGISTER:%s" % uri)
    resp = md5("%s:%s:%s" % (ha1, nonce, ha2))
    print("client HA1 (=a1-hash) = %s" % ha1)
    auth = (
        'Authorization: Digest username="%s", realm="%s", nonce="%s", uri="%s", '
        'response="%s", algorithm=MD5' % (USER, realm, nonce, uri, resp)
    )

    r2 = register(2, auth)
    status = r2.splitlines()[0]
    print("second :", status)
    return 0 if "200" in status else 2


if __name__ == "__main__":
    sys.exit(main())
