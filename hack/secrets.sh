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
  "deploy/freeswitch/json_cdr.conf.xml"
  "deploy/freeswitch/nginx/recordings.htpasswd"
)
# deploy/tls/ is a directory -> tarball deploy/tls.tar.age
TLS_DIR="deploy/tls"

recipient() { /usr/bin/age-keygen -y "$KEY"; }

# fileUnchanged <plain> — true if <plain>.age exists and decrypts to the exact
# same bytes as <plain>. Lets encrypt skip rewriting unchanged secrets (age uses
# a random nonce, so re-encrypting always changes the ciphertext = noisy diffs).
fileUnchanged() {
  local f="$1"
  [ -f "$DIR/$f.age" ] || return 1
  $AGE -d -i "$KEY" "$DIR/$f.age" 2>/dev/null | cmp -s - "$DIR/$f"
}

# content signature of a directory tree: sorted "sha256  relpath" lines.
tlsSig() { (cd "$1" && find . -type f -exec sha256sum {} \; | sort); }

# tlsUnchanged — true if tls.tar.age decrypts to the same file contents as the
# current deploy/tls/ tree (ignores tar mtimes).
tlsUnchanged() {
  [ -f "$DIR/deploy/tls.tar.age" ] || return 1
  local tmp cur old
  tmp=$(mktemp -d)
  $AGE -d -i "$KEY" "$DIR/deploy/tls.tar.age" 2>/dev/null | tar -C "$tmp" -xf - 2>/dev/null || { rm -rf "$tmp"; return 1; }
  cur=$(tlsSig "$DIR/$TLS_DIR")
  old=$(tlsSig "$tmp/$TLS_DIR")
  rm -rf "$tmp"
  [ "$cur" = "$old" ]
}

case "${1:-}" in
  encrypt)
    R=$(recipient)
    for f in "${FILES[@]}"; do
      [ -f "$DIR/$f" ] || { echo "skip (missing): $f"; continue; }
      if fileUnchanged "$f"; then
        echo "unchanged: $f.age"
        continue
      fi
      $AGE -r "$R" -o "$DIR/$f.age" "$DIR/$f"
      echo "encrypted: $f.age"
    done
    if [ -d "$DIR/$TLS_DIR" ]; then
      if tlsUnchanged; then
        echo "unchanged: deploy/tls.tar.age"
      else
        tar -C "$DIR" -cf - "$TLS_DIR" | $AGE -r "$R" -o "$DIR/deploy/tls.tar.age"
        echo "encrypted: deploy/tls.tar.age"
      fi
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
