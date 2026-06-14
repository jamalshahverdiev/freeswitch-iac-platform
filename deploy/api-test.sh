#!/usr/bin/env bash
# Full regression test of the control-plane API.
# Uses a throwaway domain (demo.test) for CRUD so it never touches live data,
# and cleans up at the end. Asserts HTTP status codes and key body content.
#
# Usage: API=http://localhost:8080 TOKEN=dev-token ./deploy/api-test.sh
set -uo pipefail

API="${API:-https://localhost:8080}"
TOKEN="${TOKEN:-dev-token}"
# TLS material (control-plane runs HTTPS + mTLS on /xml). Override CA= for HTTP.
TLSDIR="$(cd "$(dirname "$0")/.." && pwd)/deploy/tls"
CA="${CA:-$TLSDIR/ca.crt}"
CLIENT_CERT="${CLIENT_CERT:-$TLSDIR/client.crt}"
CLIENT_KEY="${CLIENT_KEY:-$TLSDIR/client.key}"
CACERT=(); [ -f "$CA" ] && CACERT=(--cacert "$CA")
CLIENTCERT=(); [ -f "$CLIENT_CERT" ] && CLIENTCERT=(--cert "$CLIENT_CERT" --key "$CLIENT_KEY")
TMP=$(mktemp)
PASS=0; FAIL=0

check() { # desc expected actual
  if [ "$2" = "$3" ]; then printf '  PASS  %-48s [%s]\n' "$1" "$3"; PASS=$((PASS+1))
  else printf '  FAIL  %-48s [expected %s, got %s]\n' "$1" "$2" "$3"; FAIL=$((FAIL+1)); fi
}
contains() { # desc needle
  if grep -q "$2" "$TMP"; then printf '  PASS  %-48s [body contains %s]\n' "$1" "$2"; PASS=$((PASS+1))
  else printf '  FAIL  %-48s [body missing %s]\n' "$1" "$2"; FAIL=$((FAIL+1)); fi
}
# req METHOD PATH [DATA] [--no-auth]  -> CODE, body in $TMP
req() {
  local m=$1 p=$2 data=${3:-} flag=${4:-}
  local hdr=(-H "Authorization: Bearer $TOKEN")
  [ "$flag" = "--no-auth" ] && hdr=()
  if [ -n "$data" ] && [ "$data" != "-" ]; then
    CODE=$(curl -s "${CACERT[@]}" -o "$TMP" -w "%{http_code}" -X "$m" "$API$p" "${hdr[@]}" -H "Content-Type: application/json" -d "$data")
  else
    CODE=$(curl -s "${CACERT[@]}" -o "$TMP" -w "%{http_code}" -X "$m" "$API$p" "${hdr[@]}")
  fi
}
idof() { grep -o '"id":"[^"]*"' "$TMP" | head -1 | cut -d'"' -f4; }

echo "== health =="
req GET /healthz "" --no-auth;            check "GET /healthz" 200 "$CODE"; contains "healthz ok" '"status":"ok"'
req GET /readyz  "" --no-auth;            check "GET /readyz"  200 "$CODE"; contains "readyz db ok" '"database":"ok"'

echo "== auth / validation =="
req GET /api/v1/domains "" --no-auth;     check "no token -> 401" 401 "$CODE"
req POST /api/v1/domains '{"bad":true}';  check "bad domain body -> 400" 400 "$CODE"
req POST /api/v1/users '{"number":"x"}';  check "user without domain -> 400/404" 400 "$CODE"

echo "== domains CRUD (demo.test) =="
req POST /api/v1/domains '{"name":"demo.test","description":"throwaway","enabled":true,"variables":{"k":"v"}}'
check "create domain -> 201" 201 "$CODE"; contains "domain has id" '"id":'
req POST /api/v1/domains '{"name":"demo.test"}';   check "duplicate domain -> 409" 409 "$CODE"
req GET  /api/v1/domains/demo.test;                check "get domain -> 200" 200 "$CODE"; contains "variable kept" '"k":"v"'
req GET  /api/v1/domains;                          check "list domains -> 200" 200 "$CODE"
req PUT  /api/v1/domains/demo.test '{"description":"updated","enabled":false,"variables":{"k":"v2"}}'
check "update domain -> 200" 200 "$CODE"; contains "domain updated" '"k":"v2"'
req GET  /api/v1/domains/nope;                     check "get missing domain -> 404" 404 "$CODE"

