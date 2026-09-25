-- +goose Up
-- +goose StatementBegin
-- Headless identity for the scheduled agent routines (ADR-0025). The user has
-- no usable password hash, so it can't sign in interactively; the client's
-- secret_hash is written at boot from OAUTH_ROUTINES_CLIENT_SECRET. No
-- existing role changes meaning, so no backfill.
ALTER TABLE global.app_users DROP CONSTRAINT IF EXISTS app_users_role_check;
ALTER TABLE global.app_users ADD CONSTRAINT app_users_role_check
CHECK (role IN ('admin', 'user', 'service'));

INSERT INTO auth.users (id, email, password_hash) VALUES (
    '131220c3-7027-4f72-b714-b6ecb114d19d',
    'routines@service.invalid',
    '!'
) ON CONFLICT (id) DO NOTHING;

INSERT INTO global.app_users (id, email, role) VALUES (
    '131220c3-7027-4f72-b714-b6ecb114d19d',
    'routines@service.invalid',
    'service'
) ON CONFLICT (id) DO UPDATE SET role = 'service';

INSERT INTO auth.oauth2_clients (id, grant_types, public, client_name)
VALUES (
    'routines',
    ARRAY['client_credentials'],
    FALSE,
    'Scheduled agent routines'
) ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM auth.oauth2_clients
WHERE id = 'routines';
DELETE FROM global.app_users
WHERE id = '131220c3-7027-4f72-b714-b6ecb114d19d';
DELETE FROM auth.users
WHERE id = '131220c3-7027-4f72-b714-b6ecb114d19d';
ALTER TABLE global.app_users DROP CONSTRAINT IF EXISTS app_users_role_check;
ALTER TABLE global.app_users ADD CONSTRAINT app_users_role_check
CHECK (role IN ('admin', 'user'));
-- +goose StatementEnd
