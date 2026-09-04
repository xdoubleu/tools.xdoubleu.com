-- +goose Up
-- +goose StatementBegin
-- transfers.txt, where the feed provides it. A router falling back to a
-- default minimum-transfer-time must still treat two stops sharing a
-- parent_station as distinct stops requiring a real transfer, never a free
-- 0-minute change (issue #1391).
CREATE TABLE trains.transfers (
    from_stop_id TEXT NOT NULL,
    to_stop_id TEXT NOT NULL,
    transfer_type INTEGER NOT NULL DEFAULT 0,
    min_transfer_time INTEGER,
    PRIMARY KEY (from_stop_id, to_stop_id)
);
CREATE INDEX ON trains.transfers (to_stop_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trains.transfers;
-- +goose StatementEnd
