#!/usr/bin/env bash
# Issue a WSS certificate for the FreeSWITCH server, signed by our own CA
# (deploy/tls/ca.{crt,key} from hack/gen-tls.sh), and build wss.pem in the
# format FreeSWITCH expects (certificate + private key concatenated).
# Output: deploy/tls/wss-server.{crt,key} + deploy/tls/wss.pem
set -euo pipefail

OUT="$(cd "$(dirname "$0")/.." && pwd)/deploy/tls"
cd "$OUT"
[ -f ca.crt ] && [ -f ca.key ] || { echo "run hack/gen-tls.sh first (need ca.crt/ca.key)"; exit 1; }

DAYS=3650
SAN="${SAN:-IP:192.168.48.143,DNS:freeswitch.local}"

openssl req -newkey rsa:2048 -nodes -keyout wss-server.key -out wss-server.csr \
  -subj "/CN=192.168.48.143" 2>/dev/null
cat > wss-server.ext <<EOF
subjectAltName=$SAN
extendedKeyUsage=serverAuth
EOF
openssl x509 -req -in wss-server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out wss-server.crt -days "$DAYS" -extfile wss-server.ext 2>/dev/null

# FreeSWITCH wss.pem = cert + key (+ optionally the CA chain) in one PEM.
cat wss-server.crt wss-server.key ca.crt > wss.pem

rm -f wss-server.csr wss-server.ext
chmod 644 wss-server.crt wss-server.key wss.pem
echo "== done =="
openssl x509 -in wss-server.crt -noout -subject -ext subjectAltName
ls -1 wss-server.crt wss-server.key wss.pem
