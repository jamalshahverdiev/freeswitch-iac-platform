#!/usr/bin/env bash
# E2E test of mod_callcenter record-template WITHOUT human agents:
#  - temp dialplan ext 9198 answers and plays a 440 Hz tone (the "agent voice")
#  - temp agent "testbot" (contact loopback/9198/company) joins the queue
#  - real agents 4201/4202 are put On Break for the duration
#  - a member call is originated with a 350 Hz tone (the "caller voice")
#  - the resulting recording must contain BOTH tones (each direction)
# Cleans everything up afterwards.
set -uo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
API="${API:-https://localhost:8080}"
TOKEN="${TOKEN:-dev-token}"
CA=(--cacert "$DIR/deploy/tls/ca.crt")
H=(-H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json")
FS() { sshpass -p "${FS_SSH_PASS:?export FS_SSH_PASS (see deploy/SECRETS.md)}" ssh -o StrictHostKeyChecking=no root@192.168.48.143 "$@"; }
Q="support@192.168.48.143"
BOT="testbot@192.168.48.143"

echo "== setup =="
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/dialplan/extensions" -d '{
  "name":"rec-testbot","domain":"192.168.48.143","context":"company","priority":6,
  "conditions":[{"field":"destination_number","expression":"^(9198)$","actions":[
    {"application":"answer"},
    {"application":"playback","data":"tone_stream://L=40;%(1000,250,440)"}]}]}' -o /dev/null -w "ext 9198: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/runtime/reloadxml" -o /dev/null -w "reloadxml: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/callcenter/agents" -d "{\"name\":\"$BOT\",\"contact\":\"{loopback_bowout=false,ignore_early_media=true}loopback/9198/company\"}" -o /dev/null -w "agent: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/callcenter/tiers" -d "{\"queue\":\"$Q\",\"agent\":\"$BOT\"}" -o /dev/null -w "tier: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/runtime/callcenter/reload" -o /dev/null -w "cc reload: %{http_code}\n"
for a in 4201 4202; do
  curl -s "${CA[@]}" "${H[@]}" -X PUT "$API/api/v1/runtime/callcenter/agents/$a@192.168.48.143/status" -d '{"status":"On Break"}' -o /dev/null -w "$a On Break: %{http_code}\n"
done

echo "== member call (350 Hz) =="
BEFORE=$(FS 'find /var/lib/freeswitch/recordings -type f | wc -l')
FS 'fs_cli -x "originate {origination_caller_id_number=8888,ignore_early_media=true}loopback/4444/company \&playback(tone_stream://L=40;%(1000,250,350))" ' >/dev/null 2>&1 &
sleep 18
echo "--- queue members mid-call ---"
FS 'fs_cli -x "callcenter_config queue list members support@192.168.48.143"' | cut -d'|' -f5,14,16 | head -3
FS 'fs_cli -x "hupall NORMAL_CLEARING"' >/dev/null
sleep 2
AFTER=$(FS 'find /var/lib/freeswitch/recordings -type f | wc -l')
echo "files before=$BEFORE after=$AFTER"
FS 'find /var/lib/freeswitch/recordings -type f -newermt "1 minute ago"'

echo "== cleanup =="
curl -s "${CA[@]}" "${H[@]}" -X DELETE "$API/api/v1/callcenter/tiers/$Q/$BOT" -o /dev/null -w "del tier: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X DELETE "$API/api/v1/callcenter/agents/$BOT" -o /dev/null -w "del agent: %{http_code}\n"
EXTID=$(curl -s "${CA[@]}" "${H[@]}" "$API/api/v1/dialplan/extensions?context=company" | python3 -c "
import json,sys
for e in json.load(sys.stdin):
    if e['name']=='rec-testbot': print(e['id'])")
[ -n "$EXTID" ] && curl -s "${CA[@]}" "${H[@]}" -X DELETE "$API/api/v1/dialplan/extensions/$EXTID" -o /dev/null -w "del ext: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/runtime/reloadxml" -o /dev/null -w "reloadxml: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/runtime/callcenter/reload" -o /dev/null -w "cc reload: %{http_code}\n"
FS 'fs_cli -x "callcenter_config tier del support@192.168.48.143 testbot@192.168.48.143"; fs_cli -x "callcenter_config agent del testbot@192.168.48.143"' >/dev/null
for a in 4201 4202; do
  curl -s "${CA[@]}" "${H[@]}" -X PUT "$API/api/v1/runtime/callcenter/agents/$a@192.168.48.143/status" -d '{"status":"Available"}' -o /dev/null -w "$a Available: %{http_code}\n"
done
echo "done"