echo "== users CRUD =="
# re-enable domain so users render later
req PUT /api/v1/domains/demo.test '{"description":"throwaway","enabled":true,"variables":{}}' >/dev/null
req POST /api/v1/users '{"domain":"demo.test","number":"3001","params":{"password":"p1"},"variables":{"user_context":"demo"}}'
check "create user -> 201" 201 "$CODE"
req POST /api/v1/users '{"domain":"demo.test","number":"3001","params":{"password":"p1"}}'
check "duplicate user -> 409" 409 "$CODE"
req POST /api/v1/users '{"domain":"nope","number":"3001"}'
check "user under missing domain -> 404" 404 "$CODE"
req GET  /api/v1/users/demo.test/3001;             check "get user -> 200" 200 "$CODE"
req GET  "/api/v1/users?domain=demo.test";         check "list users by domain -> 200" 200 "$CODE"; contains "list has 3001" '"number":"3001"'
req PUT  /api/v1/users/demo.test/3001 '{"params":{"password":"p2"},"variables":{"effective_caller_id_name":"Updated"}}'
check "update user -> 200" 200 "$CODE"; contains "user updated" 'Updated'

echo "== gateways CRUD =="
req POST /api/v1/gateways '{"name":"test-trunk","profile":"external","proxy":"sip.example.com","username":"u","password":"s","register":true}'
check "create gateway -> 201" 201 "$CODE"
req POST /api/v1/gateways '{"name":"x","profile":"external"}'; check "gateway without proxy -> 400" 400 "$CODE"
req GET  /api/v1/gateways/external/test-trunk;     check "get gateway -> 200" 200 "$CODE"; contains "gw proxy" 'sip.example.com'
req PUT  /api/v1/gateways/external/test-trunk '{"proxy":"sip2.example.com","register":false}'
check "update gateway -> 200" 200 "$CODE"; contains "gw updated" 'sip2.example.com'

echo "== callcenter CRUD (queue/agent/tier) =="
req POST /api/v1/callcenter/queues '{"name":"q-test@demo.test"}'
check "create queue -> 201" 201 "$CODE"; contains "queue default strategy" 'longest-idle-agent'
req POST /api/v1/callcenter/queues '{"name":"q-test@demo.test"}'; check "duplicate queue -> 409" 409 "$CODE"
req POST /api/v1/callcenter/agents '{"name":"a-test@demo.test","contact":"user/3001@demo.test"}'
check "create agent -> 201" 201 "$CODE"; contains "agent default status" 'Available'
req POST /api/v1/callcenter/tiers '{"queue":"q-test@demo.test","agent":"a-test@demo.test"}'
check "create tier -> 201" 201 "$CODE"
req POST /api/v1/callcenter/tiers '{"queue":"nope","agent":"a-test@demo.test"}'
check "tier with missing queue -> 404" 404 "$CODE"
req GET  /api/v1/callcenter/queues/q-test@demo.test; check "get queue -> 200" 200 "$CODE"
req PUT  /api/v1/callcenter/agents/a-test@demo.test '{"contact":"user/3001@demo.test","status":"Available","max_no_answer":5}'
check "update agent -> 200" 200 "$CODE"; contains "agent updated" '"max_no_answer":5'

echo "== conference CRUD (profile/room) =="
req POST /api/v1/conference/profiles '{"name":"p-test","video_mode":"mux"}'
check "create conf profile -> 201" 201 "$CODE"; contains "profile default layout" 'group:grid'
req POST /api/v1/conference/profiles '{"name":"p-test"}'; check "duplicate profile -> 409" 409 "$CODE"
req POST /api/v1/conference/rooms '{"name":"r-test","number":"3999","domain":"demo.test","context":"demo","profile":"p-test"}'
check "create conf room -> 201" 201 "$CODE"
req POST /api/v1/conference/rooms '{"name":"r-bad","number":"3998","domain":"demo.test","context":"demo","profile":"nope"}'
check "room with missing profile -> 404" 404 "$CODE"
req DELETE /api/v1/conference/profiles/p-test; check "delete profile in use -> 409" 409 "$CODE"
req GET  /api/v1/conference/rooms/r-test; check "get conf room -> 200" 200 "$CODE"
req PUT  /api/v1/conference/rooms/r-test '{"number":"3999","domain":"demo.test","context":"demo","profile":"p-test","pin":"1234"}'
check "update conf room -> 200" 200 "$CODE"; contains "room pin set" '"pin":"1234"'

