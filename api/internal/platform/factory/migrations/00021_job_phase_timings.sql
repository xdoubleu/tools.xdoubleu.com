-- +goose Up
-- +goose StatementBegin
-- Where a job's time went, reported by the runner when the job ends. Null for
-- jobs that ended before this existed; no backfill, nothing recorded them.
ALTER TABLE factory.jobs
    ADD COLUMN agent_seconds double precision,
    ADD COLUMN publish_seconds double precision,
    ADD COLUMN ci_seconds double precision,
    ADD COLUMN merge_seconds double precision,
    ADD COLUMN total_seconds double precision,
    ADD COLUMN gate_retries integer,
    ADD COLUMN push_retries integer,
    ADD COLUMN ci_retries integer;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs
    DROP COLUMN agent_seconds, DROP COLUMN publish_seconds,
    DROP COLUMN ci_seconds, DROP COLUMN merge_seconds, DROP COLUMN total_seconds,
    DROP COLUMN gate_retries, DROP COLUMN push_retries, DROP COLUMN ci_retries;
-- +goose StatementEnd
