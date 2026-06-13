#!/usr/bin/env bash
# E2E test of mod_callcenter record-template WITHOUT human agents.
#
# A real agent must actually ANSWER for the queue to bridge and start the
# record-template recording. mod_callcenter cannot use a loopback/... contact
# (the origination is cancelled within seconds), so we stand up a tiny SIP UAS
# with sipp on the FS host and register the agent against it:
#   sipp -sn uas -p 5070 -rtp_echo  ->  agent contact sofia/internal/bot@127.0.0.1:5070
#
# Flow: real agents -> On Break; temp "testbot" agent + tier; originate a member
# call into 4444 (MOH as the caller's audio); assert a new recording appears.
# Everything is cleaned up afterwards.
#
# Requires: FS_SSH_PASS in env (see deploy/SECRETS.md); sipp on the FS host
# (apt-get install -y sip-tester).
set -uo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
API="${API:-https://localhost:8080}"
TOKEN="${TOKEN:-dev-token}"
CA=(--cacert "$DIR/deploy/tls/ca.crt")
H=(-H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json")
# timeout guards against ssh hanging on a daemon's inherited stdout pipe.
FS() { timeout 30 sshpass -p "${FS_SSH_PASS:?export FS_SSH_PASS (see deploy/SECRETS.md)}" ssh -o StrictHostKeyChecking=no root@192.168.48.143 "$@"; }
Q="support@192.168.48.143"
BOT="testbot@192.168.48.143"

cleanup() {
  echo "== cleanup =="
  curl -s "${CA[@]}" "${H[@]}" -X DELETE "$API/api/v1/callcenter/tiers/$Q/$BOT"  -o /dev/null -w "del tier: %{http_code}\n"
  curl -s "${CA[@]}" "${H[@]}" -X DELETE "$API/api/v1/callcenter/agents/$BOT"     -o /dev/null -w "del agent: %{http_code}\n"
  curl -s "${CA[@]}" "${H[@]}" -X POST   "$API/api/v1/runtime/callcenter/reload"  -o /dev/null -w "cc reload: %{http_code}\n"
  FS 'systemctl stop sipp-bot 2>/dev/null; systemctl reset-failed sipp-bot 2>/dev/null;
      fs_cli -x "callcenter_config tier del support@192.168.48.143 testbot@192.168.48.143" >/dev/null;
      fs_cli -x "callcenter_config agent del testbot@192.168.48.143" >/dev/null' 2>/dev/null
  for a in 4201 4202; do
    curl -s "${CA[@]}" "${H[@]}" -X PUT "$API/api/v1/runtime/callcenter/agents/$a@192.168.48.143/status" \
      -d '{"status":"Available"}' -o /dev/null -w "$a Available: %{http_code}\n"
  done
}
trap cleanup EXIT

echo "== setup: sipp UAS bot on the FS host (transient systemd unit) =="
# A plain backgrounded sipp dies when the ssh session closes; run it as a
# transient systemd service so it survives and is trivially stopped.
FS 'command -v sipp >/dev/null || apt-get install -y -qq sip-tester >/dev/null 2>&1;
    systemctl reset-failed sipp-bot 2>/dev/null; systemctl stop sipp-bot 2>/dev/null; sleep 1;
    systemd-run --unit=sipp-bot --collect sipp -sn uas -p 5070 -rtp_echo;
    sleep 2;
    [ "$(systemctl is-active sipp-bot)" = active ] && echo "sipp listening :5070" || { echo "SIPP FAILED"; exit 1; }'

curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/callcenter/agents" \
  -d "{\"name\":\"$BOT\",\"contact\":\"sofia/internal/bot@127.0.0.1:5070\"}" -o /dev/null -w "agent: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/callcenter/tiers" \
  -d "{\"queue\":\"$Q\",\"agent\":\"$BOT\"}" -o /dev/null -w "tier: %{http_code}\n"
curl -s "${CA[@]}" "${H[@]}" -X POST "$API/api/v1/runtime/callcenter/reload" -o /dev/null -w "cc reload: %{http_code}\n"
for a in 4201 4202; do
  curl -s "${CA[@]}" "${H[@]}" -X PUT "$API/api/v1/runtime/callcenter/agents/$a@192.168.48.143/status" \
    -d '{"status":"On Break"}' -o /dev/null -w "$a On Break: %{http_code}\n"
done

echo "== member call into 4444 (MOH as caller audio) =="
BEFORE=$(FS 'find /var/lib/freeswitch/recordings -type f | wc -l')
FS 'fs_cli -x "originate {origination_caller_id_number=8888}loopback/4444/company &playback(local_stream://moh)"' >/dev/null 2>&1 &
sleep 12
echo "--- queue members mid-call (cid | serving_agent | state) ---"
FS 'fs_cli -x "callcenter_config queue list members support@192.168.48.143"' | awk -F'|' 'NR<=2{print $5, $14, $16}'
FS 'fs_cli -x "hupall NORMAL_CLEARING"' >/dev/null
sleep 2
AFTER=$(FS 'find /var/lib/freeswitch/recordings -type f | wc -l')

echo "== result =="
echo "recordings before=$BEFORE after=$AFTER"
NEW=$(FS 'find /var/lib/freeswitch/recordings -type f -newermt "1 minute ago"')
echo "new file(s):"; echo "$NEW"
[ "$AFTER" -gt "$BEFORE" ] && echo "PASS: queue call recorded via record-template" || { echo "FAIL: no new recording"; exit 1; }