echo "== dialplan / IVR CRUD (context demo) =="
req POST /api/v1/dialplan/extensions '{"name":"ivr-test","domain":"demo.test","context":"demo","priority":10,"conditions":[{"field":"destination_number","expression":"^(7000)$","actions":[{"application":"answer"},{"application":"playback","data":"ivr/ivr-welcome_to_freeswitch.wav"}]}]}'
check "create ivr extension -> 201" 201 "$CODE"; EXTID=$(idof)
req POST /api/v1/dialplan/extensions '{"name":"bad","domain":"demo.test","context":"demo","conditions":[{"field":"x","expression":"^(*[","actions":[{"application":"answer"}]}]}'
check "bad regex -> 400" 400 "$CODE"
req POST /api/v1/dialplan/extensions '{"name":"bad2","domain":"demo.test","context":"demo","conditions":[]}'
check "no conditions -> 400" 400 "$CODE"
req GET  "/api/v1/dialplan/extensions/$EXTID";     check "get extension -> 200" 200 "$CODE"; contains "ext has action" 'playback'
req GET  "/api/v1/dialplan/extensions?context=demo"; check "list extensions -> 200" 200 "$CODE"
req PUT  "/api/v1/dialplan/extensions/$EXTID" '{"name":"ivr-test","domain":"demo.test","context":"demo","priority":20,"conditions":[{"field":"destination_number","expression":"^(7000)$","actions":[{"application":"answer"},{"application":"echo"}]}]}'
check "update extension -> 200" 200 "$CODE"; contains "ext updated to echo" 'echo'

echo "== dialplan time-based routing =="
req POST /api/v1/dialplan/extensions '{"name":"tr-test","domain":"demo.test","context":"demo","priority":15,"conditions":[{"field":"destination_number","expression":"^(7001)$","time":{"wday":"2-6","hour":"9-17"},"actions":[{"application":"answer"}]}]}'
check "create time-routed extension -> 201" 201 "$CODE"; TRID=$(idof); contains "ext echoes time attrs" '"wday":"2-6"'
req POST /api/v1/dialplan/extensions '{"name":"tr-night","domain":"demo.test","context":"demo","priority":16,"conditions":[{"time":{"time-of-day":"17:00-9:00"},"actions":[{"application":"answer"}]}]}'
check "create pure time-gate (no field) -> 201" 201 "$CODE"; TRNID=$(idof)
req POST /api/v1/dialplan/extensions '{"name":"tr-bad","domain":"demo.test","context":"demo","conditions":[{"actions":[{"application":"answer"}]}]}'
check "condition with neither regex nor time -> 400" 400 "$CODE"

echo "== XML endpoints (mTLS + Basic auth + form-encoded, like mod_xml_curl) =="
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
[ -z "${XML_PASSWORD:-}" ] && [ -f "$ROOT/.env" ] && XML_PASSWORD="$(grep -E '^XML_PASSWORD=' "$ROOT/.env" | cut -d= -f2-)"
XML_USER="${XML_USER:-freeswitch}"; XML_PASSWORD="${XML_PASSWORD:?set XML_PASSWORD or create .env (hack/secrets.sh decrypt)}"
# /xml/* needs: client cert (mTLS) + Basic auth + form body.
xmlpost() { curl -s "${CACERT[@]}" "${CLIENTCERT[@]}" -o "$TMP" -u "$XML_USER:$XML_PASSWORD" -X POST "$API$1" --data "$2"; }
# Without a client cert (mTLS layer) -> rejected.
NOCERT=$(curl -s "${CACERT[@]}" -o /dev/null -w "%{http_code}" -u "$XML_USER:$XML_PASSWORD" -X POST "$API/xml/directory" --data 'user=3001&domain=demo.test')
check "xml without client cert -> 401" 401 "$NOCERT"
# With client cert but without Basic auth -> rejected.
NOBASIC=$(curl -s "${CACERT[@]}" "${CLIENTCERT[@]}" -o /dev/null -w "%{http_code}" -X POST "$API/xml/directory" --data 'user=3001&domain=demo.test')
check "xml without basic auth -> 401" 401 "$NOBASIC"
xmlpost /xml/directory 'user=3001&domain=demo.test'
contains "directory serves 3001" 'user id="3001"'
xmlpost /xml/directory 'user=9999&domain=demo.test'
contains "unknown user -> not found" 'status="not found"'
xmlpost /xml/dialplan 'context=demo'
contains "dialplan serves demo ctx" '<context name="demo">'
xmlpost /xml/dialplan 'context=public'
contains "unmanaged ctx -> not found" 'status="not found"'
xmlpost /xml/configuration 'key_value=callcenter.conf'
contains "configuration renders callcenter" 'odbc-dsn'
contains "configuration renders queue" 'q-test@demo.test'
xmlpost /xml/configuration 'key_value=conference.conf'
contains "configuration renders conference" 'video-mode'
xmlpost /xml/dialplan 'context=demo'
contains "room ext in dialplan" 'conference-r-test'
xmlpost /xml/configuration 'key_value=sofia.conf'
contains "unmanaged config -> not found" 'status="not found"'

echo "== recordings proxy =="
req GET "/api/v1/recordings";                       check "list recordings (today) -> 200" 200 "$CODE"; contains "recordings list shape" '"recordings"'
req GET "/api/v1/recordings?date=bad";              check "bad date -> 400" 400 "$CODE"
req GET "/api/v1/recordings/2026-01-01/nope.wav";   check "missing recording -> 404" 404 "$CODE"

