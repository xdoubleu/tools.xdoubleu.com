-- +goose Up
-- +goose StatementBegin
-- The two routines that ran as GitHub Actions workflows. A runner job cannot
-- create a routine itself, so they are seeded here. An existing row stays as
-- it is.
INSERT INTO factory.routines (
    repo_id, name, enabled, interval_minutes, updated_by
)
SELECT
    repos.id,
    seed.name,
    true AS enabled,
    seed.interval_minutes,
    'system:migration' AS updated_by
FROM factory.repos AS repos
CROSS JOIN (
    VALUES
    ('nightly-maintenance-sweep', 1440),
    ('posthog-ux-discovery', 10080)
) AS seed (name, interval_minutes)
WHERE repos.owner = 'xdoubleu' AND repos.name = 'tools.xdoubleu.com'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM factory.routines
WHERE name IN ('nightly-maintenance-sweep', 'posthog-ux-discovery');
-- +goose StatementEnd
