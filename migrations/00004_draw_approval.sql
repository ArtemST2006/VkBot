-- +goose Up
ALTER TABLE draws
    ADD COLUMN approved_at TIMESTAMPTZ,
    ADD COLUMN approved_by_vk_id BIGINT,
    ADD CONSTRAINT draws_approval_together CHECK ((approved_at IS NULL) = (approved_by_vk_id IS NULL));

-- Draws created by older versions have already queued winner notifications.
UPDATE draws SET approved_at = drawn_at, approved_by_vk_id = 0;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM draws WHERE approved_at IS NULL) THEN
        RAISE EXCEPTION 'approve pending draws before rolling back draw approval';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE draws DROP CONSTRAINT draws_approval_together;
ALTER TABLE draws DROP COLUMN approved_by_vk_id;
ALTER TABLE draws DROP COLUMN approved_at;
