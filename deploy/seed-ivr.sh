#!/usr/bin/env bash
# Seed a two-level IVR (main menu 7000 + submenu 7100) entirely from the DB,
# in context "company" / domain 192.168.48.143. Served to FreeSWITCH via
# mod_xml_curl. Re-runnable: deletes existing extensions with the same names.
#
#   7000 main : 1 -> submenu(7100) | 2 -> call 2001 | 3 -> echo
#   7100 sub  : 1 -> call 2002      | 2 -> echo       | 0 -> back to 7000
#
# Usage: API=http://localhost:8080 TOKEN=dev-token ./deploy/seed-ivr.sh
set -euo pipefail
API="${API:-https://localhost:8080}"
TOKEN="${TOKEN:-dev-token}"
DOMAIN="${DOMAIN:-192.168.48.143}"
CTX="${CTX:-company}"
H=(-H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json")
TLSDIR="$(cd "$(dirname "$0")/.." && pwd)/deploy/tls"
CACERT=(); [ -f "$TLSDIR/ca.crt" ] && CACERT=(--cacert "$TLSDIR/ca.crt")
H+=("${CACERT[@]}")

WELCOME="ivr/ivr-welcome_to_freeswitch.wav"
SUBMENU="ivr/ivr-please_enter_the_extension.wav"
INVALID="ivr/ivr-that_was_an_invalid_entry.wav"

# Delete same-named extensions first so the script is idempotent.
existing=$(curl -fsS "${API}/api/v1/dialplan/extensions?context=${CTX}" "${H[@]}")
for name in ivr-main main-1 main-2 main-3 ivr-sub sub-1 sub-2 sub-0; do
  id=$(printf '%s' "$existing" | grep -o "{[^{]*\"name\":\"${name}\"[^}]*}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || true)
  [ -n "${id:-}" ] && curl -fsS -X DELETE "${API}/api/v1/dialplan/extensions/${id}" "${H[@]}" >/dev/null || true
done

ext() { # name priority destexpr actions_json
  curl -fsS "${API}/api/v1/dialplan/extensions" "${H[@]}" -d "{
    \"name\":\"$1\",\"domain\":\"${DOMAIN}\",\"context\":\"${CTX}\",\"priority\":$2,\"enabled\":true,
    \"conditions\":[{\"field\":\"destination_number\",\"expression\":\"$3\",\"actions\":$4}]}" >/dev/null \
    && echo "  ok  $1"
}

echo "== main menu 7000 =="
ext ivr-main 100 '^(7000)$' "[
  {\"application\":\"answer\"},
  {\"application\":\"sleep\",\"data\":\"500\"},
  {\"application\":\"play_and_get_digits\",\"data\":\"1 1 3 5000 # ${WELCOME} ${INVALID} main_choice \\\\d 3000\"},
  {\"application\":\"transfer\",\"data\":\"main_\${main_choice} XML ${CTX}\"}
]"
ext main-1 101 '^main_1$' "[{\"application\":\"transfer\",\"data\":\"7100 XML ${CTX}\"}]"
ext main-2 102 '^main_2$' "[{\"application\":\"transfer\",\"data\":\"2001 XML ${CTX}\"}]"
ext main-3 103 '^main_3$' "[{\"application\":\"answer\"},{\"application\":\"echo\"}]"

echo "== submenu 7100 =="
ext ivr-sub 110 '^(7100)$' "[
  {\"application\":\"answer\"},
  {\"application\":\"play_and_get_digits\",\"data\":\"1 1 3 5000 # ${SUBMENU} ${INVALID} sub_choice \\\\d 3000\"},
  {\"application\":\"transfer\",\"data\":\"sub_\${sub_choice} XML ${CTX}\"}
]"
ext sub-1 111 '^sub_1$' "[{\"application\":\"transfer\",\"data\":\"2002 XML ${CTX}\"}]"
ext sub-2 112 '^sub_2$' "[{\"application\":\"answer\"},{\"application\":\"echo\"}]"
ext sub-0 113 '^sub_0$' "[{\"application\":\"transfer\",\"data\":\"7000 XML ${CTX}\"}]"

echo "== reloadxml via control-plane (ESL) =="
curl -fsS -X POST "${API}/api/v1/runtime/reloadxml" "${H[@]}"; echo
