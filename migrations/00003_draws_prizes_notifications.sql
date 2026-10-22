-- +goose Up
DROP TABLE active_days;
CREATE VIEW active_days AS
SELECT user_id, partner_day_id FROM raffle_entries
UNION
SELECT assigned_user_id AS user_id, partner_day_id
FROM promo_codes WHERE assigned_user_id IS NOT NULL;
ALTER TABLE raffle_entries DROP COLUMN subscription_checked_at;

CREATE TABLE draws (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('daily', 'final')),
    partner_day_id BIGINT REFERENCES partner_days(id),
    winner_count INTEGER NOT NULL CHECK (winner_count > 0),
    tickets JSONB NOT NULL DEFAULT '[]'::jsonb,
    drawn_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((kind = 'daily') = (partner_day_id IS NOT NULL)),
    UNIQUE (partner_day_id)
);
CREATE UNIQUE INDEX draws_one_final_idx ON draws (kind) WHERE kind = 'final';

-- Immutable snapshot: one ticket per daily entry or active campaign day.
CREATE TABLE draw_results (
    draw_id BIGINT NOT NULL REFERENCES draws(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    position INTEGER NOT NULL CHECK (position > 0),
    state TEXT NOT NULL DEFAULT 'waiting' CHECK (state IN ('waiting', 'active', 'claimed', 'forfeited')),
    prize_slot INTEGER CHECK (prize_slot > 0),
    response_deadline TIMESTAMPTZ,
    claim_step TEXT NOT NULL DEFAULT 'none' CHECK (claim_step IN ('none', 'name', 'consent')),
    full_name TEXT,
    consented_at TIMESTAMPTZ,
    PRIMARY KEY (draw_id, user_id),
    UNIQUE (draw_id, position),
    CHECK ((state = 'waiting') = (prize_slot IS NULL))
);
CREATE UNIQUE INDEX draw_results_active_slot_idx ON draw_results (draw_id, prize_slot)
    WHERE state IN ('active', 'claimed');

CREATE TABLE notification_outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY CHECK (id <= 2147483647),
    user_id BIGINT NOT NULL REFERENCES users(id),
    draw_id BIGINT NOT NULL REFERENCES draws(id),
    message TEXT NOT NULL,
    button_action TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (draw_id, user_id)
);
CREATE INDEX notification_outbox_ready_idx ON notification_outbox (next_attempt_at, id)
    WHERE status = 'pending';

-- +goose Down
DROP TABLE notification_outbox;
DROP TABLE draw_results;
DROP TABLE draws;
DROP VIEW active_days;
ALTER TABLE raffle_entries ADD COLUMN subscription_checked_at TIMESTAMPTZ;
UPDATE raffle_entries SET subscription_checked_at = registered_at;
ALTER TABLE raffle_entries ALTER COLUMN subscription_checked_at SET NOT NULL;
CREATE TABLE active_days (
    user_id BIGINT NOT NULL REFERENCES users(id),
    partner_day_id BIGINT NOT NULL REFERENCES partner_days(id),
    activated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, partner_day_id)
);
CREATE INDEX active_days_partner_idx ON active_days (partner_day_id);
INSERT INTO active_days (user_id, partner_day_id)
SELECT user_id, partner_day_id FROM raffle_entries
UNION
SELECT assigned_user_id, partner_day_id FROM promo_codes
WHERE assigned_user_id IS NOT NULL;
