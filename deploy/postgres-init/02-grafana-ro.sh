#!/bin/bash
# Read-only DB role for Grafana (NOC dashboard). Runs on FIRST postgres init
# (mounted in /docker-entrypoint-initdb.d). For an already-initialized volume,
# apply the same grants manually — see docs/observability.md.
#
# The password comes from $GRAFANA_DB_PASSWORD (passed to the postgres
# container), so it is never committed to the repo.
set -euo pipefail
: "${GRAFANA_DB_PASSWORD:?GRAFANA_DB_PASSWORD must be set for the grafana_ro role}"

# Role is cluster-global: create once, guarded.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
	DO \$\$ BEGIN
	  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'grafana_ro') THEN
	    CREATE ROLE grafana_ro LOGIN PASSWORD '${GRAFANA_DB_PASSWORD}';
	  END IF;
	END \$\$;
SQL

# Per-database read-only grants (+ default privileges so tables created later
# by the $POSTGRES_USER owner — control-plane migrations, FreeSWITCH ODBC — are
# readable too).
for db in "$POSTGRES_DB" freeswitch_callcenter freeswitch_core; do
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$db" <<-SQL
		GRANT CONNECT ON DATABASE "$db" TO grafana_ro;
		GRANT USAGE ON SCHEMA public TO grafana_ro;
		GRANT SELECT ON ALL TABLES IN SCHEMA public TO grafana_ro;
		ALTER DEFAULT PRIVILEGES FOR ROLE "$POSTGRES_USER" IN SCHEMA public
		  GRANT SELECT ON TABLES TO grafana_ro;
	SQL
done

echo "grafana_ro role + read-only grants applied to $POSTGRES_DB, freeswitch_callcenter, freeswitch_core"
