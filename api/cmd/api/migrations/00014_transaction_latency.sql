-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.transaction_latency_daily (
    day DATE NOT NULL,
    project TEXT NOT NULL,
    transaction_name TEXT NOT NULL,
    p95_duration_ms DOUBLE PRECISION NOT NULL,
    request_count BIGINT NOT NULL,
    PRIMARY KEY (day, project, transaction_name)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.transaction_latency_daily;
-- +goose StatementEnd
