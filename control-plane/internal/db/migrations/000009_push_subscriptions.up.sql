CREATE TABLE push_subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    subject    TEXT NOT NULL,
    domain     TEXT NOT NULL,
    number     TEXT NOT NULL,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX push_subscriptions_dn ON push_subscriptions (domain, number);
