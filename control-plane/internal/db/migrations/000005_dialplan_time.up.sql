-- Time-based routing: a dialplan condition may carry FreeSWITCH time
-- attributes (wday, hour, time-of-day, date-time, ...). Stored as JSONB so any
-- supported attribute works without further migrations. A condition with time
-- attrs and no field/expression is a pure time gate.
ALTER TABLE dialplan_conditions
    ADD COLUMN IF NOT EXISTS time_attrs JSONB NOT NULL DEFAULT '{}';

-- field/expression were NOT NULL; a time-only condition leaves them empty,
-- which is already allowed (NOT NULL permits ''), so no change needed there.
