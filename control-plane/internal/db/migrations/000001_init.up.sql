CREATE TABLE IF NOT EXISTS domains (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    variables JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_domains_enabled ON domains(enabled);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    number TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    params JSONB NOT NULL DEFAULT '{}',
    variables JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, number)
);
CREATE INDEX IF NOT EXISTS idx_users_domain_id ON users(domain_id);
CREATE INDEX IF NOT EXISTS idx_users_enabled ON users(enabled);
CREATE INDEX IF NOT EXISTS idx_users_number ON users(number);

CREATE TABLE IF NOT EXISTS gateways (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    profile TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    username TEXT,
    password TEXT,
    realm TEXT,
    proxy TEXT NOT NULL,
    register BOOLEAN NOT NULL DEFAULT TRUE,
    params JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(profile, name)
);
CREATE INDEX IF NOT EXISTS idx_gateways_profile ON gateways(profile);
CREATE INDEX IF NOT EXISTS idx_gateways_enabled ON gateways(enabled);

CREATE TABLE IF NOT EXISTS dialplan_extensions (
    id UUID PRIMARY KEY,
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    context TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, context, name)
);
CREATE INDEX IF NOT EXISTS idx_dialplan_extensions_domain_id ON dialplan_extensions(domain_id);
CREATE INDEX IF NOT EXISTS idx_dialplan_extensions_context ON dialplan_extensions(context);
CREATE INDEX IF NOT EXISTS idx_dialplan_extensions_priority ON dialplan_extensions(priority);
CREATE INDEX IF NOT EXISTS idx_dialplan_extensions_enabled ON dialplan_extensions(enabled);

CREATE TABLE IF NOT EXISTS dialplan_conditions (
    id UUID PRIMARY KEY,
    extension_id UUID NOT NULL REFERENCES dialplan_extensions(id) ON DELETE CASCADE,
    field TEXT NOT NULL,
    expression TEXT NOT NULL,
    order_index INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dialplan_conditions_extension_id ON dialplan_conditions(extension_id);

CREATE TABLE IF NOT EXISTS dialplan_actions (
    id UUID PRIMARY KEY,
    condition_id UUID NOT NULL REFERENCES dialplan_conditions(id) ON DELETE CASCADE,
    application TEXT NOT NULL,
    data TEXT,
    order_index INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dialplan_actions_condition_id ON dialplan_actions(condition_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    before JSONB,
    after JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor);
