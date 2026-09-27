-- +goose Up
-- +goose StatementBegin
-- Request logs stored the Kobo device token (a bearer credential in the sync
-- path) in attrs.endpoint; mask it as the logger now does.
UPDATE global.log_entries
SET
    attrs = jsonb_set(
        attrs, '{endpoint}',
        to_jsonb(
            regexp_replace(
                attrs ->> 'endpoint', '(/kobo/)[^/?#]+', '\1redacted', 'g'
            )
        )
    )
WHERE attrs ->> 'endpoint' LIKE '%/kobo/%';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Redaction is irreversible by design.
SELECT 1;
-- +goose StatementEnd
