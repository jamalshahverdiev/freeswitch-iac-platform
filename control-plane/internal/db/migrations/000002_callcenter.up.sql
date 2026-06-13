-- mod_callcenter desired state: queues, agents, tiers.
-- (mod_callcenter's own RUNTIME tables live in the separate
--  freeswitch_callcenter database via ODBC, not here.)

CREATE TABLE IF NOT EXISTS cc_queues (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,                 -- e.g. support@192.168.48.143
    strategy TEXT NOT NULL DEFAULT 'longest-idle-agent',
    moh_sound TEXT NOT NULL DEFAULT 'local_stream://moh',
    time_base_score TEXT NOT NULL DEFAULT 'system',
    max_wait_time INTEGER NOT NULL DEFAULT 0,
    max_wait_time_with_no_agent INTEGER NOT NULL DEFAULT 0,
    max_wait_time_with_no_agent_time_reached INTEGER NOT NULL DEFAULT 5,
    tier_rules_apply BOOLEAN NOT NULL DEFAULT FALSE,
    tier_rule_wait_second INTEGER NOT NULL DEFAULT 300,
    tier_rule_wait_multiply_level BOOLEAN NOT NULL DEFAULT TRUE,
    tier_rule_no_agent_no_wait BOOLEAN NOT NULL DEFAULT FALSE,
    discard_abandoned_after INTEGER NOT NULL DEFAULT 60,
    abandoned_resume_allowed BOOLEAN NOT NULL DEFAULT FALSE,
    params JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cc_agents (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,                 -- e.g. 2001@192.168.48.143
    type TEXT NOT NULL DEFAULT 'callback',
    contact TEXT NOT NULL,                     -- e.g. user/2001@192.168.48.143
    status TEXT NOT NULL DEFAULT 'Available',  -- initial status on load
    max_no_answer INTEGER NOT NULL DEFAULT 3,
    wrap_up_time INTEGER NOT NULL DEFAULT 10,
    reject_delay_time INTEGER NOT NULL DEFAULT 3,
    busy_delay_time INTEGER NOT NULL DEFAULT 60,
    no_answer_delay_time INTEGER NOT NULL DEFAULT 60,
    params JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cc_tiers (
    id UUID PRIMARY KEY,
    queue TEXT NOT NULL REFERENCES cc_queues(name) ON DELETE CASCADE ON UPDATE CASCADE,
    agent TEXT NOT NULL REFERENCES cc_agents(name) ON DELETE CASCADE ON UPDATE CASCADE,
    level INTEGER NOT NULL DEFAULT 1,
    position INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(queue, agent)
);

CREATE INDEX IF NOT EXISTS idx_cc_tiers_queue ON cc_tiers(queue);
CREATE INDEX IF NOT EXISTS idx_cc_tiers_agent ON cc_tiers(agent);
