-- Phone provisioning: physical SIP devices that fetch their config from
-- GET /provision/<mac>. Each device maps a normalized MAC to a directory user
-- (number@domain); the SIP password is read from that user at render time, so
-- it is never duplicated here.
CREATE TABLE IF NOT EXISTS provisioned_devices (
    id UUID PRIMARY KEY,
    mac TEXT NOT NULL UNIQUE,          -- normalized: lowercase, no separators
    vendor TEXT NOT NULL DEFAULT 'yealink',  -- yealink | grandstream | generic
    model TEXT NOT NULL DEFAULT '',
    number TEXT NOT NULL,              -- SIP user/extension
    domain TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
