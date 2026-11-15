-- +goose Up
-- +goose StatementBegin
-- #3181 only mentioned T-315 and T-316 in its body, which linked it to both.
-- Mentions no longer link, so the two stray rows go.
DELETE FROM factory.linked_pull_requests l
USING factory.tasks t
WHERE l.task_id = t.id
AND t.number IN (315, 316)
AND l.url LIKE '%/pull/3181';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
