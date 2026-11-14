-- +goose Up
-- +goose StatementBegin
-- One pull request can link several tasks, so a row is a (pull request, task)
-- pair. Existing rows keep their pair, so no backfill.
ALTER TABLE factory.linked_pull_requests
DROP CONSTRAINT IF EXISTS linked_pull_requests_pkey;
ALTER TABLE factory.linked_pull_requests
ADD PRIMARY KEY (url, task_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Keeps one task per pull request again, the newest read.
DELETE FROM factory.linked_pull_requests a
USING factory.linked_pull_requests b
WHERE a.url = b.url AND a.checked_at < b.checked_at;
ALTER TABLE factory.linked_pull_requests
DROP CONSTRAINT IF EXISTS linked_pull_requests_pkey;
ALTER TABLE factory.linked_pull_requests
ADD PRIMARY KEY (url);
-- +goose StatementEnd
