#!/usr/bin/env bash
# Seed the control-plane with a live demo served to FreeSWITCH via mod_xml_curl:
#   - domain 192.168.48.143  (the FS server's $${domain}, so registration works
#     without touching the sofia profile)
#   - NEW users 2001 / 2002  (NOT the default 1000-1019), context "company"
#   - an IVR auto-attendant at 5000, plus internal routing — all in DB.
#
# Usage: API=https://localhost:8080 TOKEN=dev-token ./deploy/seed.sh
set -euo pipefail

API="${API:-https://localhost:8080}"
TOKEN="${TOKEN:-dev-token}"
DOMAIN="${DOMAIN:-192.168.48.143}"
USER_PASS="${USER_PASS:-2580}"
AUTH=(-H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json")

# TLS (control-plane runs HTTPS; /xml also needs the client cert for mTLS).
TLSDIR="$(cd "$(dirname "$0")/.." && pwd)/deploy/tls"
CACERT=(); [ -f "$TLSDIR/ca.crt" ] && CACERT=(--cacert "$TLSDIR/ca.crt")
XMLCERT=(); [ -f "$TLSDIR/client.crt" ] && XMLCERT=(--cert "$TLSDIR/client.crt" --key "$TLSDIR/client.key" -u "freeswitch:${XML_PASSWORD:?set XML_PASSWORD or source .env}")

post() { # path json
  curl -fsS "${CACERT[@]}" "${API}$1" "${AUTH[@]}" -d "$2" >/dev/null && echo "   ok" || echo "   (exists / skipped)"
}

echo "==> domain ${DOMAIN}"
post /api/v1/domains "{\"name\":\"${DOMAIN}\",\"description\":\"IaC managed domain\",\"enabled\":true,
  \"variables\":{\"default_language\":\"en\"}}"

for n in 2001 2002; do
  echo "==> user ${n}"
  post /api/v1/users "{
    \"domain\":\"${DOMAIN}\",\"number\":\"${n}\",\"enabled\":true,
    \"params\":{\"password\":\"${USER_PASS}\",\"vm-password\":\"${n}\"},
    \"variables\":{
      \"effective_caller_id_name\":\"IaC User ${n}\",
      \"effective_caller_id_number\":\"${n}\",
      \"user_context\":\"company\"
    }}"
done

echo "==> dialplan: internal routing 20xx (context company)"
post /api/v1/dialplan/extensions "{
  \"name\":\"internal-2xxx\",\"domain\":\"${DOMAIN}\",\"context\":\"company\",\"priority\":10,\"enabled\":true,
  \"conditions\":[{\"field\":\"destination_number\",\"expression\":\"^(20[0-9][0-9])\$\",
    \"actions\":[{\"application\":\"bridge\",\"data\":\"user/\$1@${DOMAIN}\"}]}]}"

echo "==> dialplan: IVR menu at 5000 (context company)"
post /api/v1/dialplan/extensions "{
  \"name\":\"ivr-menu\",\"domain\":\"${DOMAIN}\",\"context\":\"company\",\"priority\":20,\"enabled\":true,
  \"conditions\":[{\"field\":\"destination_number\",\"expression\":\"^(5000)\$\",
    \"actions\":[
      {\"application\":\"answer\"},
      {\"application\":\"sleep\",\"data\":\"500\"},
      {\"application\":\"play_and_get_digits\",\"data\":\"1 1 3 5000 # ivr/ivr-welcome_to_freeswitch.wav ivr/ivr-that_was_an_invalid_entry.wav ivr_choice \\\\d 3000\"},
      {\"application\":\"transfer\",\"data\":\"menu_choice_\${ivr_choice} XML company\"}
    ]}]}"

echo "==> dialplan: IVR choice 1 -> call 2001"
post /api/v1/dialplan/extensions "{
  \"name\":\"ivr-choice-1\",\"domain\":\"${DOMAIN}\",\"context\":\"company\",\"priority\":30,\"enabled\":true,
  \"conditions\":[{\"field\":\"destination_number\",\"expression\":\"^menu_choice_1\$\",
    \"actions\":[{\"application\":\"transfer\",\"data\":\"2001 XML company\"}]}]}"

echo "==> dialplan: IVR choice 2 -> call 2002"
post /api/v1/dialplan/extensions "{
  \"name\":\"ivr-choice-2\",\"domain\":\"${DOMAIN}\",\"context\":\"company\",\"priority\":40,\"enabled\":true,
  \"conditions\":[{\"field\":\"destination_number\",\"expression\":\"^menu_choice_2\$\",
    \"actions\":[{\"application\":\"transfer\",\"data\":\"2002 XML company\"}]}]}"

echo "==> dialplan: IVR choice 3 -> echo test"
post /api/v1/dialplan/extensions "{
  \"name\":\"ivr-choice-3\",\"domain\":\"${DOMAIN}\",\"context\":\"company\",\"priority\":50,\"enabled\":true,
  \"conditions\":[{\"field\":\"destination_number\",\"expression\":\"^menu_choice_3\$\",
    \"actions\":[{\"application\":\"answer\"},{\"application\":\"echo\"}]}]}"

echo
echo "==> /xml/dialplan?context=company:"
curl -fsS "${CACERT[@]}" "${XMLCERT[@]}" "${API}/xml/dialplan" -d "context=company"
