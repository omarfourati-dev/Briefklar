-- Only accounts and usage counters. Letters are never stored.
CREATE TABLE app_user (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('user', 'admin', 'demo')),
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    daily_limit   INT NOT NULL DEFAULT 20 CHECK (daily_limit >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE usage_day (
    user_id UUID NOT NULL REFERENCES app_user (id) ON DELETE CASCADE,
    day     DATE NOT NULL,
    count   INT NOT NULL,
    PRIMARY KEY (user_id, day)
);
