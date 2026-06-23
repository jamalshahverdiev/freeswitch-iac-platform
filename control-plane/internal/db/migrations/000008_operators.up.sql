-- Operators bind a Keycloak identity (subject) to a SIP extension — the bridge
-- between app login and telephony identity for the webphone. RBAC roles live in
-- Keycloak (JWT), NOT here; this table is only the subject -> extension mapping.
CREATE TABLE IF NOT EXISTS operators (
    id UUID PRIMARY KEY,
    subject TEXT NOT NULL UNIQUE,        -- Keycloak `sub` (or username)
    domain TEXT NOT NULL,
    number TEXT NOT NULL,                -- SIP extension
    display_name TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
