    #!/usr/bin/env bash
# Generate a self-signed CA, a server cert for the control-plane (with IP SANs)
# and a client cert for FreeSWITCH's mod_xml_curl (mTLS on /xml/*).
# Output: deploy/tls/{ca.crt,server.crt,server.key,client.crt,client.key}
# Re-runnable. Keys are gitignored.
set -euo pipefail

OUT="$(cd "$(dirname "$0")/.." && pwd)/deploy/tls"
mkdir -p "$OUT"
cd "$OUT"

DAYS=3650
# IPs/hostnames the control-plane is reached by (server cert SANs).
SERVER_SAN="${SERVER_SAN:-IP:172.31.30.216,IP:127.0.0.1,DNS:localhost}"

echo "== CA =="
# Proper CA extensions (basicConstraints + keyUsage) so strict validators
# (e.g. Python 3.13+ VERIFY_X509_STRICT) accept the chain too.
openssl req -x509 -newkey rsa:2048 -nodes -keyout ca.key -out ca.crt -days "$DAYS" \
  -subj "/CN=fs-iac-ca" \
  -addext "basicConstraints=critical,CA:true" \
  -addext "keyUsage=critical,keyCertSign,cRLSign" 2>/dev/null

echo "== server cert (SAN: $SERVER_SAN) =="
openssl req -newkey rsa:2048 -nodes -keyout server.key -out server.csr \
  -subj "/CN=fs-iac-control-plane" 2>/dev/null
cat > server.ext <<EOF
subjectAltName=$SERVER_SAN
extendedKeyUsage=serverAuth
EOF
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt -days "$DAYS" -extfile server.ext 2>/dev/null

echo "== client cert (for FreeSWITCH mod_xml_curl) =="
openssl req -newkey rsa:2048 -nodes -keyout client.key -out client.csr \
  -subj "/CN=freeswitch-xml-client" 2>/dev/null
cat > client.ext <<EOF
extendedKeyUsage=clientAuth
EOF
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out client.crt -days "$DAYS" -extfile client.ext 2>/dev/null

# FreeSWITCH mod_xml_curl wants the client cert+key in one PEM file.
cat client.crt client.key > client.pem

rm -f server.csr client.csr server.ext client.ext
# World-readable so the non-root control-plane container can read the mounted
# key. This is local dev TLS material (gitignored); use proper secret mounts /
# perms in production.
chmod 644 *.key *.crt client.pem
echo "== done =="
ls -1 "$OUT"
