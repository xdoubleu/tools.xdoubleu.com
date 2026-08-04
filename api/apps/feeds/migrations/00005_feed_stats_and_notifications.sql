-- Issues #799 (feed health-alert emails) and #798 (feed statistics):
-- consecutive_failures/notified_at drive the error/quiet-feed email alert
-- and its dedup; read_progress_pct is the scroll-completion signal for
-- read-completion stats.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE feeds.feeds
ADD COLUMN consecutive_failures INT NOT NULL DEFAULT 0,
ADD COLUMN notified_at TIMESTAMPTZ;

ALTER TABLE feeds.items
ADD COLUMN read_progress_pct SMALLINT NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE feeds.items DROP COLUMN read_progress_pct;

ALTER TABLE feeds.feeds
DROP COLUMN consecutive_failures,
DROP COLUMN notified_at;
-- +goose StatementEnd
