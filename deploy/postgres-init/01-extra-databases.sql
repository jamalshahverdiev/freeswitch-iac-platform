-- Extra databases created on FIRST postgres container init (mounted via
-- /docker-entrypoint-initdb.d). For an already-initialized volume, create
-- manually: CREATE DATABASE freeswitch_callcenter OWNER freeswitch;

-- mod_callcenter runtime state (agents/tiers/members) over ODBC — kept in a
-- dedicated DB so FreeSWITCH runtime tables never mix with the control-plane
-- desired-state schema in freeswitch_control.
CREATE DATABASE freeswitch_callcenter OWNER freeswitch;

-- (reserved for the follow-up core-db migration)
CREATE DATABASE freeswitch_core OWNER freeswitch;
