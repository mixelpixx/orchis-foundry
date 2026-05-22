#!/bin/sh
# Orchis Foundry SQLite backup — consistent online snapshot via sqlite3 .backup
# (safe while the service is running, thanks to WAL). Keeps the last 14 days.
set -eu

DB="${ORCHIS_DB_PATH:-/var/lib/orchis-foundry/foundry.db}"
DEST="${ORCHIS_BACKUP_DIR:-/var/backups/orchis-foundry}"
KEEP="${ORCHIS_BACKUP_KEEP:-14}"

mkdir -p "$DEST"
TS="$(date +%Y%m%d-%H%M%S)"
OUT="$DEST/foundry-$TS.db"

# .backup takes a live, transactionally-consistent copy.
sqlite3 "$DB" ".backup '$OUT'"
gzip -f "$OUT"

# Prune to the most recent $KEEP snapshots.
ls -1t "$DEST"/foundry-*.db.gz 2>/dev/null | tail -n +"$((KEEP + 1))" | xargs -r rm -f

echo "backup: $OUT.gz ($(ls -1 "$DEST"/foundry-*.db.gz | wc -l) kept)"
