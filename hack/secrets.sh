#!/usr/bin/env bash
# Encrypt/decrypt the repo's secret files with age.
#
#   hack/secrets.sh encrypt   # plaintext -> *.age (run before committing changes)
#   hack/secrets.sh decrypt   # *.age -> plaintext (run after fresh clone)
#
# Key: ~/.config/age/keys.txt (age-keygen -o ~/.config/age/keys.txt).
# BACK THE KEY UP — without it the .age files are unrecoverable.
set -euo pipefail

AGE=/usr/bin/age   # full path: some shells alias `age` to apt
KEY="${AGE_KEY_FILE:-$HOME/.config/age/keys.txt}"
DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Plain files: encrypted side by side as <path>.age
FILES=(
  ".env"
  "deploy/SECRETS.md"
  "deploy/freeswitch/event_socket.conf.xml"
  "deploy/freeswitch/xml_curl.conf.xml"
  "deploy/freeswitch/nginx/recordings.htpasswd"
)
# deploy/tls/ is a directory -> tarball deploy/tls.tar.age
TLS_DIR="deploy/tls"

recipient() { /usr/bin/age-keygen -y "$KEY"; }

case "${1:-}" in
  encrypt)
    R=$(recipient)
    for f in "${FILES[@]}"; do
      [ -f "$DIR/$f" ] || { echo "skip (missing): $f"; continue; }
      $AGE -r "$R" -o "$DIR/$f.age" "$DIR/$f"
      echo "encrypted: $f.age"
    done
    if [ -d "$DIR/$TLS_DIR" ]; then
      tar -C "$DIR" -cf - "$TLS_DIR" | $AGE -r "$R" -o "$DIR/deploy/tls.tar.age"
      echo "encrypted: deploy/tls.tar.age"
    fi
    ;;
  decrypt)
    [ -f "$KEY" ] || { echo "age key not found: $KEY"; exit 1; }
    for f in "${FILES[@]}"; do
      [ -f "$DIR/$f.age" ] || { echo "skip (missing): $f.age"; continue; }
      $AGE -d -i "$KEY" -o "$DIR/$f" "$DIR/$f.age"
      echo "decrypted: $f"
    done
    if [ -f "$DIR/deploy/tls.tar.age" ]; then
      $AGE -d -i "$KEY" "$DIR/deploy/tls.tar.age" | tar -C "$DIR" -xf -
      echo "decrypted: $TLS_DIR/"
    fi
    ;;
  *)
    echo "usage: $0 encrypt|decrypt"; exit 1 ;;
esac
