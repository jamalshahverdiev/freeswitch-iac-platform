#!/usr/bin/env bash
# Prove a backup restores: load the dumps into a scratch Postgres container
# and compare row counts with the live database.
#   hack/restore-test.sh [YYYY-MM-DD]   (default: today)
set -euo pipefail

DAY="${1:-$(date +%F)}"
B="${BACKUP_BASE:-$HOME/backups/freeswitch}/$DAY"
[ -d "$B" ] || { echo "no backup dir: $B"; exit 1; }
LIVE="${PG_CONTAINER:-freeswitch-iac-platform-postgres-1}"
SCRATCH=fs-restore-test

docker rm -f $SCRATCH >/dev/null 2>&1 || true
docker run -d --name $SCRATCH -e POSTGRES_USER=freeswitch -e POSTGRES_PASSWORD=freeswitch \
  -e POSTGRES_DB=freeswitch_control postgres:16 >/dev/null
until docker exec $SCRATCH pg_isready -U freeswitch -d freeswitch_control >/dev/null 2>&1; do sleep 1; done
sleep 2
docker exec $SCRATCH psql -U freeswitch -d freeswitch_control -c "CREATE DATABASE freeswitch_callcenter;" >/dev/null
docker exec $SCRATCH psql -U freeswitch -d freeswitch_control -c "CREATE DATABASE freeswitch_core;" >/dev/null
for db in freeswitch_control freeswitch_callcenter freeswitch_core; do
  docker exec -i $SCRATCH pg_restore -U freeswitch -d $db --no-owner < "$B/$db.dump" 2>/dev/null || true
done

ok=0; fail=0
check() { # db query
  SRC=$(docker exec "$LIVE" psql -U freeswitch -d "$1" -t -c "$2" | tr -d ' ')
  DST=$(docker exec $SCRATCH psql -U freeswitch -d "$1" -t -c "$2" | tr -d ' ')
  if [ "$SRC" = "$DST" ]; then ok=$((ok+1)); st=OK; else fail=$((fail+1)); st=MISMATCH; fi
  printf "%-22s %-42s src=%-5s restored=%-5s %s\n" "$1" "$2" "$SRC" "$DST" "$st"
}
check freeswitch_control "SELECT count(*) FROM users"
check freeswitch_control "SELECT count(*) FROM dialplan_extensions"
check freeswitch_control "SELECT count(*) FROM cc_queues"
check freeswitch_control "SELECT count(*) FROM cc_agents"
check freeswitch_control "SELECT count(*) FROM cc_tiers"
check freeswitch_control "SELECT count(*) FROM conference_profiles"
check freeswitch_control "SELECT count(*) FROM conference_rooms"
check freeswitch_control "SELECT count(*) FROM gateways"
check freeswitch_control "SELECT count(*) FROM domains"
check freeswitch_callcenter "SELECT count(*) FROM agents"
check freeswitch_callcenter "SELECT count(*) FROM tiers"

docker rm -f $SCRATCH >/dev/null
echo "RESULT: $ok ok, $fail mismatch"
# NOTE: counts are compared against LIVE — if writes happened between the
# backup and this test, small diffs (e.g. audit_logs) are expected; that is
# why audit_logs is not in the list.
[ $fail -eq 0 ]
