-- +goose Up
-- VK and future MAX users share one internal ID; external IDs stay platform scoped.
CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    platform TEXT NOT NULL CHECK (platform IN ('vk', 'max')),
    external_id TEXT NOT NULL CHECK (length(btrim(external_id)) > 0),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (platform, external_id)
);

CREATE TABLE partners (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9_-]*$'),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    account_url TEXT NOT NULL CHECK (account_url ~ '^https://'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The calendar date is the campaign day in Europe/Moscow, not the draw time.
CREATE TABLE partner_days (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    partner_id BIGINT NOT NULL REFERENCES partners(id),
    event_date DATE NOT NULL UNIQUE,
    promo_valid_until TIMESTAMPTZ,
    promo_instructions TEXT,
    prize_name TEXT,
    winner_count INTEGER NOT NULL DEFAULT 1 CHECK (winner_count > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX partner_days_partner_date_idx ON partner_days (partner_id, event_date);

-- Assignment fields are both NULL while a code is available.
CREATE TABLE promo_codes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    partner_day_id BIGINT NOT NULL REFERENCES partner_days(id),
    code TEXT NOT NULL CHECK (length(btrim(code)) > 0),
    assigned_user_id BIGINT REFERENCES users(id),
    assigned_at TIMESTAMPTZ,
    UNIQUE (partner_day_id, code),
    UNIQUE (partner_day_id, assigned_user_id),
    CHECK ((assigned_user_id IS NULL) = (assigned_at IS NULL))
);

CREATE INDEX promo_codes_available_idx ON promo_codes (partner_day_id, id)
    WHERE assigned_user_id IS NULL;

-- Every button press can be counted, including repeat and exhausted requests.
CREATE TABLE promo_requests (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    partner_day_id BIGINT NOT NULL REFERENCES partner_days(id),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    outcome TEXT NOT NULL CHECK (outcome IN ('issued', 'already_issued', 'exhausted', 'unavailable'))
);

CREATE INDEX promo_requests_day_time_idx ON promo_requests (partner_day_id, requested_at);

-- A row exists only after successful subscription verification.
CREATE TABLE raffle_entries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    partner_day_id BIGINT NOT NULL REFERENCES partner_days(id),
    subscription_checked_at TIMESTAMPTZ NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, partner_day_id)
);

CREATE INDEX raffle_entries_day_time_idx ON raffle_entries (partner_day_id, registered_at);

-- Promo and raffle flows insert the same key in one transaction with their action.
-- The primary key makes an active campaign day worth at most one chance.
CREATE TABLE active_days (
    user_id BIGINT NOT NULL REFERENCES users(id),
    partner_day_id BIGINT NOT NULL REFERENCES partner_days(id),
    activated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, partner_day_id)
);

CREATE INDEX active_days_partner_idx ON active_days (partner_day_id);

-- +goose Down
DROP TABLE active_days;
DROP TABLE raffle_entries;
DROP TABLE promo_requests;
DROP TABLE promo_codes;
DROP TABLE partner_days;
DROP TABLE partners;
DROP TABLE users;
