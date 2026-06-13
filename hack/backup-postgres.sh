#!/usr/bin/env bash
# Daily backup of the platform state. Two parts:
#   1. pg_dump -Fc of the three Postgres DBs (control / callcenter / core)
#      from the compose container.
#   2. Pull of new call recordings from the FreeSWITCH host (ssh key auth).
# Layout: ~/backups/freeswitch/<YYYY-MM-DD>/{*.dump,recordings/...}
# Retention: KEEP_DAYS (default 14). Designed for cron:
#   30 3 * * * /home/jamal/github/freeswitch-iac-platform/hack/backup-postgres.sh >> ~/backups/freeswitch/backup.log 2>&1
set -euo pipefail

CONTAINER="${PG_CONTAINER:-freeswitch-iac-platform-postgres-1}"
PG_USER="${PG_USER:-freeswitch}"
DBS=(freeswitch_control freeswitch_callcenter freeswitch_core)
FS_HOST="${FS_HOST:-root@192.168.48.143}"
REC_DIR="/var/lib/freeswitch/recordings"
BASE="${BACKUP_BASE:-$HOME/backups/freeswitch}"
KEEP_DAYS="${KEEP_DAYS:-14}"

DAY=$(date +%F)
DEST="$BASE/$DAY"
mkdir -p "$DEST"

echo "[$(date '+%F %T')] backup -> $DEST"

for db in "${DBS[@]}"; do
  docker exec "$CONTAINER" pg_dump -U "$PG_USER" -Fc "$db" > "$DEST/$db.dump"
  echo "  $db.dump ($(du -h "$DEST/$db.dump" | cut -f1))"
done

# Recordings: copy the whole dated tree; tar over ssh (no rsync on this box).
# Cheap because each day only today's directory grows.
if ssh -o BatchMode=yes -o ConnectTimeout=10 "$FS_HOST" "test -d $REC_DIR"; then
  mkdir -p "$DEST/recordings"
  ssh "$FS_HOST" "tar -C $REC_DIR -cf - ." | tar -C "$DEST/recordings" -xf -
  echo "  recordings/ ($(du -sh "$DEST/recordings" | cut -f1))"
else
  echo "  WARN: FS host unreachable — recordings skipped"
fi

# Prune old backups.
find "$BASE" -maxdepth 1 -type d -name '20*' -mtime +"$KEEP_DAYS" -print -exec rm -rf {} \; | sed 's/^/  pruned: /'

echo "[$(date '+%F %T')] done"
