-- Call Detail Records, posted by FreeSWITCH mod_json_cdr to POST /cdr and
-- parsed by the control-plane. Lives in the control-plane's own DB
-- (freeswitch_control) — it is operator-facing data we own, not FreeSWITCH
-- runtime state.
CREATE TABLE IF NOT EXISTS cdr (
    -- channel uuid from the CDR; TEXT (not UUID) so a malformed CDR can never
    -- become a poison message that mod_json_cdr retries forever on a 500.
    id TEXT PRIMARY KEY,
    direction TEXT NOT NULL DEFAULT '',
    caller_id_number TEXT NOT NULL DEFAULT '',
    caller_id_name TEXT NOT NULL DEFAULT '',
    destination_number TEXT NOT NULL DEFAULT '',
    context TEXT NOT NULL DEFAULT '',
    hangup_cause TEXT NOT NULL DEFAULT '',
    start_epoch BIGINT NOT NULL DEFAULT 0,
    answer_epoch BIGINT NOT NULL DEFAULT 0,
    end_epoch BIGINT NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL DEFAULT 0,       -- total seconds
    billsec INTEGER NOT NULL DEFAULT 0,        -- answered seconds
    recording_path TEXT NOT NULL DEFAULT '',
    raw JSONB NOT NULL DEFAULT '{}',           -- full mod_json_cdr payload
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cdr_start_epoch ON cdr(start_epoch);
CREATE INDEX IF NOT EXISTS idx_cdr_caller ON cdr(caller_id_number);
CREATE INDEX IF NOT EXISTS idx_cdr_destination ON cdr(destination_number);
CREATE INDEX IF NOT EXISTS idx_cdr_hangup_cause ON cdr(hangup_cause);
