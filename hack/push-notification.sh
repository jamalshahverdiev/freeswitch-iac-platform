#!/usr/bin/env bash
#
# push-notification.sh — send a test Web Push to the webphone's registered
# browser subscriptions (dev/test helper, NOT used in production).
#
# In production the push is sent by the control-plane WebPusher on a new
# voicemail (voicemail.mwi event). This script reproduces that send manually
# for testing: it reads the VAPID keypair from .env and the stored browser
# subscriptions from the control database, then signs + encrypts a payload and
# delivers it via the `web-push` CLI.
#
# Usage:
#   ./push-notification.sh                       # default title/body, all subscribers
#   ./push-notification.sh "Title" "Body text"   # custom message
#   ./push-notification.sh -l                     # list subscriptions and exit
#
# Requirements: docker (postgres container running), node/npx.
# Overridable via env: ENV_FILE, PG_CONTAINER, PG_USER, PG_DB.

set -euo pipefail

ENV_FILE="${ENV_FILE:-$(dirname "$0")/../.env}"
PG_CONTAINER="${PG_CONTAINER:-freeswitch-iac-platform-postgres-1}"
PG_USER="${PG_USER:-freeswitch}"
PG_DB="${PG_DB:-freeswitch_control}"

psql_q() { docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" "$@"; }

# --- list mode ---------------------------------------------------------------
if [[ "${1:-}" == "-l" || "${1:-}" == "--list" ]]; then
  echo "Registered push subscriptions:"
  psql_q -c "SELECT number, split_part(endpoint,'/',3) AS push_service, created_at
             FROM push_subscriptions ORDER BY created_at;"
  exit 0
fi

TITLE="${1:-Webphone}"
BODY="${2:-Тестовое уведомление от push-notification.sh}"

# --- load VAPID keys from .env -----------------------------------------------
if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERROR: env file not found: $ENV_FILE" >&2
  exit 1
fi
set -a; . "$ENV_FILE"; set +a

: "${VAPID_PUBLIC_KEY:?VAPID_PUBLIC_KEY is not set in $ENV_FILE}"
: "${VAPID_PRIVATE_KEY:?VAPID_PRIVATE_KEY is not set in $ENV_FILE}"
VAPID_SUBJECT="${VAPID_SUBJECT:-mailto:admin@example.com}"

PAYLOAD=$(printf '{"title":%s,"body":%s}' \
  "$(printf '%s' "$TITLE" | sed 's/"/\\"/g; s/^/"/; s/$/"/')" \
  "$(printf '%s' "$BODY"  | sed 's/"/\\"/g; s/^/"/; s/$/"/')")

# --- fetch subscriptions -----------------------------------------------------
mapfile -t SUBS < <(psql_q -tA -F'|' -c \
  "SELECT endpoint, p256dh, auth, number FROM push_subscriptions")

if [[ ${#SUBS[@]} -eq 0 ]]; then
  echo "No subscriptions found. Log in to the webphone and allow notifications first."
  exit 0
fi

echo "Sending to ${#SUBS[@]} subscription(s): \"$TITLE\" — \"$BODY\""
echo

for row in "${SUBS[@]}"; do
  IFS='|' read -r EP P256 AUTH NUMBER <<< "$row"
  printf 'ext %-6s %-20s ... ' "$NUMBER" "$(echo "$EP" | cut -d/ -f3)"
  if npx --yes web-push send-notification \
      --endpoint="$EP" --key="$P256" --auth="$AUTH" \
      --vapid-subject="$VAPID_SUBJECT" \
      --vapid-pubkey="$VAPID_PUBLIC_KEY" \
      --vapid-pvtkey="$VAPID_PRIVATE_KEY" \
      --payload="$PAYLOAD" >/dev/null 2>&1; then
    echo "OK (sent)"
  else
    echo "FAILED (403 = wrong VAPID key / re-subscribe; 404/410 = dead, prune row)"
  fi
done