echo "== audit log read API =="
req GET "/api/v1/audit?limit=5";                    check "list audit -> 200" 200 "$CODE"
req GET "/api/v1/audit?resource_type=freeswitch_domain&limit=1"
check "audit filter by resource_type -> 200" 200 "$CODE"; contains "audit entry is for a domain" '"resource_type":"freeswitch_domain"'
req GET "/api/v1/audit?limit=notanint";             check "audit bad limit -> 400" 400 "$CODE"

echo "== CDR (mod_json_cdr ingest + read) =="
# POST /cdr is FreeSWITCH-facing: needs the same mTLS client cert + Basic auth.
CDRCODE=$(curl -s "${CACERT[@]}" "${CLIENTCERT[@]}" -o /dev/null -w "%{http_code}" -u "$XML_USER:$XML_PASSWORD" \
  -X POST "$API/cdr" -H "Content-Type: application/json" \
  --data '{"variables":{"uuid":"apitest-cdr-1","direction":"inbound","hangup_cause":"NORMAL_CLEARING","start_epoch":"1718200000","answer_epoch":"1718200002","end_epoch":"1718200042","duration":"42","billsec":"40"},"callflow":[{"caller_profile":{"caller_id_number":"3001","destination_number":"7000","context":"demo"}}]}')
check "post cdr (mTLS+Basic) -> 200" 200 "$CDRCODE"
CDRNOAUTH=$(curl -s "${CACERT[@]}" -o /dev/null -w "%{http_code}" -X POST "$API/cdr" --data '{}')
check "post cdr without mTLS -> 401" 401 "$CDRNOAUTH"
req GET "/api/v1/cdr?number=3001"; check "list cdr by number -> 200" 200 "$CODE"; contains "cdr has destination" '"destination_number":"7000"'
req GET "/api/v1/cdr/stats";        check "cdr stats -> 200" 200 "$CODE"; contains "cdr stats has talk_time" '"talk_time"'
req GET "/api/v1/cdr?from=notanumber&limit=bad"; check "cdr bad limit -> 400" 400 "$CODE"

echo "== pagination =="
req GET "/api/v1/domains?limit=1";   check "domains limit=1 -> 200" 200 "$CODE"
req GET "/api/v1/domains?limit=abc"; check "domains bad limit -> 400" 400 "$CODE"
req GET "/api/v1/domains?offset=-1"; check "domains bad offset -> 400" 400 "$CODE"

echo "== runtime (ESL) =="
req GET  /api/v1/runtime/health;  check "runtime health -> 200" 200 "$CODE"
req POST /api/v1/runtime/reloadxml; check "reloadxml -> 200" 200 "$CODE"
req GET  /api/v1/runtime/registrations/demo.test/3001; check "registration lookup -> 200" 200 "$CODE"; contains "registration has registered field" '"registered"'
req GET  /api/v1/runtime/gateways/external/test-trunk; check "runtime gateway (not loaded) -> 404" 404 "$CODE"

echo "== delete / cleanup =="
req DELETE "/api/v1/dialplan/extensions/$EXTID";  check "delete extension -> 204" 204 "$CODE"
req GET    "/api/v1/dialplan/extensions/$EXTID";  check "deleted extension -> 404" 404 "$CODE"
req DELETE "/api/v1/dialplan/extensions/$TRID";   check "delete time ext -> 204" 204 "$CODE"
req DELETE "/api/v1/dialplan/extensions/$TRNID";  check "delete time-gate ext -> 204" 204 "$CODE"
req DELETE /api/v1/gateways/external/test-trunk;  check "delete gateway -> 204" 204 "$CODE"
req DELETE /api/v1/conference/rooms/r-test;       check "delete conf room -> 204" 204 "$CODE"
req DELETE /api/v1/conference/profiles/p-test;    check "delete conf profile -> 204" 204 "$CODE"
req GET    /api/v1/conference/rooms/r-test;       check "conf room gone -> 404" 404 "$CODE"
req DELETE /api/v1/callcenter/queues/q-test@demo.test; check "delete queue (cascade tier) -> 204" 204 "$CODE"
req DELETE /api/v1/callcenter/agents/a-test@demo.test; check "delete agent -> 204" 204 "$CODE"
req GET    /api/v1/callcenter/queues/q-test@demo.test; check "queue gone -> 404" 404 "$CODE"
req DELETE /api/v1/users/demo.test/3001;          check "delete user -> 204" 204 "$CODE"
req DELETE /api/v1/domains/demo.test;             check "delete domain (cascade) -> 204" 204 "$CODE"
req GET    /api/v1/domains/demo.test;             check "domain gone -> 404" 404 "$CODE"

rm -f "$TMP"
echo
echo "================  RESULT: $PASS passed, $FAIL failed  ================"
[ "$FAIL" -eq 0 ]
